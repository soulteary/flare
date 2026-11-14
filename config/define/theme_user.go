package define

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/flare/config/model"
)

// An instance reads its own themes from the themes directory of its working
// directory, next to config.yml. A theme is a file there, named after the
// theme, or a directory named after the theme that holds a theme file named
// theme.yml. The directory is the form for a theme of more than one file.
const (
	UserThemeDirectory = "themes"
	userThemeExtension = ".yml"
	// userThemeManifest is the theme file of a theme that is a directory.
	userThemeManifest = "theme.yml"
	// MaximumUserThemeFiles is the number of entries of the themes directory
	// looked at, in the order of their names: theme files, directories and
	// links. Entries after them are neither read nor reported.
	MaximumUserThemeFiles = 64
	// MaximumThemeFileBytes is the largest theme file read.
	MaximumThemeFileBytes = 64 << 10
)

// Reasons a file in the themes directory is not in use.
const (
	ThemeProblemExtension = "extension"
	ThemeProblemName      = "name"
	ThemeProblemReserved  = "reserved"
	ThemeProblemType      = "type"
	ThemeProblemLink      = "link"
	ThemeProblemDuplicate = "duplicate"
	ThemeProblemSize      = "size"
	ThemeProblemLimit     = "limit"
	ThemeProblemRead      = "read"
	ThemeProblemSyntax    = "syntax"
	ThemeProblemContract  = "contract"
	ThemeProblemValue     = "value"
	ThemeProblemColor     = "color"
	ThemeProblemContrast  = "contrast"
	ThemeProblemText      = "text"
)

// ThemeProblem is a file in the themes directory that is not in use. File is
// its name in that directory, and for the theme file of a directory the path
// with a slash, such as forest/theme.yml. Subject is the part of the file the
// reason is about, such as tokens.radius or modes.light, or the message of the
// YAML parser; it can be empty.
type ThemeProblem struct {
	File    string
	Reason  string
	Subject string
}

// ThemeNotice is a theme file with parts this release does not use.
type ThemeNotice struct {
	File   string
	Unused model.ThemeFileUnused
}

// themeFileStamp is the state of a theme file when it was last read.
type themeFileStamp struct {
	modTime time.Time
	size    int64
	exists  bool
}

// Some filesystems record modification times in steps of up to two seconds,
// so a second edit within the same step can keep both the time and the size.
// A file modified this recently is read again instead of trusted.
const themeStampSettle = 2 * time.Second

// statThemeFile returns the state of a file. A file that cannot be examined
// has the state of a missing file.
func statThemeFile(path string) themeFileStamp {
	info, err := os.Lstat(path)
	if err != nil {
		return themeFileStamp{}
	}
	return themeFileStamp{modTime: info.ModTime(), size: info.Size(), exists: true}
}

// themeStamp is the state of the two files a theme of one name can be read
// from: the file of that name, and theme.yml in the directory of that name.
// Both are watched, because each can take the place of the other.
type themeStamp struct {
	file, manifest themeFileStamp
}

// statTheme returns the state of the files a theme name is read from. A name
// that cannot be a theme of the directory has the state of missing files.
func statTheme(name string) themeStamp {
	if !isUserThemeName(name) {
		return themeStamp{}
	}
	return themeStamp{
		file:     statThemeFile(filepath.Join(UserThemeDirectory, name+userThemeExtension)),
		manifest: statThemeFile(filepath.Join(UserThemeDirectory, name, userThemeManifest)),
	}
}

// current reports whether both files are still in the state they were read at.
func (stamp themeStamp) current(now themeStamp, clock time.Time) bool {
	return stamp.file.current(now.file, clock) && stamp.manifest.current(now.manifest, clock)
}

// current reports whether a file is still in the state it was read at.
func (stamp themeFileStamp) current(now themeFileStamp, clock time.Time) bool {
	if !stamp.exists || !now.exists {
		return stamp.exists == now.exists
	}
	if stamp.size != now.size || !stamp.modTime.Equal(now.modTime) {
		return false
	}
	age := clock.Sub(now.modTime)
	return age >= themeStampSettle || age <= -themeStampSettle
}

