package model

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// basedThemeFile is a theme file that gives only what it has of its own.
const basedThemeFile = "contract: 1\nextends: zelda\n"

// pairedBaseFile is a whole theme with values for light, a layout, options
// and a background, for theme files to be read on.
const pairedBaseFile = validThemeFile + `  radius: 10px
  muted: "#808080"
name: Zelda
version: 1.0.0
author: Example Author
license: MIT
requires: 0.6.0
modes:
  light:
    background: "#ffffff"
    primary: "#222222"
    muted: "#606060"
layout:
  TitleSize: prominent
  BookmarkLayout: sections
background:
  image: assets/paper.png
  dim: 40
options:
  - id: density
    type: enum
    choices:
      - value: normal
      - value: compact
        tokens: { radius: 2px }
  - id: frames
    type: boolean
  - id: highlight
    type: color
    token: accent
`

func readWholeTheme(t *testing.T, written string) ThemeFile {
	t.Helper()
	file, _, err := ReadThemeFile([]byte(written))
	if err != nil {
		t.Fatal(err)
	}
	return file
}

// A theme file names the theme it is based on, and then gives only what it
// has of its own: no color is required, and neither are tokens at all.
func TestThemeFileNamesTheThemeItIsBasedOn(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		base    string
	}{
		"nothing of its own":      {basedThemeFile, "zelda"},
		"a name":                  {basedThemeFile + "name: Zelda Compact\n", "zelda"},
		"one token":               {basedThemeFile + "tokens:\n  radius: 2px\n", "zelda"},
		"one color":               {basedThemeFile + "tokens:\n  accent: \"#00ff88\"\n", "zelda"},
		"a value for one mode":    {basedThemeFile + "modes:\n  light:\n    accent: \"#005533\"\n", "zelda"},
		"a layout option":         {basedThemeFile + "layout:\n  TitleSize: compact\n", "zelda"},
		"an option":               {basedThemeFile + "options:\n  - id: frames\n    type: boolean\n", "zelda"},
		"a color option":          {basedThemeFile + "options:\n  - id: highlight\n    type: color\n    token: accent\n", "zelda"},
		"a name of digits":        {"contract: 1\nextends: 2048\n", "2048"},
		"a name in quotes":        {"contract: 1\nextends: \"zelda\"\n", "zelda"},
		"a bundled theme":         {"contract: 1\nextends: tron\ntokens:\n  accent: \"#00ff88\"\n", "tron"},
		"a name with hyphens":     {"contract: 1\nextends: zelda-at-night\n", "zelda-at-night"},
		"a name of the most":      {"contract: 1\nextends: " + strings.Repeat("a", 32) + "\n", strings.Repeat("a", 32)},
		"the release it needs":    {basedThemeFile + "requires: 0.6.0\n", "zelda"},
		"all three colors":        {basedThemeFile + "tokens:\n  background: \"#000000\"\n  primary: \"#ffffff\"\n  accent: \"#888888\"\n", "zelda"},
		"colors too close, alone": {basedThemeFile + "tokens:\n  primary: \"#1b1b1b\"\n", "zelda"},
	} {
		file, unused, err := ReadThemeFile([]byte(tc.written))
		if err != nil || !unused.Empty() || file.Extends != tc.base {
			t.Errorf("%s: %v, not used %+v, based on %q", name, err, unused, file.Extends)
		}
	}

	// A file that stands on its own is read as before: with its colors, and
	// with nothing it is based on.
	if file := readWholeTheme(t, validThemeFile); file.Extends != "" {
		t.Fatalf("a theme of its own is based on %q", file.Extends)
	}
	if _, _, err := ReadThemeFile([]byte("contract: 1\ntokens:\n  accent: \"#00ff88\"\n")); !errors.Is(err, ErrPaletteColor) {
		t.Fatalf("a theme of its own without its colors: %v", err)
	}
	// Parts for a later release are left out and listed, as in any file.
	_, unused, err := ReadThemeFile([]byte(basedThemeFile + "sounds: []\ntokens:\n  shadow: 1px\n"))
	if err != nil || !reflect.DeepEqual(unused, ThemeFileUnused{Keys: []string{"sounds"}, Tokens: []string{"tokens.shadow"}}) {
		t.Fatalf("parts for a later release: %v %+v", err, unused)
	}
}

