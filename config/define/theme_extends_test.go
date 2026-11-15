package define

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
	version "github.com/soulteary/version-kit/v4"
)

// The theme the themes of these tests are based on: a directory with a
// stylesheet, an image for the background, and an option.
const (
	zeldaThemeFile = userThemeColors + "  radius: 10px\n  muted: \"#808080\"\nname: Zelda\n" + userThemeBackground +
		"options:\n  - id: frames\n    type: boolean\n"
	zeldaStylesheet = ".app-item { border-radius: var(--flare-radius); }\n"
	basedOnZelda    = "contract: 1\nextends: zelda\n"
)

// writeZeldaTheme writes the theme the others are based on.
func writeZeldaTheme(t *testing.T) {
	t.Helper()
	writeThemeDirectory(t, "zelda", zeldaThemeFile)
	writeThemeStylesheet(t, "zelda", zeldaStylesheet)
	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG)
}

// stylesheetAddress is the address the stylesheet of a theme in use is
// served at.
func stylesheetAddress(t *testing.T, name string) string {
	t.Helper()
	theme := userTheme(t, name)
	require.NotEmpty(t, theme.Stylesheet, name)
	return "/theme/" + name + "/" + theme.Hash + "/theme.css"
}

// A theme can be based on another: a theme of the directory, as a file or as
// a directory, or a bundled theme. It has what that theme gives where it
// gives nothing of its own, a page loads the stylesheets of the themes below
// it before its own, and an image it does not replace comes from the theme
// that brings it.
func TestThemeBasedOnAnotherTheme(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeUserTheme(t, "compact.yml", basedOnZelda+"name: Zelda Compact\ntokens:\n  radius: 2px\n")
	writeThemeDirectory(t, "night", "contract: 1\nextends: compact\ntokens:\n  accent: \"#00ff88\"\n")
	writeThemeStylesheet(t, "night", ".app-title { letter-spacing: 1px }\n")
	writeThemeDirectory(t, "deep", "contract: 1\nextends: night\nbackground:\n  image: assets/own.png\n")
	writeThemeStylesheet(t, "deep", ".app-item { background-image: url(assets/own.png) }\n")
	writeThemeAsset(t, "deep", "own.png", assetPNG)
	writeUserTheme(t, "soft.yml", "contract: 1\nextends: tron\ntokens:\n  accent: \"#ff8800\"\n  radius: 6px\n")
	writeUserTheme(t, "plain.yml", userThemeColors)
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	writeUserTheme(t, "on-styled.yml", "contract: 1\nextends: styled\n")
	ReloadUserThemes()

	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"compact", "deep", "night", "on-styled", "plain", "soft", "styled", "zelda"}, userThemeNames(themes))
	require.Empty(t, problems)
	require.Empty(t, notices)
	zelda := userTheme(t, "zelda")
	source := model.ThemeSource{Name: "zelda", Hash: zelda.Hash}
	zeldaSheet := stylesheetAddress(t, "zelda")
	wallpaper := backgroundRule("/theme/zelda/"+zelda.Hash+"/assets/wallpaper.jpg", "60")
	frames := template.HTMLAttr(` data-theme-frames="false"`)

	// The theme the others are based on is what it was.
	require.Empty(t, zelda.Extends)
	require.Empty(t, zelda.Inherited)
	require.Equal(t, model.ThemeSource{}, zelda.BackgroundSource)
	page, known := GetPageThemeFor("zelda")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-muted:#808080;--flare-radius:10px;", Stylesheet: zeldaSheet, Background: wallpaper, Attributes: frames}, page)

	// A single file based on a directory: its token over those below, the
	// stylesheet and the image of the directory, and the option.
	compact := userTheme(t, "compact")
	require.Equal(t, "zelda", compact.Extends)
	require.False(t, compact.Directory)
	require.Empty(t, compact.Stylesheet)
	require.Empty(t, compact.Hash)
	require.Equal(t, []model.ThemeSource{source}, compact.Inherited)
	require.Equal(t, source, compact.BackgroundSource)
	require.Equal(t, "Zelda Compact", compact.DisplayName("en"))
	page, known = GetPageThemeFor("compact")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-muted:#808080;--flare-radius:2px;", Inherited: []string{zeldaSheet}, Background: wallpaper, Attributes: frames}, page)

	// A directory based on that file: the stylesheet of the directory two
	// below first, then its own.
	night := userTheme(t, "night")
	require.Equal(t, "compact", night.Extends)
	require.Equal(t, []model.ThemeSource{source}, night.Inherited)
	require.Equal(t, source, night.BackgroundSource)
	require.Equal(t, "night", night.DisplayName("en"), "the name of the theme below is not its name")
	nightSheet := stylesheetAddress(t, "night")
	page, known = GetPageThemeFor("night")
	require.True(t, known)
	require.Equal(t, PageTheme{
		BodyStyle:  "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;--flare-muted:#808080;--flare-radius:2px;",
		Stylesheet: nightSheet, Inherited: []string{zeldaSheet}, Background: wallpaper, Attributes: frames,
	}, page)

	// Three themes below, two of them with a stylesheet, in the order from
	// the one that stands on its own. An image of its own is its own.
	deep := userTheme(t, "deep")
	require.Equal(t, []model.ThemeSource{source, {Name: "night", Hash: night.Hash}}, deep.Inherited)
	require.Equal(t, model.ThemeSource{}, deep.BackgroundSource)
	page, known = GetPageThemeFor("deep")
	require.True(t, known)
	require.Equal(t, []string{zeldaSheet, nightSheet}, page.Inherited)
	require.Equal(t, stylesheetAddress(t, "deep"), page.Stylesheet)
	require.Equal(t, backgroundRule("/theme/deep/"+deep.Hash+"/assets/own.png", ""), page.Background)

	// A bundled theme has colors to be based on, and nothing else.
	soft := userTheme(t, "soft")
	require.Equal(t, "tron", soft.Extends)
	require.Empty(t, soft.Inherited)
	page, known = GetPageThemeFor("soft")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#ff8800;--flare-radius:6px;"}, page)

	// Each theme serves what it brings under its own name, and a theme
	// brings nothing of the themes below it.
	_, served := ThemeStylesheet("zelda", zelda.Hash)
	require.True(t, served)
	_, served = ThemeStylesheet("compact", zelda.Hash)
	require.False(t, served)
	_, _, served = readThemeAsset(t, "night", zelda.Hash, "wallpaper.jpg")
	require.False(t, served)
	content, _, served := readThemeAsset(t, "zelda", zelda.Hash, "wallpaper.jpg")
	require.True(t, served)
	require.Equal(t, assetJPEG, content)

	// Selected, such a theme is what a page gets.
	SetTheme("night")
	require.Equal(t, []string{zeldaSheet}, GetPageTheme().Inherited)
	require.Equal(t, nightSheet, GetPageTheme().Stylesheet)
	require.Equal(t, model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#00ff88"}, CurrentThemePalette())
	// A page that leaves the themes of the directory out has none of it.
	require.Equal(t, PageTheme{}, GetPageThemeWithoutDirectory())
	// A theme without options is written for the pages once, when it is
	// applied, with the stylesheets of the themes below it. They do not stay
	// with the theme that is selected next.
	SetTheme("on-styled")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Inherited: []string{stylesheetAddress(t, "styled")}}, GetPageTheme())
	SetTheme("plain")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
	SetTheme("on-styled")
	SetTheme("nowhere")
	require.Equal(t, PageTheme{}, GetPageTheme())
}

