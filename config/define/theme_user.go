package define

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	version "github.com/soulteary/version-kit/v4"
)

// An instance reads its own themes from the themes directory of its working
// directory, next to config.yml. A theme is a file there, named after the
// theme, or a directory named after the theme that holds a theme file named
// theme.yml. The directory is the form for a theme of more than one file.
const (
	UserThemeDirectory = "themes"
	// ThemeRepositoryURL is where themes for the application are listed.
	// The settings link to it; the application never asks it for anything.
	ThemeRepositoryURL = "https://github.com/soulteary/flare-themes"
	userThemeExtension = ".yml"
	// userThemeManifest is the theme file of a theme that is a directory.
	userThemeManifest = model.ThemeManifestName
	// userThemeStylesheet is the stylesheet such a theme can bring.
	userThemeStylesheet = model.ThemeStylesheetName
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
	// The release a theme file needs: one that is later than this one, and
	// one that is not written as a release.
	ThemeProblemRequires      = "requires"
	ThemeProblemRequiresValue = "requires_value"
	// Reasons the stylesheet of a theme is not accepted. The theme is then
	// not in use: it is not applied without its stylesheet.
	ThemeProblemStyleSize      = "style_size"
	ThemeProblemStyleEncoding  = "style_encoding"
	ThemeProblemStyleURL       = "style_url"
	ThemeProblemStyleAsset     = "style_asset"
	ThemeProblemStyleRule      = "style_rule"
	ThemeProblemStyleFunction  = "style_function"
	ThemeProblemStyleCharacter = "style_character"
	// ThemeProblemOption is an option of a theme that is not accepted.
	ThemeProblemOption = "option"
	// Reasons a theme that is based on another is not in use: an extends
	// that is not the name of a theme, a theme below it that is not in use,
	// a theme that is based on itself by way of the themes below it, and
	// more themes below it than a theme can have.
	ThemeProblemExtendsValue = "extends_value"
	ThemeProblemExtends      = "extends"
	ThemeProblemExtendsCycle = "extends_cycle"
	ThemeProblemExtendsDepth = "extends_depth"
	// Reasons the background of a theme is not accepted: one that is not
	// written in the accepted form, and an image that is not among the
	// images in the assets of the theme.
	ThemeProblemBackground      = "background"
	ThemeProblemBackgroundAsset = "background_asset"
	// Reasons a file in the assets of a theme is not accepted. The theme is
	// then not in use either: it is not applied without a file it brings.
	ThemeProblemAssetName    = "asset_name"
	ThemeProblemAssetSize    = "asset_size"
	ThemeProblemAssetContent = "asset_content"
	ThemeProblemAssetLimit   = "asset_limit"
)

// ThemeProblem is a file in the themes directory that is not in use. File is
// its name in that directory, and for a file of a directory the path with
// slashes, such as forest/theme.yml. Subject is the part of the file the
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

// themeStamp is the state of the files a theme of one name can be read from:
// the file of that name, and theme.yml in the directory of that name with the
// stylesheet and the directory of the assets beside it. Both forms are
// watched, because each can take the place of the other. files holds the
// files of the assets a reading of the themes directory looked at, in the
// state they had then; the state of their directory tells when a file is
// added, removed or renamed.
type themeStamp struct {
	file, manifest, stylesheet, assets themeFileStamp
	files                              []themeAssetStamp
	// pages is the state of each template, those of the pages and those of
	// the items. A template that is added, with or without its directory,
	// changes it as an edit does.
	pages [6]themeFileStamp
	// bases holds the same state for the themes a theme is based on, one
	// below another, as far as a reading of the themes directory followed
	// them. It is set for the selected theme, whose pages take from all of
	// them.
	bases []themeBaseStamp
}

// themeBaseStamp is the state of the files of a theme another theme is based
// on.
type themeBaseStamp struct {
	name  string
	stamp themeStamp
}

// themeAssetStamp is the state of a file in the assets of a theme when a
// reading of the themes directory looked at it.
type themeAssetStamp struct {
	name  string
	stamp themeFileStamp
}

