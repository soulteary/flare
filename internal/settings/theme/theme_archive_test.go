package theme

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// An archive is stored with the checks of every write of the theme page. A
// session that is revoked after the request was let in stores no theme, and
// leaves nothing in the themes directory.
func TestThemeArchiveIsNotStoredForARevokedSession(t *testing.T) {
	isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	guarded := newRevocableTheme(t)
	var packed bytes.Buffer
	archive := zip.NewWriter(&packed)
	part, err := archive.Create("theme.yml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte(themeFileColors)); err != nil {
		t.Fatal(err)
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	upload := func(revoked bool) (int, string) {
		t.Helper()
		return guarded.post(func(csrf string) (io.Reader, string) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			for field, value := range map[string]string{"action": actionImport, "csrf-token": csrf} {
				if e := form.WriteField(field, value); e != nil {
					t.Fatal(e)
				}
			}
			file, e := form.CreateFormFile(importField, "zelda.zip")
			if e != nil {
				t.Fatal(e)
			}
			if _, e = file.Write(packed.Bytes()); e != nil {
				t.Fatal(e)
			}
			if e = form.Close(); e != nil {
				t.Fatal(e)
			}
			return &body, form.FormDataContentType()
		}, revoked)
	}
	entries := func() []string {
		t.Helper()
		listed, e := os.ReadDir(define.UserThemeDirectory)
		if e != nil && !os.IsNotExist(e) {
			t.Fatal(e)
		}
		names := make([]string, 0, len(listed))
		for _, entry := range listed {
			names = append(names, entry.Name())
		}
		return names
	}

	if status, body := upload(true); status != http.StatusUnauthorized || !strings.Contains(body, "AUTH_REQUIRED") {
		t.Fatalf("with a revoked session: %d %s", status, body)
	}
	if left := entries(); len(left) != 0 {
		t.Fatalf("a revoked session left %v in the themes directory", left)
	}
	if define.IsUserTheme("zelda") {
		t.Fatal("a revoked session stored the theme")
	}
	// The same upload is carried out for a session that holds.
	if status, body := upload(false); status != http.StatusOK {
		t.Fatalf("with a session: %d %s", status, body)
	}
	if left := entries(); len(left) != 1 || left[0] != "zelda" {
		t.Fatalf("themes directory: %v", left)
	}
	if _, err = os.Stat(filepath.Join(define.UserThemeDirectory, "zelda", "theme.yml")); err != nil || !define.IsUserTheme("zelda") {
		t.Fatalf("the theme was not stored: %v", err)
	}
}

// The entries of an archive that were left out are named in one line of text
// of a bounded length: a number of them, each made plain, and a mark that
// there are more.
func TestLeftOutText(t *testing.T) {
	if text := leftOutText([]string{"notes.html", "src/design.psd"}); text != "notes.html, src/design.psd" {
		t.Fatalf("two entries: %q", text)
	}
	many := make([]string, 0, maximumLeftOutShown+5)
	for i := range maximumLeftOutShown + 5 {
		many = append(many, string(rune('a'+i))+".html")
	}
	text := leftOutText(many)
	if want := strings.Join(many[:maximumLeftOutShown], ", ") + ", …"; text != want {
		t.Fatalf("more entries than are named: %q", text)
	}
	if text = leftOutText(many[:maximumLeftOutShown]); strings.Contains(text, "…") || text != strings.Join(many[:maximumLeftOutShown], ", ") {
		t.Fatalf("as many entries as are named: %q", text)
	}
	// Names come from the archive: line breaks and bytes that are no text
	// are taken out, and a long name is cut.
	text = leftOutText([]string{"a\nb\x00c.html", "bad\xffname.html", strings.Repeat("x", 300) + ".html"})
	if want := "a b c.html, bad?name.html, " + strings.Repeat("x", maximumShownName) + "…"; text != want {
		t.Fatalf("names that are made plain: %q", text)
	}
	if leftOutText(nil) != "" {
		t.Fatal("no entries")
	}
}

