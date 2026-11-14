package theme

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
)

// writeThemeDirectory writes the theme.yml of a theme that is a directory,
// and makes the directory if it is not there.
func writeThemeDirectory(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(define.UserThemeDirectory, name), 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, filepath.Join(name, "theme.yml"), content)
}

// A theme that is a directory is selected by the name of the directory. It
// is not offered as a download of one file, and no file is imported under
// its name.
func TestThemeDirectoryOnTheThemePage(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	app.Get(define.SettingPages.Theme.Path+exportPath, exportTheme)
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	// Written after the instance started: selecting reads the directory first.
	writeThemeDirectory(t, "zelda", themeFileColors+"  radius: 10px\n")
	writeThemeFile(t, "plain.yml", themeFileColors)

	status, page := postTheme(t, app, url.Values{"theme": {"zelda"}})
	if status != http.StatusOK {
		t.Fatalf("select: %d %s", status, page)
	}
	want := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;--flare-radius:10px;"
	if name, _ := define.ThemeSnapshot(); name != "zelda" || string(define.GetAppBodyStyle()) != want {
		t.Fatalf("active theme %q with the style %q", name, define.GetAppBodyStyle())
	}
	if theme, _ := data.GetThemeSelection(); theme != "zelda" {
		t.Fatalf("stored selection %q", theme)
	}

	// What is in the directory is not a theme of its own.
	for _, name := range []string{"zelda/theme", "zelda/theme.yml", "zelda/", "/zelda", "theme", "themes/zelda"} {
		if status, body := postTheme(t, app, url.Values{"theme": {name}}); status != http.StatusBadRequest {
			t.Errorf("%q: %d %s", name, status, body)
		}
		if current, _ := define.ThemeSnapshot(); current != "zelda" {
			t.Errorf("%q: the selection changed to %q", name, current)
		}
	}

	// No file is stored under the name of the directory, with the box for
	// replacing or without it.
	for _, replace := range []bool{false, true} {
		if status := uploadTheme(t, app, "zelda.yml", themeFileColors, replace); status != http.StatusConflict {
			t.Errorf("import beside the directory, replace %v: %d", replace, status)
		}
	}
	if files := storedThemeFiles(t); !slices.Equal(files, []string{"plain.yml", "zelda"}) {
		t.Fatalf("themes directory: %v", files)
	}
	if string(define.GetAppBodyStyle()) != want {
		t.Fatalf("the theme changed: %q", define.GetAppBodyStyle())
	}

	export := func(name string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, define.SettingPages.Theme.Path+exportPath+"?name="+url.QueryEscape(name), nil)
		res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	for name, want := range map[string]int{
		"plain":           http.StatusOK,
		"zelda":           http.StatusNotFound,
		"zelda/theme":     http.StatusNotFound,
		"zelda/theme.yml": http.StatusNotFound,
	} {
		if got := export(name); got != want {
			t.Errorf("export of %q: %d, want %d", name, got, want)
		}
	}
}