// A theme that is based on a theme that is not in use is not in use, and the
// remarks about the directory stay in the order of its entries.
func TestThemeBasedOnAThemeNotInUse(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	based := func(base string) string { return "contract: 1\nextends: " + base + "\n" }
	for file, content := range map[string]string{
		// Based on each other.
		"a.yml": based("b"), "b.yml": based("a"),
		// Based on a theme that is based on itself by way of another.
		"c.yml": based("a"),
		// A file that cannot be read, and a theme based on it.
		"broken.yml": "contract: 1\ntokens: [\n", "on-broken.yml": based("broken"),
		// A theme of its own without its colors is no theme, as before.
		"colors.yml": "contract: 1\ntokens:\n  accent: \"#00ff88\"\n",
		// The custom colors are no theme to be based on.
		"custom-based.yml": based("custom"),
		// One theme below another: three are as many as a theme can have.
		"d0.yml": userThemeColors, "d1.yml": based("d0"), "d2.yml": based("d1"), "d3.yml": based("d2"), "d4.yml": based("d3"), "d5.yml": based("d4"),
		// Colors that do not go together with those below.
		"dim.yml": based("d0") + "tokens:\n  primary: \"#1b2a22\"\n",
		// A theme that is not there.
		"missing.yml": based("nowhere"),
		// A theme that is based on itself.
		"self.yml": based("self"),
		// A single file has no assets for an image of its own, also above
		// a theme that has one.
		"walled.yml": basedOnZelda + "background:\n  image: assets/wallpaper.jpg\n",
		// A circle of three.
		"x.yml": based("y"), "y.yml": based("z"), "z.yml": based("x"),
	} {
		writeUserTheme(t, file, content)
	}
	// A directory whose image is not among its own assets, whatever the
	// theme below it has.
	writeThemeDirectory(t, "bare", basedOnZelda+"background:\n  image: assets/wallpaper.jpg\n")
	ReloadUserThemes()

	themes, problems, _ := UserThemes()
	require.Equal(t, []string{"d0", "d1", "d2", "d3", "zelda"}, userThemeNames(themes))
	require.Equal(t, []ThemeProblem{
		{File: "a.yml", Reason: ThemeProblemExtendsCycle, Subject: "b → a"},
		{File: "b.yml", Reason: ThemeProblemExtendsCycle, Subject: "a → b"},
		{File: "bare/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"},
		{File: "broken.yml", Reason: ThemeProblemSyntax, Subject: problems[3].Subject},
		{File: "c.yml", Reason: ThemeProblemExtends, Subject: "a"},
		{File: "colors.yml", Reason: ThemeProblemColor, Subject: "tokens.background"},
		{File: "custom-based.yml", Reason: ThemeProblemExtends, Subject: "custom"},
		{File: "d4.yml", Reason: ThemeProblemExtendsDepth, Subject: "d3 → d2 → d1 → d0"},
		{File: "d5.yml", Reason: ThemeProblemExtendsDepth, Subject: "d4 → d3 → d2 → d1 → d0"},
		{File: "dim.yml", Reason: ThemeProblemContrast},
		{File: "missing.yml", Reason: ThemeProblemExtends, Subject: "nowhere"},
		{File: "on-broken.yml", Reason: ThemeProblemExtends, Subject: "broken"},
		{File: "self.yml", Reason: ThemeProblemExtendsCycle, Subject: "self"},
		{File: "walled.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"},
		{File: "x.yml", Reason: ThemeProblemExtendsCycle, Subject: "y → z → x"},
		{File: "y.yml", Reason: ThemeProblemExtendsCycle, Subject: "z → x → y"},
		{File: "z.yml", Reason: ThemeProblemExtendsCycle, Subject: "x → y → z"},
	}, problems)
	for _, name := range []string{"a", "b", "c", "bare", "on-broken", "custom-based", "d4", "d5", "dim", "missing", "self", "walled", "x"} {
		require.False(t, IsKnownTheme(name), name)
		_, known := GetPageThemeFor(name)
		require.False(t, known, name)
	}
	// The third theme below is the last one a theme can have.
	require.Equal(t, "d2", userTheme(t, "d3").Extends)

	// A theme that needs a later release is not in use in this one, and
	// neither is a theme based on it.
	previous := version.Version
	version.Version = "0.7.0"
	t.Cleanup(func() { version.Version = previous })
	writeUserTheme(t, "later.yml", userThemeColors+"requires: 0.8.0\n")
	writeUserTheme(t, "on-later.yml", based("later"))
	ReloadUserThemes()
	_, problems, _ = UserThemes()
	require.Contains(t, problems, ThemeProblem{File: "later.yml", Reason: ThemeProblemRequires, Subject: "0.8.0"})
	require.Contains(t, problems, ThemeProblem{File: "on-later.yml", Reason: ThemeProblemExtends, Subject: "later"})
	require.False(t, IsKnownTheme("on-later"))
}

