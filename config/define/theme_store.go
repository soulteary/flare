package define

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	version "github.com/soulteary/version-kit/v4"
)

// Reasons a theme file is not stored. A file that is not a usable theme is
// refused with the *model.ThemeFileError of its content instead.
var (
	// ErrThemeStoreName reports a name that cannot be the name of a theme
	// file: not of the accepted form, or the name of a bundled theme or of
	// the custom colors.
	ErrThemeStoreName = errors.New("the name cannot be the name of a theme file")
	// ErrThemeStoreExists reports a theme file of the same name that the
	// caller did not ask to replace.
	ErrThemeStoreExists = errors.New("a theme file of this name exists")
	// ErrThemeStoreDirectory reports a directory that is the theme of this
	// name. A file of the name would not be read, and a directory is not
	// for an upload to replace.
	ErrThemeStoreDirectory = errors.New("a directory is the theme of this name")
	// ErrThemeStoreFull reports a themes directory that already holds as
	// many entries as are looked at.
	ErrThemeStoreFull = errors.New("the themes directory holds as many entries as are looked at")
	// ErrThemeStoreSize reports a file larger than a theme file can be.
	ErrThemeStoreSize = errors.New("the file is larger than a theme file can be")
	// ErrThemeStoreWrite reports that the file could not be written.
	ErrThemeStoreWrite = errors.New("the theme file could not be written")
	// ErrThemeStoreSafeMode reports that the themes directory is left alone
	// because the instance runs in safe mode.
	ErrThemeStoreSafeMode = errors.New("the themes directory is not used in safe mode")
)

// Suffixes of the file names offered for a download. A bundled theme and the
// custom colors cannot be stored under their own names, so the copy that is
// handed out has a name that can be.
const (
	bundledThemeCopySuffix = "-copy"
	customThemeCopyName    = "custom-colors"
)

// IsUserThemeName reports whether a name is one a theme of the themes
// directory can have: the name of a theme that no bundled theme has and that
// is not the name of the custom colors.
func IsUserThemeName(name string) bool { return isUserThemeName(name) }

// CheckUserThemeFile reports why the content of a theme file would not be
// stored as a theme of one file, whatever its name and whatever is in the
// themes directory: its size, and what a reading of the file refuses.
func CheckUserThemeFile(raw []byte) error {
	_, err := readStoredThemeFile(raw)
	return err
}

// readStoredThemeFile reads the content of a theme file that is to be
// stored as a theme of one file.
func readStoredThemeFile(raw []byte) (model.ThemeFile, error) {
	if len(raw) > MaximumThemeFileBytes {
		return model.ThemeFile{}, ErrThemeStoreSize
	}
	theme, _, rejected := model.ReadThemeFileFor(raw, version.Version)
	if rejected != nil {
		return model.ThemeFile{}, rejected
	}
	// What is stored here is a theme of one file, which has no assets: a
	// background image it names would never be there.
	if theme.Background != nil {
		return model.ThemeFile{}, &model.ThemeFileError{Reason: model.ErrThemeBackgroundAsset, Subject: theme.Background.Image}
	}
	return theme, nil
}

// StoreUserTheme writes a theme file into the themes directory under the
// given theme name and reads the directory again. The content has to be a
// usable theme file, and it is written as given, so comments and parts for
// later releases stay. An existing theme file is replaced only if replace is
// set, and nothing is stored under a name whose theme is a directory. The
// file is written beside its place and moved into it, so a reader sees the
// previous file or the new one.
func StoreUserTheme(name string, raw []byte, replace bool) error {
	if AppFlags.ThemeSafeMode {
		return ErrThemeStoreSafeMode
	}
	if !isUserThemeName(name) {
		return ErrThemeStoreName
	}
	theme, rejected := readStoredThemeFile(raw)
	if rejected != nil {
		return rejected
	}
	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	// A file that is based on another theme is stored if it is a theme in
	// use once it is there.
	if theme.Extends != "" {
		if err := checkThemeBase(name, theme); err != nil {
			return err
		}
	}
	if err := os.Mkdir(UserThemeDirectory, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return errors.Join(ErrThemeStoreWrite, err)
	}
	root, err := os.OpenRoot(UserThemeDirectory)
	if err != nil {
		return errors.Join(ErrThemeStoreWrite, err)
	}
	defer root.Close()
	if entry, statErr := root.Lstat(name); statErr == nil && entry.IsDir() && isThemeDirectory(root, name) {
		return ErrThemeStoreDirectory
	}
	file := name + userThemeExtension
	mode := os.FileMode(0600)
	existing, err := root.Lstat(file)
	switch {
	case err == nil && !existing.Mode().IsRegular():
		// A directory or a link has that name. It is not a theme file, and
		// it is not for an upload to remove.
		return ErrThemeStoreExists
	case err == nil && !replace:
		return ErrThemeStoreExists
	case err == nil:
		mode = existing.Mode().Perm()
	case !errors.Is(err, fs.ErrNotExist):
		return errors.Join(ErrThemeStoreWrite, err)
	default:
		// One more file is only read while the directory holds fewer
		// entries than the number that are looked at.
		if count, countErr := countThemeFiles(root); countErr != nil {
			return errors.Join(ErrThemeStoreWrite, countErr)
		} else if count >= MaximumUserThemeFiles {
			return ErrThemeStoreFull
		}
	}
	if err = writeThemeFile(root, file, raw, mode); err != nil {
		return errors.Join(ErrThemeStoreWrite, err)
	}
	reloadUserThemes()
	return nil
}