func TestThemeFileExtendsNotAccepted(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		subject string
	}{
		"an upper-case letter": {"extends: Zelda\n", "Zelda"},
		"a space":              {"extends: my theme\n", "my theme"},
		"a path":               {"extends: ../zelda\n", "../zelda"},
		"a file name":          {"extends: zelda.yml\n", "zelda.yml"},
		"a directory":          {"extends: zelda/theme\n", "zelda/theme"},
		"a hyphen first":       {"extends: \"-zelda\"\n", "-zelda"},
		"a hyphen last":        {"extends: zelda-\n", "zelda-"},
		"two hyphens":          {"extends: zelda--night\n", "zelda--night"},
		"an underscore":        {"extends: zelda_night\n", "zelda_night"},
		"too long":             {"extends: " + strings.Repeat("a", 33) + "\n", strings.Repeat("a", 32) + "…"},
		"markup":               {"extends: \"<b>zelda</b>\"\n", "<b>zelda</b>"},
		"an address":           {"extends: https://example.test/zelda.yml\n", "https://example.test/zelda.yml"},
		"an empty text":        {"extends: \"\"\n", ""},
		"nothing":              {"extends:\n", "extends"},
		"a list":               {"extends: [zelda]\n", "extends"},
		"two themes":           {"extends:\n  - zelda\n  - tron\n", "extends"},
		"a mapping":            {"extends: {name: zelda}\n", "extends"},
	} {
		_, _, err := ReadThemeFile([]byte("contract: 1\n" + tc.written))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeExtends) || rejection.Subject != tc.subject {
			t.Errorf("%s: %v, want %q", name, err, tc.subject)
		}
	}
	// The same holds for a theme file that is not read from text.
	whole := readWholeTheme(t, validThemeFile)
	whole.Extends = "Zelda"
	if err := whole.Validate(); !errors.Is(err, ErrThemeExtends) {
		t.Fatalf("validated: %v", err)
	}
}

// Each part of a file that is based on another theme is checked as it is in
// any file. What holds only for a whole theme is left to On.
func TestThemeFileBasedOnAnotherIsCheckedInItsParts(t *testing.T) {
	for name, tc := range map[string]struct {
		written string
		reason  error
		subject string
	}{
		"another contract":            {"contract: 2\nextends: zelda\n", ErrThemeContract, "2"},
		"a later release":             {basedThemeFile + "requires: 99.0.0\n", ErrThemeRequires, "99.0.0"},
		"a release that is none":      {basedThemeFile + "requires: soon\n", ErrThemeRequiresValue, "soon"},
		"a token value":               {basedThemeFile + "tokens:\n  radius: 4em\n", ErrThemeTokenValue, "tokens.radius"},
		"a color that is none":        {basedThemeFile + "tokens:\n  accent: red\n", ErrThemeTokenValue, "tokens.accent"},
		"a background that is none":   {basedThemeFile + "tokens:\n  background: \"#fff\"\n", ErrThemeTokenValue, "tokens.background"},
		"a token value of a mode":     {basedThemeFile + "modes:\n  light:\n    radius: 4em\n", ErrThemeTokenValue, "modes.light.radius"},
		"a layout value":              {basedThemeFile + "layout:\n  TitleSize: [compact]\n", ErrThemeSyntax, "layout: "},
		"an option":                   {basedThemeFile + "options:\n  - id: density\n    type: enum\n", ErrThemeOption, "options.density.choices"},
		"a default that is no value":  {basedThemeFile + "options:\n  - id: frames\n    type: boolean\n    default: perhaps\n", ErrThemeOption, "options.frames.default"},
		"a color option, no token":    {basedThemeFile + "options:\n  - id: highlight\n    type: color\n", ErrThemeOption, "options.highlight.token"},
		"a color default that is one": {basedThemeFile + "options:\n  - id: highlight\n    type: color\n    token: accent\n    default: red\n", ErrThemeOption, "options.highlight.default"},
		"a background":                {basedThemeFile + "background:\n  image: paper.png\n", ErrThemeBackground, "background.image"},
		"a dim":                       {basedThemeFile + "background:\n  image: assets/paper.png\n  dim: 0.6\n", ErrThemeBackground, "background.dim"},
		"a name":                      {basedThemeFile + "name: \" Zelda\"\n", ErrThemeText, "name"},
		"tokens that are a list":      {basedThemeFile + "tokens: [radius]\n", ErrThemeSyntax, "tokens: "},
		"no document":                 {"extends: zelda\n", ErrThemeContract, "0"},
	} {
		_, _, err := ReadThemeFileFor([]byte(tc.written), "0.7.0")
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, tc.reason) || !strings.HasPrefix(rejection.Subject, tc.subject) {
			t.Errorf("%s: %v, want %v with %q", name, err, tc.reason, tc.subject)
		}
	}
}

