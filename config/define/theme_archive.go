package define

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
)

// A theme that is a directory is handed out and taken in as an archive: a ZIP
// file with the files of one theme. The files stand at the top of the
// archive, or all of them in one directory of any name.

// Reasons an archive of a theme is not taken in or handed out. A file of the
// archive that is no usable part of a theme is refused with a
// *ThemeArchiveProblem instead, and a theme that would not be in use because
// of the theme it is based on with a *model.ThemeFileError.
var (
	// ErrThemeArchiveSize reports an archive that is larger than an archive
	// of a theme can be.
	ErrThemeArchiveSize = errors.New("the archive is larger than an archive of a theme can be")
	// ErrThemeArchiveRead reports an archive, or a file in it, that cannot be
	// read: not a ZIP file, damaged, or packed in a way that is not read.
	ErrThemeArchiveRead = errors.New("the archive cannot be read")
	// ErrThemeArchiveEntries reports an archive with more entries than an
	// archive of a theme can have.
	ErrThemeArchiveEntries = errors.New("the archive has more entries than an archive of a theme can have")
	// ErrThemeArchiveEmpty reports an archive without a theme file at its
	// top or at the top of a directory.
	ErrThemeArchiveEmpty = errors.New("the archive holds no theme file")
	// ErrThemeArchiveSeveral reports an archive with more than one directory
	// that holds a theme file.
	ErrThemeArchiveSeveral = errors.New("the archive holds more than one theme")
	// ErrThemeArchiveTwice reports a file of the theme that the archive holds
	// more than once, also under names that differ in case alone.
	ErrThemeArchiveTwice = errors.New("the archive holds a file of the theme twice")
	// ErrThemeArchiveDocument reports a text document that is not text of
	// the accepted size, or one more than travel with a theme.
	ErrThemeArchiveDocument = errors.New("a text document of the archive is not accepted")
	// ErrThemeArchiveUnknown reports a name that is not a theme in use that
	// is a directory.
	ErrThemeArchiveUnknown = errors.New("not a theme in use that is a directory")
	// ErrThemeStoreOccupied reports something of the name in the themes
	// directory that is no theme: a directory without a theme file, a link,
	// or a file. It is not for an import to replace.
	ErrThemeStoreOccupied = errors.New("something of this name in the themes directory is no theme")
)

// ThemeArchiveFileError is a refusal about one file of an archive. Reason is
// one of the errors above, and File the path of the file in the theme.
type ThemeArchiveFileError struct {
	Reason error
	File   string
}

func (e *ThemeArchiveFileError) Error() string { return e.Reason.Error() + ": " + e.File }
func (e *ThemeArchiveFileError) Unwrap() error { return e.Reason }

// ThemeArchiveProblem is a file of an archive that the checks of a theme do
// not accept, described as the theme page describes such a file of the
// themes directory. File is its path in the theme, such as theme.css or
// assets/paper.png.
type ThemeArchiveProblem struct {
	Problem ThemeProblem
}

func (e *ThemeArchiveProblem) Error() string {
	return "the archive holds a file that is not accepted: " + e.Problem.File + ": " + e.Problem.Reason
}

// themeArchiveFile is a file of the theme in an archive: the entry, what it
// is, its name for an asset and for a document, and its path in the theme.
type themeArchiveFile struct {
	entry *zip.File
	kind  model.ThemeArchiveKind
	name  string
	path  string
}

