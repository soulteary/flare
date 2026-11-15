package model

import (
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// themeOptionsFile declares one option of each kind.
const themeOptionsFile = validThemeFile + `options:
  - id: density
    type: enum
    label: { en: Density, zh: 密度 }
    default: compact
    choices:
      - value: normal
        label: { en: Normal, zh: 标准 }
      - value: compact
        label: Compact
        tokens: { radius: 4px, font-scale: 0.9 }
        layout: { HideAppDescriptions: true, DesktopColumns: 8 }
  - id: frames
    type: boolean
    label: Frames
    tokens: { border: "#ffffff" }
    layout: { TitleSize: compact }
  - id: highlight
    type: color
    token: accent
    default: "#ffcc66"
`

func TestThemeFileOptions(t *testing.T) {
	file, unused, err := ReadThemeFile([]byte(themeOptionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !unused.Empty() {
		t.Fatalf("unused %+v", unused)
	}
	want := []ThemeOption{
		{ID: "density", Kind: ThemeOptionEnum, Label: ThemeText{Localized: map[string]string{"en": "Density", "zh": "密度"}}, Default: "compact", Choices: []ThemeOptionChoice{
			{Value: "normal", Label: ThemeText{Localized: map[string]string{"en": "Normal", "zh": "标准"}}},
			{Value: "compact", Label: ThemeText{Plain: "Compact"}, Tokens: map[string]string{"radius": "4px", "font-scale": "0.9"},
				Layout: map[string]string{"HideAppDescriptions": "true", "DesktopColumns": "8"}},
		}},
		{ID: "frames", Kind: ThemeOptionBoolean, Label: ThemeText{Plain: "Frames"}, Default: ThemeOptionOff, Choices: []ThemeOptionChoice{
			{Value: ThemeOptionOff},
			{Value: ThemeOptionOn, Tokens: map[string]string{"border": "#ffffff"}, Layout: map[string]string{"TitleSize": "compact"}},
		}},
		{ID: "highlight", Kind: ThemeOptionColor, Token: "accent", Default: "#ffcc66"},
	}
	if !reflect.DeepEqual(file.Options, want) {
		t.Fatalf("options:\n got %+v\nwant %+v", file.Options, want)
	}
	if theme := file.Theme("zelda"); !reflect.DeepEqual(theme.Options, want) || theme.Attributes != nil {
		t.Fatalf("the theme has %+v with %+v", theme.Options, theme.Attributes)
	}
	// A file without options has none, and so has one whose options are
	// left empty.
	for _, written := range []string{"", "options:\n", "options: []\n"} {
		file, unused, err = ReadThemeFile([]byte(validThemeFile + written))
		if err != nil || len(file.Options) != 0 || !unused.Empty() {
			t.Errorf("%q: %v %+v %+v", written, err, file.Options, unused)
		}
	}
}

// What an option has when the file does not say, and the ways a value can be
// written.
func TestThemeFileOptionDefaults(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(validThemeFile + `options:
  - id: columns
    type: enum
    choices:
      - value: 4
      - value: 6
      - value: auto
  - id: later
    type: enum
    default: 6
    choices: [{value: 4}, {value: 6}]
  - id: frames
    type: boolean
  - id: shadows
    type: boolean
    default: true
  - id: plain
    type: boolean
    default: false
`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, option := range file.Options {
		got[option.ID] = option.Default + " of " + strconv.Itoa(len(option.Choices))
	}
	want := map[string]string{"columns": "4 of 3", "later": "6 of 2", "frames": "false of 2", "shadows": "true of 2", "plain": "false of 2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if file.Options[0].DisplayName("en") != "columns" || file.Options[0].Choices[2].DisplayName("zh") != "auto" {
		t.Fatal("an option and a value without a label are shown by their names")
	}
	labelled, _, err := ReadThemeFile([]byte(themeOptionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if labelled.Options[0].DisplayName("zh") != "密度" || labelled.Options[0].Choices[0].DisplayName("en") != "Normal" || labelled.Options[1].DisplayName("zh") != "Frames" {
		t.Fatal("labels are shown in the interface language")
	}
}

// A color option without a default has the color the theme gives its token.
func TestThemeFileColorOptionDefault(t *testing.T) {
	for name, tc := range map[string]struct {
		tokens, option, want string
	}{
		"the accent of the theme":   {"", "token: accent\n", "#5c5c5c"},
		"a token the theme sets":    {"  muted: \"#808080\"\n", "token: muted\n", "#808080"},
		"a default of its own":      {"  muted: \"#808080\"\n", "token: muted\n    default: \"#909090\"\n", "#909090"},
		"a default left as nothing": {"", "token: accent\n    default: \"\"\n", "#5c5c5c"},
	} {
		file, _, err := ReadThemeFile([]byte(validThemeFile + tc.tokens + "options:\n  - id: highlight\n    type: color\n    " + tc.option))
		if err != nil || len(file.Options) != 1 {
			t.Errorf("%s: %v %+v", name, err, file.Options)
			continue
		}
		// The theme has the default; the file keeps the option as it is
		// written, for a theme that is based on this one.
		theme := file.Theme("zelda")
		if len(theme.Options) != 1 || theme.Options[0].Default != tc.want {
			t.Errorf("%s: the option of the theme is %+v", name, theme.Options)
			continue
		}
		if written := strings.Contains(tc.option, "default: \"#"); (file.Options[0].Default != "") != written {
			t.Errorf("%s: the option of the file is %+v", name, file.Options)
		}
		// With its default the option leaves the colors of the theme as
		// they are.
		if with := theme.WithOptions(nil); tc.want == theme.Colors.Accent && with.Colors != theme.Colors {
			t.Errorf("%s: the colors changed to %+v", name, with.Colors)
		}
	}
}

// A file written for a later release stays usable: an option of a kind this
// release does not have is left out and reported, and so are keys, tokens and
// layout options it does not know.
func TestThemeFileOptionsReportWhatIsNotUsed(t *testing.T) {
	file, unused, err := ReadThemeFile([]byte(validThemeFile + `options:
  - id: volume
    type: range
    minimum: 0
    maximum: 10
  - id: density
    type: enum
    help: How much fits on a page.
    token: accent
    choices:
      - value: normal
        icon: circle
      - value: compact
        tokens: { radius: 4px, space: 2px }
        layout: { CardStyle: flat, DesktopColumns: 12, TitleSize: compact }
  - id: frames
    type: boolean
    choices: [{value: thin}, {value: thick}]
    tokens: { border: "#ffffff", glow: "#000000" }
    layout: { BookmarkLayout: grid }
  - id: highlight
    type: color
    token: accent
    default: "#ffcc66"
    alpha: true
`))
	if err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{
		"options.density.choices.compact.layout.CardStyle", "options.density.choices.compact.layout.DesktopColumns: 12",
		"options.density.choices.normal.icon", "options.density.help", "options.density.token",
		"options.frames.choices", "options.frames.layout.BookmarkLayout: grid", "options.highlight.alpha", "options.volume",
	}
	wantTokens := []string{"options.density.choices.compact.tokens.space", "options.frames.tokens.glow"}
	if !slices.Equal(unused.Keys, wantKeys) || !slices.Equal(unused.Tokens, wantTokens) {
		t.Fatalf("unused:\n got %q\n     %q\nwant %q\n     %q", unused.Keys, unused.Tokens, wantKeys, wantTokens)
	}
	ids := make([]string, 0, len(file.Options))
	for _, option := range file.Options {
		ids = append(ids, option.ID)
	}
	if !slices.Equal(ids, []string{"density", "frames", "highlight"}) {
		t.Fatalf("options in use: %v", ids)
	}
	compact := file.Options[0].Choices[1]
	if !reflect.DeepEqual(compact.Tokens, map[string]string{"radius": "4px"}) || !reflect.DeepEqual(compact.Layout, map[string]string{"TitleSize": "compact"}) {
		t.Fatalf("the value in use: %+v", compact)
	}
	on := file.Options[1].Choices[1]
	if !reflect.DeepEqual(on.Tokens, map[string]string{"border": "#ffffff"}) || len(on.Layout) != 0 {
		t.Fatalf("the option in use: %+v", on)
	}
	// A file that has to be used in full is not accepted with such a part.
	if _, err = ParseThemeFile([]byte(validThemeFile + "options:\n  - id: volume\n    type: range\n")); !errors.Is(err, ErrThemeSyntax) {
		t.Fatalf("in full: %v", err)
	}
}

func TestThemeFileOptionsNotAccepted(t *testing.T) {
	enum := func(rest string) string {
		return "options:\n  - id: density\n    type: enum\n" + rest
	}
	two := "    choices: [{value: normal}, {value: compact}]\n"
	many := "options:\n"
	for i := range MaximumThemeOptions + 1 {
		many += "  - id: option-" + strconv.Itoa(i) + "\n    type: boolean\n"
	}
	values := "    choices:\n"
	for i := range MaximumThemeOptionChoices + 1 {
		values += "      - value: v" + strconv.Itoa(i) + "\n"
	}
	for name, tc := range map[string]struct {
		written string
		reason  error
		subject string
	}{
		// The list and what names an option.
		"not a list":                         {"options: {}\n", ErrThemeSyntax, "options: "},
		"a text":                             {"options: density\n", ErrThemeSyntax, "options: "},
		"a list of texts":                    {"options:\n  - density\n", ErrThemeSyntax, "options: "},
		"too many":                           {many, ErrThemeOption, "options"},
		"no id":                              {"options:\n  - type: boolean\n", ErrThemeOption, "options[1].id"},
		"no id, second":                      {"options:\n  - id: a\n    type: boolean\n  - type: boolean\n", ErrThemeOption, "options[2].id"},
		"an id in upper case":                {"options:\n  - id: Density\n    type: boolean\n", ErrThemeOption, "options[1].id"},
		"an id with a space":                 {"options:\n  - id: my option\n    type: boolean\n", ErrThemeOption, "options[1].id"},
		"an id with markup":                  {"options:\n  - id: \"a\\\" onload=\\\"x\"\n    type: boolean\n", ErrThemeOption, "options[1].id"},
		"an id that is a number":             {"options:\n  - id: 5\n    type: boolean\n", ErrThemeOption, "options[1].id"},
		"an id that is too long":             {"options:\n  - id: " + strings.Repeat("a", 33) + "\n    type: boolean\n", ErrThemeOption, "options[1].id"},
		"an id twice":                        {"options:\n  - id: a\n    type: boolean\n  - id: a\n    type: boolean\n", ErrThemeOption, "options.a"},
		"no type":                            {"options:\n  - id: density\n", ErrThemeOption, "options.density.type"},
		"a type that is a list":              {"options:\n  - id: density\n    type: [enum]\n", ErrThemeOption, "options.density.type"},
		"a label that is long":               {"options:\n  - id: a\n    type: boolean\n    label: " + strings.Repeat("a", 49) + "\n", ErrThemeText, "options.a.label"},
		"a label of two lines":               {"options:\n  - id: a\n    type: boolean\n    label: \"a\\nb\"\n", ErrThemeText, "options.a.label"},
		"a label in a language that is none": {"options:\n  - id: a\n    type: boolean\n    label: { \"e n\": A }\n", ErrThemeText, "options.a.label: \"e n\""},
		"a label that is a list":             {"options:\n  - id: a\n    type: boolean\n    label: [A]\n", ErrThemeSyntax, "options.a.label: "},
		// An option with values.
		"no values":                  {enum(""), ErrThemeOption, "options.density.choices"},
		"one value":                  {enum("    choices: [{value: normal}]\n"), ErrThemeOption, "options.density.choices"},
		"too many values":            {enum(values), ErrThemeOption, "options.density.choices"},
		"values that are no list":    {enum("    choices: normal\n"), ErrThemeSyntax, "options.density.choices: "},
		"a value without a name":     {enum("    choices: [{label: Normal}, {value: compact}]\n"), ErrThemeOption, "options.density.choices[1].value"},
		"a value in upper case":      {enum("    choices: [{value: normal}, {value: Compact}]\n"), ErrThemeOption, "options.density.choices[2].value"},
		"a value with a fraction":    {enum("    choices: [{value: 1}, {value: 1.5}]\n"), ErrThemeOption, "options.density.choices[2].value"},
		"a value that is a list":     {enum("    choices: [{value: [a]}, {value: b}]\n"), ErrThemeOption, "options.density.choices[1].value"},
		"a value twice":              {enum("    choices: [{value: normal}, {value: normal}]\n"), ErrThemeOption, "options.density.choices.normal"},
		"a default that is no value": {enum(two + "    default: wide\n"), ErrThemeOption, "options.density.default"},
		"a default left empty":       {enum(two + "    default:\n"), ErrThemeOption, "options.density.default"},
		"a default that is a list":   {enum(two + "    default: [normal]\n"), ErrThemeOption, "options.density.default"},
		"a value label that is long": {enum("    choices: [{value: normal, label: " + strings.Repeat("a", 49) + "}, {value: compact}]\n"), ErrThemeText, "options.density.choices.normal.label"},
		// What a value changes. The background and the primary color are
		// not for an option: their contrast is checked for the theme.
		"a value that sets the background":   {enum("    choices: [{value: normal}, {value: compact, tokens: {background: \"#000000\"}}]\n"), ErrThemeOption, "options.density.choices.compact.tokens.background"},
		"a value that sets the primary":      {enum("    choices: [{value: normal, tokens: {primary: \"#ffffff\"}}, {value: compact}]\n"), ErrThemeOption, "options.density.choices.normal.tokens.primary"},
		"a token value that is not accepted": {enum("    choices: [{value: normal}, {value: compact, tokens: {radius: 4em}}]\n"), ErrThemeTokenValue, "options.density.choices.compact.tokens.radius"},
		"a token value with a declaration":   {enum("    choices: [{value: normal}, {value: compact, tokens: {font-family: \"x;background:url(//example.test/)\"}}]\n"), ErrThemeTokenValue, "options.density.choices.compact.tokens.font-family"},
		"tokens that are a text":             {enum("    choices: [{value: normal}, {value: compact, tokens: radius}]\n"), ErrThemeSyntax, "options.density.choices.compact.tokens: "},
		"layout that is a text":              {enum("    choices: [{value: normal}, {value: compact, layout: compact}]\n"), ErrThemeSyntax, "options.density.choices.compact.layout: "},
		// A boolean option.
		"a default in words":                                {"options:\n  - id: frames\n    type: boolean\n    default: yes\n", ErrThemeOption, "options.frames.default"},
		"a default in quotes":                               {"options:\n  - id: frames\n    type: boolean\n    default: \"true\"\n", ErrThemeOption, "options.frames.default"},
		"a default that is a number":                        {"options:\n  - id: frames\n    type: boolean\n    default: 1\n", ErrThemeOption, "options.frames.default"},
		"an option that sets the background":                {"options:\n  - id: frames\n    type: boolean\n    tokens: {background: \"#000000\"}\n", ErrThemeOption, "options.frames.tokens.background"},
		"an option with a token value that is not accepted": {"options:\n  - id: frames\n    type: boolean\n    tokens: {border: red}\n", ErrThemeTokenValue, "options.frames.tokens.border"},
		"an option with tokens that are a list":             {"options:\n  - id: frames\n    type: boolean\n    tokens: [border]\n", ErrThemeSyntax, "options.frames.tokens: "},
		// A color option.
		"no token":                                {"options:\n  - id: highlight\n    type: color\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"the background":                          {"options:\n  - id: highlight\n    type: color\n    token: background\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"the primary color":                       {"options:\n  - id: highlight\n    type: color\n    token: primary\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"a token that is no color":                {"options:\n  - id: highlight\n    type: color\n    token: radius\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"a token that can be translucent":         {"options:\n  - id: highlight\n    type: color\n    token: hover\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"a token that is not known":               {"options:\n  - id: highlight\n    type: color\n    token: shadow\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"a token that is a list":                  {"options:\n  - id: highlight\n    type: color\n    token: [accent]\n    default: \"#ffcc66\"\n", ErrThemeOption, "options.highlight.token"},
		"no default color, and none of the theme": {"options:\n  - id: highlight\n    type: color\n    token: icon\n", ErrThemeOption, "options.highlight.default"},
		"a default that is a name":                {"options:\n  - id: highlight\n    type: color\n    token: accent\n    default: red\n", ErrThemeOption, "options.highlight.default"},
		"a default of three digits":               {"options:\n  - id: highlight\n    type: color\n    token: accent\n    default: \"#fc6\"\n", ErrThemeOption, "options.highlight.default"},
		"a default that is translucent":           {"options:\n  - id: highlight\n    type: color\n    token: accent\n    default: \"#ffcc6680\"\n", ErrThemeOption, "options.highlight.default"},
	} {
		_, _, err := ReadThemeFile([]byte(validThemeFile + tc.written))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) {
			t.Errorf("%s: accepted (%v)", name, err)
			continue
		}
		exact := !errors.Is(tc.reason, ErrThemeSyntax)
		if !errors.Is(err, tc.reason) || !strings.HasPrefix(rejection.Subject, tc.subject) || (exact && rejection.Subject != tc.subject) {
			t.Errorf("%s: got %v with %q, want %v with %q", name, rejection.Reason, rejection.Subject, tc.reason, tc.subject)
		}
	}

	// The limits themselves are accepted.
	full := "options:\n"
	for i := range MaximumThemeOptions {
		full += "  - id: option-" + strconv.Itoa(i) + "\n    type: boolean\n"
	}
	most := "    choices:\n"
	for i := range MaximumThemeOptionChoices {
		most += "      - value: v" + strconv.Itoa(i) + "\n"
	}
	for name, written := range map[string]string{
		"as many options as a theme can have":  full,
		"as many values as an option can have": enum(most),
		"a name of the greatest length":        "options:\n  - id: " + strings.Repeat("a", 32) + "\n    type: boolean\n    label: " + strings.Repeat("字", 48) + "\n",
	} {
		if _, _, err := ReadThemeFile([]byte(validThemeFile + written)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The same holds for options that are not read from a file.
func TestThemeOptionsAreValidated(t *testing.T) {
	valid, _, err := ReadThemeFile([]byte(themeOptionsFile))
	if err != nil {
		t.Fatal(err)
	}
	change := func(edit func(options []ThemeOption)) error {
		file := valid
		file.Options = slices.Clone(valid.Options)
		for i := range file.Options {
			file.Options[i].Choices = slices.Clone(file.Options[i].Choices)
		}
		edit(file.Options)
		return file.Validate()
	}
	for name, tc := range map[string]struct {
		edit    func(options []ThemeOption)
		reason  error
		subject string
	}{
		"a kind that is not known":         {func(o []ThemeOption) { o[0].Kind = "range" }, ErrThemeOption, "options.density.type"},
		"a boolean with three values":      {func(o []ThemeOption) { o[1].Choices = append(o[1].Choices, ThemeOptionChoice{Value: "maybe"}) }, ErrThemeOption, "options.frames"},
		"a boolean with other values":      {func(o []ThemeOption) { o[1].Choices[0].Value = "off" }, ErrThemeOption, "options.frames"},
		"a default no value has":           {func(o []ThemeOption) { o[0].Default = "wide" }, ErrThemeOption, "options.density.default"},
		"a boolean default":                {func(o []ThemeOption) { o[1].Default = "yes" }, ErrThemeOption, "options.frames.default"},
		"a color default":                  {func(o []ThemeOption) { o[2].Default = "red" }, ErrThemeOption, "options.highlight.default"},
		"a layout value that is not known": {func(o []ThemeOption) { o[0].Choices[1].Layout = map[string]string{"DesktopColumns": "12"} }, ErrThemeSyntax, "options.density.choices.compact.layout.DesktopColumns"},
		"a layout value of a boolean":      {func(o []ThemeOption) { o[1].Choices[1].Layout = map[string]string{"TitleSize": "huge"} }, ErrThemeSyntax, "options.frames.layout.TitleSize"},
		"a token that is not known":        {func(o []ThemeOption) { o[0].Choices[1].Tokens = map[string]string{"space": "2px"} }, ErrThemeOption, "options.density.choices.compact.tokens.space"},
	} {
		refused := change(tc.edit)
		var rejection *ThemeFileError
		if !errors.As(refused, &rejection) || !errors.Is(refused, tc.reason) || rejection.Subject != tc.subject {
			t.Errorf("%s: %v", name, refused)
		}
	}
	if err = change(func([]ThemeOption) {}); err != nil {
		t.Fatal(err)
	}
}

// An option has the value that is stored for it if it can have that value,
// and its default otherwise.
func TestThemeOptionValue(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(themeOptionsFile))
	if err != nil {
		t.Fatal(err)
	}
	density, frames, highlight := file.Options[0], file.Options[1], file.Options[2]
	for _, tc := range []struct {
		option ThemeOption
		stored string
		want   string
	}{
		{density, "normal", "normal"}, {density, "compact", "compact"}, {density, "", "compact"}, {density, "wide", "compact"},
		{density, "Normal", "compact"}, {density, "normal ", "compact"}, {density, "true", "compact"},
		{frames, "true", "true"}, {frames, "false", "false"}, {frames, "", "false"}, {frames, "yes", "false"}, {frames, "1", "false"},
		{frames, "TRUE", "false"}, {frames, "compact", "false"},
		{highlight, "#112233", "#112233"}, {highlight, "#AABBCC", "#AABBCC"}, {highlight, "", "#ffcc66"}, {highlight, "red", "#ffcc66"},
		{highlight, "#abc", "#ffcc66"}, {highlight, "#11223344", "#ffcc66"}, {highlight, "#112233;color:red", "#ffcc66"},
		{highlight, "url(//example.test/)", "#ffcc66"}, {highlight, "true", "#ffcc66"},
	} {
		if got := tc.option.Value(tc.stored); got != tc.want {
			t.Errorf("%s with %q: %q, want %q", tc.option.ID, tc.stored, got, tc.want)
		}
	}
}

// Only values other than the defaults are stored, so what is the default of
// an option is decided here. A color is the same color in either case: a
// browser submits one in lower case, whatever the case of the theme file.
func TestThemeOptionIsDefault(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(strings.Replace(themeOptionsFile, "#ffcc66", "#FFCC66", 1)))
	if err != nil {
		t.Fatal(err)
	}
	density, frames, highlight := file.Options[0], file.Options[1], file.Options[2]
	if highlight.Default != "#FFCC66" {
		t.Fatalf("the default is kept as it is written: %q", highlight.Default)
	}
	for _, tc := range []struct {
		option ThemeOption
		value  string
		want   bool
	}{
		{density, "compact", true}, {density, "normal", false}, {density, "Compact", false}, {density, "", false},
		{frames, "false", true}, {frames, "true", false}, {frames, "FALSE", false}, {frames, "", false},
		{highlight, "#FFCC66", true}, {highlight, "#ffcc66", true}, {highlight, "#FfCc66", true},
		{highlight, "#ffcc67", false}, {highlight, "#FFCC67", false}, {highlight, "", false}, {highlight, "ffcc66", false},
	} {
		if got := tc.option.IsDefault(tc.value); got != tc.want {
			t.Errorf("%s with %q: %v, want %v", tc.option.ID, tc.value, got, tc.want)
		}
	}
}

// A color option that has the color the theme gives its token changes
// nothing: the theme keeps what it has for light and for dark until another
// color is picked. A color that is picked holds for both.
func TestThemeColorOptionLeavesTheThemeUntilAColorIsPicked(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(validThemeFile + "  muted: \"#808080\"\n" +
		"modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    accent: \"#AA5500\"\n    muted: \"#606060\"\n" +
		"options:\n  - id: highlight\n    type: color\n    token: accent\n  - id: tint\n    type: color\n    token: muted\n"))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("zelda")
	for name, chosen := range map[string]map[string]string{
		"nothing":                               nil,
		"the colors of the theme":               {"highlight": "#5c5c5c", "tint": "#808080"},
		"the colors of the theme in upper case": {"highlight": "#5C5C5C", "tint": "#808080"},
		"what is no color":                      {"highlight": "red", "tint": ""},
	} {
		with := theme.WithOptions(chosen)
		if with.Colors != theme.Colors || !reflect.DeepEqual(with.Tokens, theme.Tokens) || !reflect.DeepEqual(with.Modes, theme.Modes) {
			t.Errorf("%s: the theme changed: %+v %+v %+v", name, with.Colors, with.Tokens, with.Modes)
		}
		light, dark := with.In(ThemeModeLight), with.In(ThemeModeDark)
		if light.Colors.Accent != "#AA5500" || light.Tokens["muted"] != "#606060" || dark.Colors.Accent != "#5c5c5c" || dark.Tokens["muted"] != "#808080" {
			t.Errorf("%s: light %s %s, dark %s %s", name, light.Colors.Accent, light.Tokens["muted"], dark.Colors.Accent, dark.Tokens["muted"])
		}
	}

	// A color that is picked for one token holds for light and for dark, and
	// the other token keeps what the theme has for each.
	picked := theme.WithOptions(map[string]string{"highlight": "#00ff88"})
	light, dark := picked.In(ThemeModeLight), picked.In(ThemeModeDark)
	if light.Colors.Accent != "#00ff88" || dark.Colors.Accent != "#00ff88" || light.Tokens["muted"] != "#606060" || dark.Tokens["muted"] != "#808080" {
		t.Fatalf("a picked color: light %s %s, dark %s %s", light.Colors.Accent, light.Tokens["muted"], dark.Colors.Accent, dark.Tokens["muted"])
	}
	// The color the theme has for one mode is a color like any other.
	other := theme.WithOptions(map[string]string{"highlight": "#aa5500", "tint": "#606060"})
	light, dark = other.In(ThemeModeLight), other.In(ThemeModeDark)
	if dark.Colors.Accent != "#aa5500" || light.Colors.Accent != "#aa5500" || dark.Tokens["muted"] != "#606060" || light.Tokens["muted"] != "#606060" {
		t.Fatalf("the color of a mode: light %s %s, dark %s %s", light.Colors.Accent, light.Tokens["muted"], dark.Colors.Accent, dark.Tokens["muted"])
	}

	// A default of its own that is another color than that of the theme is
	// a color the option gives the token, for light and for dark.
	own, _, err := ReadThemeFile([]byte(validThemeFile + "modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    accent: \"#AA5500\"\n" +
		"options:\n  - id: highlight\n    type: color\n    token: accent\n    default: \"#ffcc66\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	defaults := own.Theme("zelda").WithOptions(nil)
	if defaults.In(ThemeModeLight).Colors.Accent != "#ffcc66" || defaults.In(ThemeModeDark).Colors.Accent != "#ffcc66" {
		t.Fatalf("a default of its own: %+v", defaults)
	}
}

// The values of the options change the theme: its tokens, also where it has
// values for light and dark, its layout, and what a page writes on its body.
func TestThemeWithOptions(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(validThemeFile + "  radius: 10px\n  muted: \"#808080\"\n" +
		"modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    accent: \"#888888\"\n    radius: 12px\n    muted: \"#606060\"\n" +
		"layout:\n  TitleSize: prominent\n  BookmarkLayout: sections\n" + strings.TrimPrefix(themeOptionsFile, validThemeFile)))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("zelda")
	original := file.Theme("zelda")

	// The defaults: density is compact, frames are off, and the highlight
	// has the color the file gives.
	defaults := theme.WithOptions(nil)
	if !reflect.DeepEqual(defaults.Attributes, []ThemeAttribute{{"density", "compact"}, {"frames", "false"}}) {
		t.Fatalf("attributes %+v", defaults.Attributes)
	}
	if !reflect.DeepEqual(defaults.Tokens, map[string]string{"radius": "4px", "font-scale": "0.9", "muted": "#808080"}) || defaults.Colors.Accent != "#ffcc66" {
		t.Fatalf("tokens %+v with the accent %s", defaults.Tokens, defaults.Colors.Accent)
	}
	if !reflect.DeepEqual(defaults.Layout, map[string]string{"TitleSize": "prominent", "BookmarkLayout": "sections", "HideAppDescriptions": "true", "DesktopColumns": "8"}) {
		t.Fatalf("layout %+v", defaults.Layout)
	}
	// A token a value sets holds for light and for dark; a token no value
	// sets keeps what the theme has for each.
	light := defaults.In(ThemeModeLight)
	if light.Tokens["radius"] != "4px" || light.Colors.Accent != "#ffcc66" || light.Tokens["muted"] != "#606060" || light.Colors.Background != "#ffffff" {
		t.Fatalf("in light: %+v %+v", light.Tokens, light.Colors)
	}
	dark := defaults.In(ThemeModeDark)
	if dark.Tokens["radius"] != "4px" || dark.Colors.Accent != "#ffcc66" || dark.Tokens["muted"] != "#808080" || dark.Colors.Background != "#1a1a1a" {
		t.Fatalf("in dark: %+v %+v", dark.Tokens, dark.Colors)
	}

	// Values that are chosen. A value that changes nothing leaves the
	// tokens and the layout of the theme, and a layout option of a value
	// holds over that of the theme.
	chosen := theme.WithOptions(map[string]string{"density": "normal", "frames": "true", "highlight": "#00ff88"})
	if !reflect.DeepEqual(chosen.Attributes, []ThemeAttribute{{"density", "normal"}, {"frames", "true"}}) {
		t.Fatalf("attributes %+v", chosen.Attributes)
	}
	if !reflect.DeepEqual(chosen.Tokens, map[string]string{"radius": "10px", "muted": "#808080", "border": "#ffffff"}) || chosen.Colors.Accent != "#00ff88" {
		t.Fatalf("tokens %+v with the accent %s", chosen.Tokens, chosen.Colors.Accent)
	}
	if !reflect.DeepEqual(chosen.Layout, map[string]string{"TitleSize": "compact", "BookmarkLayout": "sections"}) {
		t.Fatalf("layout %+v", chosen.Layout)
	}
	if chosen.In(ThemeModeLight).Tokens["radius"] != "12px" {
		t.Fatal("a token no value sets keeps the value of the mode")
	}

	// What cannot be a value of an option is not used, and neither is a
	// value for an option the theme does not have.
	stale := theme.WithOptions(map[string]string{"density": "wide", "frames": "yes", "highlight": "red", "volume": "9", "": "x"})
	if !reflect.DeepEqual(stale.Attributes, defaults.Attributes) || !reflect.DeepEqual(stale.Tokens, defaults.Tokens) || stale.Colors != defaults.Colors || !reflect.DeepEqual(stale.Layout, defaults.Layout) {
		t.Fatalf("values that are not usable: %+v", stale)
	}

	// The theme itself is not changed.
	if !reflect.DeepEqual(theme, original) {
		t.Fatalf("the theme was changed: %+v", theme)
	}

	// A theme without options is what it is.
	plain, _, err := ReadThemeFile([]byte(validThemeFile + "  radius: 10px\nlayout:\n  TitleSize: compact\n"))
	if err != nil {
		t.Fatal(err)
	}
	bare := plain.Theme("plain")
	if with := bare.WithOptions(map[string]string{"density": "compact"}); !reflect.DeepEqual(with, bare) || with.Attributes != nil {
		t.Fatalf("a theme without options: %+v", with)
	}

	// A theme with options and nothing else for them to change.
	only, _, err := ReadThemeFile([]byte(validThemeFile + "options:\n  - id: frames\n    type: boolean\n    default: true\n    tokens: { radius: 2px }\n    layout: { TitleSize: compact }\n"))
	if err != nil {
		t.Fatal(err)
	}
	on := only.Theme("only").WithOptions(nil)
	if !reflect.DeepEqual(on.Tokens, map[string]string{"radius": "2px"}) || !reflect.DeepEqual(on.Layout, map[string]string{"TitleSize": "compact"}) {
		t.Fatalf("an option that is on: %+v %+v", on.Tokens, on.Layout)
	}
	off := only.Theme("only").WithOptions(map[string]string{"frames": "false"})
	if len(off.Tokens) != 0 || len(off.Layout) != 0 || !reflect.DeepEqual(off.Attributes, []ThemeAttribute{{"frames", "false"}}) {
		t.Fatalf("an option that is off: %+v", off)
	}
}

// Where two options set the same token or layout option, the later one in
// the file holds.
func TestThemeOptionsInTheOrderOfTheFile(t *testing.T) {
	file, _, err := ReadThemeFile([]byte(validThemeFile + `options:
  - id: first
    type: boolean
    default: true
    tokens: { radius: 2px, muted: "#808080" }
    layout: { TitleSize: compact, BookmarkLayout: sections }
  - id: second
    type: boolean
    default: true
    tokens: { radius: 6px }
    layout: { TitleSize: prominent }
  - id: tint
    type: color
    token: muted
    default: "#909090"
`))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("zelda")
	both := theme.WithOptions(nil)
	if both.Tokens["radius"] != "6px" || both.Tokens["muted"] != "#909090" || both.Layout["TitleSize"] != "prominent" || both.Layout["BookmarkLayout"] != "sections" {
		t.Fatalf("both on: %+v %+v", both.Tokens, both.Layout)
	}
	first := theme.WithOptions(map[string]string{"second": "false", "tint": "#a0a0a0"})
	if first.Tokens["radius"] != "2px" || first.Tokens["muted"] != "#a0a0a0" || first.Layout["TitleSize"] != "compact" {
		t.Fatalf("the first on: %+v %+v", first.Tokens, first.Layout)
	}
}
