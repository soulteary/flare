package theme

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

const themeFileColors = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n"

// writeThemeFile writes a theme file that was last modified a minute ago, as
// a file nobody is editing.
func writeThemeFile(t *testing.T, file, content string) {
	t.Helper()
	path := filepath.Join(define.UserThemeDirectory, file)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	modified := time.Now().Add(-time.Minute)
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestThemeFileIsSelectedAndStored(t *testing.T) {
	app := isolateTheme(t)
	// Runs before the cleanup of isolateTheme reads the directory again.
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	// The files are written after the instance started: selecting one reads
	// the directory first.
	writeThemeFile(t, "forest.yml", themeFileColors+"  radius: 10px\n")
	writeThemeFile(t, "rejected.yml", themeFileColors+"  radius: 4em\n")
	writeThemeFile(t, "Mixed.yml", themeFileColors)

	if status, body := postTheme(t, app, url.Values{"theme": {"forest"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, body)
	}
	if name, primary := define.ThemeSnapshot(); name != "forest" || primary != "#e8f1e4" {
		t.Fatalf("active theme %q %q", name, primary)
	}
	want := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;--flare-radius:10px;"
	if style := string(define.GetAppBodyStyle()); style != want {
		t.Fatalf("body style %q", style)
	}
	if theme, _ := data.GetThemeSelection(); theme != "forest" {
		t.Fatalf("stored selection %q", theme)
	}

	// What is not a usable theme file cannot be selected, whatever is
	// submitted; the selection and config.yml stay as they are.
	stored, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rejected", "Mixed", "mixed", "missing", "", "forest.yml", "themes/forest", "../config", "forest\x00", strings.Repeat("a", 4096)} {
		if status, body := postTheme(t, app, url.Values{"theme": {name}}); status != http.StatusBadRequest {
			t.Errorf("%q: %d %s", name, status, body)
		}
		if current, _ := define.ThemeSnapshot(); current != "forest" || string(define.GetAppBodyStyle()) != want {
			t.Errorf("%q: the selection changed to %q", name, current)
		}
		if raw, readErr := os.ReadFile("config.yml"); readErr != nil || string(raw) != string(stored) {
			t.Errorf("%q: config.yml changed: %q %v", name, raw, readErr)
		}
	}

	// A file removed since the page was shown cannot be selected either.
	if status, body := postTheme(t, app, url.Values{"theme": {"cloud"}}); status != http.StatusOK {
		t.Fatalf("bundled theme: %d %s", status, body)
	}
	if err = os.Remove(filepath.Join(define.UserThemeDirectory, "forest.yml")); err != nil {
		t.Fatal(err)
	}
	if status, body := postTheme(t, app, url.Values{"theme": {"forest"}}); status != http.StatusBadRequest {
		t.Fatalf("removed file: %d %s", status, body)
	}
	if current, _ := define.ThemeSnapshot(); current != "cloud" {
		t.Fatalf("the selection changed to %q", current)
	}
}

func TestThemeFileCardsShowTheTextsOfTheFile(t *testing.T) {
	colors := model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}
	themes := []model.Theme{
		{Name: "forest", Colors: colors, Version: "1.2", Author: "Example Author", License: "MIT",
			Title: model.ThemeText{Localized: map[string]string{"en": "Forest at dusk", "zh": "暮色森林"}}},
		{Name: "plain", Colors: colors},
		{Name: "signed", Colors: colors, Author: "Example Author"},
		{Name: "licensed", Colors: colors, Version: "2", License: "CC0-1.0", Title: model.ThemeText{Plain: "Licensed"}},
		{Name: "zelda", Colors: colors, Directory: true},
	}
	for locale, want := range map[string][]themeFileCard{
		"en": {
			{Name: "forest", Title: "Forest at dusk", About: "1.2 · Example Author · MIT", Colors: colors},
			{Name: "plain", Title: "plain", Colors: colors},
			{Name: "signed", Title: "signed", About: "Example Author", Colors: colors},
			{Name: "licensed", Title: "Licensed", About: "2 · CC0-1.0", Colors: colors},
			{Name: "zelda", Title: "zelda", Colors: colors, Directory: true},
		},
		"zh": {
			{Name: "forest", Title: "暮色森林", About: "1.2 · Example Author · MIT", Colors: colors},
			{Name: "plain", Title: "plain", Colors: colors},
			{Name: "signed", Title: "signed", About: "Example Author", Colors: colors},
			{Name: "licensed", Title: "Licensed", About: "2 · CC0-1.0", Colors: colors},
			{Name: "zelda", Title: "zelda", Colors: colors, Directory: true},
		},
	} {
		got := themeFileCards(locale, themes)
		if len(got) != len(want) {
			t.Fatalf("%s: %d cards", locale, len(got))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: card %d: got %+v, want %+v", locale, i, got[i], want[i])
			}
		}
	}
	if cards := themeFileCards("en", nil); len(cards) != 0 {
		t.Fatalf("no theme files: %+v", cards)
	}
}