// themeArchiveFiles picks the files of the theme out of an archive, and
// returns the other entries by their names in the archive: what is no file
// of a theme is not stored, and whoever imports the archive is told. Hidden
// entries, those a system adds by itself and directories are left out
// without a remark. The theme file stands at the top of the archive, or at
// the top of the one directory that holds one; the other files are looked
// for beside it.
func themeArchiveFiles(reader *zip.Reader) ([]themeArchiveFile, []string, error) {
	// Where the theme is: a theme file at the top decides, and otherwise the
	// directories that hold one.
	top, nested := false, map[string]bool{}
	for _, entry := range reader.File {
		if entry.Name == model.ThemeManifestName {
			top = true
		}
		// The one directory of a theme has a name of its own: not the top
		// of the archive written as a directory, and not a way out of it.
		directory, rest, inside := strings.Cut(entry.Name, "/")
		named := directory != "" && directory != "." && directory != ".."
		if kind, _ := model.ThemeArchiveEntry(directory); inside && named && rest == model.ThemeManifestName && kind != model.ThemeArchiveIgnored {
			nested[directory] = true
		}
	}
	prefix := ""
	switch {
	case top:
	case len(nested) == 0:
		return nil, nil, ErrThemeArchiveEmpty
	case len(nested) > 1:
		return nil, nil, ErrThemeArchiveSeveral
	default:
		for directory := range nested {
			prefix = directory + "/"
		}
	}
	var files []themeArchiveFile
	var others []string
	seen := map[string]bool{}
	assets, documents, manifest := 0, 0, false
	for _, entry := range reader.File {
		if strings.HasSuffix(entry.Name, "/") || entry.FileInfo().IsDir() {
			continue
		}
		path, inside := strings.CutPrefix(entry.Name, prefix)
		kind, name := model.ThemeArchiveOther, ""
		if inside {
			kind, name = model.ThemeArchiveEntry(path)
		} else if outside, _ := model.ThemeArchiveEntry(entry.Name); outside == model.ThemeArchiveIgnored {
			kind = model.ThemeArchiveIgnored
		}
		if kind == model.ThemeArchiveTemplate && !slices.Contains(themeTemplateFiles(), name) {
			// Other files of that directory are not read in a theme.
			kind = model.ThemeArchiveOther
		}
		switch {
		case kind == model.ThemeArchiveIgnored:
			continue
		case kind == model.ThemeArchiveOther, !entry.FileInfo().Mode().IsRegular():
			// A link is not followed, in an archive as in the directory.
			others = append(others, entry.Name)
			continue
		}
		// Two names that differ in case alone are one file where the
		// directory does not tell them apart.
		key := strings.ToLower(path)
		if seen[key] {
			return nil, nil, &ThemeArchiveFileError{Reason: ErrThemeArchiveTwice, File: path}
		}
		seen[key] = true
		switch kind {
		case model.ThemeArchiveManifest:
			manifest = true
		case model.ThemeArchiveAsset:
			// The assets are counted and named here, before any of them is
			// unpacked: the reader of the directory would say the same of
			// them once they were there.
			if assets++; assets > model.MaximumThemeAssets {
				return nil, nil, &ThemeArchiveProblem{Problem: ThemeProblem{File: model.ThemeAssetDirectory, Reason: ThemeProblemAssetLimit}}
			}
			if !model.IsThemeAssetName(name) {
				return nil, nil, &ThemeArchiveProblem{Problem: ThemeProblem{File: path, Reason: ThemeProblemAssetName}}
			}
		case model.ThemeArchiveDocument:
			if documents++; documents > model.MaximumThemeDocuments {
				return nil, nil, &ThemeArchiveFileError{Reason: ErrThemeArchiveDocument, File: path}
			}
		}
		files = append(files, themeArchiveFile{entry: entry, kind: kind, name: name, path: path})
	}
	if !manifest {
		return nil, nil, ErrThemeArchiveEmpty
	}
	return files, others, nil
}