// checkThemeBase reports why a theme file that is based on another theme
// would not be in use under a name, as a *model.ThemeFileError: the theme it
// is based on is not in use, the themes below it lead back to the name or
// are more than a theme can have, or the file and the themes below it are
// not a theme together. The themes directory is read as it is now, and the
// file is looked at in the place of the theme of that name. Callers hold
// userThemeScanMu.
func checkThemeBase(name string, file model.ThemeFile) error {
	scan := scanUserThemes()
	scan.candidates[name] = &themeCandidate{file: file}
	below, reason := scan.followBases(name)
	if _, inUse := scan.themes[file.Extends]; reason == "" && !inUse && !isBundledTheme(file.Extends) {
		reason = ThemeProblemExtends
	}
	switch reason {
	case ThemeProblemExtends:
		return &model.ThemeFileError{Reason: model.ErrThemeBase, Subject: file.Extends}
	case ThemeProblemExtendsCycle:
		return &model.ThemeFileError{Reason: model.ErrThemeBaseCycle, Subject: strings.Join(below, " → ")}
	case ThemeProblemExtendsDepth:
		return &model.ThemeFileError{Reason: model.ErrThemeBaseDepth, Subject: strings.Join(below, " → ")}
	}
	_, err := file.On(scan.wholeFile(file.Extends))
	return err
}

// countThemeFiles counts the entries of the themes directory that a reading
// of it looks at.
func countThemeFiles(root *os.Root) (int, error) {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if themeEntryKind(entry) != themeEntryIgnored {
			count++
		}
	}
	return count, nil
}

// writeThemeFile writes a file of the themes directory through a hidden
// temporary file, which a reading of the directory leaves alone.
func writeThemeFile(root *os.Root, file string, raw []byte, mode os.FileMode) error {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := ".import-" + hex.EncodeToString(random[:]) + ".tmp"
	handle, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, err = handle.Write(raw)
	if err == nil {
		err = handle.Sync()
	}
	if closeErr := handle.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = root.Rename(temporary, file)
	}
	if err != nil {
		// A temporary file that cannot be removed either is reported with
		// the failure; a reading of the directory leaves it alone.
		return errors.Join(err, root.Remove(temporary))
	}
	syncThemesDirectory(root)
	return nil
}

// syncThemesDirectory asks for the entries of the themes directory to be
// written through, so that a new entry survives an interruption as its
// content does. What was stored is in place by now, so a failure here is
// logged, not returned.
func syncThemesDirectory(root *os.Root) {
	if directory, openErr := root.Open("."); openErr == nil {
		if syncErr := errors.Join(directory.Sync(), directory.Close()); syncErr != nil {
			logger.GetLogger().Warn("The themes directory could not be synced after storing a theme", "reason", syncErr.Error())
		}
	}
}

// ExportTheme returns a theme as a theme file, with the file name to offer
// for it: the file of a theme in the themes directory as it is, the file a
// bundled theme is defined by, or a file written from the custom colors. The
// copies of the last two have names under which they can be stored again. It
// reports false for a name that is not a theme in use, and for a theme that
// is a directory, which ExportThemeArchive hands out.
func ExportTheme(name string) (string, []byte, bool) {
	if name == model.ThemeCustomName {
		themeMu.RLock()
		palette, stored := themeCustom, themeCustomSet
		themeMu.RUnlock()
		if !stored {
			return "", nil, false
		}
		raw := "contract: 1\ntokens:\n  background: \"" + palette.Background + "\"\n  primary: \"" + palette.Primary + "\"\n  accent: \"" + palette.Accent + "\"\n"
		return customThemeCopyName + userThemeExtension, []byte(raw), true
	}
	if isBundledTheme(name) {
		raw, err := bundledThemeFiles.ReadFile("bundled/" + name + userThemeExtension)
		if err != nil {
			return "", nil, false
		}
		return name + bundledThemeCopySuffix + userThemeExtension, raw, true
	}
	// A theme that is a directory is not handed out as a single file: the
	// directory is the form for a theme of more than one file.
	themeMu.RLock()
	definition, known := userThemeState.themes[name]
	themeMu.RUnlock()
	if !known || definition.Directory {
		return "", nil, false
	}
	root, err := os.OpenRoot(UserThemeDirectory)
	if err != nil {
		return "", nil, false
	}
	defer root.Close()
	file := name + userThemeExtension
	if entry, statErr := root.Lstat(file); statErr != nil || !entry.Mode().IsRegular() {
		return "", nil, false
	}
	handle, err := root.Open(file)
	if err != nil {
		return "", nil, false
	}
	defer handle.Close()
	raw, err := io.ReadAll(io.LimitReader(handle, MaximumThemeFileBytes+1))
	if err != nil || len(raw) > MaximumThemeFileBytes {
		return "", nil, false
	}
	return file, raw, true
}