// The selected theme follows the files of every theme below it: an edit of
// one shows on the next page, a theme that goes away leaves the selection
// without a theme, and one that comes back brings it back.
func TestSelectedThemeFollowsTheThemesBelowIt(t *testing.T) {
	userThemeDirectory(t, "night")
	writeZeldaTheme(t)
	writeUserTheme(t, "compact.yml", basedOnZelda+"tokens:\n  radius: 2px\n")
	writeThemeDirectory(t, "night", "contract: 1\nextends: compact\ntokens:\n  accent: \"#00ff88\"\n")
	UpdatePagePalettes()
	colors := template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;")
	frames := template.HTMLAttr(` data-theme-frames="false"`)
	expected := func(tokens template.CSS) PageTheme {
		t.Helper()
		zelda := userTheme(t, "zelda")
		return PageTheme{
			BodyStyle: colors + tokens, Inherited: []string{"/theme/zelda/" + zelda.Hash + "/theme.css"},
			Background: backgroundRule("/theme/zelda/"+zelda.Hash+"/assets/wallpaper.jpg", "60"), Attributes: frames,
		}
	}
	// The page is read first: reading it is what reads the directory again,
	// and the addresses it is compared with are those of that reading.
	check := func(tokens template.CSS) {
		t.Helper()
		page := GetPageTheme()
		require.Equal(t, expected(tokens), page)
	}
	check("--flare-muted:#808080;--flare-radius:2px;")
	watched := func() []string {
		themeMu.RLock()
		defer themeMu.RUnlock()
		names := make([]string, 0, len(selectedThemeStamp.bases))
		for _, base := range selectedThemeStamp.bases {
			names = append(names, base.name)
		}
		return names
	}
	require.Equal(t, []string{"compact", "zelda"}, watched())
	// A reading notes the state of the files of each theme it comes to,
	// which is what the themes below the selected one are watched by.
	userThemeScanMu.Lock()
	scan := scanUserThemes()
	userThemeScanMu.Unlock()
	for _, name := range []string{"compact", "night", "zelda"} {
		require.Contains(t, scan.stamps, name)
		require.Equal(t, statTheme(name), scan.stamps[name], name)
	}
	require.NotContains(t, scan.stamps, "nowhere")

	// While the files keep their state the directory is not read.
	_, _, generation := ThemeRenderSnapshot()
	writeUserTheme(t, "added.yml", userThemeColors)
	for range 3 {
		check("--flare-muted:#808080;--flare-radius:2px;")
	}
	require.False(t, IsUserTheme("added"))
	_, _, unchanged := ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)

	// The theme file of the theme two below.
	first := GetPageTheme()
	writeThemeDirectory(t, "zelda", userThemeColors+"  radius: 10px\n  muted: \"#909090\"\nname: Zelda\n"+userThemeBackground+"options:\n  - id: frames\n    type: boolean\n")
	check("--flare-muted:#909090;--flare-radius:2px;")
	require.True(t, IsUserTheme("added"), "the directory was read again")

	// Its stylesheet: the page links another address.
	writeThemeStylesheet(t, "zelda", zeldaStylesheet+".app-title { letter-spacing: 1px }\n")
	second := GetPageTheme()
	require.NotEqual(t, first.Inherited, second.Inherited)
	check("--flare-muted:#909090;--flare-radius:2px;")

	// Its assets: the image is served under another address.
	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG+"more")
	third := GetPageTheme()
	require.NotEqual(t, second.Background, third.Background)
	require.NotEqual(t, second.Inherited, third.Inherited, "one hash names the stylesheet and the assets of a theme")
	check("--flare-muted:#909090;--flare-radius:2px;")

	// An image below that is edited in place has the state of its directory
	// as it was.
	assets := filepath.Join(UserThemeDirectory, "zelda", "assets")
	directory, err := os.Stat(assets)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(assets, "wallpaper.jpg"), []byte(assetJPEG+"edited in place"), 0600))
	setUserThemeAge(t, filepath.Join("zelda", "assets", "wallpaper.jpg"), 2*time.Minute)
	require.NoError(t, os.Chtimes(assets, directory.ModTime(), directory.ModTime()))
	fourth := GetPageTheme()
	require.NotEqual(t, third.Background, fourth.Background)
	check("--flare-muted:#909090;--flare-radius:2px;")

	// The file of the theme right below.
	writeUserTheme(t, "compact.yml", basedOnZelda+"tokens:\n  radius: 4px\n")
	check("--flare-muted:#909090;--flare-radius:4px;")

	// A theme below that is not accepted any more leaves the selection
	// without a theme, and so does one that goes away.
	writeUserTheme(t, "compact.yml", basedOnZelda+"tokens:\n  radius: 4em\n")
	require.Equal(t, PageTheme{}, GetPageTheme())
	require.False(t, IsKnownTheme("night"))
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{
		{File: "compact.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
		{File: "night/theme.yml", Reason: ThemeProblemExtends, Subject: "compact"},
	}, problems)
	writeUserTheme(t, "compact.yml", basedOnZelda+"tokens:\n  radius: 4px\n")
	setUserThemeAge(t, "compact.yml", 2*time.Minute)
	check("--flare-muted:#909090;--flare-radius:4px;")

	require.NoError(t, os.Rename(filepath.Join(UserThemeDirectory, "zelda"), filepath.Join(UserThemeDirectory, ".zelda")))
	require.Equal(t, PageTheme{}, GetPageTheme())
	_, problems, _ = UserThemes()
	require.Equal(t, []ThemeProblem{
		{File: "compact.yml", Reason: ThemeProblemExtends, Subject: "zelda"},
		{File: "night/theme.yml", Reason: ThemeProblemExtends, Subject: "compact"},
	}, problems)
	// The theme that went away is still looked for.
	require.Equal(t, []string{"compact", "zelda"}, watched())
	for range 3 {
		require.Equal(t, PageTheme{}, GetPageTheme())
	}
	require.NoError(t, os.Rename(filepath.Join(UserThemeDirectory, ".zelda"), filepath.Join(UserThemeDirectory, "zelda")))
	check("--flare-muted:#909090;--flare-radius:4px;")

	// A theme that is based on another theme from now on follows that one.
	writeUserTheme(t, "plain.yml", userThemeColors+"  radius: 6px\n")
	writeUserTheme(t, "compact.yml", "contract: 1\nextends: plain\n")
	setUserThemeAge(t, "compact.yml", 3*time.Minute)
	require.Equal(t, PageTheme{BodyStyle: colors + "--flare-radius:6px;"}, GetPageTheme())
	require.Equal(t, []string{"compact", "plain"}, watched())
	writeUserTheme(t, "plain.yml", userThemeColors+"  radius: 8px\n")
	require.Equal(t, PageTheme{BodyStyle: colors + "--flare-radius:8px;"}, GetPageTheme())
	// The theme it was based on before is no longer part of the page.
	_, _, generation = ThemeRenderSnapshot()
	writeThemeStylesheet(t, "zelda", zeldaStylesheet)
	require.Equal(t, PageTheme{BodyStyle: colors + "--flare-radius:8px;"}, GetPageTheme())
	_, _, unchanged = ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)
}