// Every reason a theme is not imported has a status and a text of its own.
// A refusal about a file names the file, and one the checks of a theme give
// is worded as the list of files that are not used words it.
func TestImportRefusalWording(t *testing.T) {
	text := func(key string) string { return i18n.T("en", key) }
	rejected := func(reason string) string {
		return i18n.Format("en", "theme_import_rejected", map[string]any{"reason": text("theme_file_problem_" + reason)})
	}
	seen := map[string]string{}
	for name, tc := range map[string]struct {
		refusal error
		status  int
		text    string
	}{
		"a name that is none":      {define.ErrThemeStoreName, http.StatusBadRequest, text("theme_import_name")},
		"a theme file too large":   {define.ErrThemeStoreSize, http.StatusBadRequest, text("theme_import_size")},
		"a theme of the name":      {define.ErrThemeStoreExists, http.StatusConflict, text("theme_import_exists")},
		"a directory of the name":  {define.ErrThemeStoreDirectory, http.StatusConflict, text("theme_import_directory")},
		"something else of it":     {define.ErrThemeStoreOccupied, http.StatusConflict, text("theme_import_occupied")},
		"safe mode":                {define.ErrThemeStoreSafeMode, http.StatusConflict, text("theme_import_safe_mode")},
		"a full directory":         {define.ErrThemeStoreFull, http.StatusConflict, text("theme_import_full")},
		"an archive too large":     {define.ErrThemeArchiveSize, http.StatusBadRequest, text("theme_import_archive_size")},
		"too many entries":         {define.ErrThemeArchiveEntries, http.StatusBadRequest, text("theme_import_archive_entries")},
		"no theme file":            {define.ErrThemeArchiveEmpty, http.StatusBadRequest, text("theme_import_archive_empty")},
		"more than one theme":      {define.ErrThemeArchiveSeveral, http.StatusBadRequest, text("theme_import_archive_several")},
		"an archive not read":      {define.ErrThemeArchiveRead, http.StatusBadRequest, text("theme_import_archive_read")},
		"a file of it not read":    {&define.ThemeArchiveFileError{Reason: define.ErrThemeArchiveRead, File: "assets/a.png"}, http.StatusBadRequest, text("theme_import_archive_read") + " assets/a.png"},
		"a file twice":             {&define.ThemeArchiveFileError{Reason: define.ErrThemeArchiveTwice, File: "theme.css"}, http.StatusBadRequest, text("theme_import_archive_twice") + " theme.css"},
		"a document not accepted":  {&define.ThemeArchiveFileError{Reason: define.ErrThemeArchiveDocument, File: "LICENSE"}, http.StatusBadRequest, text("theme_import_archive_document") + " LICENSE"},
		"a file with markup":       {&define.ThemeArchiveFileError{Reason: define.ErrThemeArchiveTwice, File: "a\nb\x00.png"}, http.StatusBadRequest, text("theme_import_archive_twice") + " a b .png"},
		"a part of the theme":      {&define.ThemeArchiveProblem{Problem: define.ThemeProblem{File: "theme.css", Reason: define.ThemeProblemStyleRule, Subject: "line 1: @import"}}, http.StatusBadRequest, rejected("style_rule") + " theme.css line 1: @import"},
		"a part without a detail":  {&define.ThemeArchiveProblem{Problem: define.ThemeProblem{File: "assets/big.png", Reason: define.ThemeProblemAssetSize}}, http.StatusBadRequest, rejected("asset_size") + " assets/big.png"},
		"a theme file of one file": {&model.ThemeFileError{Reason: model.ErrThemeTokenValue, Subject: "tokens.radius"}, http.StatusBadRequest, rejected("value") + " tokens.radius"},
		"a theme below not in use": {&model.ThemeFileError{Reason: model.ErrThemeBase, Subject: "nowhere"}, http.StatusBadRequest, rejected("extends") + " nowhere"},
		"a failure to write":       {errors.Join(define.ErrThemeStoreWrite, errors.New("disk")), http.StatusInternalServerError, text("theme_import_write")},
		"anything else":            {errors.New("unexpected"), http.StatusInternalServerError, text("theme_import_write")},
	} {
		status, got := importRefusal("en", tc.refusal)
		if status != tc.status || got != tc.text {
			t.Errorf("%s: %d %q, want %d %q", name, status, got, tc.status, tc.text)
		}
		if !strings.Contains(got, "Nothing was imported.") {
			t.Errorf("%s: the text does not say that nothing was imported: %q", name, got)
		}
		if other, repeated := seen[got]; repeated && tc.status != http.StatusInternalServerError {
			t.Errorf("%s and %s share a text", name, other)
		}
		seen[got] = name
		// The same refusal has a text in Chinese that is not the English one.
		if _, zh := importRefusal("zh", tc.refusal); zh == got || zh == "" {
			t.Errorf("%s: no text of its own in Chinese: %q", name, zh)
		}
	}
}
