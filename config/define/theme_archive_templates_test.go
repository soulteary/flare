package define

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const pageTemplateSource = `<div class="shell">` + everyPagePart + `</div>`

// templateEntries are the six templates of a theme as entries of an
// archive, under a prefix.
func templateEntries(prefix string) []archiveEntry {
	return []archiveEntry{
		{name: prefix + "templates/home.html", content: pageTemplateSource},
		{name: prefix + "templates/applications.html", content: nightApplicationsSource},
		{name: prefix + "templates/bookmarks.html", content: pageTemplateSource},
		{name: prefix + "templates/app.html", content: nightAppSource},
		{name: prefix + "templates/bookmark.html", content: bookmarkItemSource},
		{name: prefix + "templates/group.html", content: groupItemSource},
	}
}

var templateFileNames = []string{"home.html", "applications.html", "bookmarks.html", "app.html", "bookmark.html", "group.html"}

// An archive brings the templates of a theme: the six files of that
// directory a theme is read with. Other entries of the directory are no
// files of a theme.
func TestStoreUserThemeArchiveWithTemplates(t *testing.T) {
	for _, prefix := range []string{"", "packed/"} {
		userThemeDirectory(t, "blackboard")
		entries := zeldaArchive(prefix, templateEntries(prefix)...)
		entries = append(entries,
			archiveEntry{name: prefix + "templates/notes.txt", content: "notes"},
			archiveEntry{name: prefix + "templates/apps.html", content: "<script></script>"},
			archiveEntry{name: prefix + "templates/sub/home.html", content: "<div></div>"},
			archiveEntry{name: prefix + "templates/.home.html", content: "hidden"},
			archiveEntry{name: prefix + "templates/group.html.bak", content: "a backup"},
			archiveEntry{name: prefix + "Templates/home.html", content: "<div></div>"},
			archiveEntry{name: prefix + "templates/linked.html", content: "home.html", mode: fs.ModeSymlink | 0777},
		)
		others, err := storeArchive("zelda", packArchive(t, entries...), false)
		require.NoError(t, err, prefix)
		require.Equal(t, []string{
			prefix + "templates/notes.txt", prefix + "templates/apps.html", prefix + "templates/sub/home.html",
			prefix + "templates/group.html.bak", prefix + "Templates/home.html", prefix + "templates/linked.html",
		}, others, prefix)
		want := allFiles(zeldaFiles)
		for _, entry := range templateEntries("") {
			want[entry.name] = entry.content
		}
		require.Equal(t, want, themeDirectoryFiles(t, "zelda"), prefix)
		require.Equal(t, templateFileNames, ThemeTemplateFiles("zelda"), prefix)
		require.Empty(t, templateRemarks(t), prefix)
	}
}

// A template of an archive that is read and not accepted is stored with the
// theme, as it could be written into the directory: the theme is in use
// without it, and the theme page lists it. A template that cannot be read
// is a reason not to take the archive in, and nothing is left behind.
func TestStoreUserThemeArchiveWithTemplatesThatAreNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	entries := zeldaArchive("",
		archiveEntry{name: "templates/home.html", content: pageTemplateSource},
		archiveEntry{name: "templates/app.html", content: `<div><script>x()</script></div>`},
	)
	_, err := storeArchive("zelda", packArchive(t, entries...), false)
	require.NoError(t, err)
	require.Equal(t, `<div><script>x()</script></div>`, themeDirectoryFiles(t, "zelda")["templates/app.html"])
	require.Equal(t, []string{"home.html"}, ThemeTemplateFiles("zelda"))
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/app.html", Reason: "template_element", Subject: "script"}}, templateRemarks(t))
	before := themeDirectoryFiles(t, "zelda")

	for name, tc := range map[string]struct {
		entry  archiveEntry
		reason string
	}{
		"too large":         {archiveEntry{name: "templates/home.html", content: "<div>" + strings.Repeat("a", MaximumThemeTemplateBytes) + "</div>"}, ThemeProblemSize},
		"not text in UTF-8": {archiveEntry{name: "templates/group.html", content: "<div>\xff</div>"}, ThemeProblemTemplateEncoding},
	} {
		for _, replace := range []bool{true, false} {
			target := "zelda"
			if !replace {
				target = "another"
			}
			_, err = storeArchive(target, packArchive(t, zeldaArchive("", tc.entry)...), replace)
			var refused *ThemeArchiveProblem
			require.ErrorAs(t, err, &refused, name)
			require.Equal(t, ThemeProblem{File: tc.entry.name, Reason: tc.reason}, refused.Problem, name)
			require.Equal(t, []string{"zelda"}, allThemeEntries(t), name)
			require.Equal(t, before, themeDirectoryFiles(t, "zelda"), name)
		}
	}
	// A template of the exact size is read.
	exact := "<div>" + strings.Repeat("a", MaximumThemeTemplateBytes-len("<div></div>")-len(everyPagePart)) + everyPagePart + "</div>"
	require.Equal(t, MaximumThemeTemplateBytes, len(exact))
	_, err = storeArchive("another", packArchive(t, zeldaArchive("", archiveEntry{name: "templates/home.html", content: exact})...), false)
	require.NoError(t, err)
	require.Equal(t, []string{"home.html"}, ThemeTemplateFiles("another"))

	_, err = storeArchive("twice", packArchive(t, zeldaArchive("",
		archiveEntry{name: "templates/home.html", content: pageTemplateSource}, archiveEntry{name: "templates/home.html", content: pageTemplateSource})...), false)
	var twice *ThemeArchiveFileError
	require.ErrorAs(t, err, &twice)
	require.ErrorIs(t, err, ErrThemeArchiveTwice)
	require.Equal(t, "templates/home.html", twice.File)
}