// A theme that is selected before the theme it is based on is there comes
// into use when that theme appears, in either form, and a bundled theme
// below needs no watching.
func TestSelectedThemeWaitsForTheThemeBelowIt(t *testing.T) {
	userThemeDirectory(t, "late")
	writeUserTheme(t, "late.yml", "contract: 1\nextends: soon\ntokens:\n  radius: 2px\n")
	UpdatePagePalettes()
	require.Equal(t, PageTheme{}, GetPageTheme())
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "late.yml", Reason: ThemeProblemExtends, Subject: "soon"}}, problems)
	for range 3 {
		require.Equal(t, PageTheme{}, GetPageTheme())
	}

	writeUserTheme(t, "soon.yml", userThemeColors)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-radius:2px;"}, GetPageTheme())
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "soon.yml")))
	require.Equal(t, PageTheme{}, GetPageTheme())
	writeThemeDirectory(t, "soon", userThemeColors+"  muted: \"#808080\"\n")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-muted:#808080;--flare-radius:2px;"}, GetPageTheme())

	// Based on a bundled theme, only the file of the theme is watched.
	writeUserTheme(t, "late.yml", "contract: 1\nextends: tron\ntokens:\n  radius: 2px\n")
	setUserThemeAge(t, "late.yml", 2*time.Minute)
	require.Equal(t, PageTheme{BodyStyle: "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#6EE2FF;--flare-radius:2px;"}, GetPageTheme())
	themeMu.RLock()
	bases := selectedThemeStamp.bases
	themeMu.RUnlock()
	require.Empty(t, bases)
}