// A file read on the theme it is based on has what that theme gives where it
// gives nothing of its own.
func TestThemeFileOnAnotherTheme(t *testing.T) {
	base := readWholeTheme(t, pairedBaseFile)
	kept := readWholeTheme(t, pairedBaseFile)

	// A file with nothing of its own is the other theme under another name.
	bare, err := readWholeTheme(t, basedThemeFile).On(base)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bare.Tokens, base.Tokens) || !reflect.DeepEqual(bare.Modes, base.Modes) || !reflect.DeepEqual(bare.Layout, base.Layout) ||
		!reflect.DeepEqual(bare.Options, base.Options) || !reflect.DeepEqual(bare.Background, base.Background) {
		t.Fatalf("nothing of its own: %+v", bare)
	}
	// What describes a file is that of the file alone.
	if bare.Extends != "zelda" || bare.Name.Plain != "" || bare.Version != "" || bare.Author != "" || bare.License != "" || bare.Requires != "" {
		t.Fatalf("what describes the file: %+v", bare)
	}
	if theme := bare.Theme("copy"); theme.Extends != "zelda" || theme.DisplayName("en") != "copy" || theme.Colors != base.Theme("zelda").Colors {
		t.Fatalf("as a theme: %+v", theme)
	}

	child := readWholeTheme(t, basedThemeFile+`name: Zelda at night
version: 2.0.0
requires: 0.7.0
tokens:
  accent: "#00ff88"
  radius: 4px
  border: "#333333"
modes:
  light:
    primary: "#111111"
    border: "#cccccc"
  dark:
    muted: "#909090"
layout:
  TitleSize: compact
  DesktopColumns: 6
background:
  image: assets/night.jpg
options:
  - id: frames
    type: boolean
    default: true
    tokens: { border: "#ffffff" }
  - id: tint
    type: color
    token: muted
`)
	whole, err := child.On(base)
	if err != nil {
		t.Fatal(err)
	}
	// A token of the file replaces the token of the other theme, in tokens
	// and in each mode by itself.
	if !reflect.DeepEqual(whole.Tokens, map[string]string{"background": "#1a1a1a", "primary": "#FFFDEA", "accent": "#00ff88", "radius": "4px", "muted": "#808080", "border": "#333333"}) {
		t.Fatalf("tokens %+v", whole.Tokens)
	}
	if !reflect.DeepEqual(whole.Modes, map[string]map[string]string{
		"light": {"background": "#ffffff", "primary": "#111111", "muted": "#606060", "border": "#cccccc"},
		"dark":  {"muted": "#909090"},
	}) {
		t.Fatalf("modes %+v", whole.Modes)
	}
	if !reflect.DeepEqual(whole.Layout, map[string]string{"TitleSize": "compact", "BookmarkLayout": "sections", "DesktopColumns": "6"}) {
		t.Fatalf("layout %+v", whole.Layout)
	}
	// An option of the file stands where the option of that ID stood, and
	// the other options of the file follow.
	ids := make([]string, 0, len(whole.Options))
	for _, option := range whole.Options {
		ids = append(ids, option.ID)
	}
	if !reflect.DeepEqual(ids, []string{"density", "frames", "highlight", "tint"}) || whole.Options[1].Default != "true" || whole.Options[1].Choices[1].Tokens["border"] != "#ffffff" {
		t.Fatalf("options %v %+v", ids, whole.Options[1])
	}
	// The background of the file is the background.
	if whole.Background == nil || *whole.Background != (ThemeBackground{Image: "assets/night.jpg"}) {
		t.Fatalf("background %+v", whole.Background)
	}
	if whole.Name.Plain != "Zelda at night" || whole.Version != "2.0.0" || whole.Requires != "0.7.0" || whole.Author != "" || whole.License != "" {
		t.Fatalf("what describes the file: %+v", whole)
	}
	// A color option without a default has the color the two give its token
	// together: the accent of the file, and the muted color of the other
	// theme.
	theme := whole.Theme("night")
	if theme.Options[2].Default != "#00ff88" || theme.Options[3].Default != "#808080" {
		t.Fatalf("defaults %+v", theme.Options)
	}
	// In a mode, a value the other theme gives for that mode holds over a
	// token the file gives for both.
	if light := theme.In(ThemeModeLight); light.Tokens["muted"] != "#606060" || light.Tokens["border"] != "#cccccc" || light.Tokens["radius"] != "4px" || light.Colors.Accent != "#00ff88" {
		t.Fatalf("in light: %+v %+v", light.Tokens, light.Colors)
	}

	// Neither file is changed, and a change of the tokens, the modes, the
	// layout, the background or the list of options of the result reaches
	// neither of them.
	if !reflect.DeepEqual(base, kept) {
		t.Fatal("the other theme was changed")
	}
	whole.Tokens["accent"], whole.Modes["light"]["background"], whole.Layout["TitleSize"] = "#000000", "#000000", "prominent"
	whole.Background.Dim, whole.Options[0].ID = 99, "changed"
	again, err := child.On(base)
	if err != nil || again.Tokens["accent"] != "#00ff88" || again.Modes["light"]["background"] != "#ffffff" || again.Layout["TitleSize"] != "compact" ||
		again.Background.Dim != 0 || again.Options[0].ID != "density" || !reflect.DeepEqual(base, kept) {
		t.Fatalf("read again: %v %+v", err, again)
	}
	inherited, err := readWholeTheme(t, basedThemeFile).On(base)
	if err != nil {
		t.Fatal(err)
	}
	inherited.Background.Dim = 99
	if base.Background.Dim != 40 {
		t.Fatal("the background of the other theme was changed")
	}
}