// statTheme returns the state of the files a theme name is read from. A name
// that cannot be a theme of the directory has the state of missing files.
func statTheme(name string) themeStamp {
	if !isUserThemeName(name) {
		return themeStamp{}
	}
	stamp := themeStamp{
		file:     statThemeFile(filepath.Join(UserThemeDirectory, name+userThemeExtension)),
		manifest: statThemeFile(filepath.Join(UserThemeDirectory, name, userThemeManifest)),
	}
	// A stylesheet and assets belong to a theme that is a directory; beside
	// no theme.yml they are not read.
	if stamp.manifest.exists {
		stamp.stylesheet = statThemeFile(filepath.Join(UserThemeDirectory, name, userThemeStylesheet))
		stamp.assets = statThemeFile(filepath.Join(UserThemeDirectory, name, model.ThemeAssetDirectory))
		for i, file := range themeTemplateFiles() {
			stamp.pages[i] = statThemeFile(filepath.Join(UserThemeDirectory, name, model.ThemeTemplateDirectory, file))
		}
	}
	return stamp
}

// current reports whether the files are still in the state they were read at.
// The files of the assets are compared by assetsCurrent.
func (stamp themeStamp) current(now themeStamp, clock time.Time) bool {
	for i := range stamp.pages {
		if !stamp.pages[i].current(now.pages[i], clock) {
			return false
		}
	}
	return stamp.file.current(now.file, clock) && stamp.manifest.current(now.manifest, clock) &&
		stamp.stylesheet.current(now.stylesheet, clock) && stamp.assets.current(now.assets, clock)
}

