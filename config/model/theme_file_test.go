package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const validThemeFile = "contract: 1\ntokens:\n  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n"

func TestParseThemeFileAcceptsColorsAndOptionalTokens(t *testing.T) {
	file, err := ParseThemeFile([]byte(validThemeFile))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("blackboard")
	if theme.Name != "blackboard" || theme.Colors != (Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#5c5c5c"}) {
		t.Fatalf("unexpected theme: %+v", theme)
	}
	if theme.Tokens != nil {
		t.Fatalf("a theme of three colors sets no other token: %+v", theme.Tokens)
	}

	raw := validThemeFile + "  icon: \"#ffffff\"\n  hover: \"#00000033\"\n  radius: 8px\n  font-family: Helvetica Neue, -apple-system, 思源黑体, sans-serif\n"
	file, err = ParseThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	theme = file.Theme("mine")
	want := map[string]string{"icon": "#ffffff", "hover": "#00000033", "radius": "8px", "font-family": "Helvetica Neue, -apple-system, 思源黑体, sans-serif"}
	if len(theme.Tokens) != len(want) {
		t.Fatalf("unexpected tokens: %+v", theme.Tokens)
	}
	for name, value := range want {
		if theme.Tokens[name] != value {
			t.Errorf("token %s: got %q, want %q", name, theme.Tokens[name], value)
		}
	}
	// The three colors stay in Colors; Tokens holds only what is set besides them.
	for _, name := range ThemeTokenNames[:3] {
		if _, present := theme.Tokens[name]; present {
			t.Errorf("color %s repeated in Tokens", name)
		}
	}
}

func TestParseThemeFileRejectsUnusableFiles(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want error
	}{
		"empty":             {"", ErrThemeSyntax},
		"not a mapping":     {"- contract: 1\n", ErrThemeSyntax},
		"tokens as text":    {"contract: 1\ntokens: red\n", ErrThemeSyntax},
		"unknown key":       {validThemeFile + "script: alert(1)\n", ErrThemeSyntax},
		"repeated key":      {validThemeFile + "contract: 1\n", ErrThemeSyntax},
		"no contract":       {strings.TrimPrefix(validThemeFile, "contract: 1\n"), ErrThemeContract},
		"later contract":    {strings.Replace(validThemeFile, "contract: 1", "contract: 2", 1), ErrThemeContract},
		"unknown token":     {validThemeFile + "  shadow: \"#000000\"\n", ErrThemeToken},
		"missing color":     {"contract: 1\ntokens:\n  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n", ErrPaletteColor},
		"unquoted color":    {"contract: 1\ntokens:\n  background: #1a1a1a\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n", ErrThemeTokenValue},
		"low contrast":      {"contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#5c5c5c\"\n", ErrPaletteContrast},
		"bad optional":      {validThemeFile + "  radius: 4em\n", ErrThemeTokenValue},
		"no tokens at all":  {"contract: 1\n", ErrPaletteColor},
		"tokens as a list":  {"contract: 1\ntokens:\n  - \"#1a1a1a\"\n", ErrThemeSyntax},
		"nested token":      {"contract: 1\ntokens:\n  background:\n    dark: \"#1a1a1a\"\n", ErrThemeSyntax},
		"number as a color": {"contract: 1\ntokens:\n  background: 101010\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n", ErrThemeTokenValue},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			file, err := ParseThemeFile([]byte(tc.raw))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if file.Contract != 0 || file.Tokens != nil {
				t.Fatalf("a rejected file must not be returned: %+v", file)
			}
		})
	}
}