// unpack returns the content of a file of the theme, up to the size a file
// of its kind can have. The size the archive states is not relied on: no
// more than the limit is read. A file that is larger is refused with the
// reason the reader of the themes directory gives for such a file.
func (file themeArchiveFile) unpack() ([]byte, error) {
	limit, tooLarge := int64(model.MaximumThemeDocumentBytes), error(&ThemeArchiveFileError{Reason: ErrThemeArchiveDocument, File: file.path})
	switch file.kind {
	case model.ThemeArchiveManifest:
		limit, tooLarge = MaximumThemeFileBytes, &ThemeArchiveProblem{Problem: ThemeProblem{File: file.path, Reason: ThemeProblemSize}}
	case model.ThemeArchiveStylesheet:
		limit, tooLarge = model.MaximumThemeStylesheetBytes, &ThemeArchiveProblem{Problem: ThemeProblem{File: file.path, Reason: ThemeProblemStyleSize}}
	case model.ThemeArchiveAsset:
		limit, tooLarge = model.MaximumThemeAssetBytes, &ThemeArchiveProblem{Problem: ThemeProblem{File: file.path, Reason: ThemeProblemAssetSize}}
	case model.ThemeArchiveTemplate:
		limit, tooLarge = MaximumThemeTemplateBytes, &ThemeArchiveProblem{Problem: ThemeProblem{File: file.path, Reason: ThemeProblemSize}}
	}
	if file.entry.UncompressedSize64 > uint64(limit) {
		return nil, tooLarge
	}
	content, err := file.entry.Open()
	if err != nil {
		return nil, &ThemeArchiveFileError{Reason: ErrThemeArchiveRead, File: file.path}
	}
	defer content.Close()
	raw, err := io.ReadAll(io.LimitReader(content, limit+1))
	switch {
	case int64(len(raw)) > limit:
		return nil, tooLarge
	case err != nil:
		return nil, &ThemeArchiveFileError{Reason: ErrThemeArchiveRead, File: file.path}
	case file.kind == model.ThemeArchiveDocument && !model.IsThemeDocumentContent(raw):
		return nil, tooLarge
	}
	return raw, nil
}