func TestThemeFileLinesExplainEveryReason(t *testing.T) {
	reasons := []string{
		define.ThemeProblemExtension, define.ThemeProblemName, define.ThemeProblemReserved, define.ThemeProblemType,
		define.ThemeProblemLink, define.ThemeProblemDuplicate,
		define.ThemeProblemSize, define.ThemeProblemLimit, define.ThemeProblemRead, define.ThemeProblemSyntax,
		define.ThemeProblemContract, define.ThemeProblemValue, define.ThemeProblemColor, define.ThemeProblemContrast,
		define.ThemeProblemText,
	}
	for _, locale := range []string{"en", "zh"} {
		seen := map[string]string{}
		for _, reason := range reasons {
			lines := themeFileLines(locale, []define.ThemeProblem{{File: "file.yml", Reason: reason}}, nil)
			if len(lines) != 1 || lines[0].File != "file.yml" || lines[0].Detail != "" {
				t.Fatalf("%s %s: %+v", locale, reason, lines)
			}
			text := lines[0].Text
			if text == "" || strings.HasPrefix(text, "theme_file_problem_") {
				t.Errorf("%s: reason %q has no text", locale, reason)
			}
			if other, repeated := seen[text]; repeated {
				t.Errorf("%s: reasons %q and %q share a text", locale, reason, other)
			}
			seen[text] = reason
		}
	}

	// A file with a problem and unused parts keeps its lines together, and
	// the files follow the order of their names.
	lines := themeFileLines("en",
		[]define.ThemeProblem{
			{File: "b.yml", Reason: define.ThemeProblemValue, Subject: "tokens.radius"},
			{File: "c.yaml", Reason: define.ThemeProblemExtension},
		},
		[]define.ThemeNotice{
			{File: "a.yml", Unused: model.ThemeFileUnused{Keys: []string{"modes.sepia", "options"}, Tokens: []string{"modes.dark.glow", "tokens.space"}}},
			{File: "b.yml", Unused: model.ThemeFileUnused{Tokens: []string{"tokens.shadow"}}},
		})
	type line struct{ file, detail string }
	got := make([]line, 0, len(lines))
	for _, entry := range lines {
		got = append(got, line{entry.File, entry.Detail})
	}
	want := []line{{"a.yml", "modes.sepia, options, modes.dark.glow, tokens.space"}, {"b.yml", "tokens.radius"}, {"b.yml", "tokens.shadow"}, {"c.yaml", ""}}
	if len(got) != len(want) {
		t.Fatalf("lines: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if lines := themeFileLines("en", nil, nil); len(lines) != 0 {
		t.Fatalf("no remarks: %+v", lines)
	}
}

// File names and parser messages come from the themes directory. They are
// shown as one line of valid text of a bounded length.
func TestThemeFileTextIsPlain(t *testing.T) {
	for name, tc := range map[string]struct {
		text  string
		limit int
		want  string
	}{
		"unchanged":        {"forest.yml", 80, "forest.yml"},
		"markup stays":     {"<b>.yml", 80, "<b>.yml"},
		"line breaks":      {"yaml: unmarshal errors:\n  line 1: cannot unmarshal", 80, "yaml: unmarshal errors: line 1: cannot unmarshal"},
		"control":          {"a\x00b\x1bc\x7fd.yml", 80, "a b c d.yml"},
		"tabs and spaces":  {" a \t b ", 80, "a b"},
		"invalid UTF-8":    {"a\xff\xfeb", 80, "a?b"},
		"at the limit":     {strings.Repeat("字", 80), 80, strings.Repeat("字", 80)},
		"over the limit":   {strings.Repeat("字", 81), 80, strings.Repeat("字", 80) + "…"},
		"long ASCII":       {strings.Repeat("a", 500), 240, strings.Repeat("a", 240) + "…"},
		"empty":            {"", 80, ""},
		"only whitespace":  {" \n\t ", 80, ""},
		"other separators": {"a b\u0085c", 80, "a b c"},
	} {
		if got := plainText(tc.text, tc.limit); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

const pairedThemeFile = themeFileColors + "modes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n"

func modeForm(mode string) url.Values {
	return url.Values{"action": {actionSaveMode}, "theme-mode": {mode}}
}

func TestThemeModeIsSavedAndApplied(t *testing.T) {
	app := isolateTheme(t)
	// Runs before the cleanup of isolateTheme reads the configuration again.
	t.Cleanup(func() {
		_ = os.RemoveAll(define.UserThemeDirectory)
		define.SetThemeMode(model.ThemeModeAuto)
	})
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, "paired.yml", pairedThemeFile)
	if status, body := postTheme(t, app, url.Values{"theme": {"paired"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, body)
	}
	light := "--color-background:#f6f3ea;--color-primary:#2b2a27;--color-accent:#f2b84b;"
	dark := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;"
	rules := "@media not all and (prefers-color-scheme:dark){body{" + light + "}}@media (prefers-color-scheme:dark){body{" + dark + "}}"
	if page := define.GetPageTheme(); page.BodyStyle != "" || string(page.Rules) != rules {
		t.Fatalf("the device chooses by default: %+v", page)
	}

	// In this order: the checks that follow rely on light being in use.
	for _, set := range []struct{ mode, style string }{{model.ThemeModeDark, dark}, {model.ThemeModeLight, light}} {
		mode, style := set.mode, set.style
		if status, body := postTheme(t, app, modeForm(mode)); status != http.StatusOK {
			t.Fatalf("%s: %d %s", mode, status, body)
		}
		if page := define.GetPageTheme(); string(page.BodyStyle) != style || page.Rules != "" {
			t.Fatalf("%s: %+v", mode, page)
		}
		if stored := data.GetThemeMode(); stored != mode {
			t.Fatalf("stored %q, want %q", stored, mode)
		}
		if theme, _ := data.GetThemeSelection(); theme != "paired" {
			t.Fatalf("%s: the selection changed to %q", mode, theme)
		}
	}

	// Anything else is rejected and changes neither the pages nor config.yml.
	stored, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "Dark", "night", "auto;", "light dark", strings.Repeat("a", 4096)} {
		if status, body := postTheme(t, app, modeForm(mode)); status != http.StatusBadRequest {
			t.Errorf("%q: %d %s", mode, status, body)
		}
		if page := define.GetPageTheme(); string(page.BodyStyle) != light {
			t.Errorf("%q: the pages changed: %+v", mode, page)
		}
		if raw, readErr := os.ReadFile("config.yml"); readErr != nil || string(raw) != string(stored) {
			t.Errorf("%q: config.yml changed: %q %v", mode, raw, readErr)
		}
	}

	if status, body := postTheme(t, app, modeForm(model.ThemeModeAuto)); status != http.StatusOK {
		t.Fatalf("auto: %d %s", status, body)
	}
	if page := define.GetPageTheme(); page.BodyStyle != "" || string(page.Rules) != rules {
		t.Fatalf("back to the device: %+v", page)
	}
}

// A failed save leaves the mode the pages are shown in.
func TestFailedThemeModeSavePreservesRuntimeState(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { define.SetThemeMode(model.ThemeModeAuto) })
	if err := os.Remove("config.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	status, _ := postTheme(t, app, modeForm(model.ThemeModeDark))
	if mode, _ := define.ThemeModeState(); status != http.StatusInternalServerError || mode != model.ThemeModeAuto {
		t.Fatalf("failed save status=%d mode=%q", status, mode)
	}
}

func TestThemeFileCardsShowBothSetsOfColors(t *testing.T) {
	base := model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}
	light := model.Palette{Background: "#f6f3ea", Primary: "#2b2a27", Accent: "#f2b84b"}
	themes := []model.Theme{
		{Name: "single", Colors: base},
		{Name: "light-given", Colors: base, Modes: map[string]map[string]string{model.ThemeModeLight: {"background": "#f6f3ea", "primary": "#2b2a27"}}},
		{Name: "dark-given", Colors: light, Modes: map[string]map[string]string{model.ThemeModeDark: {"background": "#10241b", "primary": "#e8f1e4"}}},
		// The colors are the same in both modes; only another token differs.
		{Name: "same-colors", Colors: base, Modes: map[string]map[string]string{model.ThemeModeDark: {"radius": "0"}}},
	}
	cards := themeFileCards("en", themes)
	if len(cards) != len(themes) {
		t.Fatalf("%d cards", len(cards))
	}
	if cards[0].Colors != base || cards[0].Dark != nil {
		t.Errorf("single: %+v", cards[0])
	}
	for _, card := range cards[1:3] {
		if card.Colors != light || card.Dark == nil || *card.Dark != base {
			t.Errorf("%s: %+v %+v", card.Name, card.Colors, card.Dark)
		}
	}
	if cards[3].Colors != base || cards[3].Dark != nil {
		t.Errorf("same-colors: %+v", cards[3])
	}
}

// importRequest builds the upload the import form sends: the action, the
// file under the name a browser gives it, and the box if it is ticked.
func importRequest(t *testing.T, filename, content string, replace bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("action", actionImport); err != nil {
		t.Fatal(err)
	}
	if replace {
		if err := writer.WriteField(importReplace, "1"); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		// Written by hand: the helper of the standard library would clean
		// the name, and a browser is free to send what it likes.
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", `form-data; name="`+importField+`"; filename="`+strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(filename)+`"`)
		header.Set("Content-Type", "application/octet-stream")
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, define.SettingPages.Theme.Path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func uploadTheme(t *testing.T, app *fiber.App, filename, content string, replace bool) int {
	t.Helper()
	res, err := app.Test(importRequest(t, filename, content, replace), fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func storedThemeFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(define.UserThemeDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestThemeFileIsImported(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	config, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}

	// The file is stored under its name, as it was sent, and listed. It is
	// not selected, and config.yml is not written.
	content := "# By hand.\n" + themeFileColors + "  radius: 10px\n"
	if status := uploadTheme(t, app, "forest.yml", content, false); status != http.StatusOK {
		t.Fatalf("import: %d", status)
	}
	stored, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "forest.yml"))
	if err != nil || string(stored) != content {
		t.Fatalf("stored %q %v", stored, err)
	}
	if !define.IsUserTheme("forest") {
		t.Fatal("the imported theme is not in use")
	}
	if name, _ := define.ThemeSnapshot(); name != "blackboard" {
		t.Fatalf("importing selected %q", name)
	}
	if raw, readErr := os.ReadFile("config.yml"); readErr != nil || string(raw) != string(config) {
		t.Fatalf("config.yml changed: %q %v", raw, readErr)
	}

	// The same name again needs the box, and then replaces the file.
	if status := uploadTheme(t, app, "forest.yml", themeFileColors, false); status != http.StatusConflict {
		t.Fatalf("existing file: %d", status)
	}
	if stored, _ = os.ReadFile(filepath.Join(define.UserThemeDirectory, "forest.yml")); string(stored) != content {
		t.Fatalf("an import without the box replaced the file: %q", stored)
	}
	if status := uploadTheme(t, app, "forest.yml", themeFileColors, true); status != http.StatusOK {
		t.Fatalf("replacement: %d", status)
	}
	if stored, _ = os.ReadFile(filepath.Join(define.UserThemeDirectory, "forest.yml")); string(stored) != themeFileColors {
		t.Fatalf("stored after replacing: %q", stored)
	}

	// A browser may send the path the file has on its device. Only the
	// name counts, and the file lands in the themes directory.
	for sent, want := range map[string]string{
		`C:\Users\someone\Desktop\from-windows.yml`: "from-windows.yml",
		"/home/someone/themes/from-unix.yml":        "from-unix.yml",
		"../../escaping.yml":                        "escaping.yml",
		`..\..\backslashes.yml`:                     "backslashes.yml",
	} {
		if status := uploadTheme(t, app, sent, themeFileColors, false); status != http.StatusOK {
			t.Errorf("%q: %d", sent, status)
		}
		if _, statErr := os.Stat(filepath.Join(define.UserThemeDirectory, want)); statErr != nil {
			t.Errorf("%q was not stored as %s: %v", sent, want, statErr)
		}
	}
	got := storedThemeFiles(t)
	want := []string{"backslashes.yml", "escaping.yml", "forest.yml", "from-unix.yml", "from-windows.yml"}
	if !slices.Equal(got, want) {
		t.Fatalf("themes directory: %v", got)
	}
	// Nothing was written beside the directory.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "config.yml" && entry.Name() != define.UserThemeDirectory {
			t.Errorf("unexpected %q in the data directory", entry.Name())
		}
	}
}

func TestThemeFileImportRefusals(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	large := themeFileColors + "#" + strings.Repeat("x", define.MaximumThemeFileBytes)
	for name, tc := range map[string]struct {
		filename, content string
		status            int
	}{
		"no file":                 {"", "", http.StatusBadRequest},
		"other extension":         {"forest.yaml", themeFileColors, http.StatusBadRequest},
		"no extension":            {"forest", themeFileColors, http.StatusBadRequest},
		"extension in capitals":   {"forest.YML", themeFileColors, http.StatusBadRequest},
		"two extensions":          {"forest.yml.txt", themeFileColors, http.StatusBadRequest},
		"name in capitals":        {"Forest.yml", themeFileColors, http.StatusBadRequest},
		"name with a space":       {"my forest.yml", themeFileColors, http.StatusBadRequest},
		"hidden file":             {".forest.yml", themeFileColors, http.StatusBadRequest},
		"only an extension":       {".yml", themeFileColors, http.StatusBadRequest},
		"bundled theme":           {"blackboard.yml", themeFileColors, http.StatusBadRequest},
		"custom colors":           {"custom.yml", themeFileColors, http.StatusBadRequest},
		"long name":               {strings.Repeat("a", 33) + ".yml", themeFileColors, http.StatusBadRequest},
		"too large":               {"forest.yml", large, http.StatusBadRequest},
		"not a theme":             {"forest.yml", "<!doctype html><script>alert(1)</script>", http.StatusBadRequest},
		"value that is not valid": {"forest.yml", themeFileColors + "  radius: 4em\n", http.StatusBadRequest},
		"declaration in a value":  {"forest.yml", themeFileColors + "  font-family: \"x;background:url(//example.test/)\"\n", http.StatusBadRequest},
		"empty file":              {"forest.yml", "", http.StatusBadRequest},
	} {
		if status := uploadTheme(t, app, tc.filename, tc.content, true); status != tc.status {
			t.Errorf("%s: %d, want %d", name, status, tc.status)
		}
		if files := storedThemeFiles(t); len(files) != 0 {
			t.Fatalf("%s: something was stored: %v", name, files)
		}
	}
	if define.IsUserTheme("forest") {
		t.Fatal("a refused file is in use")
	}
}

func TestThemeIsExported(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	app.Get(define.SettingPages.Theme.Path+exportPath, exportTheme)
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	content := "# By hand.\n" + themeFileColors + "  radius: 10px\n"
	writeThemeFile(t, "forest.yml", content)
	export := func(name string) (*http.Response, string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, define.SettingPages.Theme.Path+exportPath+"?name="+url.QueryEscape(name), nil)
		res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res, string(body)
	}

	// A theme file written since the instance started is handed out as it is.
	res, body := export("forest")
	if res.StatusCode != http.StatusOK || body != content {
		t.Fatalf("theme file: %d %q", res.StatusCode, body)
	}
	for header, want := range map[string]string{
		"Content-Disposition":    `attachment; filename="forest.yml"`,
		"Content-Type":           "application/yaml; charset=utf-8",
		"Cache-Control":          "no-store",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := res.Header.Get(header); got != want {
			t.Errorf("%s: %q, want %q", header, got, want)
		}
	}

	// A bundled theme comes under a name it can be imported under.
	res, body = export("tron")
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Disposition") != `attachment; filename="tron-copy.yml"` || !strings.Contains(body, `"#242B33"`) {
		t.Fatalf("bundled theme: %d %q %q", res.StatusCode, res.Header.Get("Content-Disposition"), body)
	}
	if status := uploadTheme(t, app, "tron-copy.yml", body, false); status != http.StatusOK {
		t.Fatalf("importing the copy of a bundled theme: %d", status)
	}

	// The custom colors come as a file once there are any.
	if res, _ = export(model.ThemeCustomName); res.StatusCode != http.StatusNotFound {
		t.Fatalf("custom colors before any are stored: %d", res.StatusCode)
	}
	if status, page := postTheme(t, app, customForm("#10241b", "#e8f1e4", "#f2b84b")); status != http.StatusOK {
		t.Fatalf("save: %d %s", status, page)
	}
	res, body = export(model.ThemeCustomName)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Disposition") != `attachment; filename="custom-colors.yml"` {
		t.Fatalf("custom colors: %d %q", res.StatusCode, res.Header.Get("Content-Disposition"))
	}
	if status := uploadTheme(t, app, "custom-colors.yml", body, false); status != http.StatusOK {
		t.Fatalf("importing the custom colors as a theme file: %d", status)
	}

	// Anything else is not found, and no other file is handed out.
	if err := os.WriteFile("secret.yml", []byte("Secret: value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "missing", "forest.yml", "../secret", "../config", "config", "Forest", "themes/forest", strings.Repeat("a", 4096)} {
		if res, body = export(name); res.StatusCode != http.StatusNotFound || strings.Contains(body, "Secret") {
			t.Errorf("%q: %d %q", name, res.StatusCode, body)
		}
		if res.Header.Get("Content-Disposition") != "" {
			t.Errorf("%q: offered as a download", name)
		}
	}
}