// Token values are written into a style attribute without escaping. Every
// value that could close the declaration, the attribute or the element, or
// load a resource, has to be rejected.
func TestThemeTokenValues(t *testing.T) {
	cases := []struct {
		token    string
		accepted []string
		rejected []string
	}{
		{
			token:    TokenIcon,
			accepted: []string{"#000000", "#ABCDEF", "#abcdef"},
			rejected: []string{"", "red", "#fff", "#12345", "#1234567", "#12345678", "#gggggg", " #000000", "#000000 ", "#000000;", "rgb(0,0,0)", "var(--x)", "#000;background:url(//example.test/)"},
		},
		{
			token:    TokenHover,
			accepted: []string{"#000000", "#00000033", "#FFFFFF80"},
			rejected: []string{"", "#0000003", "#000000333", "#000000gg", "rgba(0, 0, 0, 0.2)", "transparent", "#00000033;"},
		},
		{
			token:    TokenRadius,
			accepted: []string{"0", "0px", "1px", "4px", "12px", "32px"},
			rejected: []string{"", "px", "4", "04px", "33px", "100px", "-4px", "4.5px", "4 px", "4PX", "4em", "50%", "4px;", "4px 8px", "calc(4px)", "4px}body{"},
		},
		{
			token:    TokenFontScale,
			accepted: []string{"1", "1.0", "1.00", "0.8", "0.80", "0.9", "0.95", "1.1", "1.25", "1.5", "1.50"},
			rejected: []string{"", "0", "0.7", "0.79", "1.51", "1.6", "2", "10", "1.", ".9", "1.250", "01", "1,2", "1.2em", "120%", "-1", "+1", "1e0", " 1", "1 ", "１", "calc(1)", "1;", "1.1;zoom:2", "var(--x)", "1.a", "1.1a", "0.x", "1.-1"},
		},
		{
			token:    TokenFontFamily,
			accepted: []string{"Roboto", "Roboto, sans-serif", "Helvetica Neue,Arial", "-apple-system, BlinkMacSystemFont, Segoe UI", "思源黑体, PingFang SC, system-ui", "Font_Name-2"},
			rejected: []string{"", ",", "Roboto,", ", Roboto", "Roboto,,Arial", "Helvetica  Neue", "2Fast", "--x", "-", "\"Roboto\"", "'Roboto'", "Roboto;color:red", "Roboto)", "url(x)", "a/b", "a\\b", "Roboto\nArial", "Roboto\tArial", "<b>", "a:b", "a{b}", strings.Repeat("a", 161)},
		},
	}
	for _, tc := range cases {
		check := themeTokenChecks[tc.token]
		for _, value := range tc.accepted {
			if !check(value) {
				t.Errorf("%s: %q should be accepted", tc.token, value)
			}
		}
		for _, value := range tc.rejected {
			if check(value) {
				t.Errorf("%s: %q should be rejected", tc.token, value)
			}
		}
	}
}

func TestEveryThemeTokenHasACheck(t *testing.T) {
	if len(themeTokenChecks) != len(ThemeTokenNames) {
		t.Fatalf("%d checks for %d token names", len(themeTokenChecks), len(ThemeTokenNames))
	}
	for _, name := range ThemeTokenNames {
		if themeTokenChecks[name] == nil {
			t.Errorf("token %q has no check", name)
		}
	}
	if got := ThemeTokenNames[:3]; got[0] != TokenBackground || got[1] != TokenPrimary || got[2] != TokenAccent {
		t.Fatalf("the three colors come first: %v", got)
	}
}

// A file written for a later release stays usable: what this release does not
// know is left out and reported, and the rest is used.
func TestReadThemeFileReportsWhatItDoesNotUse(t *testing.T) {
	raw := validThemeFile + "  shadow: \"#000000\"\n  space: 4px\n  radius: 8px\noptions:\n  - id: density\nbackground:\n  image: wallpaper.jpg\n" +
		"modes:\n  dark:\n    background: \"#000000\"\n    glow: \"#ffffff\"\n  sepia:\n    background: \"#f4ecd8\"\n"
	file, unused, err := ReadThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := unused.Keys; len(got) != 3 || got[0] != "background" || got[1] != "modes.sepia" || got[2] != "options" {
		t.Fatalf("unused keys: %v", got)
	}
	if got := unused.Tokens; len(got) != 3 || got[0] != "modes.dark.glow" || got[1] != "tokens.shadow" || got[2] != "tokens.space" {
		t.Fatalf("unused tokens: %v", got)
	}
	if len(file.Modes) != 1 || len(file.Modes[ThemeModeDark]) != 1 || file.Modes[ThemeModeDark][TokenBackground] != "#000000" {
		t.Fatalf("modes in use: %+v", file.Modes)
	}
	if unused.Empty() {
		t.Fatal("unused parts were reported as none")
	}
	theme := file.Theme("later")
	if len(theme.Tokens) != 1 || theme.Tokens[TokenRadius] != "8px" {
		t.Fatalf("tokens in use: %+v", theme.Tokens)
	}
	if _, kept := file.Tokens["shadow"]; kept {
		t.Fatal("an unknown token was kept")
	}

	// The same file is an error where every part has to be used.
	if _, err = ParseThemeFile([]byte(raw)); !errors.Is(err, ErrThemeSyntax) {
		t.Fatalf("strict reading: %v", err)
	}

	file, unused, err = ReadThemeFile([]byte(validThemeFile))
	if err != nil || !unused.Empty() {
		t.Fatalf("a file of three colors: %v %+v", err, unused)
	}
	if file.Contract != ThemeContract {
		t.Fatalf("contract: %d", file.Contract)
	}

	// What is used still has to be valid, and the unused parts of a rejected
	// file are reported with the rejection.
	file, unused, err = ReadThemeFile([]byte(validThemeFile + "  radius: 4em\noptions: {}\n"))
	if !errors.Is(err, ErrThemeTokenValue) || file.Tokens != nil {
		t.Fatalf("got %v %+v", err, file)
	}
	if len(unused.Keys) != 1 || unused.Keys[0] != "options" {
		t.Fatalf("unused keys of a rejected file: %v", unused.Keys)
	}
}

