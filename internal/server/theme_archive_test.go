package server

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// packThemeArchive packs files into a ZIP file, in the order given: a name
// and the content of each.
func packThemeArchive(t *testing.T, files ...string) string {
	t.Helper()
	require.Zero(t, len(files)%2)
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for i := 0; i < len(files); i += 2 {
		part, err := writer.Create(files[i])
		require.NoError(t, err)
		_, err = part.Write([]byte(files[i+1]))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return packed.String()
}

// unpackThemeArchive returns the names of the entries of a ZIP file in their
// order, and their content by name.
func unpackThemeArchive(t *testing.T, packed []byte) ([]string, map[string]string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(packed), int64(len(packed)))
	require.NoError(t, err)
	names := make([]string, 0, len(reader.File))
	contents := map[string]string{}
	for _, entry := range reader.File {
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

func themesDirectoryNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(define.UserThemeDirectory)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// A theme that is a directory is downloaded as an archive and imported from
// one: its theme file, its stylesheet, its assets and its text documents.
func TestThemeArchiveHTTP(t *testing.T) {
	zeldaFile := themeFileColors + "name: Zelda\nbackground:\n  image: assets/paper.png\n"
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", map[string]string{
		"zelda/theme.yml":          zeldaFile,
		"zelda/theme.css":          themeAssetStylesheet,
		"zelda/assets/paper.png":   themeAssetPNG,
		"zelda/assets/brand.woff2": themeAssetWOFF2,
		"zelda/LICENSE.txt":        "A license.\n",
		"zelda/notes.html":         "<p>notes</p>",
		"plain.yml":                themeFileColors,
		"folder/kept.txt":          "no theme",
	})
	rejected := func(reason string) string {
		return i18n.Format("en", "theme_import_rejected", map[string]any{"reason": i18n.T("en", reason)})
	}

	// The card of a directory has a download like the card of a file, and
	// the form takes a theme file or an archive.
	raw := getPage(t, handler, "/settings/theme")
	require.Equal(t, map[string][]string{
		"plain": {"/?theme-preview=plain", "/settings/theme/export?name=plain"},
		"zelda": {"/?theme-preview=zelda", "/settings/theme/export?name=zelda"},
	}, themeCardLinks(t, raw))
	document, err := html.Parse(strings.NewReader(raw))
	require.NoError(t, err)
	accepted := ""
	walkNodes(document, func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" && nodeAttribute(node, "name") == "theme-file" {
			accepted = nodeAttribute(node, "accept")
		}
	})
	require.Equal(t, ".yml,.zip", accepted)

	// The download is an archive with a directory of the name of the theme:
	// the files of the theme, and nothing else of its directory.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=zelda", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="zelda.zip"`, rec.Header().Get("Content-Disposition"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	downloaded := rec.Body.Bytes()
	names, contents := unpackThemeArchive(t, downloaded)
	require.Equal(t, []string{"zelda/theme.yml", "zelda/theme.css", "zelda/assets/brand.woff2", "zelda/assets/paper.png", "zelda/LICENSE.txt"}, names)
	require.Equal(t, zeldaFile, contents["zelda/theme.yml"])
	require.Equal(t, themeAssetStylesheet, contents["zelda/theme.css"])
	require.Equal(t, themeAssetPNG, contents["zelda/assets/paper.png"])
	require.Equal(t, "A license.\n", contents["zelda/LICENSE.txt"])

	// What was downloaded is imported under the name of the uploaded file.
	// The theme is listed and not selected, and is what the first one is.
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	imported := importFile(t, handler, "hyrule.zip", string(downloaded), false)
	require.Equal(t, http.StatusOK, imported.Code)
	page := readImportPage(t, imported.Body.String())
	require.Equal(t, []string{i18n.Format("en", "theme_import_done", map[string]any{"name": "Zelda"})}, page.status)
	require.Empty(t, page.alerts)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Equal(t, []string{"folder", "hyrule", "plain.yml", "zelda"}, themesDirectoryNames(t))
	stored, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "hyrule", "assets", "paper.png"))
	require.NoError(t, err)
	require.Equal(t, themeAssetPNG, string(stored))
	previewed := getPage(t, handler, "/?theme-preview=hyrule")
	styles := pageStyles(t, previewed)
	require.Len(t, styles, 3)
	require.True(t, strings.HasPrefix(styles[1], "/theme/hyrule/"), styles[1])
	served := getStylesheet(handler, http.MethodGet, styles[1])
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, themeAssetStylesheet, served.Body.String())
	hash := strings.Split(styles[1], "/")[3]
	require.Equal(t, [][4]string{{"hyrule", hash, "paper.png", ""}}, pageBackgrounds(t, previewed))
	require.Equal(t, http.StatusOK, getStylesheet(handler, http.MethodGet, "/theme/hyrule/"+hash+"/assets/brand.woff2").Code)
	// A text document is stored, and is not served.
	license, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "hyrule", "LICENSE.txt"))
	require.NoError(t, err)
	require.Equal(t, "A license.\n", string(license))
	for _, address := range []string{"/theme/hyrule/" + hash + "/LICENSE.txt", "/theme/hyrule/" + hash + "/assets/LICENSE.txt", "/theme/hyrule/" + hash + "/theme.yml"} {
		require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, address).Code, address)
	}

	// Entries that are no files of a theme are left out, and the page names
	// them as text.
	many := []string{"theme.yml", themeFileColors, "notes.html", "<p>notes</p>", "src/<b>design</b>.psd", "layers", ".DS_Store", "finder"}
	withMore := importFile(t, handler, "with-more.zip", packThemeArchive(t, many...), false)
	require.Equal(t, http.StatusOK, withMore.Code)
	page = readImportPage(t, withMore.Body.String())
	require.Equal(t, []string{
		i18n.Format("en", "theme_import_done", map[string]any{"name": "with-more"}),
		i18n.Format("en", "theme_import_left_out", map[string]any{"files": "notes.html, src/<b>design</b>.psd"}),
	}, page.status)
	require.NotContains(t, withMore.Body.String(), "<b>design</b>")
	require.Equal(t, []string{"theme.yml"}, func() []string {
		entries, readErr := os.ReadDir(filepath.Join(define.UserThemeDirectory, "with-more"))
		require.NoError(t, readErr)
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		return names
	}())
	// The page names a number of them, and says that there are more.
	crowded := []string{"theme.yml", themeFileColors}
	for i := range 20 {
		crowded = append(crowded, fmt.Sprintf("note-%02d.html", i), "x")
	}
	page = readImportPage(t, importFile(t, handler, "crowded.zip", packThemeArchive(t, crowded...), false).Body.String())
	require.Len(t, page.status, 2)
	require.True(t, strings.HasSuffix(page.status[1], "note-11.html, …"), page.status[1])
	require.NotContains(t, page.status[1], "note-12.html")

	// Every refusal says why and that nothing was imported, and nothing is
	// stored.
	listed := themesDirectoryNames(t)
	tooMany := []string{"theme.yml", themeFileColors}
	for i := range model.MaximumThemeArchiveEntries {
		tooMany = append(tooMany, fmt.Sprintf("notes/%03d.html", i), "x")
	}
	for name, tc := range map[string]struct {
		filename, content string
		replace           bool
		status            int
		text              string
	}{
		"not an archive":        {"broken.zip", "contract: 1\n", false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_read")},
		"no theme file":         {"empty.zip", packThemeArchive(t, "theme.css", themeStylesheetText), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_empty")},
		"two themes":            {"two.zip", packThemeArchive(t, "a/theme.yml", themeFileColors, "b/theme.yml", themeFileColors), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_several")},
		"too many entries":      {"many.zip", packThemeArchive(t, tooMany...), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_entries")},
		"a file twice":          {"twice.zip", packThemeArchive(t, "theme.yml", themeFileColors, "theme.yml", themeFileColors), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_twice") + " theme.yml"},
		"a document of no text": {"binary.zip", packThemeArchive(t, "theme.yml", themeFileColors, "LICENSE", "a\x00b"), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_document") + " LICENSE"},
		"a stylesheet that loads a file": {"loads.zip", packThemeArchive(t, "theme.yml", themeFileColors, "theme.css", "@import 'a.css';"), false, http.StatusBadRequest,
			rejected("theme_file_problem_style_rule") + " theme.css line 1: @import"},
		"an asset that is no image": {"fake.zip", packThemeArchive(t, "theme.yml", themeFileColors, "assets/fake.png", "<svg onload=alert(1)>"), false, http.StatusBadRequest,
			rejected("theme_file_problem_asset_content") + " assets/fake.png"},
		"a theme file with a bad value": {"value.zip", packThemeArchive(t, "theme.yml", themeFileColors+"  radius: 4em\n"), false, http.StatusBadRequest,
			rejected("theme_file_problem_value") + " theme.yml tokens.radius"},
		"a theme below that is not there": {"orphan.zip", packThemeArchive(t, "theme.yml", "contract: 1\nextends: nowhere\n"), false, http.StatusBadRequest,
			rejected("theme_file_problem_extends") + " nowhere"},
		"a name in upper case":       {"Zelda.zip", string(downloaded), false, http.StatusBadRequest, i18n.T("en", "theme_import_name")},
		"the name of a bundled one":  {"tron.zip", string(downloaded), false, http.StatusBadRequest, i18n.T("en", "theme_import_name")},
		"another kind of archive":    {"zelda.tar.gz", string(downloaded), false, http.StatusBadRequest, i18n.T("en", "theme_import_name")},
		"a theme of the name":        {"zelda.zip", string(downloaded), false, http.StatusConflict, i18n.T("en", "theme_import_exists")},
		"a theme file of the name":   {"plain.zip", string(downloaded), false, http.StatusConflict, i18n.T("en", "theme_import_exists")},
		"a folder that is no theme":  {"folder.zip", string(downloaded), true, http.StatusConflict, i18n.T("en", "theme_import_occupied")},
		"larger than an archive can": {"large.zip", strings.Repeat("\x00", model.MaximumThemeArchiveBytes+1), false, http.StatusBadRequest, i18n.T("en", "theme_import_archive_size")},
	} {
		refused := importFile(t, handler, tc.filename, tc.content, tc.replace)
		require.Equal(t, tc.status, refused.Code, name)
		page = readImportPage(t, refused.Body.String())
		require.Equal(t, []string{tc.text}, page.alerts, name)
		require.Contains(t, page.alerts[0], "Nothing was imported.", name)
		require.Empty(t, page.status, name)
		require.Equal(t, listed, themesDirectoryNames(t), name)
	}

	// With the box ticked, a theme of the name is replaced: a directory as
	// a whole, and a theme file by the directory.
	replaced := importFile(t, handler, "zelda.zip", packThemeArchive(t, "theme.yml", themeFileColors+"name: Replaced\n"), true)
	require.Equal(t, http.StatusOK, replaced.Code)
	require.Equal(t, []string{i18n.Format("en", "theme_import_done", map[string]any{"name": "Replaced"})}, readImportPage(t, replaced.Body.String()).status)
	entries, err := os.ReadDir(filepath.Join(define.UserThemeDirectory, "zelda"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, http.StatusOK, importFile(t, handler, "plain.zip", string(downloaded), true).Code)
	require.NotContains(t, themesDirectoryNames(t), "plain.yml")
	require.Contains(t, themesDirectoryNames(t), "plain")
	require.True(t, define.IsUserTheme("plain"))
}

// A theme whose archive would be larger than an archive that is taken in has
// no download, and the answer says what to do instead.
func TestThemeArchiveTooLargeToHandOutHTTP(t *testing.T) {
	files := map[string]string{"heavy/theme.yml": themeFileColors}
	filling := make([]byte, model.MaximumThemeAssetBytes-len(themeAssetPNG))
	for i := 0; i <= model.MaximumThemeArchiveBytes/model.MaximumThemeAssetBytes; i++ {
		_, err := rand.Read(filling)
		require.NoError(t, err)
		files[fmt.Sprintf("heavy/assets/a%02d.png", i)] = themeAssetPNG + string(filling)
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", files)
	require.True(t, define.IsUserTheme("heavy"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=heavy", nil))
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, i18n.T("en", "theme_export_archive_size"), rec.Body.String())
	require.Empty(t, rec.Header().Get("Content-Disposition"))
}