// A selected theme that is based on itself by way of another is watched with
// that other theme, and not a second time as a theme below itself. Taking the
// circle apart brings it into use.
func TestSelectedThemeInACircleIsWatchedOnce(t *testing.T) {
	userThemeDirectory(t, "a")
	writeUserTheme(t, "a.yml", "contract: 1\nextends: b\n")
	writeUserTheme(t, "b.yml", "contract: 1\nextends: a\n")
	UpdatePagePalettes()
	require.Equal(t, PageTheme{}, GetPageTheme())
	themeMu.RLock()
	bases := selectedThemeStamp.bases
	themeMu.RUnlock()
	require.Len(t, bases, 1)
	require.Equal(t, "b", bases[0].name)

	writeUserTheme(t, "b.yml", userThemeColors+"  radius: 6px\n")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-radius:6px;"}, GetPageTheme())
}

// The values chosen for the options of a theme are stored for that theme.
// What is stored for the theme below does not reach a theme based on it,
// which has the options of that theme with values of its own.
func TestThemeBasedOnAnotherHasItsOwnValues(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "zelda.yml", userThemeColors+"options:\n  - id: density\n    type: enum\n    choices:\n      - value: normal\n      - value: compact\n        tokens: { radius: 2px }\n"+
		"  - id: highlight\n    type: color\n    token: accent\n")
	writeUserTheme(t, "night.yml", basedOnZelda+"tokens:\n  accent: \"#00ff88\"\noptions:\n  - id: frames\n    type: boolean\n")
	config := func(options string) {
		t.Helper()
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: night\nThemeSettings:\n    zelda:\n        Options:\n            density: compact\n            highlight: \"#112233\"\n"+options), 0600))
	}
	config("")
	UpdatePagePalettes()

	// The options of both, those of the theme below first, with the
	// defaults: the color option has the accent the theme gives.
	options, values, known := ThemeOptions("night")
	require.True(t, known)
	require.Len(t, options, 3)
	require.Equal(t, map[string]string{"density": "normal", "highlight": "#00ff88", "frames": "false"}, values)
	require.Equal(t, PageTheme{
		BodyStyle:  "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;",
		Attributes: ` data-theme-density="normal" data-theme-frames="false"`,
	}, GetPageTheme())
	// The theme below has what is stored for it.
	page, known := GetPageThemeFor("zelda")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#112233;--flare-radius:2px;", Attributes: ` data-theme-density="compact"`}, page)

	// Values of its own, for an option of the theme below and for its own.
	config("    night:\n        Options:\n            density: compact\n            frames: \"true\"\n")
	require.Equal(t, PageTheme{
		BodyStyle:  "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;--flare-radius:2px;",
		Attributes: ` data-theme-density="compact" data-theme-frames="true"`,
	}, GetPageTheme())
}