// The archive of a theme holds its templates as a reading of the directory
// reads them, accepted or not, and nothing else of that directory. What is
// handed out is taken in again as the same theme.
func TestExportThemeArchiveWithTemplates(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	_, err := storeArchive("zelda", packArchive(t, zeldaArchive("", templateEntries("")...)...), false)
	require.NoError(t, err)
	writeThemeTemplate(t, "zelda", "app.html", `<div><script>x()</script></div>`)
	writeThemeTemplate(t, "zelda", "group.html", "<div>\xff</div>")
	writeThemeTemplate(t, "zelda", "notes.txt", "notes")
	writeThemeTemplate(t, "zelda", "apps.html", "<div></div>")
	ReloadUserThemes()
	require.Len(t, templateRemarks(t), 2)

	file, packed, err := ExportThemeArchive("zelda")
	require.NoError(t, err)
	require.Equal(t, "zelda.zip", file)
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	require.NoError(t, err)
	var names []string
	contents := map[string]string{}
	for _, entry := range reader.File {
		handle, openErr := entry.Open()
		require.NoError(t, openErr)
		content, readErr := io.ReadAll(handle)
		require.NoError(t, readErr)
		require.NoError(t, handle.Close())
		if path, isTemplate := strings.CutPrefix(entry.Name, "zelda/templates/"); isTemplate {
			names = append(names, path)
			// An entry has the time of its file.
			file, statErr := os.Stat(filepath.Join(UserThemeDirectory, "zelda", "templates", path))
			require.NoError(t, statErr)
			require.Equal(t, file.ModTime().Unix(), entry.Modified.Unix(), entry.Name)
		}
		contents[strings.TrimPrefix(entry.Name, "zelda/")] = string(content)
	}
	require.Equal(t, []string{"home.html", "applications.html", "bookmarks.html", "app.html", "bookmark.html"}, names)
	require.Equal(t, `<div><script>x()</script></div>`, contents["templates/app.html"])

	_, err = storeArchive("again", packed, false)
	require.NoError(t, err)
	require.Equal(t, contents, themeDirectoryFiles(t, "again"))
	require.Equal(t, ThemeTemplateFiles("zelda"), ThemeTemplateFiles("again"))
	require.FileExists(t, filepath.Join(UserThemeDirectory, "again", "templates", "home.html"))

	// A theme without templates has no such directory in its archive.
	writeThemeDirectory(t, "bare", userThemeColors)
	ReloadUserThemes()
	_, packed, err = ExportThemeArchive("bare")
	require.NoError(t, err)
	reader, err = zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)
}

// The themes that can be shown with templates of a theme are the theme and
// those that have it below.
func TestThemesShownWith(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeNightTheme(t)
	writeThemeDirectory(t, "deep", "contract: 1\nextends: night\n")
	writeThemeDirectory(t, "other", userThemeColors)
	writeUserTheme(t, "soft.yml", "contract: 1\nextends: tron\n")
	writeUserTheme(t, "lost.yml", "contract: 1\nextends: absent\n")
	ReloadUserThemes()
	require.Equal(t, []string{"compact", "deep", "night", "zelda"}, ThemesShownWith("zelda"))
	require.Equal(t, []string{"compact", "deep", "night"}, ThemesShownWith("compact"))
	require.Equal(t, []string{"deep"}, ThemesShownWith("deep"))
	require.Equal(t, []string{"other"}, ThemesShownWith("other"))
	require.Equal(t, []string{"soft"}, ThemesShownWith("tron"))
	require.Empty(t, ThemesShownWith("absent"))
	require.Empty(t, ThemesShownWith("lost"))
	require.Empty(t, ThemesShownWith(""))
}