// The reason a file is rejected names the part of the file, and a file with
// several defects always reports the same one.
func TestThemeFileRejectionNamesItsSubject(t *testing.T) {
	colors := "  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n"
	cases := map[string]struct {
		raw     string
		reason  error
		subject string
	}{
		"contract missing":        {"tokens:\n" + colors, ErrThemeContract, "0"},
		"contract of a later one": {"contract: 3\ntokens:\n" + colors, ErrThemeContract, "3"},
		"token value":             {validThemeFile + "  radius: 4em\n", ErrThemeTokenValue, "tokens.radius"},
		"first of two values":     {validThemeFile + "  radius: 4em\n  icon: red\n  hover: blue\n", ErrThemeTokenValue, "tokens.icon"},
		"value before a missing":  {"contract: 1\ntokens:\n  background: \"#1a1a1a\"\n  radius: 4em\n", ErrThemeTokenValue, "tokens.radius"},
		"missing background":      {"contract: 1\ntokens:\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n", ErrPaletteColor, "tokens.background"},
		"missing primary first":   {"contract: 1\ntokens:\n  background: \"#1a1a1a\"\n", ErrPaletteColor, "tokens.primary"},
		"missing accent":          {"contract: 1\ntokens:\n  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n", ErrPaletteColor, "tokens.accent"},
		"contrast":                {"contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#5c5c5c\"\n", ErrPaletteContrast, ""},
		"value in a mode":         {validThemeFile + "modes:\n  light:\n    radius: 4em\n", ErrThemeTokenValue, "modes.light.radius"},
		"dark before light":       {validThemeFile + "modes:\n  light:\n    radius: 4em\n  dark:\n    icon: red\n", ErrThemeTokenValue, "modes.dark.icon"},
		"contrast in a mode":      {validThemeFile + "modes:\n  light:\n    background: \"#fffdea\"\n", ErrPaletteContrast, "modes.light"},
		"theme before its modes":  {validThemeFile + "  radius: 4em\nmodes:\n  dark:\n    icon: red\n", ErrThemeTokenValue, "tokens.radius"},
		"name":                    {validThemeFile + "name: \" padded\"\n", ErrThemeText, "name"},
		"name in a language":      {validThemeFile + "name:\n  en: fine\n  zh: \"" + strings.Repeat("字", 49) + "\"\n", ErrThemeText, `name: "zh"`},
		"language code":           {validThemeFile + "name:\n  \"not a code\": text\n", ErrThemeText, `name: "not a code"`},
		"version":                 {validThemeFile + "version: \"" + strings.Repeat("1", 65) + "\"\n", ErrThemeText, "version"},
		"author":                  {validThemeFile + "author: \"a\\tb\"\n", ErrThemeText, "author"},
		"license":                 {validThemeFile + "license: \"MIT\\n\"\n", ErrThemeText, "license"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for range 20 {
				_, _, err := ReadThemeFile([]byte(tc.raw))
				var rejection *ThemeFileError
				if !errors.As(err, &rejection) {
					t.Fatalf("got %v, want a *ThemeFileError", err)
				}
				if !errors.Is(err, tc.reason) || rejection.Subject != tc.subject {
					t.Fatalf("got %v with subject %q, want %v with %q", rejection.Reason, rejection.Subject, tc.reason, tc.subject)
				}
				if want := tc.reason.Error(); tc.subject == "" && err.Error() != want || tc.subject != "" && err.Error() != want+": "+tc.subject {
					t.Fatalf("message %q", err.Error())
				}
			}
		})
	}

	// A syntax error carries the message of the parser, with the key when the
	// value of one key has the wrong shape.
	_, _, err := ReadThemeFile([]byte("contract: 1\ntokens:\n  background: \"#1a1a1a\"\n   primary: x\n"))
	var rejection *ThemeFileError
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeSyntax) || !strings.Contains(rejection.Subject, "line ") {
		t.Fatalf("syntax error: %v", err)
	}
	_, _, err = ReadThemeFile([]byte("contract: one\ntokens: {}\n"))
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeSyntax) || !strings.HasPrefix(rejection.Subject, "contract: ") {
		t.Fatalf("wrong shape: %v", err)
	}
	// An unknown token is a rejection of Validate, which strict reading uses.
	err = ThemeFile{Contract: 1, Tokens: map[string]string{"zeta": "1", "shadow": "2"}}.Validate()
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeToken) || rejection.Subject != "tokens.shadow" {
		t.Fatalf("unknown token: %v", err)
	}
}

