package define

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// archiveEntry is an entry of an archive a test packs. mode is the mode the
// archive states for it where that is not the one of a regular file, and
// raw marks an entry that is written as it is, with what the header says.
type archiveEntry struct {
	name    string
	content string
	mode    fs.FileMode
	raw     *zip.FileHeader
}

// packArchive packs entries into a ZIP file, in their order.
func packArchive(t *testing.T, entries ...archiveEntry) []byte {
	t.Helper()
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for _, entry := range entries {
		if entry.raw != nil {
			part, err := writer.CreateRaw(entry.raw)
			require.NoError(t, err)
			_, err = part.Write([]byte(entry.content))
			require.NoError(t, err)
			continue
		}
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		require.NoError(t, err)
		_, err = part.Write([]byte(entry.content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return packed.Bytes()
}

// The files of the theme the archives of these tests hold.
const (
	archivedThemeFile  = userThemeColors + "  radius: 10px\nname: Zelda\nbackground:\n  image: assets/paper.png\n"
	archivedStylesheet = ".app-item { background-image: url(assets/paper.png); font-family: Brand; }\n@font-face { font-family: Brand; src: url(assets/brand.woff2); }\n"
	archivedLicense    = "Permission is hereby granted, free of charge.\n"
)

// zeldaArchive is the theme of these tests as an archive, with its files
// under a prefix.
func zeldaArchive(prefix string, more ...archiveEntry) []archiveEntry {
	return append([]archiveEntry{
		{name: prefix + "theme.yml", content: archivedThemeFile},
		{name: prefix + "theme.css", content: archivedStylesheet},
		{name: prefix + "assets/paper.png", content: assetPNG + "paper"},
		{name: prefix + "assets/brand.woff2", content: assetWOFF2 + "brand"},
		{name: prefix + "LICENSE.txt", content: archivedLicense},
	}, more...)
}

func storeArchive(name string, packed []byte, replace bool) ([]string, error) {
	return StoreUserThemeArchive(name, bytes.NewReader(packed), int64(len(packed)), replace)
}

// themeDirectoryFiles lists the files of a directory of the themes
// directory with their content, by their paths in it with slashes.
func themeDirectoryFiles(t *testing.T, name string) map[string]string {
	t.Helper()
	files := map[string]string{}
	base := filepath.Join(UserThemeDirectory, name)
	require.NoError(t, filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		relative, _ := filepath.Rel(base, path)
		files[filepath.ToSlash(relative)] = string(content)
		return readErr
	}))
	return files
}

// allThemeEntries lists every entry of the themes directory, the hidden ones
// too: an import leaves nothing behind beside the themes.
func allThemeEntries(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(UserThemeDirectory)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

var zeldaFiles = map[string]string{
	"theme.yml": archivedThemeFile, "theme.css": archivedStylesheet, "LICENSE.txt": archivedLicense,
	"assets/paper.png": assetPNG + "paper", "assets/brand.woff2": assetWOFF2 + "brand",
}

// An archive is stored as the directory of a theme: the theme file, the
// stylesheet, the assets and the text documents. Its files stand at the top
// or in one directory of any name, and the name of the theme is the one the
// archive is stored under. What is no file of a theme is left out and named.
func TestStoreUserThemeArchive(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for name, entries := range map[string][]archiveEntry{
		"at-the-top":     zeldaArchive(""),
		"in-a-directory": zeldaArchive("flare-theme-zelda-main/"),
	} {
		others, err := storeArchive(name, packArchive(t, entries...), false)
		require.NoError(t, err, name)
		require.Empty(t, others, name)
		require.Equal(t, zeldaFiles, themeDirectoryFiles(t, name), name)
		theme := userTheme(t, name)
		require.True(t, theme.Directory, name)
		require.Equal(t, "Zelda", theme.DisplayName("en"), name)
		require.Equal(t, archivedStylesheet, theme.Stylesheet, name)
		require.Equal(t, "paper.png", theme.BackgroundImage, name)
		content, _, served := readThemeAsset(t, name, theme.Hash, "paper.png")
		require.True(t, served, name)
		require.Equal(t, assetPNG+"paper", content, name)
	}
	require.Equal(t, []string{"at-the-top", "in-a-directory"}, allThemeEntries(t))
	_, problems, _ := UserThemes()
	require.Empty(t, problems)

	// Entries that are no files of a theme are not stored, and are returned
	// by the names they have in the archive. Hidden entries, those a system
	// adds and directories are left out without a word.
	others, err := storeArchive("with-more", packArchive(t, zeldaArchive("zelda/",
		archiveEntry{name: "zelda/"},
		archiveEntry{name: "zelda/assets/"},
		archiveEntry{name: "zelda/.DS_Store", content: "finder"},
		archiveEntry{name: "__MACOSX/zelda/._theme.yml", content: "resource fork"},
		archiveEntry{name: "zelda/notes.html", content: "<p>notes</p>"},
		archiveEntry{name: "zelda/src/design.psd", content: "layers"},
		archiveEntry{name: "zelda/assets/readme.txt", content: "about the image"},
		archiveEntry{name: "zelda/assets/vector.svg", content: "<svg/>"},
		archiveEntry{name: "zelda/preview.png", content: assetPNG},
		archiveEntry{name: "beside.txt", content: "outside the directory of the theme"},
		archiveEntry{name: "zelda/assets/link.png", content: "paper.png", mode: fs.ModeSymlink | 0777},
		archiveEntry{name: "zelda/NOTICE", content: "A notice.\n"},
	)...), false)
	require.NoError(t, err)
	require.Equal(t, []string{
		"zelda/notes.html", "zelda/src/design.psd", "zelda/assets/readme.txt", "zelda/assets/vector.svg", "zelda/preview.png",
		"beside.txt", "zelda/assets/link.png",
	}, others)
	stored := themeDirectoryFiles(t, "with-more")
	require.Equal(t, "A notice.\n", stored["NOTICE"])
	delete(stored, "NOTICE")
	require.Equal(t, zeldaFiles, stored)

	// A theme file at the top is the theme; one in a directory beside it is
	// an entry like any other.
	others, err = storeArchive("top-wins", packArchive(t,
		archiveEntry{name: "theme.yml", content: userThemeColors + "name: Top\n"},
		archiveEntry{name: "other/theme.yml", content: userThemeColors + "name: Other\n"},
		archiveEntry{name: "other/theme.css", content: archivedStylesheet},
	), false)
	require.NoError(t, err)
	require.Equal(t, []string{"other/theme.yml", "other/theme.css"}, others)
	require.Equal(t, "Top", userTheme(t, "top-wins").DisplayName("en"))
	require.Equal(t, map[string]string{"theme.yml": userThemeColors + "name: Top\n"}, themeDirectoryFiles(t, "top-wins"))

	// The same holds for a theme file deeper in another directory: the
	// archive is the one theme that stands at the top of a directory.
	others, err = storeArchive("one-directory", packArchive(t,
		archiveEntry{name: "zelda/theme.yml", content: userThemeColors + "name: Inner\n"},
		archiveEntry{name: "examples/other/theme.yml", content: userThemeColors + "name: Example\n"},
		archiveEntry{name: "zelda/more/theme.yml", content: userThemeColors + "name: Deeper\n"},
	), false)
	require.NoError(t, err)
	require.Equal(t, []string{"examples/other/theme.yml", "zelda/more/theme.yml"}, others)
	require.Equal(t, "Inner", userTheme(t, "one-directory").DisplayName("en"))

	// A theme that is based on another is stored if that theme is in use.
	_, err = storeArchive("night", packArchive(t, archiveEntry{name: "theme.yml", content: "contract: 1\nextends: at-the-top\ntokens:\n  radius: 2px\n"}), false)
	require.NoError(t, err)
	require.Equal(t, "at-the-top", userTheme(t, "night").Extends)
	require.Equal(t, []string{"at-the-top", "in-a-directory", "night", "one-directory", "top-wins", "with-more"}, allThemeEntries(t))
}

// Entries that are named to reach another place are no files of a theme:
// they are left out and named like any other, and nothing is written outside
// the directory of the theme. That does not depend on what the reader of
// archives is told to do with such names.
func TestStoreUserThemeArchiveWithEntriesThatNameOtherPlaces(t *testing.T) {
	entries := []archiveEntry{
		{name: "theme.yml", content: userThemeColors},
		{name: "../evil.yml", content: userThemeColors},
		{name: "../../evil.txt", content: "outside"},
		{name: "assets/../../evil.png", content: assetPNG},
		{name: "/tmp/evil.txt", content: "absolute"},
		{name: "assets/../theme.css", content: "@import 'a.css';"},
		{name: "assets/sub/evil.png", content: assetPNG},
		{name: "C:/evil.txt", content: "a drive"},
	}
	named := []string{"../evil.yml", "../../evil.txt", "assets/../../evil.png", "/tmp/evil.txt", "assets/../theme.css", "assets/sub/evil.png", "C:/evil.txt"}
	for _, setting := range []string{"zipinsecurepath=1", "zipinsecurepath=0"} {
		t.Run(setting, func(t *testing.T) {
			t.Setenv("GODEBUG", setting)
			userThemeDirectory(t, "blackboard")
			outside, err := os.ReadDir(".")
			require.NoError(t, err)
			others, err := storeArchive("zelda", packArchive(t, entries...), false)
			require.NoError(t, err)
			require.Equal(t, named, others)
			require.Equal(t, map[string]string{"theme.yml": userThemeColors}, themeDirectoryFiles(t, "zelda"))
			require.Equal(t, []string{"zelda"}, allThemeEntries(t))
			after, err := os.ReadDir(".")
			require.NoError(t, err)
			require.Len(t, after, len(outside))
			_, err = os.Stat(filepath.Join("..", "evil.yml"))
			require.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}

// An archive that is not the archive of a usable theme is refused with the
// reason, and nothing of it stays in the themes directory.
func TestStoreUserThemeArchiveRefusals(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "plain.yml", userThemeColors)
	ReloadUserThemes()
	theme := archiveEntry{name: "theme.yml", content: userThemeColors}
	many := make([]archiveEntry, 0, model.MaximumThemeArchiveEntries+1)
	for i := range model.MaximumThemeArchiveEntries {
		many = append(many, archiveEntry{name: fmt.Sprintf("notes/%03d.html", i), content: "x"})
	}
	assets := []archiveEntry{theme}
	for i := range model.MaximumThemeAssets + 1 {
		assets = append(assets, archiveEntry{name: fmt.Sprintf("assets/a%02d.png", i), content: assetPNG})
	}
	documents := []archiveEntry{theme}
	for i := range model.MaximumThemeDocuments + 1 {
		documents = append(documents, archiveEntry{name: fmt.Sprintf("note-%d.txt", i), content: "text"})
	}
	// An entry that is packed in a way that is not read, and one whose
	// content is not what the archive says it is.
	unreadable := &zip.FileHeader{Name: "theme.yml", Method: 99, CompressedSize64: uint64(len(userThemeColors)), UncompressedSize64: uint64(len(userThemeColors)), CRC32: crc32.ChecksumIEEE([]byte(userThemeColors))}
	damaged := &zip.FileHeader{Name: "theme.yml", Method: zip.Store, CompressedSize64: uint64(len(userThemeColors)), UncompressedSize64: uint64(len(userThemeColors)), CRC32: 1}

	type refusal struct {
		packed []byte
		// One of: a reason by itself, a reason for a file of the archive, a
		// file the checks of a theme do not accept, a theme file that is
		// refused for the theme it is based on.
		reason  error
		file    string
		problem ThemeProblem
	}
	for name, tc := range map[string]refusal{
		"not an archive":                     {packed: []byte("contract: 1\n" + strings.Repeat("tokens\n", 40)), reason: ErrThemeArchiveRead},
		"nothing at all":                     {packed: nil, reason: ErrThemeArchiveRead},
		"an empty archive":                   {packed: packArchive(t), reason: ErrThemeArchiveEmpty},
		"no theme file":                      {packed: packArchive(t, archiveEntry{name: "theme.css", content: archivedStylesheet}), reason: ErrThemeArchiveEmpty},
		"a theme file too deep":              {packed: packArchive(t, archiveEntry{name: "a/b/theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file in a hidden directory": {packed: packArchive(t, archiveEntry{name: ".git/theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file above the archive":     {packed: packArchive(t, archiveEntry{name: "../theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file at an absolute path":   {packed: packArchive(t, archiveEntry{name: "/theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file in the directory .":    {packed: packArchive(t, archiveEntry{name: "./theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file with a backslash":      {packed: packArchive(t, archiveEntry{name: "zelda\\theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file of another case":       {packed: packArchive(t, archiveEntry{name: "Theme.yml", content: userThemeColors}), reason: ErrThemeArchiveEmpty},
		"a theme file that is a link":        {packed: packArchive(t, archiveEntry{name: "theme.yml", content: "../config.yml", mode: fs.ModeSymlink | 0777}), reason: ErrThemeArchiveEmpty},
		"a theme file that is a directory":   {packed: packArchive(t, archiveEntry{name: "theme.yml/"}), reason: ErrThemeArchiveEmpty},
		"two themes":                         {packed: packArchive(t, archiveEntry{name: "a/theme.yml", content: userThemeColors}, archiveEntry{name: "b/theme.yml", content: userThemeColors}), reason: ErrThemeArchiveSeveral},
		"more entries than are looked at":    {packed: packArchive(t, append(many, theme)...), reason: ErrThemeArchiveEntries},
		"the theme file twice":               {packed: packArchive(t, theme, theme), reason: ErrThemeArchiveTwice, file: "theme.yml"},
		"an asset twice, in two cases":       {packed: packArchive(t, theme, archiveEntry{name: "assets/A.png", content: assetPNG}, archiveEntry{name: "assets/a.png", content: assetPNG}), reason: ErrThemeArchiveTwice, file: "assets/a.png"},
		"a theme file that cannot be read":   {packed: packArchive(t, archiveEntry{raw: unreadable, content: userThemeColors}), reason: ErrThemeArchiveRead, file: "theme.yml"},
		"a theme file that is damaged":       {packed: packArchive(t, archiveEntry{raw: damaged, content: userThemeColors}), reason: ErrThemeArchiveRead, file: "theme.yml"},
		"one document too many":              {packed: packArchive(t, documents...), reason: ErrThemeArchiveDocument, file: fmt.Sprintf("note-%d.txt", model.MaximumThemeDocuments)},
		"a document that is no text":         {packed: packArchive(t, theme, archiveEntry{name: "LICENSE", content: "a\x00b"}), reason: ErrThemeArchiveDocument, file: "LICENSE"},
		"a document that is too large":       {packed: packArchive(t, theme, archiveEntry{name: "notes.md", content: strings.Repeat("a", model.MaximumThemeDocumentBytes+1)}), reason: ErrThemeArchiveDocument, file: "notes.md"},

		// The files of the theme get the checks of a theme of the directory.
		"a theme file that is too large":   {packed: packArchive(t, archiveEntry{name: "theme.yml", content: userThemeColors + "#" + strings.Repeat("x", MaximumThemeFileBytes)}), problem: ThemeProblem{File: "theme.yml", Reason: ThemeProblemSize}},
		"a theme file with a bad value":    {packed: packArchive(t, archiveEntry{name: "theme.yml", content: userThemeColors + "  radius: 4em\n"}), problem: ThemeProblem{File: "theme.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"}},
		"a stylesheet that is too large":   {packed: packArchive(t, theme, archiveEntry{name: "theme.css", content: "/*" + strings.Repeat("x", model.MaximumThemeStylesheetBytes) + "*/"}), problem: ThemeProblem{File: "theme.css", Reason: ThemeProblemStyleSize}},
		"a stylesheet that loads a file":   {packed: packArchive(t, theme, archiveEntry{name: "theme.css", content: "@import 'a.css';"}), problem: ThemeProblem{File: "theme.css", Reason: ThemeProblemStyleRule, Subject: "line 1: @import"}},
		"a stylesheet without its asset":   {packed: packArchive(t, theme, archiveEntry{name: "theme.css", content: archivedStylesheet}), problem: ThemeProblem{File: "theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 1: url(assets/paper.png)"}},
		"an asset that is too large":       {packed: packArchive(t, theme, archiveEntry{name: "assets/big.png", content: assetPNG + strings.Repeat("x", model.MaximumThemeAssetBytes)}), problem: ThemeProblem{File: "assets/big.png", Reason: ThemeProblemAssetSize}},
		"an asset with a name not taken":   {packed: packArchive(t, theme, archiveEntry{name: "assets/my photo.png", content: assetPNG}), problem: ThemeProblem{File: "assets/my photo.png", Reason: ThemeProblemAssetName}},
		"an asset with a name no file has": {packed: packArchive(t, theme, archiveEntry{name: "assets/a\x00b.png", content: assetPNG}), problem: ThemeProblem{File: "assets/a\x00b.png", Reason: ThemeProblemAssetName}},
		"an asset that is something else":  {packed: packArchive(t, theme, archiveEntry{name: "assets/fake.png", content: "<svg onload=alert(1)>"}), problem: ThemeProblem{File: "assets/fake.png", Reason: ThemeProblemAssetContent}},
		"more assets than a theme has":     {packed: packArchive(t, assets...), problem: ThemeProblem{File: "assets", Reason: ThemeProblemAssetLimit}},
		"an image that is not there":       {packed: packArchive(t, archiveEntry{name: "theme.yml", content: archivedThemeFile}), problem: ThemeProblem{File: "theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/paper.png"}},
		"a theme below that is not there":  {packed: packArchive(t, archiveEntry{name: "theme.yml", content: "contract: 1\nextends: nowhere\n"}), reason: model.ErrThemeBase},
		"colors that do not go together":   {packed: packArchive(t, archiveEntry{name: "theme.yml", content: "contract: 1\nextends: plain\ntokens:\n  primary: \"#1b2a22\"\n"}), reason: model.ErrPaletteContrast},
	} {
		others, err := storeArchive("zelda", tc.packed, false)
		require.Empty(t, others, name)
		var entry *ThemeArchiveFileError
		var part *ThemeArchiveProblem
		switch {
		case tc.problem.Reason != "":
			require.ErrorAs(t, err, &part, name)
			require.Equal(t, tc.problem, part.Problem, name)
			require.Contains(t, err.Error(), tc.problem.File+": "+tc.problem.Reason, name)
		case tc.file != "":
			require.ErrorAs(t, err, &entry, name)
			require.ErrorIs(t, err, tc.reason, name)
			require.Equal(t, tc.file, entry.File, name)
			require.Equal(t, tc.reason.Error()+": "+tc.file, err.Error(), name)
		default:
			require.ErrorIs(t, err, tc.reason, name)
			require.False(t, strings.Contains(fmt.Sprintf("%T", err), "ThemeArchiveFileError"), name)
		}
		require.Equal(t, []string{"plain.yml"}, allThemeEntries(t), name)
		require.False(t, IsKnownTheme("zelda"), name)
	}

	// A name that cannot be the name of a theme, an archive that is larger
	// than one can be, and an instance in safe mode.
	valid := packArchive(t, theme)
	for _, name := range []string{"Zelda", "my theme", "../zelda", "zelda.zip", "tron", "custom", ""} {
		_, err := storeArchive(name, valid, true)
		require.ErrorIs(t, err, ErrThemeStoreName, name)
	}
	_, err := StoreUserThemeArchive("zelda", bytes.NewReader(valid), model.MaximumThemeArchiveBytes+1, false)
	require.ErrorIs(t, err, ErrThemeArchiveSize)
	AppFlags.ThemeSafeMode = true
	_, err = storeArchive("zelda", valid, false)
	AppFlags.ThemeSafeMode = false
	require.ErrorIs(t, err, ErrThemeStoreSafeMode)
	require.Equal(t, []string{"plain.yml"}, allThemeEntries(t))

	// The same archive of a usable theme is taken.
	_, err = storeArchive("zelda", valid, false)
	require.NoError(t, err)
	require.True(t, IsUserTheme("zelda"))
}

// A theme of the name is replaced only where that is asked for: a directory
// as a whole, and a theme file by being removed. Something of the name that
// is no theme is never replaced.
func TestStoreUserThemeArchiveOverWhatHasTheName(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	packed := packArchive(t, zeldaArchive("")...)

	// A directory that is a theme, with files the archive does not hold.
	writeThemeDirectory(t, "zelda", userThemeColors+"name: Before\n")
	writeThemeAsset(t, "zelda", "old.png", assetPNG)
	writeUserTheme(t, filepath.Join("zelda", "notes.html"), "notes")
	ReloadUserThemes()
	_, err := storeArchive("zelda", packed, false)
	require.ErrorIs(t, err, ErrThemeStoreExists)
	require.Equal(t, "Before", userTheme(t, "zelda").DisplayName("en"))
	_, err = storeArchive("zelda", packed, true)
	require.NoError(t, err)
	require.Equal(t, zeldaFiles, themeDirectoryFiles(t, "zelda"), "the directory is replaced as a whole")
	require.Equal(t, "Zelda", userTheme(t, "zelda").DisplayName("en"))
	require.Equal(t, []string{"zelda"}, allThemeEntries(t))

	// A theme file of the name.
	writeUserTheme(t, "single.yml", userThemeColors+"name: Before\n")
	ReloadUserThemes()
	_, err = storeArchive("single", packed, false)
	require.ErrorIs(t, err, ErrThemeStoreExists)
	require.False(t, userTheme(t, "single").Directory)
	_, err = storeArchive("single", packed, true)
	require.NoError(t, err)
	require.True(t, userTheme(t, "single").Directory)
	require.Equal(t, []string{"single", "zelda"}, allThemeEntries(t), "the theme file is removed")
	_, problems, _ := UserThemes()
	require.Empty(t, problems)

	// What has the name and is no theme stays as it is.
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, "folder"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, "folder", "kept.txt"), []byte("kept"), 0600))
	require.NoError(t, os.Symlink("zelda", filepath.Join(UserThemeDirectory, "linked")))
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, "bare"), []byte("a file without an extension"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, "odd.yml"), 0700))
	before := allThemeEntries(t)
	for _, name := range []string{"folder", "linked", "bare", "odd"} {
		for _, replace := range []bool{false, true} {
			_, err = storeArchive(name, packed, replace)
			require.ErrorIs(t, err, ErrThemeStoreOccupied, "%s, replace %v", name, replace)
		}
	}
	require.Equal(t, before, allThemeEntries(t))
	kept, err := os.ReadFile(filepath.Join(UserThemeDirectory, "folder", "kept.txt"))
	require.NoError(t, err)
	require.Equal(t, "kept", string(kept))
}

// One more theme is only stored while the themes directory holds fewer
// entries than are looked at. A theme that is replaced takes no place.
func TestStoreUserThemeArchiveIntoAFullDirectory(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for i := range MaximumUserThemeFiles - 1 {
		writeUserTheme(t, fmt.Sprintf("f%02d.yml", i), userThemeColors)
	}
	writeThemeDirectory(t, "zelda", userThemeColors)
	ReloadUserThemes()
	packed := packArchive(t, zeldaArchive("")...)
	_, err := storeArchive("one-more", packed, false)
	require.ErrorIs(t, err, ErrThemeStoreFull)
	require.Len(t, allThemeEntries(t), MaximumUserThemeFiles)
	_, err = storeArchive("zelda", packed, true)
	require.NoError(t, err)
	require.Equal(t, "Zelda", userTheme(t, "zelda").DisplayName("en"))
	require.Len(t, allThemeEntries(t), MaximumUserThemeFiles)
}

// A theme that is a directory is handed out as an archive of the files an
// import takes, in a directory of its name, and what was handed out is taken
// in again as the same theme.
func TestExportThemeArchive(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "zelda", archivedThemeFile)
	writeThemeStylesheet(t, "zelda", archivedStylesheet)
	writeThemeAsset(t, "zelda", "paper.png", assetPNG+"paper")
	writeThemeAsset(t, "zelda", "brand.woff2", assetWOFF2+"brand")
	// Text documents travel with the theme. Other files of the directory do
	// not: what is not a file of a theme, a document that is no text or too
	// large, and what is in the assets without being one.
	for file, content := range map[string]string{
		"LICENSE.txt": archivedLicense, "NOTICE": "A notice.\n", "README.md": "# Zelda\n",
		"notes.html": "<p>notes</p>", ".hidden.txt": "hidden", "binary.txt": "a\x00b", "theme.yml~": "a backup",
		"large.md": strings.Repeat("a", model.MaximumThemeDocumentBytes+1), filepath.Join("assets", "about.txt"): "about the image",
	} {
		writeUserTheme(t, filepath.Join("zelda", file), content)
	}
	require.NoError(t, os.Symlink("LICENSE.txt", filepath.Join(UserThemeDirectory, "zelda", "linked.txt")))
	writeUserTheme(t, "plain.yml", userThemeColors)
	ReloadUserThemes()

	unpacked := func(packed []byte) ([]string, map[string]string) {
		t.Helper()
		reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
		require.NoError(t, err)
		names := make([]string, 0, len(reader.File))
		contents := map[string]string{}
		for _, entry := range reader.File {
			require.True(t, entry.FileInfo().Mode().IsRegular(), entry.Name)
			handle, openErr := entry.Open()
			require.NoError(t, openErr)
			content, readErr := io.ReadAll(handle)
			require.NoError(t, readErr)
			require.NoError(t, handle.Close())
			names = append(names, entry.Name)
			contents[entry.Name] = string(content)
		}
		return names, contents
	}
	file, packed, err := ExportThemeArchive("zelda")
	require.NoError(t, err)
	require.Equal(t, "zelda.zip", file)
	names, contents := unpacked(packed)
	require.Equal(t, []string{
		"zelda/theme.yml", "zelda/theme.css", "zelda/assets/brand.woff2", "zelda/assets/paper.png",
		"zelda/LICENSE.txt", "zelda/NOTICE", "zelda/README.md",
	}, names)
	require.Equal(t, map[string]string{
		"zelda/theme.yml": archivedThemeFile, "zelda/theme.css": archivedStylesheet,
		"zelda/assets/brand.woff2": assetWOFF2 + "brand", "zelda/assets/paper.png": assetPNG + "paper",
		"zelda/LICENSE.txt": archivedLicense, "zelda/NOTICE": "A notice.\n", "zelda/README.md": "# Zelda\n",
	}, contents)

	// What was handed out is taken in under another name, with nothing left
	// out, and is the same theme.
	others, err := storeArchive("hyrule", packed, false)
	require.NoError(t, err)
	require.Empty(t, others)
	want := allFiles(zeldaFiles, map[string]string{"NOTICE": "A notice.\n", "README.md": "# Zelda\n"})
	require.Equal(t, want, themeDirectoryFiles(t, "hyrule"))
	zelda, hyrule := userTheme(t, "zelda"), userTheme(t, "hyrule")
	require.Equal(t, zelda.Hash != "", hyrule.Hash != "")
	require.Equal(t, zelda.Stylesheet, hyrule.Stylesheet)
	require.Equal(t, zelda.Colors, hyrule.Colors)
	again, repacked, err := ExportThemeArchive("hyrule")
	require.NoError(t, err)
	require.Equal(t, "hyrule.zip", again)
	_, recontents := unpacked(repacked)
	for name, content := range contents {
		require.Equal(t, content, recontents[strings.Replace(name, "zelda/", "hyrule/", 1)], name)
	}

	// No more documents travel than an import takes.
	for i := range model.MaximumThemeDocuments + 2 {
		writeUserTheme(t, filepath.Join("zelda", fmt.Sprintf("extra-%d.txt", i)), "text")
	}
	_, packed, err = ExportThemeArchive("zelda")
	require.NoError(t, err)
	names, _ = unpacked(packed)
	require.Len(t, names, 4+model.MaximumThemeDocuments)
	_, err = storeArchive("capped", packed, false)
	require.NoError(t, err)

	// A theme that is based on another is handed out by itself.
	writeThemeDirectory(t, "night", "contract: 1\nextends: zelda\ntokens:\n  radius: 2px\n")
	ReloadUserThemes()
	_, packed, err = ExportThemeArchive("night")
	require.NoError(t, err)
	names, _ = unpacked(packed)
	require.Equal(t, []string{"night/theme.yml"}, names)

	// What is not a theme in use that is a directory has no archive.
	for _, name := range []string{"plain", "tron", "custom", "missing", "zelda/theme", "../zelda", ""} {
		_, _, err = ExportThemeArchive(name)
		require.ErrorIs(t, err, ErrThemeArchiveUnknown, name)
	}
	// A file that is no longer the one that was read is not handed out.
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, "zelda", "assets", "paper.png"), []byte(assetPNG+"changed since"), 0600))
	_, _, err = ExportThemeArchive("zelda")
	require.ErrorIs(t, err, ErrThemeArchiveRead)
	ReloadUserThemes()
	_, _, err = ExportThemeArchive("zelda")
	require.NoError(t, err)
}

// allFiles returns the entries of several mappings as one.
func allFiles(parts ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	return merged
}

// A theme whose archive would be larger than an archive that is taken in is
// not handed out as one.
func TestExportThemeArchiveOfATooLargeTheme(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "heavy", userThemeColors)
	// Content that does not get smaller by being packed.
	filling := make([]byte, model.MaximumThemeAssetBytes-len(assetPNG))
	for i := 0; i <= model.MaximumThemeArchiveBytes/model.MaximumThemeAssetBytes; i++ {
		_, err := rand.Read(filling)
		require.NoError(t, err)
		writeThemeAsset(t, "heavy", fmt.Sprintf("a%02d.png", i), assetPNG+string(filling))
	}
	ReloadUserThemes()
	require.True(t, IsUserTheme("heavy"))
	_, packed, err := ExportThemeArchive("heavy")
	require.ErrorIs(t, err, ErrThemeArchiveSize)
	require.Empty(t, packed)
}

// Pages are rendered while the selected theme is replaced by an archive.
// Each page has the theme as it was or as it is, and never none: the
// directory is moved to its place while no page reads the themes.
func TestStoreUserThemeArchiveIsSafeForConcurrentPages(t *testing.T) {
	userThemeDirectory(t, "zelda")
	first := packArchive(t, archiveEntry{name: "theme.yml", content: userThemeColors + "  radius: 10px\n"})
	second := packArchive(t, archiveEntry{name: "theme.yml", content: userThemeColors + "  radius: 0px\n"})
	_, err := storeArchive("zelda", first, false)
	require.NoError(t, err)
	UpdatePagePalettes()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-radius:10px;"}, GetPageTheme())
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				if page := GetPageTheme(); page.BodyStyle != userThemeStyle+"--flare-radius:10px;" && page.BodyStyle != userThemeStyle+"--flare-radius:0px;" {
					t.Errorf("a page got %+v", page)
					return
				}
			}
		})
	}
	for i := range 30 {
		packed := second
		if i%2 == 1 {
			packed = first
		}
		_, err = storeArchive("zelda", packed, true)
		require.NoError(t, err)
	}
	close(stop)
	readers.Wait()
	require.True(t, slices.Equal([]string{"zelda"}, allThemeEntries(t)))
}

// A theme is handed out while it is replaced by an archive. Each archive
// that is handed out holds the files of the theme as it was or as it is,
// and never some of each.
func TestExportThemeArchiveWhileTheThemeIsReplaced(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	version := func(name string) []byte {
		return packArchive(t,
			archiveEntry{name: "theme.yml", content: userThemeColors + "name: " + name + "\n"},
			archiveEntry{name: "theme.css", content: "/* " + name + " */\n.app-item { background-image: url(assets/" + name + ".png) }\n"},
			archiveEntry{name: "assets/" + name + ".png", content: assetPNG + name},
			archiveEntry{name: name + ".txt", content: name + "\n"},
		)
	}
	one, two := version("one"), version("two")
	_, err := storeArchive("zelda", one, false)
	require.NoError(t, err)
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, packed, exportErr := ExportThemeArchive("zelda")
				if exportErr != nil {
					t.Errorf("the theme was not handed out: %v", exportErr)
					return
				}
				reader, readErr := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
				if readErr != nil {
					t.Errorf("the archive cannot be read: %v", readErr)
					return
				}
				names := make([]string, 0, len(reader.File))
				for _, entry := range reader.File {
					names = append(names, entry.Name)
				}
				held := strings.Join(names, " ")
				if held != "zelda/theme.yml zelda/theme.css zelda/assets/one.png zelda/one.txt" && held != "zelda/theme.yml zelda/theme.css zelda/assets/two.png zelda/two.txt" {
					t.Errorf("an archive holds %s", held)
					return
				}
				// The theme file and the stylesheet are those of the asset.
				which := strings.TrimSuffix(strings.TrimPrefix(names[2], "zelda/assets/"), ".png")
				for i, want := range []string{userThemeColors + "name: " + which + "\n", "/* " + which + " */\n.app-item { background-image: url(assets/" + which + ".png) }\n"} {
					handle, openErr := reader.File[i].Open()
					if openErr != nil {
						t.Errorf("%s: %v", names[i], openErr)
						return
					}
					content, _ := io.ReadAll(handle)
					_ = handle.Close()
					if string(content) != want {
						t.Errorf("%s of an archive with the files of %q holds %q", names[i], which, content)
						return
					}
				}
			}
		})
	}
	for i := range 40 {
		packed := two
		if i%2 == 1 {
			packed = one
		}
		_, err = storeArchive("zelda", packed, true)
		require.NoError(t, err)
	}
	close(stop)
	readers.Wait()
}

// A themes directory that cannot be written to takes no archive and keeps
// what it holds, and a theme whose theme file cannot be read is not handed
// out.
func TestThemeArchivesAndADirectoryThatCannotBeUsed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser reads and writes everything")
	}
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "zelda", userThemeColors+"name: Before\n")
	ReloadUserThemes()
	packed := packArchive(t, zeldaArchive("")...)

	require.NoError(t, os.Chmod(UserThemeDirectory, 0500))
	t.Cleanup(func() { _ = os.Chmod(UserThemeDirectory, 0700) })
	for name, replace := range map[string]bool{"hyrule": false, "zelda": true} {
		others, err := storeArchive(name, packed, replace)
		require.ErrorIs(t, err, ErrThemeStoreWrite, name)
		require.Empty(t, others, name)
	}
	require.NoError(t, os.Chmod(UserThemeDirectory, 0700))
	require.Equal(t, []string{"zelda"}, allThemeEntries(t))
	require.Equal(t, map[string]string{"theme.yml": userThemeColors + "name: Before\n"}, themeDirectoryFiles(t, "zelda"))
	ReloadUserThemes()
	require.Equal(t, "Before", userTheme(t, "zelda").DisplayName("en"))

	manifest := filepath.Join(UserThemeDirectory, "zelda", userThemeManifest)
	require.NoError(t, os.Chmod(manifest, 0))
	t.Cleanup(func() { _ = os.Chmod(manifest, 0600) })
	file, archive, err := ExportThemeArchive("zelda")
	require.ErrorIs(t, err, ErrThemeArchiveRead)
	require.Empty(t, file)
	require.Empty(t, archive)
}
