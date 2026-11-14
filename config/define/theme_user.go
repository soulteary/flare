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

// An instance reads its own themes from files: one theme per file in the
// themes directory of its working directory, next to config.yml. The name of
// the file without the extension is the name of the theme.
const (
	UserThemeDirectory = "themes"
	userThemeExtension = ".yml"
	// MaximumUserThemeFiles is the number of theme files looked at, in the
	// order of their names. Files after them are neither read nor reported.
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

// ThemeProblem is a file in the themes directory that is not in use. Subject
// is the part of the file the reason is about, such as tokens.radius, or the
// message of the YAML parser; it can be empty.
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

// statThemeFile returns the state of the file a theme name is read from. A
// name that cannot be a theme file, and a file that cannot be examined, have
// the state of a missing file.
func statThemeFile(name string) themeFileStamp {
	if !isUserThemeName(name) {
		return themeFileStamp{}
	}
	info, err := os.Lstat(filepath.Join(UserThemeDirectory, name+userThemeExtension))
	if err != nil {
		return themeFileStamp{}
	}
	return themeFileStamp{modTime: info.ModTime(), size: info.Size(), exists: true}
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
	// selectedThemeStamp is the state of the selected theme's file at the
	// reading userThemeState comes from. Guarded by themeMu.
	selectedThemeStamp themeFileStamp
)

// isUserThemeName reports whether a theme name can belong to a theme file.
// The bundled themes do not change after startup.
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

// scanUserThemes reads the themes directory. An instance without the
// directory has no theme files; that is not an error.
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
	examined := 0
	for _, entry := range entries {
		file := entry.Name()
		// Hidden files are left alone: editors and file managers create them.
		if strings.HasPrefix(file, ".") {
			continue
		}
		name, isTheme := strings.CutSuffix(file, userThemeExtension)
		// The other usual extension of YAML is reported: such a file is most
		// likely meant as a theme file.
		misnamed := !isTheme && strings.HasSuffix(file, ".yaml")
		if !isTheme && !misnamed {
			continue
		}
		// The limit covers every file that is read or reported, so it also
		// bounds what the settings page lists.
		if examined == MaximumUserThemeFiles {
			scan.problems = append(scan.problems, ThemeProblem{File: file, Reason: ThemeProblemLimit})
			break
		}
		examined++
		if misnamed {
			scan.problems = append(scan.problems, ThemeProblem{File: file, Reason: ThemeProblemExtension})
			continue
		}
		if !model.IsThemeName(name) {
			scan.problems = append(scan.problems, ThemeProblem{File: file, Reason: ThemeProblemName})
			continue
		}
		if !isUserThemeName(name) {
			scan.problems = append(scan.problems, ThemeProblem{File: file, Reason: ThemeProblemReserved})
			continue
		}
		theme, unused, problem := readUserTheme(root, file)
		if !unused.Empty() {
			scan.notices = append(scan.notices, ThemeNotice{File: file, Unused: unused})
		}
		if problem != nil {
			problem.File = file
			scan.problems = append(scan.problems, *problem)
			continue
		}
		scan.themes[name] = theme.Theme(name)
		scan.order = append(scan.order, name)
	}
	return scan
}

// readUserTheme reads one theme file. A theme file is a regular file in the
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
		rejected := ThemeProblem{Reason: themeProblemReason(err)}
		var rejection *model.ThemeFileError
		if errors.As(err, &rejection) {
			rejected.Subject = rejection.Subject
		}
		switch rejected.Reason {
		case ThemeProblemValue, ThemeProblemColor:
			// The model names the token; the file has it under tokens.
			rejected.Subject = "tokens." + rejected.Subject
		case ThemeProblemContract:
			rejected.Subject = ""
		}
		return model.ThemeFile{}, unused, &rejected
	}
	return theme, unused, nil
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
	stamp := statThemeFile(selected)
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
// a restart. While the selection can belong to a theme file it costs one stat
// per call, as reading apps.yml and bookmarks.yml does; a bundled theme and
// the custom colors cost nothing.
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
	return !isUserThemeName(selected) || stamp.current(statThemeFile(selected), time.Now())
}
