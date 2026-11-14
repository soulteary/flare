package theme

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	}
	for locale, want := range map[string][]themeFileCard{
		"en": {
			{Name: "forest", Title: "Forest at dusk", About: "1.2 · Example Author · MIT", Colors: colors},
			{Name: "plain", Title: "plain", Colors: colors},
			{Name: "signed", Title: "signed", About: "Example Author", Colors: colors},
			{Name: "licensed", Title: "Licensed", About: "2 · CC0-1.0", Colors: colors},
		},
		"zh": {
			{Name: "forest", Title: "暮色森林", About: "1.2 · Example Author · MIT", Colors: colors},
			{Name: "plain", Title: "plain", Colors: colors},
			{Name: "signed", Title: "signed", About: "Example Author", Colors: colors},
			{Name: "licensed", Title: "Licensed", About: "2 · CC0-1.0", Colors: colors},
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
			{File: "a.yml", Unused: model.ThemeFileUnused{Keys: []string{"layout", "modes.sepia"}, Tokens: []string{"modes.dark.glow", "tokens.font-scale"}}},
			{File: "b.yml", Unused: model.ThemeFileUnused{Tokens: []string{"tokens.shadow"}}},
		})
	type line struct{ file, detail string }
	got := make([]line, 0, len(lines))
	for _, entry := range lines {
		got = append(got, line{entry.File, entry.Detail})
	}
	want := []line{{"a.yml", "layout, modes.sepia, modes.dark.glow, tokens.font-scale"}, {"b.yml", "tokens.radius"}, {"b.yml", "tokens.shadow"}, {"c.yaml", ""}}
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

	for mode, style := range map[string]string{model.ThemeModeDark: dark, model.ThemeModeLight: light} {
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