// StoreUserThemeArchive stores the theme of an archive in the themes
// directory, as the directory of the given theme name, and reads the
// directory again. It returns the entries of the archive that are no files
// of a theme and were left out.
//
// Only files of a theme are written, under names that are put together here:
// the theme file, the stylesheet, the images and fonts of the assets, the
// templates, and text documents. A path of the archive is never used as a path of the
// directory. The files are written into a hidden directory beside the
// themes, which a reading of the themes directory leaves alone, and are read
// there as a theme of the directory is read. A theme that is accepted is
// moved to its place; one that is not leaves nothing behind.
//
// A theme of the same name is replaced only if replace is set: a directory
// as a whole, and a theme file by being removed. Something of the name that
// is no theme is never replaced.
func StoreUserThemeArchive(name string, archive io.ReaderAt, size int64, replace bool) ([]string, error) {
	if AppFlags.ThemeSafeMode {
		return nil, ErrThemeStoreSafeMode
	}
	if !isUserThemeName(name) {
		return nil, ErrThemeStoreName
	}
	if size > model.MaximumThemeArchiveBytes {
		return nil, ErrThemeArchiveSize
	}
	// The reader can be told to refuse an archive with an entry that names
	// another place, such as ../a. It still hands the archive out with that
	// refusal, and such an entry is no file of a theme here in any case, so
	// the archive is read the same way whatever the reader is told.
	reader, err := zip.NewReader(archive, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, ErrThemeArchiveRead
	}
	if len(reader.File) > model.MaximumThemeArchiveEntries {
		return nil, ErrThemeArchiveEntries
	}
	files, others, err := themeArchiveFiles(reader)
	if err != nil {
		return nil, err
	}

	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	if err = os.Mkdir(UserThemeDirectory, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, errors.Join(ErrThemeStoreWrite, err)
	}
	root, err := os.OpenRoot(UserThemeDirectory)
	if err != nil {
		return nil, errors.Join(ErrThemeStoreWrite, err)
	}
	defer root.Close()

	// What has the name now. A directory that is a theme and a theme file
	// are themes of the name; anything else of the name is left as it is.
	file := name + userThemeExtension
	directory, directoryErr := root.Lstat(name)
	single, singleErr := root.Lstat(file)
	switch {
	case directoryErr == nil && (!directory.IsDir() || !isThemeDirectory(root, name)):
		return nil, ErrThemeStoreOccupied
	case singleErr == nil && !single.Mode().IsRegular():
		return nil, ErrThemeStoreOccupied
	case directoryErr != nil && !errors.Is(directoryErr, fs.ErrNotExist):
		return nil, errors.Join(ErrThemeStoreWrite, directoryErr)
	case singleErr != nil && !errors.Is(singleErr, fs.ErrNotExist):
		return nil, errors.Join(ErrThemeStoreWrite, singleErr)
	case (directoryErr == nil || singleErr == nil) && !replace:
		return nil, ErrThemeStoreExists
	case directoryErr != nil && singleErr != nil:
		// One more entry is only read while the directory holds fewer
		// entries than the number that are looked at.
		if count, countErr := countThemeFiles(root); countErr != nil {
			return nil, errors.Join(ErrThemeStoreWrite, countErr)
		} else if count >= MaximumUserThemeFiles {
			return nil, ErrThemeStoreFull
		}
	}

	staging, err := hiddenThemeName(".import-")
	if err != nil {
		return nil, errors.Join(ErrThemeStoreWrite, err)
	}
	if err = root.Mkdir(staging, 0700); err != nil {
		return nil, errors.Join(ErrThemeStoreWrite, err)
	}
	// Whatever is still under the hidden name at the end is removed.
	defer func() {
		if removeErr := root.RemoveAll(staging); removeErr != nil {
			logger.GetLogger().Warn("A directory an import was unpacked into could not be removed from the themes directory", "directory", staging, "reason", removeErr.Error())
		}
	}()
	if err = unpackThemeArchive(root, staging, files); err != nil {
		return nil, err
	}

	// The files are read as a theme of the directory is, in the place they
	// were written to. The first remark about them is the reason the
	// archive is refused, with the path of the file in the theme.
	scan := newUserThemeScan()
	scan.readTheme(root, staging, filepath.Join(staging, userThemeManifest), staging+"/"+userThemeManifest, true)
	if len(scan.problems) > 0 {
		problem := scan.problems[0]
		problem.File = strings.TrimPrefix(problem.File, staging+"/")
		return nil, &ThemeArchiveProblem{Problem: problem}
	}
	// A template that cannot be read is refused like any other file. One
	// that is read and not accepted is for the theme page to list: the
	// theme is in use without it, as in the directory, and whether it is
	// accepted can depend on the themes below.
	if len(scan.templateRemarks) > 0 {
		problem := scan.templateRemarks[0]
		problem.File = strings.TrimPrefix(problem.File, staging+"/")
		return nil, &ThemeArchiveProblem{Problem: problem}
	}
	if candidate := scan.candidates[staging]; candidate.file.Extends != "" {
		if err = checkThemeBase(name, candidate.file); err != nil {
			return nil, err
		}
	}

	// The theme takes its place. A theme of the name that is a directory is
	// moved away first and removed afterwards; pages that notice the change
	// in between wait for this reading of the directory.
	replaced := ""
	if directoryErr == nil {
		if replaced, err = hiddenThemeName(".replaced-"); err != nil {
			return nil, errors.Join(ErrThemeStoreWrite, err)
		}
		if err = root.Rename(name, replaced); err != nil {
			return nil, errors.Join(ErrThemeStoreWrite, err)
		}
	}
	if err = root.Rename(staging, name); err != nil {
		if replaced != "" {
			// The theme that was there is put back.
			err = errors.Join(err, root.Rename(replaced, name))
		}
		return nil, errors.Join(ErrThemeStoreWrite, err)
	}
	if replaced != "" {
		if removeErr := root.RemoveAll(replaced); removeErr != nil {
			logger.GetLogger().Warn("The theme that was replaced could not be removed from the themes directory", "directory", replaced, "reason", removeErr.Error())
		}
	}
	if singleErr == nil {
		// The directory is the theme of the name from now on, and the file
		// of the name would be listed as one that is not read.
		if removeErr := root.Remove(file); removeErr != nil {
			logger.GetLogger().Warn("The theme file that was replaced could not be removed from the themes directory", "file", file, "reason", removeErr.Error())
		}
	}
	syncThemesDirectory(root)
	reloadUserThemes()
	return others, nil
}