// A theme file that is based on another theme is stored if it is a theme in
// use once it is there. One that would not be is refused with the reason, and
// nothing is written.
func TestStoreUserThemeBasedOnAnother(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeUserTheme(t, "plain.yml", userThemeColors)
	writeUserTheme(t, "one.yml", "contract: 1\nextends: plain\n")
	writeUserTheme(t, "two.yml", "contract: 1\nextends: one\n")
	writeUserTheme(t, "three.yml", "contract: 1\nextends: two\n")
	ReloadUserThemes()
	before := themesDirectoryEntries(t)
	based := func(base string) []byte {
		return []byte("contract: 1\nextends: " + base + "\ntokens:\n  radius: 2px\n")
	}

	for name, tc := range map[string]struct {
		theme   string
		raw     []byte
		replace bool
		reason  error
		subject string
		// shown is the reason as the theme page names it.
		shown string
	}{
		"a theme that is not there":          {"child", based("nowhere"), false, model.ErrThemeBase, "nowhere", ThemeProblemExtends},
		"the custom colors":                  {"child", based("custom"), false, model.ErrThemeBase, "custom", ThemeProblemExtends},
		"itself":                             {"child", based("child"), false, model.ErrThemeBaseCycle, "child", ThemeProblemExtendsCycle},
		"a theme that is based on it":        {"plain", based("three"), true, model.ErrThemeBaseCycle, "three → two → one → plain", ThemeProblemExtendsCycle},
		"more themes below than can be":      {"child", based("three"), false, model.ErrThemeBaseDepth, "three → two → one → plain", ThemeProblemExtendsDepth},
		"colors that do not go with those":   {"child", []byte("contract: 1\nextends: plain\ntokens:\n  primary: \"#1b2a22\"\n"), false, model.ErrPaletteContrast, "", ThemeProblemContrast},
		"a name that is none":                {"child", []byte("contract: 1\nextends: Plain\n"), false, model.ErrThemeExtends, "Plain", ThemeProblemExtendsValue},
		"an image of its own, above a theme": {"child", []byte("contract: 1\nextends: zelda\nbackground:\n  image: assets/wallpaper.jpg\n"), false, model.ErrThemeBackgroundAsset, "assets/wallpaper.jpg", ThemeProblemBackgroundAsset},
	} {
		err := StoreUserTheme(tc.theme, tc.raw, tc.replace)
		var rejection *model.ThemeFileError
		require.ErrorAs(t, err, &rejection, name)
		require.ErrorIs(t, err, tc.reason, name)
		require.Equal(t, tc.subject, rejection.Subject, name)
		require.Equal(t, ThemeProblem{Reason: tc.shown, Subject: tc.subject}, ThemeFileProblem(err), name)
		require.Equal(t, before, themesDirectoryEntries(t), name)
	}
	stored, err := os.ReadFile(filepath.Join(UserThemeDirectory, "plain.yml"))
	require.NoError(t, err)
	require.Equal(t, userThemeColors, string(stored), "a refused file replaced nothing")

	// Based on a directory, on a file, on a file that is based on another,
	// and on a bundled theme.
	for name, base := range map[string]string{"on-zelda": "zelda", "on-plain": "plain", "on-two": "two", "on-tron": "tron"} {
		require.NoError(t, StoreUserTheme(name, based(base), false), name)
		require.Equal(t, base, userTheme(t, name).Extends, name)
	}
	require.Equal(t, []model.ThemeSource{{Name: "zelda", Hash: userTheme(t, "zelda").Hash}}, userTheme(t, "on-zelda").Inherited)
	require.Equal(t, "2px", userTheme(t, "on-two").Tokens["radius"])
	// A file that is exported is the file as it is, with the theme it names.
	file, raw, exported := ExportTheme("on-zelda")
	require.True(t, exported)
	require.Equal(t, "on-zelda.yml", file)
	require.Equal(t, string(based("zelda")), string(raw))

	// A file of a name can be replaced by one that is based on another
	// theme, and the themes based on that name follow.
	require.ErrorIs(t, StoreUserTheme("one", based("zelda"), false), ErrThemeStoreExists)
	require.NoError(t, StoreUserTheme("one", based("zelda"), true))
	require.Equal(t, []model.ThemeSource{{Name: "zelda", Hash: userTheme(t, "zelda").Hash}}, userTheme(t, "three").Inherited)

	// Nothing is stored in safe mode, as for any theme file.
	AppFlags.ThemeSafeMode = true
	t.Cleanup(func() { AppFlags.ThemeSafeMode = false })
	require.ErrorIs(t, StoreUserTheme("safe", based("plain"), false), ErrThemeStoreSafeMode)
}