// userThemeScan is one reading of the themes directory.
type userThemeScan struct {
	themes   map[string]model.Theme
	order    []string
	problems []ThemeProblem
	notices  []ThemeNotice
}

var (
	// userThemeScanMu orders readings of the themes directory, so that an
	// earlier reading never replaces a later one.
	userThemeScanMu sync.Mutex
	// userThemeState holds the theme files in use. Guarded by themeMu.
	userThemeState userThemeScan
	// selectedThemeStamp is the state of the selected theme's files at the
	// reading userThemeState comes from. Guarded by themeMu.
	selectedThemeStamp themeStamp
)

// isUserThemeName reports whether a theme name can belong to a theme of the
// themes directory. The bundled themes do not change after startup.
func isUserThemeName(name string) bool {
	return model.IsThemeName(name) && name != model.ThemeCustomName && !isBundledTheme(name)
}

func isBundledTheme(name string) bool {
	for _, theme := range ThemePalettes {
		if theme.Name == name {
			return true
		}
	}
	return false
}

// What a reading of the themes directory does with an entry.
const (
	// themeEntryIgnored is left alone without a remark.
	themeEntryIgnored = iota
	// themeEntryFile is named <name>.yml and read as the file of a theme.
	themeEntryFile
	// themeEntryMisnamed is named <name>.yaml and reported.
	themeEntryMisnamed
	// themeEntryDirectory is a theme if it holds a theme.yml.
	themeEntryDirectory
	// themeEntryLink is reported: a link is not followed.
	themeEntryLink
)

// themeEntryKind says what a reading of the themes directory does with an
// entry, going by its name and its type alone. Hidden entries are left alone,
// because editors and file managers create them, and so is a file with an
// extension other than the two of YAML. The other usual extension of YAML is
// reported: such a file is most likely meant as a theme file.
func themeEntryKind(entry fs.DirEntry) int {
	name := entry.Name()
	switch {
	case strings.HasPrefix(name, "."):
		return themeEntryIgnored
	case strings.HasSuffix(name, userThemeExtension):
		return themeEntryFile
	case strings.HasSuffix(name, ".yaml"):
		return themeEntryMisnamed
	case entry.IsDir():
		return themeEntryDirectory
	case entry.Type()&fs.ModeSymlink != 0:
		return themeEntryLink
	}
	return themeEntryIgnored
}

// isThemeDirectory reports whether a directory of the themes directory holds
// a theme.yml. That makes it the theme of its name, whether or not the file
// is usable.
func isThemeDirectory(root *os.Root, directory string) bool {
	_, err := root.Lstat(filepath.Join(directory, userThemeManifest))
	return err == nil
}

// scanUserThemes reads the themes directory. An instance without the
// directory has no themes of its own; that is not an error.
func scanUserThemes() userThemeScan {
	scan := userThemeScan{themes: map[string]model.Theme{}}
	root, err := os.OpenRoot(UserThemeDirectory)
	if err != nil {
		return scan
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return scan
	}
	// A name has one theme. Where a directory is the theme of a name, the
	// file of that name is reported instead of read. Which entries are
	// directories is taken from the listing, so the result does not depend on
	// the order the two are looked at in.
	directories := map[string]bool{}
	for _, entry := range entries {
		if themeEntryKind(entry) == themeEntryDirectory {
			directories[entry.Name()] = true
		}
	}
	examined := 0
	for _, entry := range entries {
		kind := themeEntryKind(entry)
		if kind == themeEntryIgnored {
			continue
		}
		file := entry.Name()
		// The limit covers every entry that is read or reported, so it also
		// bounds what the settings page lists.
		if examined == MaximumUserThemeFiles {
			scan.report(file, ThemeProblemLimit)
			break
		}
		examined++
		switch kind {
		case themeEntryMisnamed:
			scan.report(file, ThemeProblemExtension)
		case themeEntryLink:
			scan.report(file, ThemeProblemLink)
		case themeEntryDirectory:
			scan.readThemeDirectory(root, file)
		default:
			name := strings.TrimSuffix(file, userThemeExtension)
			if !scan.acceptName(name, file) {
				continue
			}
			if directories[name] && isThemeDirectory(root, name) {
				scan.report(file, ThemeProblemDuplicate)
				continue
			}
			scan.readTheme(root, name, file, file, false)
		}
	}
	return scan
}