// hiddenThemeName returns a name for a directory beside the themes that a
// reading of the themes directory leaves alone.
func hiddenThemeName(prefix string) (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random[:]), nil
}

// unpackThemeArchive writes the files of a theme into a directory of the
// themes directory. Each is written under the name it has in a theme, which
// is one of a few fixed names or a name that was checked.
func unpackThemeArchive(root *os.Root, directory string, files []themeArchiveFile) error {
	assets, templates := false, false
	for _, file := range files {
		content, err := file.unpack()
		if err != nil {
			return err
		}
		target := ""
		switch file.kind {
		case model.ThemeArchiveManifest:
			target = filepath.Join(directory, userThemeManifest)
		case model.ThemeArchiveStylesheet:
			target = filepath.Join(directory, userThemeStylesheet)
		case model.ThemeArchiveDocument:
			target = filepath.Join(directory, file.name)
		case model.ThemeArchiveAsset:
			if !assets {
				if err = root.Mkdir(filepath.Join(directory, model.ThemeAssetDirectory), 0700); err != nil {
					return errors.Join(ErrThemeStoreWrite, err)
				}
				assets = true
			}
			target = filepath.Join(directory, model.ThemeAssetDirectory, file.name)
		case model.ThemeArchiveTemplate:
			if !templates {
				if err = root.Mkdir(filepath.Join(directory, model.ThemeTemplateDirectory), 0700); err != nil {
					return errors.Join(ErrThemeStoreWrite, err)
				}
				templates = true
			}
			target = filepath.Join(directory, model.ThemeTemplateDirectory, file.name)
		}
		handle, err := root.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return errors.Join(ErrThemeStoreWrite, err)
		}
		_, err = handle.Write(content)
		if err == nil {
			err = handle.Sync()
		}
		if closeErr := handle.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return errors.Join(ErrThemeStoreWrite, err)
		}
	}
	return nil
}

// boundedArchive holds an archive that is being written, up to the size an
// archive of a theme can have.
type boundedArchive struct {
	bytes.Buffer
}

func (archive *boundedArchive) Write(part []byte) (int, error) {
	if archive.Len()+len(part) > model.MaximumThemeArchiveBytes {
		return 0, ErrThemeArchiveSize
	}
	return archive.Buffer.Write(part)
}