// What holds only for the two together is found when the file is read on the
// other theme, and named as it is for a whole theme.
func TestThemeFileOnAnotherThemeIsCheckedAsAWhole(t *testing.T) {
	base := readWholeTheme(t, pairedBaseFile)
	many := "options:\n"
	for i := range MaximumThemeOptions - 2 {
		many += "  - id: option-" + strconv.Itoa(i) + "\n    type: boolean\n"
	}
	for name, tc := range map[string]struct {
		written string
		reason  error
		subject string
	}{
		"a primary color too close to the background below":   {"tokens:\n  primary: \"#2a2a2a\"\n", ErrPaletteContrast, ""},
		"a background too close to the primary color below":   {"tokens:\n  background: \"#fffbe0\"\n", ErrPaletteContrast, ""},
		"a color of a mode too close to one below":            {"modes:\n  light:\n    primary: \"#eeeeee\"\n", ErrPaletteContrast, "modes.light"},
		"a mode the other theme has no colors for":            {"modes:\n  dark:\n    primary: \"#222222\"\n", ErrPaletteContrast, "modes.dark"},
		"a color that suits one mode below and not the other": {"tokens:\n  primary: \"#ffffff\"\n  background: \"#000000\"\nmodes:\n  light:\n    primary: \"#f0f0f0\"\n", ErrPaletteContrast, "modes.light"},
		"more options than a theme can have":                  {many, ErrThemeOption, "options"},
		"a color option for a token neither gives":            {"options:\n  - id: lines\n    type: color\n    token: border\n", ErrThemeOption, "options.lines.default"},
	} {
		file, _, err := ReadThemeFile([]byte(basedThemeFile + tc.written))
		if err != nil {
			t.Errorf("%s: not read: %v", name, err)
			continue
		}
		_, err = file.On(base)
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, tc.reason) || rejection.Subject != tc.subject {
			t.Errorf("%s: %v, want %v with %q", name, err, tc.reason, tc.subject)
		}
	}
	// As many options as a theme can have are accepted, and so is a color
	// option whose token the other theme gives a color.
	most := "options:\n"
	for i := range MaximumThemeOptions - 3 {
		most += "  - id: option-" + strconv.Itoa(i) + "\n    type: boolean\n"
	}
	for name, written := range map[string]string{
		"the most options":             most,
		"a color option for its token": "options:\n  - id: tint\n    type: color\n    token: muted\n",
		"a color for both modes":       "tokens:\n  primary: \"#ffffff\"\nmodes:\n  light:\n    primary: \"#000000\"\n",
	} {
		file := readWholeTheme(t, basedThemeFile+written)
		if _, err := file.On(base); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A theme that has to be used in full, as a bundled theme is, stands on its
// own.
func TestParseThemeFileRefusesAThemeBasedOnAnother(t *testing.T) {
	_, err := ParseThemeFile([]byte(basedThemeFile + "tokens:\n  accent: \"#00ff88\"\n"))
	var rejection *ThemeFileError
	if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeBase) || rejection.Subject != "zelda" {
		t.Fatalf("parsed: %v", err)
	}
}
