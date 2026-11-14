package model

import (
	"errors"
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