// ExportThemeArchive returns a theme that is a directory as an archive, with
// the file name to offer for it. The archive holds a directory of the name
// of the theme with the files an import takes: the theme file as it is, the
// stylesheet and the assets as they were checked when the themes directory
// was read, and the text documents beside the theme file. A theme that is
// based on another is handed out by itself.
//
// It reports ErrThemeArchiveUnknown for a name that is not a theme in use
// that is a directory, ErrThemeArchiveSize for a theme whose archive would
// be larger than an archive that is taken in, and ErrThemeArchiveRead where
// a file is no longer the one that was read.
func ExportThemeArchive(name string) (string, []byte, error) {
	// The archive is put together while no theme is stored and the themes
	// directory is not read, so that its files are those of one state of
	// the theme.
	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	themeMu.RLock()
	definition, known := userThemeState.themes[name]
	themeMu.RUnlock()
	if !known || !definition.Directory {
		return "", nil, ErrThemeArchiveUnknown
	}
	root, err := os.OpenRoot(UserThemeDirectory)
	if err != nil {
		return "", nil, ErrThemeArchiveRead
	}
	defer root.Close()

	var archive boundedArchive
	writer := zip.NewWriter(&archive)
	add := func(path string, content []byte, modified time.Time) error {
		entry, createErr := writer.CreateHeader(&zip.FileHeader{Name: name + "/" + path, Method: zip.Deflate, Modified: modified})
		if createErr != nil {
			return createErr
		}
		_, writeErr := entry.Write(content)
		return writeErr
	}
	modified := func(path string) time.Time {
		if entry, statErr := root.Lstat(filepath.Join(name, path)); statErr == nil {
			return entry.ModTime()
		}
		return time.Time{}
	}
	manifest, reason := readThemeDirectoryFile(root, filepath.Join(name, userThemeManifest), MaximumThemeFileBytes)
	if reason != "" {
		return "", nil, ErrThemeArchiveRead
	}
	if err = add(userThemeManifest, manifest, modified(userThemeManifest)); err != nil {
		return "", nil, exportFailure(err)
	}
	if definition.Stylesheet != "" {
		if err = add(userThemeStylesheet, []byte(definition.Stylesheet), modified(userThemeStylesheet)); err != nil {
			return "", nil, exportFailure(err)
		}
	}
	for _, asset := range definition.Assets {
		handle, _, opened := OpenThemeAsset(name, definition.Hash, asset.Name)
		if !opened {
			return "", nil, ErrThemeArchiveRead
		}
		content, readErr := io.ReadAll(io.LimitReader(handle, model.MaximumThemeAssetBytes+1))
		_ = handle.Close()
		if readErr != nil || len(content) > model.MaximumThemeAssetBytes {
			return "", nil, ErrThemeArchiveRead
		}
		if err = add(model.ThemeAssetDirectory+"/"+asset.Name, content, asset.ModTime); err != nil {
			return "", nil, exportFailure(err)
		}
	}
	// The templates are those a reading of the directory reads, accepted
	// or not: an import checks them again.
	templates, _ := readThemeTemplates(root, name)
	for _, template := range templates {
		path := model.ThemeTemplateDirectory + "/" + template.Name
		if err = add(path, []byte(template.Source), modified(path)); err != nil {
			return "", nil, exportFailure(err)
		}
	}
	for _, document := range themeDocuments(root, name) {
		if err = add(document.name, document.content, modified(document.name)); err != nil {
			return "", nil, exportFailure(err)
		}
	}
	if err = writer.Close(); err != nil {
		return "", nil, exportFailure(err)
	}
	return name + ".zip", archive.Bytes(), nil
}

// exportFailure names why an archive could not be written: it grew beyond
// the size an archive can have, or it could not be put together.
func exportFailure(err error) error {
	if errors.Is(err, ErrThemeArchiveSize) {
		return ErrThemeArchiveSize
	}
	return ErrThemeArchiveRead
}

// themeDocument is a text document beside the theme file of a theme.
type themeDocument struct {
	name    string
	content []byte
}

// themeDocuments reads the text documents of a theme that is a directory, in
// the order of their names: regular files beside the theme file with the
// name and the content of a text document, as many as travel with a theme.
// A file that is not one of them is left where it is, as the reading of the
// themes directory leaves it.
func themeDocuments(root *os.Root, theme string) []themeDocument {
	entries, err := fs.ReadDir(root.FS(), theme)
	if err != nil {
		return nil
	}
	var documents []themeDocument
	for _, entry := range entries {
		if len(documents) == model.MaximumThemeDocuments {
			break
		}
		if !entry.Type().IsRegular() || !model.IsThemeDocumentName(entry.Name()) {
			continue
		}
		content, reason := readThemeDirectoryFile(root, filepath.Join(theme, entry.Name()), model.MaximumThemeDocumentBytes)
		if reason != "" || !model.IsThemeDocumentContent(content) {
			continue
		}
		documents = append(documents, themeDocument{name: entry.Name(), content: content})
	}
	return documents
}