func TestThemeFileTexts(t *testing.T) {
	raw := validThemeFile + "name: Forest at dusk\nversion: 1.2\nauthor: Example Author\nlicense: MIT\n"
	file, err := ParseThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("forest")
	if theme.Name != "forest" || theme.Version != "1.2" || theme.Author != "Example Author" || theme.License != "MIT" {
		t.Fatalf("texts: %+v", theme)
	}
	for _, locale := range []string{"en", "zh", ""} {
		if got := theme.DisplayName(locale); got != "Forest at dusk" {
			t.Fatalf("display name for %q: %q", locale, got)
		}
	}
	// Without a name, the name of the file is shown.
	file, err = ParseThemeFile([]byte(validThemeFile))
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Theme("plain").DisplayName("en"); got != "plain" {
		t.Fatalf("display name without a name: %q", got)
	}

	// Texts are limited by characters, not bytes, and are otherwise free:
	// markup in them is text, which the pages escape.
	longest := strings.Repeat("名", maximumThemeNameLength)
	file, err = ParseThemeFile([]byte(validThemeFile + "name: " + longest + "\nauthor: \"<b>O'Neil & Co</b>\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.Name.Plain != longest || file.Author != "<b>O'Neil & Co</b>" {
		t.Fatalf("texts: %+v", file)
	}
	for name, text := range map[string]string{
		"too long":         strings.Repeat("a", maximumThemeNameLength+1),
		"leading space":    " name",
		"trailing space":   "name ",
		"line break":       "two\nlines",
		"tab":              "a\tb",
		"escape character": "a\x1bb",
		"deletion":         "a\x7fb",
		"invalid UTF-8":    "a\xffb",
	} {
		if isThemeText(text, maximumThemeNameLength) {
			t.Errorf("%s: %q should be rejected", name, text)
		}
	}
	for _, text := range []string{"", "a", "Forest at dusk", "名字 with words", longest, "O'Neil & Co <b>"} {
		if !isThemeText(text, maximumThemeNameLength) {
			t.Errorf("%q should be accepted", text)
		}
	}
}

func TestThemeNameInLanguages(t *testing.T) {
	file, err := ParseThemeFile([]byte(validThemeFile + "name:\n  en: Solarized Light\n  zh-CN: 日光浅色\n  de: Solarisiert\n"))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("solarized-light")
	for locale, want := range map[string]string{
		"en": "Solarized Light",
		// A regional form answers for its language.
		"zh": "日光浅色",
		"de": "Solarisiert",
		// A language the file does not have falls back to English.
		"fr": "Solarized Light",
		"":   "Solarized Light",
	} {
		if got := theme.DisplayName(locale); got != want {
			t.Errorf("name for %q: got %q, want %q", locale, got, want)
		}
	}

	// Language codes are compared in lower case with hyphens.
	for _, code := range []string{"zh-CN", "zh_CN", "ZH-cn", "zh"} {
		file, err = ParseThemeFile([]byte(validThemeFile + "name:\n  " + code + ": 中文名\n"))
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if got := file.Name.For("zh"); got != "中文名" {
			t.Errorf("%s: name for zh is %q", code, got)
		}
		// Without English, another language is shown instead of nothing,
		// and always the same one.
		if got := file.Name.For("en"); got != "中文名" {
			t.Errorf("%s: name for en is %q", code, got)
		}
	}
	text := ThemeText{Localized: map[string]string{"fr": "Forêt", "de": "Wald", "zh": ""}}
	for range 20 {
		if got := text.For("en"); got != "Wald" {
			t.Fatalf("fallback language: %q", got)
		}
	}
	// An exact entry wins over a regional form, and an empty text is no text.
	text = ThemeText{Plain: "Plain", Localized: map[string]string{"zh": "", "zh-tw": "繁體", "zh-cn": "简体", "en": "English"}}
	if got := text.For("zh"); got != "简体" {
		t.Fatalf("regional forms are tried in order: %q", got)
	}
	if got := text.For("zh-tw"); got != "繁體" {
		t.Fatalf("exact entry: %q", got)
	}
	if got := text.For("ja"); got != "Plain" {
		t.Fatalf("the plain text comes before English: %q", got)
	}
	if got := (ThemeText{}).For("en"); got != "" {
		t.Fatalf("no text: %q", got)
	}
	// "zh" must not answer for a language that merely starts with the same letters.
	text = ThemeText{Localized: map[string]string{"zhx": "wrong"}}
	if got := text.inLanguage("zh"); got != "" {
		t.Fatalf("prefix without a hyphen matched: %q", got)
	}

	for name, raw := range map[string]string{
		"the same language twice": "name:\n  zh-CN: 一\n  zh_cn: 二\n",
		"a list":                  "name:\n  - one\n",
		"a nested mapping":        "name:\n  en:\n    short: x\n",
	} {
		if _, err = ParseThemeFile([]byte(validThemeFile + raw)); !errors.Is(err, ErrThemeSyntax) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestIsThemeName(t *testing.T) {
	for _, name := range []string{"a", "forest", "solarized-light", "theme-2", "2024", "a-b-c", strings.Repeat("a", 32)} {
		if !IsThemeName(name) {
			t.Errorf("%q should be a theme name", name)
		}
	}
	for _, name := range []string{
		"", "Forest", "forest_night", "forest night", "-forest", "forest-", "a--b", "forest.yml", "../config", "a/b", `a\b`,
		".", "..", "主题", "forest\n", "forest;", strings.Repeat("a", 33),
	} {
		if IsThemeName(name) {
			t.Errorf("%q should not be a theme name", name)
		}
	}
}

// Anchors and aliases are YAML like any other. A document that uses them to
// grow is rejected by the parser instead of being expanded.
func TestThemeFileWithAliases(t *testing.T) {
	file, err := ParseThemeFile([]byte("contract: 1\nauthor: &who Example Author\nlicense: *who\nname: &names\n  en: Forest\n  zh: 森林\ntokens: &colors\n  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if file.License != "Example Author" || file.Name.For("zh") != "森林" {
		t.Fatalf("texts: %+v", file)
	}
	_, unused, err := ReadThemeFile([]byte("contract: 1\nshared: &names\n  en: Forest\n  zh: 森林\nname: *names\ntokens:\n  background: \"#1a1a1a\"\n  primary: \"#FFFDEA\"\n  accent: \"#5c5c5c\"\n"))
	if err != nil || len(unused.Keys) != 1 || unused.Keys[0] != "shared" {
		t.Fatalf("an alias to a mapping of names: %v %+v", err, unused)
	}

	// Twelve levels of a list that holds the previous level ten times.
	var bomb strings.Builder
	bomb.WriteString("contract: 1\nlevel0: &level0 [x, x, x, x, x, x, x, x, x, x]\n")
	for level := 1; level < 12; level++ {
		previous := fmt.Sprintf("*level%d", level-1)
		fmt.Fprintf(&bomb, "level%d: &level%d [%s%s]\n", level, level, strings.Repeat(previous+", ", 9), previous)
	}
	for _, key := range []string{"name", "version", "tokens"} {
		_, _, err = ReadThemeFile([]byte(bomb.String() + key + ": *level11\n"))
		if !errors.Is(err, ErrThemeSyntax) {
			t.Errorf("%s as an alias to a growing list: %v", key, err)
		}
	}
	// Unused keys are not expanded at all, so the rest of the file decides.
	if _, _, err = ReadThemeFile([]byte(bomb.String())); !errors.Is(err, ErrPaletteColor) {
		t.Fatalf("a growing list under unknown keys: %v", err)
	}
}

// A theme can replace token values in light and in dark mode. What a mode
// does not replace stays as the theme has it.
func TestThemeFileModes(t *testing.T) {
	raw := validThemeFile + "  radius: 8px\n  muted: \"#808080\"\n" +
		"modes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n    muted: \"#6b6860\"\n    hover: \"#0000001a\"\n"
	file, err := ParseThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("paired")
	if len(theme.Modes) != 1 || len(theme.Modes[ThemeModeLight]) != 4 {
		t.Fatalf("modes: %+v", theme.Modes)
	}

	light := theme.In(ThemeModeLight)
	if light.Colors != (Palette{Background: "#f6f3ea", Primary: "#2b2a27", Accent: "#5c5c5c"}) {
		t.Fatalf("light colors: %+v", light.Colors)
	}
	if len(light.Tokens) != 3 || light.Tokens[TokenRadius] != "8px" || light.Tokens[TokenMuted] != "#6b6860" || light.Tokens[TokenHover] != "#0000001a" {
		t.Fatalf("light tokens: %+v", light.Tokens)
	}
	// The theme without values for a mode is the theme as written.
	dark := theme.In(ThemeModeDark)
	if dark.Colors != theme.Colors || len(dark.Tokens) != 2 || dark.Tokens[TokenMuted] != "#808080" {
		t.Fatalf("dark: %+v", dark)
	}
	for _, resolved := range []Theme{light, dark, theme.In(ThemeModeAuto), theme.In("")} {
		if resolved.Modes != nil || resolved.Name != "paired" {
			t.Fatalf("a theme in a mode has no modes of its own: %+v", resolved)
		}
	}
	// Resolving a mode does not change the theme it came from.
	if theme.Colors.Background != "#1a1a1a" || theme.Tokens[TokenMuted] != "#808080" || len(theme.Tokens) != 2 {
		t.Fatalf("the theme changed: %+v", theme)
	}
	light.Tokens[TokenRadius] = "0"
	if theme.Tokens[TokenRadius] != "8px" || theme.In(ThemeModeLight).Tokens[TokenRadius] != "8px" {
		t.Fatal("a resolved theme shares its tokens with the theme")
	}

	// Both modes, and a mode that sets a token the theme does not set.
	file, err = ParseThemeFile([]byte(validThemeFile + "modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n  dark:\n    accent: \"#ffcc66\"\n    radius: 0\n"))
	if err != nil {
		t.Fatal(err)
	}
	theme = file.Theme("both")
	if theme.Tokens != nil {
		t.Fatalf("tokens of the theme: %+v", theme.Tokens)
	}
	if got := theme.In(ThemeModeLight); got.Colors != (Palette{Background: "#ffffff", Primary: "#222222", Accent: "#5c5c5c"}) || got.Tokens != nil {
		t.Fatalf("light: %+v", got)
	}
	if got := theme.In(ThemeModeDark); got.Colors != (Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#ffcc66"}) || len(got.Tokens) != 1 || got.Tokens[TokenRadius] != "0" {
		t.Fatalf("dark: %+v", got)
	}

	// A mode without values, and a theme without modes, have no modes.
	for _, raw := range []string{validThemeFile, validThemeFile + "modes: {}\n", validThemeFile + "modes:\n  light: {}\n  dark:\n", validThemeFile + "modes:\n"} {
		file, err = ParseThemeFile([]byte(raw))
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if theme = file.Theme("plain"); theme.Modes != nil {
			t.Fatalf("%q: modes %+v", raw, theme.Modes)
		}
	}
}

func TestThemeFileModesAreChecked(t *testing.T) {
	cases := map[string]struct {
		modes string
		want  error
	}{
		"value":                  {"modes:\n  dark:\n    radius: 4em\n", ErrThemeTokenValue},
		"declaration in a color": {"modes:\n  light:\n    background: \"#fff;background:url(//example.test/)\"\n", ErrThemeTokenValue},
		"contrast after merging": {"modes:\n  dark:\n    primary: \"#222222\"\n", ErrPaletteContrast},
		"unknown token":          {"modes:\n  dark:\n    shadow: \"#000000\"\n", ErrThemeToken},
		"unknown mode":           {"modes:\n  sepia:\n    background: \"#f4ecd8\"\n", ErrThemeSyntax},
		"mode as text":           {"modes:\n  dark: \"#000000\"\n", ErrThemeSyntax},
		"modes as a list":        {"modes:\n  - dark\n", ErrThemeSyntax},
		"nested token":           {"modes:\n  dark:\n    background:\n      value: \"#000000\"\n", ErrThemeSyntax},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			file, err := ParseThemeFile([]byte(validThemeFile + tc.modes))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
			if file.Modes != nil || file.Tokens != nil {
				t.Fatalf("a rejected file must not be returned: %+v", file)
			}
		})
	}
	// A theme that is checked without being read from a file has to name
	// only the two modes and only known tokens.
	colors := map[string]string{TokenBackground: "#1a1a1a", TokenPrimary: "#FFFDEA", TokenAccent: "#5c5c5c"}
	var rejection *ThemeFileError
	err := ThemeFile{Contract: 1, Tokens: colors, Modes: map[string]map[string]string{"sepia": {TokenBackground: "#f4ecd8"}}}.Validate()
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeSyntax) || rejection.Subject != `unknown mode "sepia"` {
		t.Fatalf("unknown mode: %v", err)
	}
	err = ThemeFile{Contract: 1, Tokens: colors, Modes: map[string]map[string]string{ThemeModeDark: {"shadow": "#000000"}}}.Validate()
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeToken) || rejection.Subject != "modes.dark.shadow" {
		t.Fatalf("unknown token in a mode: %v", err)
	}
	for _, mode := range []string{ThemeModeAuto, ThemeModeLight, ThemeModeDark} {
		if !IsThemeMode(mode) {
			t.Errorf("%q is a mode setting", mode)
		}
	}
	for _, mode := range []string{"", "Auto", "LIGHT", "night", "light ", "system"} {
		if IsThemeMode(mode) {
			t.Errorf("%q is not a mode setting", mode)
		}
	}
}