// Pages are rendered while the files of a theme below the selected one
// change. Each page has the theme of one reading of the directory: the
// tokens, the stylesheet of the theme below and its image belong together.
func TestThemesBelowAreSafeForConcurrentPages(t *testing.T) {
	userThemeDirectory(t, "night")
	writeZeldaTheme(t)
	writeThemeDirectory(t, "night", basedOnZelda+"tokens:\n  accent: \"#00ff88\"\n")
	UpdatePagePalettes()
	colors := template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;")
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				page := GetPageTheme()
				if page.BodyStyle == "" {
					// Between two writes of the theme below, its file can be
					// one that is not accepted yet.
					continue
				}
				soft := page.BodyStyle == colors+"--flare-muted:#808080;--flare-radius:10px;"
				hard := page.BodyStyle == colors+"--flare-muted:#808080;--flare-radius:0px;"
				if (!soft && !hard) || len(page.Inherited) != 1 || page.Background == "" || page.Attributes != ` data-theme-frames="false"` {
					t.Errorf("a page got %+v", page)
					return
				}
			}
		})
	}
	for i := range 40 {
		radius := "10px"
		if i%2 == 1 {
			radius = "0px"
		}
		content := userThemeColors + "  radius: " + radius + "\n  muted: \"#808080\"\nname: Zelda\n" + userThemeBackground + "options:\n  - id: frames\n    type: boolean\n"
		require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, "zelda", userThemeManifest), []byte(content), 0600))
		setUserThemeAge(t, filepath.Join("zelda", userThemeManifest), time.Duration(i+1)*time.Minute)
		time.Sleep(time.Millisecond)
	}
	close(stop)
	readers.Wait()
}

