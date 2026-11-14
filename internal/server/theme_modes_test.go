package server

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

const (
	// A theme that is dark as written and gives other values for light.
	pairedThemeFile = themeFileColors + "  radius: 8px\nmodes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n    muted: \"#6b6860\"\n"
	pairedLight     = "--color-background:#f6f3ea;--color-primary:#2b2a27;--color-accent:#f2b84b;--flare-muted:#6b6860;--flare-radius:8px;"
	pairedDark      = themeFileStyle + "--flare-radius:8px;"
	pairedRules     = "@media not all and (prefers-color-scheme:dark){body{" + pairedLight + "}}" +
		"@media (prefers-color-scheme:dark){body{" + pairedDark + "}}"
)

// The pages that use the theme and are served with login disabled.
var themedPages = []string{"/", "/bookmarks", "/applications", "/help", "/settings/theme", "/settings/appearance", "/settings/search", "/settings/weather"}

// modesRouter is themeFilesRouter with the site setting for light and dark
// put back to its default afterwards.
func modesRouter(t *testing.T, config string, files map[string]string) http.Handler {
	t.Helper()
	handler := themeFilesRouter(t, config, files)
	t.Cleanup(func() { define.SetThemeMode(model.ThemeModeAuto) })
	return handler
}

// The parts of a page that carry the theme.
type themedPage struct {
	// styles holds the content of the style elements of the head, in order.
	styles []string
	body   string
	// The form for light and dark on the theme page.
	modeForm     bool
	modeOptions  []string
	modeSelected []string
	status       []string
}

func readThemedPage(t *testing.T, page string) themedPage {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	read := themedPage{body: bodyStyle(t, page)}
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch {
		case node.Data == "style" && node.Parent != nil && node.Parent.Data == "head":
			read.styles = append(read.styles, nodeText(node))
		case node.Data == "select" && nodeAttribute(node, "name") == "theme-mode":
			read.modeForm = true
			walkNodes(node, func(option *html.Node) {
				if option.Type != html.ElementNode || option.Data != "option" {
					return
				}
				read.modeOptions = append(read.modeOptions, nodeAttribute(option, "value")+"="+nodeText(option))
				for _, attribute := range option.Attr {
					if attribute.Key == "selected" {
						read.modeSelected = append(read.modeSelected, nodeAttribute(option, "value"))
					}
				}
			})
		case nodeHasClass(node, "theme-notice") && nodeAttribute(node, "role") == "status":
			read.status = append(read.status, nodeText(node))
		}
	})
	return read
}

func modeRequest(handler http.Handler, mode string) (int, string) {
	rec := themeRequest(handler, url.Values{"action": {"save-mode"}, "theme-mode": {mode}})
	return rec.Code, rec.Body.String()
}

func TestThemeModesHTTP(t *testing.T) {
	handler := modesRouter(t, "Locale: en\nTheme: paired\nShowApps: true\n", map[string]string{
		"paired.yml": pairedThemeFile,
		"plain.yml":  themeFileColors,
	})

	// While the device chooses, every themed page carries both sets of
	// values as rules after its stylesheet, and no style on the body.
	for _, path := range themedPages {
		page := readThemedPage(t, getPage(t, handler, path))
		require.Empty(t, page.body, path)
		require.Len(t, page.styles, 2, path)
		require.True(t, strings.Contains(page.styles[0], "--flare-background: var(--color-background)"), "%s: the first style element is not the stylesheet", path)
		require.Equal(t, pairedRules, page.styles[1], path)
	}

	// The theme page offers the setting, and the card shows both sets.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemedPage(t, raw)
	require.True(t, page.modeForm)
	require.Equal(t, []string{
		"auto=" + i18n.T("en", "theme_mode_auto"), "light=" + i18n.T("en", "theme_mode_light"), "dark=" + i18n.T("en", "theme_mode_dark"),
	}, page.modeOptions)
	require.Equal(t, []string{"auto"}, page.modeSelected)
	require.Empty(t, page.status)
	require.Contains(t, raw, i18n.T("en", "theme_mode_help"))
	cards := readThemeFilePage(t, raw).cards
	require.Len(t, cards, 2)
	require.Equal(t, "paired", cards[0].value)
	require.Equal(t, []string{
		"background-color: #f6f3ea;", "background-color: #2b2a27;", "background-color: #f2b84b;",
		"background-color: #10241b;", "background-color: #e8f1e4;", "background-color: #f2b84b;",
	}, cards[0].colors)
	require.Len(t, cards[1].colors, 3)

	// Set to dark or to light, the values are in the style of the body, as
	// for every other theme, and the rules are gone.
	// In this order: the checks that follow rely on light being in use.
	for _, set := range []struct{ mode, style string }{{"dark", pairedDark}, {"light", pairedLight}} {
		mode, style := set.mode, set.style
		status, saved := modeRequest(handler, mode)
		require.Equal(t, http.StatusOK, status, mode)
		page = readThemedPage(t, saved)
		require.Equal(t, style, page.body, mode)
		require.Len(t, page.styles, 1, mode)
		require.Equal(t, []string{mode}, page.modeSelected)
		require.Equal(t, []string{i18n.T("en", "theme_mode_saved")}, page.status)
		for _, path := range themedPages {
			page = readThemedPage(t, getPage(t, handler, path))
			require.Equal(t, style, page.body, "%s in %s", path, mode)
			require.Len(t, page.styles, 1, "%s in %s", path, mode)
		}
		stored, err := os.ReadFile("config.yml")
		require.NoError(t, err)
		require.Contains(t, string(stored), "ThemeMode: "+mode)
		require.Contains(t, string(stored), "Theme: paired")
	}

	// Anything else is rejected.
	for _, mode := range []string{"", "Dark", "night", "auto;"} {
		status, _ := modeRequest(handler, mode)
		require.Equal(t, http.StatusBadRequest, status, "%q", mode)
		require.Equal(t, pairedLight, bodyStyle(t, getPage(t, handler, "/")), "%q", mode)
	}

	// A theme that is the same in light and dark is not affected by the
	// setting, and the theme page does not offer it. The setting is kept.
	for name, style := range map[string]string{"plain": themeFileStyle, "blackboard": "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"} {
		require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		for _, path := range themedPages {
			page = readThemedPage(t, getPage(t, handler, path))
			require.Equal(t, style, page.body, "%s with %s", path, name)
			require.Len(t, page.styles, 1, "%s with %s", path, name)
			require.False(t, page.modeForm, "%s with %s", path, name)
		}
	}
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(stored), "ThemeMode: light")

	// Back on the theme with two sets, the stored setting applies again.
	selected := themeRequest(handler, url.Values{"theme": {"paired"}})
	require.Equal(t, http.StatusOK, selected.Code)
	page = readThemedPage(t, selected.Body.String())
	require.Equal(t, pairedLight, page.body)
	require.Equal(t, []string{"light"}, page.modeSelected)

	status, saved := modeRequest(handler, "auto")
	require.Equal(t, http.StatusOK, status)
	page = readThemedPage(t, saved)
	require.Empty(t, page.body)
	require.Equal(t, pairedRules, page.styles[len(page.styles)-1])
	require.Equal(t, []string{"auto"}, page.modeSelected)

	// The file of the theme is followed in this form as well.
	writeThemeFile(t, "paired.yml", strings.Replace(pairedThemeFile, "radius: 8px", "radius: 2px", 1))
	page = readThemedPage(t, getPage(t, handler, "/"))
	require.Equal(t, strings.ReplaceAll(pairedRules, "--flare-radius:8px;", "--flare-radius:2px;"), page.styles[len(page.styles)-1])
}