// The font scale is written as a number, with or without quotes.
func TestThemeFileFontScale(t *testing.T) {
	for written, want := range map[string]string{"1.1": "1.1", "\"1.25\"": "1.25", "1": "1", "0.9": "0.9", "1.10": "1.10", "1.5": "1.5"} {
		file, err := ParseThemeFile([]byte(validThemeFile + "  font-scale: " + written + "\n"))
		if err != nil {
			t.Fatalf("%s: %v", written, err)
		}
		if got := file.Theme("scaled").Tokens[TokenFontScale]; got != want {
			t.Errorf("%s: token is %q, want %q", written, got, want)
		}
	}
	for _, written := range []string{"2", "0.5", "1.75", "big", "120%", "1.1em", "[1.1]", "true"} {
		_, err := ParseThemeFile([]byte(validThemeFile + "  font-scale: " + written + "\n"))
		if !errors.Is(err, ErrThemeTokenValue) && !errors.Is(err, ErrThemeSyntax) {
			t.Errorf("%s: got %v", written, err)
		}
	}
	// A mode can have a scale of its own, like any other token.
	file, err := ParseThemeFile([]byte(validThemeFile + "  font-scale: 1.1\nmodes:\n  dark:\n    font-scale: 1.2\n"))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("scaled")
	if theme.In(ThemeModeLight).Tokens[TokenFontScale] != "1.1" || theme.In(ThemeModeDark).Tokens[TokenFontScale] != "1.2" {
		t.Fatalf("scale per mode: %+v %+v", theme.Tokens, theme.Modes)
	}
	// It is the last token in the order of the documentation.
	if ThemeTokenNames[len(ThemeTokenNames)-1] != TokenFontScale {
		t.Fatalf("order of the tokens: %v", ThemeTokenNames)
	}
}