// A theme below that the reading does not come to, in a directory with more
// entries than are looked at, is not in use, and neither is a theme based on
// it. The pages of that theme do not read the directory again for it each
// time: nothing of the theme below was read that could have changed.
func TestThemeBelowBeyondTheEntriesLookedAt(t *testing.T) {
	userThemeDirectory(t, "a-child")
	writeUserTheme(t, "a-child.yml", "contract: 1\nextends: zz-base\n")
	for i := range MaximumUserThemeFiles - 1 {
		writeUserTheme(t, fmt.Sprintf("f%02d.yml", i), userThemeColors)
	}
	writeUserTheme(t, "zz-base.yml", userThemeColors)
	UpdatePagePalettes()
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{
		{File: "a-child.yml", Reason: ThemeProblemExtends, Subject: "zz-base"},
		{File: "zz-base.yml", Reason: ThemeProblemLimit},
	}, problems)
	require.Equal(t, PageTheme{}, GetPageTheme())

	// A file that is added is seen by the next reading of the directory,
	// and there is none while the files keep their state.
	writeUserTheme(t, "a-added.yml", userThemeColors)
	for range 3 {
		require.Equal(t, PageTheme{}, GetPageTheme())
	}
	require.False(t, IsUserTheme("a-added"), "the directory was read again")

	// With fewer entries the theme below is come to, and the theme is used.
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "a-added.yml")))
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "f00.yml")))
	ReloadUserThemes()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
}