// report notes a file of the themes directory that is not in use.
func (scan *userThemeScan) report(file, reason string) {
	scan.problems = append(scan.problems, ThemeProblem{File: file, Reason: reason})
}

// acceptName reports whether a name can be the name of a theme of the
// directory, and notes a name that cannot for the file it was found with.
func (scan *userThemeScan) acceptName(name, shown string) bool {
	switch {
	case !model.IsThemeName(name):
		scan.report(shown, ThemeProblemName)
	case !isUserThemeName(name):
		scan.report(shown, ThemeProblemReserved)
	default:
		return true
	}
	return false
}

// readThemeDirectory looks at a directory of the themes directory. One that
// holds a theme.yml is the theme of its name. Any other directory is left
// alone, except that a theme.yaml in it is reported, as that extension is for
// a file.
func (scan *userThemeScan) readThemeDirectory(root *os.Root, directory string) {
	manifest := filepath.Join(directory, userThemeManifest)
	shown := directory + "/" + userThemeManifest
	if _, err := root.Lstat(manifest); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			scan.report(shown, ThemeProblemRead)
			return
		}
		if _, err = root.Lstat(filepath.Join(directory, "theme.yaml")); err == nil {
			scan.report(directory+"/theme.yaml", ThemeProblemExtension)
		}
		return
	}
	if scan.acceptName(directory, shown) {
		scan.readTheme(root, directory, manifest, shown, true)
	}
}

// readTheme reads the theme file of a name and takes the theme into use, or
// notes why it does not. file is the path of the theme file in the themes
// directory, shown is how the settings page names it, and directory reports
// the theme.yml of a directory.
func (scan *userThemeScan) readTheme(root *os.Root, name, file, shown string, directory bool) {
	theme, unused, problem := readUserTheme(root, file)
	if !unused.Empty() {
		scan.notices = append(scan.notices, ThemeNotice{File: shown, Unused: unused})
	}
	if problem != nil {
		problem.File = shown
		scan.problems = append(scan.problems, *problem)
		return
	}
	definition := theme.Theme(name)
	definition.Directory = directory
	scan.themes[name] = definition
	scan.order = append(scan.order, name)
}

// readUserTheme reads one theme file. A theme file is a regular file of the
// directory: a link is reported instead of followed. The file is examined
// again once it is open, and no more than the size limit is read from it,
// because the entry can be replaced between the two steps.
func readUserTheme(root *os.Root, file string) (model.ThemeFile, model.ThemeFileUnused, *ThemeProblem) {
	problem := func(reason string) (model.ThemeFile, model.ThemeFileUnused, *ThemeProblem) {
		return model.ThemeFile{}, model.ThemeFileUnused{}, &ThemeProblem{Reason: reason}
	}
	entry, err := root.Lstat(file)
	if err != nil {
		return problem(ThemeProblemRead)
	}
	if !entry.Mode().IsRegular() {
		return problem(ThemeProblemType)
	}
	handle, err := root.Open(file)
	if err != nil {
		return problem(ThemeProblemRead)
	}
	defer handle.Close()
	opened, err := handle.Stat()
	if err != nil {
		return problem(ThemeProblemRead)
	}
	if !opened.Mode().IsRegular() {
		return problem(ThemeProblemType)
	}
	if opened.Size() > MaximumThemeFileBytes {
		return problem(ThemeProblemSize)
	}
	raw, err := io.ReadAll(io.LimitReader(handle, MaximumThemeFileBytes+1))
	if err != nil {
		return problem(ThemeProblemRead)
	}
	if len(raw) > MaximumThemeFileBytes {
		return problem(ThemeProblemSize)
	}
	theme, unused, err := model.ReadThemeFile(raw)
	if err != nil {
		rejected := ThemeFileProblem(err)
		return model.ThemeFile{}, unused, &rejected
	}
	return theme, unused, nil
}