// assetsCurrent reports whether the files in the assets of a theme that a
// reading looked at are still in the state they had then. A file that is
// edited in place leaves the state of its directory as it was.
func (stamp themeStamp) assetsCurrent(name string, clock time.Time) bool {
	for _, asset := range stamp.files {
		now := statThemeFile(filepath.Join(UserThemeDirectory, name, model.ThemeAssetDirectory, asset.name))
		if !asset.stamp.current(now, clock) {
			return false
		}
	}
	return true
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

// userThemeScan is one reading of the themes directory. watched holds, by the
// name of a theme, the files of its assets the reading looked at, whether or
// not the theme is in use.
type userThemeScan struct {
	themes   map[string]model.Theme
	order    []string
	problems []ThemeProblem
	notices  []ThemeNotice
	watched  map[string][]themeAssetStamp
	// files holds the theme file of each theme in use as a whole theme:
	// what its file gives together with the themes it is based on.
	files map[string]model.ThemeFile
	// bases holds, for a theme that is based on another, the names of the
	// themes below it as far as they could be followed, the nearest first,
	// whether or not the theme is in use.
	bases map[string][]string
	// stamps holds the state of the files of each name the reading looked
	// at, as it was before they were read, and listed the names the themes
	// directory had entries for.
	stamps map[string]themeStamp
	listed map[string]bool
	// candidates holds the theme files that were read and accepted as far
	// as they go, by the names of their themes, and pending those names in
	// the order they were read. A candidate is a theme in use once the
	// themes it is based on are known to be.
	candidates map[string]*themeCandidate
	pending    []string
	// templateRemarks are the templates of themes that are not accepted.
	// Their themes are in use without them, so they are kept apart from
	// the remarks about themes that are not in use.
	templateRemarks []ThemeProblem
}

// themeCandidate is a theme file of the themes directory that was read and
// accepted as far as it goes, with what its directory brings.
type themeCandidate struct {
	file model.ThemeFile
	// shown is how the settings page names the theme file.
	shown      string
	directory  bool
	stylesheet string
	assets     []model.ThemeAsset
	hash       string
	// templates are the page templates as they were read.
	templates []model.ThemeTemplate
	// rejected is why the theme is not in use after all, found when the
	// themes it is based on were followed. place is where that remark
	// belongs among the remarks of the reading, which are in the order of
	// the entries of the directory.
	rejected *ThemeProblem
	place    int
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
// directory has no themes of its own; that is not an error. In safe mode the
// directory is not opened, so nothing in it can keep the instance from being
// used: there are no themes of the directory and no remarks about it.
func scanUserThemes() userThemeScan {
	scan := newUserThemeScan()
	if AppFlags.ThemeSafeMode {
		return scan
	}
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
		switch themeEntryKind(entry) {
		case themeEntryDirectory:
			directories[entry.Name()] = true
			scan.listed[entry.Name()] = true
		case themeEntryFile:
			scan.listed[strings.TrimSuffix(entry.Name(), userThemeExtension)] = true
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
			scan.noteState(file)
			scan.readThemeDirectory(root, file)
		default:
			name := strings.TrimSuffix(file, userThemeExtension)
			if !scan.acceptName(name, file) {
				continue
			}
			scan.noteState(name)
			if directories[name] && isThemeDirectory(root, name) {
				scan.report(file, ThemeProblemDuplicate)
				continue
			}
			scan.readTheme(root, name, file, file, false)
		}
	}
	scan.resolveThemes()
	scan.problems = append(scan.problems, scan.templateRemarks...)
	return scan
}

// newUserThemeScan returns a reading of the themes directory that has not
// looked at anything yet.
func newUserThemeScan() userThemeScan {
	return userThemeScan{
		themes: map[string]model.Theme{}, watched: map[string][]themeAssetStamp{}, files: map[string]model.ThemeFile{},
		bases: map[string][]string{}, stamps: map[string]themeStamp{}, listed: map[string]bool{}, candidates: map[string]*themeCandidate{},
	}
}

// noteState notes the state of the files a theme name is read from, before
// they are read. A change made from then on differs from it, and is seen by
// the next page that depends on the theme. Both forms of a name are noted
// when the first of the two is come to.
func (scan *userThemeScan) noteState(name string) {
	if _, noted := scan.stamps[name]; !noted && isUserThemeName(name) {
		scan.stamps[name] = statTheme(name)
	}
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

// readTheme reads the theme file of a name with what its directory brings,
// and notes why the theme is not in use where one of them is not accepted.
// What is accepted is a candidate: it is a theme in use once the themes it is
// based on are followed, by resolveThemes. file is the path of the theme file
// in the themes directory, shown is how the settings page names it, and
// directory reports the theme.yml of a directory.
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
	candidate := &themeCandidate{file: theme, shown: shown, directory: directory}
	// The image a theme file names is one of the assets beside it. Whether
	// that is the name of an image was settled when the file was read.
	image := ""
	if theme.Background != nil {
		image, _ = theme.Background.Asset()
	}
	if directory {
		// A theme is used as a whole: with a file of its assets or a
		// stylesheet that is not accepted it is not applied without them.
		// The assets are read first, because the background image and the
		// stylesheet are held to them.
		assets, rejected := scan.readThemeAssets(root, name)
		if rejected != nil {
			scan.problems = append(scan.problems, *rejected)
			return
		}
		if !hasThemeBackground(image, assets) {
			scan.problems = append(scan.problems, themeBackgroundProblem(image, shown))
			return
		}
		stylesheet, rejected := readThemeStylesheet(root, name, assets)
		if rejected != nil {
			rejected.File = name + "/" + userThemeStylesheet
			scan.problems = append(scan.problems, *rejected)
			return
		}
		candidate.stylesheet, candidate.assets = stylesheet, assets
		if stylesheet != "" || len(assets) > 0 {
			candidate.hash = themeFilesHash(stylesheet, assets)
		}
		// A template that is not accepted leaves the theme in use: its
		// page keeps the markup of the application.
		var remarks []ThemeProblem
		candidate.templates, remarks = readThemeTemplates(root, name)
		scan.templateRemarks = append(scan.templateRemarks, remarks...)
	} else if !hasThemeBackground(image, nil) {
		// A theme that is a single file has no assets to take an image from.
		scan.problems = append(scan.problems, themeBackgroundProblem(image, shown))
		return
	}
	candidate.place = len(scan.problems)
	scan.candidates[name] = candidate
	scan.pending = append(scan.pending, name)
}

// resolveThemes takes the candidates into use. A theme that stands on its
// own is in use as it was read. A theme that is based on another is read on
// that theme, which has to be in use itself, and the two together are checked
// as one theme. A theme that is not in use after all is noted among the
// remarks where its file was come to, so the remarks stay in the order of the
// directory.
func (scan *userThemeScan) resolveThemes() {
	settled := make(map[string]bool, len(scan.pending))
	var resolve func(name string) bool
	resolve = func(name string) bool {
		candidate := scan.candidates[name]
		if candidate == nil {
			// Not a theme of the directory that was accepted: a bundled
			// theme can be what a theme is based on, and nothing else.
			return isBundledTheme(name)
		}
		if inUse, known := settled[name]; known {
			return inUse
		}
		file := candidate.file
		if base := file.Extends; base != "" {
			below, reason := scan.followBases(name)
			scan.bases[name] = below
			// The themes below are followed first, so this one is only
			// looked into where they end, within the number a theme can
			// have, and do not lead back.
			if reason == "" && !resolve(base) {
				reason = ThemeProblemExtends
			}
			if reason != "" {
				subject := base
				if reason != ThemeProblemExtends {
					subject = strings.Join(below, " → ")
				}
				candidate.rejected = &ThemeProblem{File: candidate.shown, Reason: reason, Subject: subject}
				settled[name] = false
				return false
			}
			whole, err := file.On(scan.wholeFile(base))
			if err != nil {
				rejected := ThemeFileProblem(err)
				rejected.File = candidate.shown
				candidate.rejected = &rejected
				settled[name] = false
				return false
			}
			file = whole
		}
		definition := file.Theme(name)
		definition.Directory = candidate.directory
		definition.Stylesheet, definition.Assets, definition.Hash = candidate.stylesheet, candidate.assets, candidate.hash
		if base, known := scan.themes[file.Extends]; known {
			// A page loads the stylesheets of the themes below before the
			// one of the theme, and an image the theme does not replace is
			// still among the assets of the theme that brings it.
			definition.Inherited = slices.Clone(base.Inherited)
			if base.Stylesheet != "" {
				definition.Inherited = append(definition.Inherited, model.ThemeSource{Name: base.Name, Hash: base.Hash})
			}
			if candidate.file.Background == nil && base.BackgroundImage != "" {
				definition.BackgroundSource = base.BackgroundSource
				if definition.BackgroundSource.Name == "" {
					definition.BackgroundSource = model.ThemeSource{Name: base.Name, Hash: base.Hash}
				}
			}
		}
		// The templates are compiled with what the theme is as a whole,
		// and those of the themes below are looked at for what the theme
		// brings no template for. A theme below has said what is to say
		// about its own templates.
		templates, remarks := checkThemeTemplates(definition, scan.templatesBelow(name, definition.Hash), func(below string) model.Theme { return scan.themes[below] })
		definition.Templates = templates
		for _, remark := range remarks {
			if !slices.Contains(scan.templateRemarks, remark) {
				scan.templateRemarks = append(scan.templateRemarks, remark)
			}
		}
		scan.themes[name], scan.files[name] = definition, file
		settled[name] = true
		return true
	}
	var remarks []ThemeProblem
	noted := 0
	for _, name := range scan.pending {
		if resolve(name) {
			scan.order = append(scan.order, name)
			continue
		}
		candidate := scan.candidates[name]
		remarks = append(remarks, scan.problems[noted:candidate.place]...)
		remarks = append(remarks, *candidate.rejected)
		noted = candidate.place
	}
	if remarks != nil {
		scan.problems = append(remarks, scan.problems[noted:]...)
	}
}

// templatesBelow returns the templates a theme brings and those of the
// themes below it as they were read, the nearest theme first, each with the
// theme that brings it. The themes below are in use by the time a theme is
// taken into use; a bundled theme brings no template.
func (scan *userThemeScan) templatesBelow(name, hash string) []model.ThemeTemplate {
	var templates []model.ThemeTemplate
	from := model.ThemeSource{Name: name, Hash: hash}
	for _, theme := range append([]string{name}, scan.bases[name]...) {
		candidate := scan.candidates[theme]
		if candidate == nil {
			continue
		}
		if theme != name {
			from = model.ThemeSource{Name: theme, Hash: scan.themes[theme].Hash}
		}
		for _, template := range candidate.templates {
			template.From = from
			templates = append(templates, template)
		}
	}
	return templates
}

// followBases returns the names of the themes below a theme, the nearest
// first, as far as the theme files that were accepted lead, and why the
// theme cannot be in use where they do not end as they have to: a theme that
// comes again, which is the theme itself where it is based on itself, and
// more themes than a theme can have below it. Whether the last name is a
// theme at all is for the caller to see. The names are returned in every
// case: a page of the theme depends on the files of each of them.
func (scan *userThemeScan) followBases(name string) ([]string, string) {
	var below []string
	seen := map[string]bool{name: true}
	for current := scan.candidates[name]; current != nil && current.file.Extends != ""; {
		base := current.file.Extends
		below = append(below, base)
		switch {
		case base == name:
			return below, ThemeProblemExtendsCycle
		case seen[base]:
			// The themes below are based on one another in a circle this
			// theme is not part of. None of them is in use.
			return below, ThemeProblemExtends
		}
		seen[base] = true
		current = scan.candidates[base]
	}
	if len(below) > model.MaximumThemeBases {
		return below, ThemeProblemExtendsDepth
	}
	return below, ""
}

// wholeFile returns the theme file of a theme in use as a whole theme: that
// of a theme of the directory with the themes it is based on, or the file a
// bundled theme is defined by.
func (scan *userThemeScan) wholeFile(name string) model.ThemeFile {
	if file, known := scan.files[name]; known {
		return file
	}
	file, _ := bundledThemeFile(name)
	return file
}

// baseStamps returns the state of the files of the themes below a theme, as
// this reading found them: the state noted before a theme was read, with the
// files of its assets. A name the directory had no entry for has the state of
// missing files, so the theme is read again once the entry is there. A name
// with an entry the reading did not come to, in a directory with more entries
// than are looked at, has the state it is in now: nothing of it was read that
// a change could differ from.
func (scan *userThemeScan) baseStamps(name string) []themeBaseStamp {
	var stamps []themeBaseStamp
	seen := map[string]bool{name: true}
	for _, base := range scan.bases[name] {
		if seen[base] || !isUserThemeName(base) {
			continue
		}
		seen[base] = true
		stamp, noted := scan.stamps[base]
		if !noted && scan.listed[base] {
			stamp = statTheme(base)
		}
		stamp.files = scan.watched[base]
		stamps = append(stamps, themeBaseStamp{name: base, stamp: stamp})
	}
	return stamps
}

// hasThemeBackground reports whether the background image a theme file names
// is among the assets beside it. A file that names none has what it needs.
func hasThemeBackground(image string, assets []model.ThemeAsset) bool {
	if image == "" {
		return true
	}
	for _, asset := range assets {
		if asset.Name == image {
			return true
		}
	}
	return false
}

// themeBackgroundProblem describes a theme whose background image is not
// among its assets, for the theme file that names the image.
func themeBackgroundProblem(image, file string) ThemeProblem {
	return ThemeProblem{File: file, Reason: ThemeProblemBackgroundAsset, Subject: model.ThemeAssetDirectory + "/" + image}
}

// themeFilesHash names the stylesheet and the assets of a theme, in the state
// they were read in, for the addresses they are served at. The stylesheet
// goes in by its content, which is kept. A file of the assets is served from
// the directory and goes in by its name, its size and its modification time:
// it is served for as long as it has that state. A stylesheet holds no NUL
// and a name of the assets neither, so the parts cannot be taken for one
// another. A theme without assets has the hash of its stylesheet.
func themeFilesHash(stylesheet string, assets []model.ThemeAsset) string {
	digest := sha256.New()
	digest.Write([]byte(stylesheet))
	for _, asset := range assets {
		fmt.Fprintf(digest, "\x00%s\x00%d\x00%d", asset.Name, asset.Size, asset.ModTime.UnixNano())
	}
	return hex.EncodeToString(digest.Sum(nil)[:8])
}

// readThemeAssets reads the assets of a theme that is a directory: the images
// and fonts in the directory named assets beside its theme.yml. A theme
// without that directory has none. An entry is meant as an asset if its name
// ends in the extension of an image or a font a theme can bring. Every other
// entry is left alone and never served, and so is a hidden one: editors, file
// managers and storage devices put files of their own into a directory. An
// entry that is meant as an asset and not accepted is the reason the theme is
// not in use.
func (scan *userThemeScan) readThemeAssets(root *os.Root, theme string) ([]model.ThemeAsset, *ThemeProblem) {
	shown := theme + "/" + model.ThemeAssetDirectory
	info, err := root.Lstat(filepath.Join(theme, model.ThemeAssetDirectory))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, &ThemeProblem{File: shown, Reason: ThemeProblemRead}
	case info.Mode()&fs.ModeSymlink != 0:
		return nil, &ThemeProblem{File: shown, Reason: ThemeProblemLink}
	case !info.IsDir():
		// A file of that name is not a directory of assets.
		return nil, nil
	}
	entries, err := fs.ReadDir(root.FS(), shown)
	if err != nil {
		return nil, &ThemeProblem{File: shown, Reason: ThemeProblemRead}
	}
	var assets []model.ThemeAsset
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || !model.HasThemeAssetExtension(name) {
			continue
		}
		if len(assets) == model.MaximumThemeAssets {
			return nil, &ThemeProblem{File: shown, Reason: ThemeProblemAssetLimit}
		}
		asset, reason := scan.readThemeAsset(root, theme, name)
		if reason != "" {
			return nil, &ThemeProblem{File: shown + "/" + name, Reason: reason}
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

// readThemeAsset looks at one file in the assets of a theme: its name, that
// it is a regular file of a bounded size, and that its content starts as the
// kind of file its name says. The content is not kept; the file is served
// from the directory for as long as it has the size and the modification
// time noted here. The second result is the reason a file is not accepted.
// The state of the file is noted before it is opened, so that a change made
// from then on is seen by the next page.
func (scan *userThemeScan) readThemeAsset(root *os.Root, theme, name string) (model.ThemeAsset, string) {
	file := filepath.Join(theme, model.ThemeAssetDirectory, name)
	entry, err := root.Lstat(file)
	if err != nil {
		return model.ThemeAsset{}, ThemeProblemRead
	}
	scan.watched[theme] = append(scan.watched[theme], themeAssetStamp{
		name: name, stamp: themeFileStamp{modTime: entry.ModTime(), size: entry.Size(), exists: true},
	})
	switch {
	case entry.Mode()&fs.ModeSymlink != 0:
		return model.ThemeAsset{}, ThemeProblemLink
	case !entry.Mode().IsRegular():
		return model.ThemeAsset{}, ThemeProblemType
	case !model.IsThemeAssetName(name):
		return model.ThemeAsset{}, ThemeProblemAssetName
	}
	handle, opened, reason := openThemeDirectoryFile(root, file, model.MaximumThemeAssetBytes)
	switch reason {
	case "":
	case ThemeProblemSize:
		return model.ThemeAsset{}, ThemeProblemAssetSize
	default:
		return model.ThemeAsset{}, reason
	}
	defer handle.Close()
	head := make([]byte, model.ThemeAssetHeadBytes)
	read, err := io.ReadFull(handle, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return model.ThemeAsset{}, ThemeProblemRead
	}
	if !model.IsThemeAssetContent(name, head[:read]) {
		return model.ThemeAsset{}, ThemeProblemAssetContent
	}
	kind, _ := model.ThemeAssetType(name)
	return model.ThemeAsset{Name: name, Type: kind, Size: opened.Size(), ModTime: opened.ModTime()}, ""
}

// readThemeStylesheet reads the stylesheet of a theme that is a directory. A
// theme without a theme.css, or with an empty one, has no stylesheet. The
// file is read with the care of a theme file, and checked: a stylesheet of a
// theme loads nothing but the assets of the theme, which are given here.
func readThemeStylesheet(root *os.Root, directory string, assets []model.ThemeAsset) (string, *ThemeProblem) {
	file := filepath.Join(directory, userThemeStylesheet)
	if _, err := root.Lstat(file); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	raw, reason := readThemeDirectoryFile(root, file, model.MaximumThemeStylesheetBytes)
	switch reason {
	case "":
	case ThemeProblemSize:
		return "", &ThemeProblem{Reason: ThemeProblemStyleSize}
	default:
		return "", &ThemeProblem{Reason: reason}
	}
	names := make(map[string]bool, len(assets))
	for _, asset := range assets {
		names[asset.Name] = true
	}
	if err := model.CheckThemeStylesheetFor(raw, names); err != nil {
		rejected := ThemeProblem{Reason: ThemeProblemStyleEncoding}
		var rejection *model.ThemeStyleError
		if errors.As(err, &rejection) {
			rejected.Subject = rejection.Subject()
		}
		switch {
		case errors.Is(err, model.ErrThemeStyleURL):
			rejected.Reason = ThemeProblemStyleURL
		case errors.Is(err, model.ErrThemeStyleAsset):
			rejected.Reason = ThemeProblemStyleAsset
		case errors.Is(err, model.ErrThemeStyleRule):
			rejected.Reason = ThemeProblemStyleRule
		case errors.Is(err, model.ErrThemeStyleFunction):
			rejected.Reason = ThemeProblemStyleFunction
		case errors.Is(err, model.ErrThemeStyleCharacter):
			rejected.Reason = ThemeProblemStyleCharacter
		}
		return "", &rejected
	}
	return string(raw), nil
}

// openThemeDirectoryFile opens a file of the themes directory of up to a
// size. Such a file is a regular file: a link is reported instead of
// followed. The file is examined again once it is open, because the entry can
// be replaced between the two steps, and the state returned is that of the
// open file. Where the file is not opened, the last result is the reason:
// that it cannot be read, that it is not a regular file, or that it is too
// large. The caller closes the file.
func openThemeDirectoryFile(root *os.Root, file string, limit int64) (*os.File, fs.FileInfo, string) {
	entry, err := root.Lstat(file)
	if err != nil {
		return nil, nil, ThemeProblemRead
	}
	if !entry.Mode().IsRegular() {
		return nil, nil, ThemeProblemType
	}
	handle, err := root.Open(file)
	if err != nil {
		return nil, nil, ThemeProblemRead
	}
	opened, err := handle.Stat()
	reason := ""
	switch {
	case err != nil:
		reason = ThemeProblemRead
	case !opened.Mode().IsRegular():
		reason = ThemeProblemType
	case opened.Size() > limit:
		reason = ThemeProblemSize
	}
	if reason != "" {
		_ = handle.Close()
		return nil, nil, reason
	}
	return handle, opened, ""
}

// readThemeDirectoryFile reads a file of the themes directory up to a size,
// with the care of openThemeDirectoryFile. No more than the limit is read
// from the file, whatever size it had when it was opened. Where the file is
// not read, the second result is the reason.
func readThemeDirectoryFile(root *os.Root, file string, limit int) ([]byte, string) {
	handle, _, reason := openThemeDirectoryFile(root, file, int64(limit))
	if reason != "" {
		return nil, reason
	}
	defer handle.Close()
	raw, err := io.ReadAll(io.LimitReader(handle, int64(limit)+1))
	if err != nil {
		return nil, ThemeProblemRead
	}
	if len(raw) > limit {
		return nil, ThemeProblemSize
	}
	return raw, ""
}

// readUserTheme reads one theme file.
func readUserTheme(root *os.Root, file string) (model.ThemeFile, model.ThemeFileUnused, *ThemeProblem) {
	raw, reason := readThemeDirectoryFile(root, file, MaximumThemeFileBytes)
	if reason != "" {
		return model.ThemeFile{}, model.ThemeFileUnused{}, &ThemeProblem{Reason: reason}
	}
	theme, unused, err := model.ReadThemeFileFor(raw, version.Version)
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
	case errors.Is(err, model.ErrThemeRequires):
		return ThemeProblemRequires
	case errors.Is(err, model.ErrThemeRequiresValue):
		return ThemeProblemRequiresValue
	case errors.Is(err, model.ErrThemeOption):
		return ThemeProblemOption
	case errors.Is(err, model.ErrThemeExtends):
		return ThemeProblemExtendsValue
	case errors.Is(err, model.ErrThemeBase):
		return ThemeProblemExtends
	case errors.Is(err, model.ErrThemeBaseCycle):
		return ThemeProblemExtendsCycle
	case errors.Is(err, model.ErrThemeBaseDepth):
		return ThemeProblemExtendsDepth
	case errors.Is(err, model.ErrThemeBackground):
		return ThemeProblemBackground
	case errors.Is(err, model.ErrThemeBackgroundAsset):
		return ThemeProblemBackgroundAsset
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
		stamp.files, stamp.bases = scan.watched[selected], scan.baseStamps(selected)
		selectedThemeStamp = stamp
	}
}

// refreshSelectedTheme notices a change of the selected theme's files without
// a restart. While the selection can belong to a theme of the directory it
// costs two stats per call, one for each form the theme can have, as reading
// apps.yml and bookmarks.yml costs one each. A theme that is a directory
// costs one more for its stylesheet, one for the directory of its assets and
// one for each file of them, of which there are at most as many as a theme
// can bring. A theme that is based on others costs the same again for each
// of the directory below it. A bundled theme and the custom colors cost
// nothing.
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
	if !isUserThemeName(selected) {
		return true
	}
	clock := time.Now()
	if !stamp.current(statTheme(selected), clock) || !stamp.assetsCurrent(selected, clock) {
		return false
	}
	// A page of the theme takes from every theme below it.
	for _, base := range stamp.bases {
		if !base.stamp.current(statTheme(base.name), clock) || !base.stamp.assetsCurrent(base.name, clock) {
			return false
		}
	}
	return true
}
