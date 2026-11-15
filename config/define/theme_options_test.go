package define

import (
	"html/template"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

// userThemeOptions declares one option of each kind.
const userThemeOptions = `options:
  - id: density
    type: enum
    label: { en: Density, zh: 密度 }
    choices:
      - value: normal
      - value: compact
        tokens: { radius: 2px, font-scale: 0.9 }
        layout: { HideAppDescriptions: true }
  - id: frames
    type: boolean
    tokens: { border: "#ffffff" }
    layout: { TitleSize: compact }
  - id: highlight
    type: color
    token: accent
`

// The attributes a page has for the default values of the options above.
const userThemeOptionDefaults = template.HTMLAttr(` data-theme-density="normal" data-theme-frames="false"`)

// chooseThemeOptions writes config.yml with a selected theme and values for
// its options, as an administrator who edits the file would.
func chooseThemeOptions(t *testing.T, theme, options string) {
	t.Helper()
	config := "Theme: " + theme + "\n"
	if options != "" {
		config += "ThemeSettings:\n    " + theme + ":\n        Options:\n" + options
	}
	require.NoError(t, os.WriteFile("config.yml", []byte(config), 0600))
}

// The values of the options of the selected theme reach the pages: as tokens,
// as layout, and as attributes for the stylesheet of the theme.
func TestThemeOptionsReachThePage(t *testing.T) {
	userThemeDirectory(t, "zelda")
	writeUserTheme(t, "zelda.yml", userThemeColors+"  radius: 10px\n"+userThemeOptions)
	UpdatePagePalettes()
	site := func() model.Application {
		t.Helper()
		options, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		return options
	}

	// Without a choice every option has its default: the first value, off,
	// and the color the theme gives the token.
	defaults := PageTheme{BodyStyle: userThemeStyle + "--flare-radius:10px;", Attributes: userThemeOptionDefaults}
	require.Equal(t, defaults, GetPageTheme())
	require.Nil(t, HomepageInEffect(site()))
	name, layout := SelectedThemeLayout()
	require.Equal(t, "zelda", name)
	require.Empty(t, layout)

	declared, values, known := ThemeOptions("zelda")
	require.True(t, known)
	require.Len(t, declared, 3)
	require.Equal(t, map[string]string{"density": "normal", "frames": "false", "highlight": "#f2b84b"}, values)

	// Values in config.yml are taken by the next page, without a restart.
	chooseThemeOptions(t, "zelda", "            density: compact\n            frames: true\n            highlight: \"#00ff88\"\n")
	chosen := PageTheme{
		BodyStyle:  "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;--flare-border:#ffffff;--flare-radius:2px;--flare-font-scale:0.9;",
		Attributes: ` data-theme-density="compact" data-theme-frames="true"`,
	}
	require.Equal(t, chosen, GetPageTheme())
	homepage := HomepageInEffect(site())
	require.NotNil(t, homepage)
	require.True(t, homepage.HideAppDescriptions)
	require.Equal(t, "compact", homepage.TitleSize)
	_, layout = SelectedThemeLayout()
	require.Equal(t, map[string]string{"HideAppDescriptions": "true", "TitleSize": "compact"}, layout)
	_, values, _ = ThemeOptions("zelda")
	require.Equal(t, map[string]string{"density": "compact", "frames": "true", "highlight": "#00ff88"}, values)
	require.Equal(t, "#00ff88", CurrentThemePalette().Accent)

	// A page that shows the theme without selecting it has them as well.
	previewed, known := GetPageThemeFor("zelda")
	require.True(t, known)
	require.Equal(t, chosen, previewed)
	require.True(t, HomepageInEffectFor(site(), "zelda").HideAppDescriptions)

	// A layout option that is turned off for the theme is not applied,
	// whether the theme or a value of an option gives it.
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: zelda\nThemeSettings:\n    zelda:\n        IgnoreLayout: [HideAppDescriptions]\n        Options:\n            density: compact\n            frames: true\n"), 0600))
	homepage = HomepageInEffect(site())
	require.False(t, homepage.HideAppDescriptions)
	require.Equal(t, "compact", homepage.TitleSize)

	// What an option cannot have as a value is not used, and neither is a
	// value for an option the theme does not have: the options have their
	// defaults.
	chooseThemeOptions(t, "zelda", "            density: wide\n            frames: perhaps\n            highlight: red\n            volume: 9\n")
	require.Equal(t, defaults, GetPageTheme())
	require.Nil(t, HomepageInEffect(site()))
	_, values, _ = ThemeOptions("zelda")
	require.Equal(t, map[string]string{"density": "normal", "frames": "false", "highlight": "#f2b84b"}, values)

	// One value that is usable is used beside one that is not.
	chooseThemeOptions(t, "zelda", "            density: compact\n            highlight: \"url(//example.test/)\"\n")
	require.Equal(t, PageTheme{
		BodyStyle:  userThemeStyle + "--flare-radius:2px;--flare-font-scale:0.9;",
		Attributes: ` data-theme-density="compact" data-theme-frames="false"`,
	}, GetPageTheme())

	// The values of another theme are not those of this one.
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: zelda\nThemeSettings:\n    other:\n        Options:\n            density: compact\n"), 0600))
	require.Equal(t, defaults, GetPageTheme())

	// A page without the themes of the directory has nothing of them.
	require.Equal(t, PageTheme{}, GetPageThemeWithoutDirectory())
}

// A theme without options has no attributes and reads no settings for a
// page, and neither has a bundled theme.
func TestThemesWithoutOptions(t *testing.T) {
	userThemeDirectory(t, "plain")
	writeUserTheme(t, "plain.yml", userThemeColors)
	chooseThemeOptions(t, "plain", "            density: compact\n")
	UpdatePagePalettes()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
	declared, values, known := ThemeOptions("plain")
	require.True(t, known)
	require.Empty(t, declared)
	require.Empty(t, values)
	declared, values, known = ThemeOptions("blackboard")
	require.True(t, known)
	require.Empty(t, declared)
	require.Empty(t, values)
	_, _, known = ThemeOptions("missing")
	require.False(t, known)
	page, known := GetPageThemeFor("blackboard")
	require.True(t, known)
	require.Empty(t, page.Attributes)
}

// A token a value of an option sets holds for light and for dark, and the
// setting for the two is offered while the theme still differs between them.
func TestThemeOptionsWithLightAndDark(t *testing.T) {
	modeDirectory(t, "Theme: paired\n")
	writeUserTheme(t, "paired.yml", pairedThemeFile+"options:\n  - id: soft\n    type: boolean\n    tokens: { radius: 16px, muted: \"#909090\" }\n")
	UpdatePagePalettes()
	require.Equal(t, PageTheme{Rules: template.CSS(pairedRules), Attributes: ` data-theme-soft="false"`}, GetPageTheme())
	mode, choice := ThemeModeState()
	require.Equal(t, model.ThemeModeAuto, mode)
	require.True(t, choice)

	chooseThemeOptions(t, "paired", "            soft: true\n")
	light := "--color-background:#f6f3ea;--color-primary:#2b2a27;--color-accent:#f2b84b;--flare-muted:#909090;--flare-radius:16px;"
	dark := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;--flare-muted:#909090;--flare-radius:16px;"
	require.Equal(t, PageTheme{
		Rules:      template.CSS("@media not all and (prefers-color-scheme:dark){body{" + light + "}}@media (prefers-color-scheme:dark){body{" + dark + "}}"),
		Attributes: ` data-theme-soft="true"`,
	}, GetPageTheme())
	_, choice = ThemeModeState()
	require.True(t, choice, "the colors still differ")

	// A theme whose modes differ in one token only is the same in both once
	// a value of an option sets that token.
	writeUserTheme(t, "single.yml", userThemeColors+"modes:\n  light:\n    muted: \"#6b6860\"\n"+
		"options:\n  - id: soft\n    type: boolean\n    tokens: { muted: \"#909090\" }\n")
	chooseThemeOptions(t, "single", "")
	UpdatePagePalettes()
	_, choice = ThemeModeState()
	require.True(t, choice)
	require.NotEmpty(t, GetPageTheme().Rules)
	chooseThemeOptions(t, "single", "            soft: true\n")
	_, choice = ThemeModeState()
	require.False(t, choice)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-muted:#909090;", Attributes: ` data-theme-soft="true"`}, GetPageTheme())

	// A color option that is left alone changes nothing: the theme keeps
	// the color it has for each mode. A color that is picked is the color
	// of both, and the theme is then the same in both.
	writeUserTheme(t, "tinted.yml", userThemeColors+"modes:\n  light:\n    accent: \"#aa5500\"\n"+
		"options:\n  - id: highlight\n    type: color\n    token: accent\n")
	chooseThemeOptions(t, "tinted", "")
	UpdatePagePalettes()
	colors := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:"
	require.Equal(t, PageTheme{
		Rules: template.CSS("@media not all and (prefers-color-scheme:dark){body{" + colors + "#aa5500;}}@media (prefers-color-scheme:dark){body{" + colors + "#f2b84b;}}"),
	}, GetPageTheme())
	_, choice = ThemeModeState()
	require.True(t, choice)
	chooseThemeOptions(t, "tinted", "            highlight: \"#F2B84B\"\n")
	require.NotEmpty(t, GetPageTheme().Rules, "the color of the theme in another case is no other color")
	chooseThemeOptions(t, "tinted", "            highlight: \"#00ff88\"\n")
	require.Equal(t, PageTheme{BodyStyle: template.CSS(colors + "#00ff88;")}, GetPageTheme())
	_, choice = ThemeModeState()
	require.False(t, choice)
}

// Options that are not accepted leave the theme out of use, and parts of
// options for a later release are left out and listed.
func TestThemeOptionsNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "no-values.yml", userThemeColors+"options:\n  - id: density\n    type: enum\n")
	writeUserTheme(t, "background.yml", userThemeColors+"options:\n  - id: paper\n    type: color\n    token: background\n")
	writeUserTheme(t, "value.yml", userThemeColors+"options:\n  - id: soft\n    type: boolean\n    tokens: { radius: 4em }\n")
	writeThemeDirectory(t, "in-directory", userThemeColors+"options:\n  - id: Density\n    type: boolean\n")
	writeUserTheme(t, "later.yml", userThemeColors+"options:\n  - id: volume\n    type: range\n  - id: soft\n    type: boolean\n    tokens: { glow: \"#ffffff\" }\n    help: Rounder corners.\n")
	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"later"}, userThemeNames(themes))
	require.Len(t, themes[0].Options, 1)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "no-values.yml", Reason: ThemeProblemOption, Subject: "options.density.choices"},
		{File: "background.yml", Reason: ThemeProblemOption, Subject: "options.paper.token"},
		{File: "value.yml", Reason: ThemeProblemValue, Subject: "options.soft.tokens.radius"},
		{File: "in-directory/theme.yml", Reason: ThemeProblemOption, Subject: "options[1].id"},
	}, problems)
	require.Equal(t, []ThemeNotice{{File: "later.yml", Unused: model.ThemeFileUnused{Keys: []string{"options.soft.help", "options.volume"}, Tokens: []string{"options.soft.tokens.glow"}}}}, notices)

	// A theme file with options is stored like any other.
	require.NoError(t, StoreUserTheme("added", []byte(userThemeColors+userThemeOptions), false))
	declared, _, known := ThemeOptions("added")
	require.True(t, known)
	require.Len(t, declared, 3)
	require.ErrorIs(t, StoreUserTheme("refused", []byte(userThemeColors+"options:\n  - id: density\n    type: enum\n"), false), model.ErrThemeOption)
}