// ThemeFileProblem describes why the content of a theme file was rejected:
// the reason, and the part of the file it is about. It is the same
// description for a file of the themes directory and for one that is
// submitted to be stored there.
func ThemeFileProblem(err error) ThemeProblem {
	rejected := ThemeProblem{Reason: themeProblemReason(err)}
	var rejection *model.ThemeFileError
	if errors.As(err, &rejection) {
		rejected.Subject = rejection.Subject
	}
	// The text of the reason says which contract is read.
	if rejected.Reason == ThemeProblemContract {
		rejected.Subject = ""
	}
	return rejected
}

// themeProblemReason names the reason a theme file was rejected.
func themeProblemReason(err error) string {
	switch {
	case errors.Is(err, model.ErrThemeContract):
		return ThemeProblemContract
	case errors.Is(err, model.ErrThemeTokenValue):
		return ThemeProblemValue
	case errors.Is(err, model.ErrPaletteColor):
		return ThemeProblemColor
	case errors.Is(err, model.ErrPaletteContrast):
		return ThemeProblemContrast
	case errors.Is(err, model.ErrThemeText):
		return ThemeProblemText
	default:
		return ThemeProblemSyntax
	}
}

// IsUserTheme reports whether a name belongs to a theme file in use.
func IsUserTheme(name string) bool {
	themeMu.RLock()
	defer themeMu.RUnlock()
	_, known := userThemeState.themes[name]
	return known
}

// UserThemes returns the theme files in use in the order of their names, the
// files that are not in use, and the files with parts that are not used.
func UserThemes() ([]model.Theme, []ThemeProblem, []ThemeNotice) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	themes := make([]model.Theme, 0, len(userThemeState.order))
	for _, name := range userThemeState.order {
		themes = append(themes, userThemeState.themes[name])
	}
	return themes, slices.Clone(userThemeState.problems), slices.Clone(userThemeState.notices)
}

// ReloadUserThemes reads the themes directory again. The selected theme
// follows its file: an edit is applied, a file that appeared is used, and a
// file that went away or was rejected leaves the selection without a theme.
func ReloadUserThemes() {
	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	reloadUserThemes()
}

// reloadUserThemes is ReloadUserThemes for callers that hold userThemeScanMu.
func reloadUserThemes() {
	themeMu.RLock()
	selected := ThemeCurrent
	themeMu.RUnlock()
	// The state is taken before the files are read: a change made in between
	// differs from it, and the next check reads the directory again.
	stamp := statTheme(selected)
	scan := scanUserThemes()
	themeMu.Lock()
	defer themeMu.Unlock()
	previous, wasKnown := themeDefinition(ThemeCurrent)
	userThemeState = scan
	if current, known := themeDefinition(ThemeCurrent); known != wasKnown || !reflect.DeepEqual(previous, current) {
		applyTheme(ThemeCurrent)
	}
	if ThemeCurrent == selected {
		selectedThemeStamp = stamp
	}
}

// refreshSelectedTheme notices a change of the selected theme's file without
// a restart. While the selection can belong to a theme of the directory it
// costs two stats per call, one for each form the theme can have, as reading
// apps.yml and bookmarks.yml costs one each; a bundled theme and the custom
// colors cost nothing.
func refreshSelectedTheme() {
	if selectedThemeFileCurrent() {
		return
	}
	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	// Requests that noticed the same change wait for one reading of the
	// directory instead of each starting another.
	if selectedThemeFileCurrent() {
		return
	}
	reloadUserThemes()
}

// selectedThemeFileCurrent reports whether the selected theme needs no new
// reading of the themes directory.
func selectedThemeFileCurrent() bool {
	themeMu.RLock()
	selected, stamp := ThemeCurrent, selectedThemeStamp
	themeMu.RUnlock()
	return !isUserThemeName(selected) || stamp.current(statTheme(selected), time.Now())
}