// The setting of config.yml is applied when the instance starts. A value
// that is not a mode leaves the choice to the device.
func TestThemeModeAtStartup(t *testing.T) {
	for stored, want := range map[string]themedPage{
		"":                   {styles: []string{pairedRules}, modeSelected: []string{"auto"}},
		"ThemeMode: auto\n":  {styles: []string{pairedRules}, modeSelected: []string{"auto"}},
		"ThemeMode: dark\n":  {body: pairedDark, modeSelected: []string{"dark"}},
		"ThemeMode: light\n": {body: pairedLight, modeSelected: []string{"light"}},
		"ThemeMode: night\n": {styles: []string{pairedRules}, modeSelected: []string{"auto"}},
	} {
		t.Run(stored, func(t *testing.T) {
			handler := modesRouter(t, "Locale: en\nTheme: paired\n"+stored, map[string]string{"paired.yml": pairedThemeFile})
			for _, path := range []string{"/", "/settings/theme"} {
				page := readThemedPage(t, getPage(t, handler, path))
				require.Equal(t, want.body, page.body, path)
				// Everything after the stylesheet.
				require.Equal(t, want.styles, append([]string(nil), page.styles[1:]...), path)
			}
			// The theme page shows the setting in use as selected.
			require.Equal(t, want.modeSelected, readThemedPage(t, getPage(t, handler, "/settings/theme")).modeSelected)
		})
	}
}

// The rules are text of a style element. Nothing a theme file holds can end
// the element or add a rule: values that could are rejected with the file.
func TestThemeRulesHoldOnlyAcceptedValues(t *testing.T) {
	handler := modesRouter(t, "Locale: en\nTheme: closing\n", map[string]string{
		"closing.yml":  themeFileColors + "modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    font-family: \"x</style><script>alert(1)</script>\"\n",
		"breaking.yml": themeFileColors + "modes:\n  light:\n    background: \"#ffffff}body{background:url(//example.test/)\"\n    primary: \"#222222\"\n",
		"quoted.yml":   themeFileColors + "modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    font-family: Georgia, serif\n",
	})
	for _, name := range []string{"closing", "breaking"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
	}
	raw := getPage(t, handler, "/")
	require.Empty(t, bodyStyle(t, raw))
	require.Len(t, readThemedPage(t, raw).styles, 1)
	require.NotContains(t, raw, "alert(1)")
	require.NotContains(t, raw, "example.test")
	lines := readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lines
	require.Contains(t, lines, []string{"closing.yml", "modes.light.font-family"})
	require.Contains(t, lines, []string{"breaking.yml", "modes.light.background"})

	// A font family with a space and a comma is written as it is.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"quoted"}}).Code)
	page := readThemedPage(t, getPage(t, handler, "/"))
	require.Equal(t,
		"@media not all and (prefers-color-scheme:dark){body{--color-background:#ffffff;--color-primary:#222222;--color-accent:#f2b84b;--flare-font-family:Georgia, serif;}}"+
			"@media (prefers-color-scheme:dark){body{"+themeFileStyle+"}}",
		page.styles[1])
}
