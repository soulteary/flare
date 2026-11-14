package appearance

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

const arrangedThemeFile = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n" +
	"layout:\n  DesktopColumns: 6\n  TitleSize: compact\n  HideAppDescriptions: true\n"

// layoutApp runs a test in its own data directory with a theme that
// specifies layout selected, and leaves no theme file behind.
func layoutApp(t *testing.T, config string) *fiber.App {
	t.Helper()
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	// Registered after t.Chdir, so it runs while the test directory is current.
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(define.UserThemeDirectory))
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\n"), 0600))
		define.UpdatePagePalettes()
		define.AppFlags = flags
	})
	define.AppFlags.DisableLoginMode = true
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	path := filepath.Join(define.UserThemeDirectory, "arranged.yml")
	require.NoError(t, os.WriteFile(path, []byte(arrangedThemeFile), 0600))
	modified := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(path, modified, modified))
	require.NoError(t, os.WriteFile("config.yml", []byte(config), 0600))
	define.UpdatePagePalettes()
	app := fiber.New(fiber.Config{Views: appearanceTestViews{}})
	app.Post("/appearance", updateAppearanceOptions)
	return app
}

func postAppearance(t *testing.T, app *fiber.App, form url.Values) int {
	t.Helper()
	// What every appearance form submits; the layout fields come on top.
	body := url.Values{"title": {"Site"}, "show-title": {"0"}, "desktop-columns": {"4"}, "title-size": {"prominent"}}
	for name, values := range form {
		body[name] = values
	}
	req := httptest.NewRequest(http.MethodPost, "/appearance", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", fiber.MIMEApplicationForm)
	res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	return res.StatusCode
}

func storedLayoutSettings(t *testing.T) model.Application {
	t.Helper()
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	return options
}

// The boxes next to the options the theme specifies turn single options
// off, for the theme the form names.
func TestAppearanceFormTurnsThemeLayoutOptionsOff(t *testing.T) {
	app := layoutApp(t, "Title: Site\nTheme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n  TitleSize: prominent\n")
	effective := func() model.HomepageOptions {
		return *define.HomepageInEffect(storedLayoutSettings(t))
	}
	require.Equal(t, model.HomepageOptions{DesktopColumns: "6", TitleSize: "compact", HideAppDescriptions: true}, effective())

	// Ticked boxes are stored, whatever value they carry.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{
		"theme-layout-all": {"apply"}, "theme-layout-theme": {"arranged"},
		"theme-layout-ignore-DesktopColumns": {"1"}, "theme-layout-ignore-HideAppDescriptions": {""},
	}))
	options := storedLayoutSettings(t)
	require.Equal(t, []string{model.LayoutHideAppDescriptions, model.LayoutDesktopColumns}, options.ThemeSettings["arranged"].IgnoreLayout)
	require.False(t, options.Homepage.IgnoreThemeLayout)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "4", TitleSize: "compact"}, effective())

	// A form without the name of the theme has no boxes, and leaves the choice.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{}))
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-ignore-TitleSize": {"1"}}))
	require.Equal(t, []string{model.LayoutHideAppDescriptions, model.LayoutDesktopColumns}, storedLayoutSettings(t).ThemeSettings["arranged"].IgnoreLayout)

	// With the name of the theme and no box ticked, every option is on again.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-all": {"apply"}, "theme-layout-theme": {"arranged"}}))
	require.Empty(t, storedLayoutSettings(t).ThemeSettings["arranged"].IgnoreLayout)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "6", TitleSize: "compact", HideAppDescriptions: true}, effective())

	// The boxes are stored for the theme the form names, which is not
	// necessarily the one selected by now.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-theme": {"another"}, "theme-layout-ignore-TitleSize": {"1"}}))
	options = storedLayoutSettings(t)
	require.Equal(t, []string{model.LayoutTitleSize}, options.ThemeSettings["another"].IgnoreLayout)
	require.Empty(t, options.ThemeSettings["arranged"].IgnoreLayout)

	// A box for something a theme cannot specify is not a box of the form.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-theme": {"arranged"}, "theme-layout-ignore-NavigationTextSize": {"1"}, "theme-layout-ignore-": {"1"}}))
	require.Empty(t, storedLayoutSettings(t).ThemeSettings["arranged"].IgnoreLayout)
}

// The switch for all options is stored with the Homepage block. While it is
// off the form has no boxes, and the choice per option is kept.
func TestAppearanceFormTurnsThemeLayoutOff(t *testing.T) {
	app := layoutApp(t, "Title: Site\nTheme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n  TitleSize: prominent\n")
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{
		"theme-layout-all": {"apply"}, "theme-layout-theme": {"arranged"}, "theme-layout-ignore-TitleSize": {"1"},
	}))

	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-all": {"ignore"}}))
	options := storedLayoutSettings(t)
	require.True(t, options.Homepage.IgnoreThemeLayout)
	require.Equal(t, "4", options.Homepage.DesktopColumns)
	require.Equal(t, []string{model.LayoutTitleSize}, options.ThemeSettings["arranged"].IgnoreLayout)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "4", TitleSize: "prominent", IgnoreThemeLayout: true}, *define.HomepageInEffect(options))

	// A form without the switch, as rendered for a theme without a layout,
	// leaves it as it is.
	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{}))
	require.True(t, storedLayoutSettings(t).Homepage.IgnoreThemeLayout)

	require.Equal(t, http.StatusOK, postAppearance(t, app, url.Values{"theme-layout-all": {"apply"}}))
	options = storedLayoutSettings(t)
	require.False(t, options.Homepage.IgnoreThemeLayout)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "6", TitleSize: "prominent", HideAppDescriptions: true}, *define.HomepageInEffect(options))
}

// The switch can arrive in a form that carries no other layout field. The
// stored options are then the ones it is set on, and a configuration that
// cannot be read saves nothing.
func TestThemeLayoutSwitchWithoutOtherLayoutFields(t *testing.T) {
	app := layoutApp(t, "Title: Site\nTheme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n  TitleSize: prominent\n")
	post := func(form url.Values) int {
		req := httptest.NewRequest(http.MethodPost, "/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", fiber.MIMEApplicationForm)
		res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		return res.StatusCode
	}
	require.Equal(t, http.StatusOK, post(url.Values{"title": {"Site"}, "theme-layout-all": {"ignore"}}))
	options := storedLayoutSettings(t)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "4", TitleSize: "prominent", IgnoreThemeLayout: true}, *options.Homepage)

	// Without a Homepage block the switch is stored on the defaults.
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: Site\nTheme: arranged\n"), 0600))
	require.Equal(t, http.StatusOK, post(url.Values{"title": {"Site"}, "theme-layout-all": {"ignore"}}))
	require.Equal(t, model.HomepageOptions{IgnoreThemeLayout: true}, *storedLayoutSettings(t).Homepage)

	require.NoError(t, os.Remove("config.yml"))
	require.NoError(t, os.Mkdir("config.yml", 0700))
	require.Equal(t, http.StatusInternalServerError, post(url.Values{"title": {"Site"}, "theme-layout-all": {"apply"}}))
	require.NoError(t, os.Remove("config.yml"))
}

func TestAppearanceFormRejectsOtherThemeLayoutValues(t *testing.T) {
	app := layoutApp(t, "Title: Site\nTheme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n")
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	for name, form := range map[string]url.Values{
		"switch":             {"theme-layout-all": {"maybe"}},
		"switch in capitals": {"theme-layout-all": {"IGNORE"}},
		"theme name":         {"theme-layout-theme": {"../config"}},
		"theme in capitals":  {"theme-layout-theme": {"Arranged"}},
		"long theme name":    {"theme-layout-theme": {strings.Repeat("a", 64)}},
	} {
		require.Equal(t, http.StatusBadRequest, postAppearance(t, app, form), name)
		raw, readErr := os.ReadFile("config.yml")
		require.NoError(t, readErr)
		require.Equal(t, string(stored), string(raw), name)
	}
}

// The page describes the layout of the selected theme next to the options
// it specifies, and says when it is not in use.
func TestThemeLayoutNotes(t *testing.T) {
	layoutApp(t, "Title: Site\nLocale: en\nTheme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n")
	notes := func(options model.Application) (map[string]any, map[string]*themeLayoutNote) {
		m := map[string]any{}
		themeLayoutNotes(m, "en", options)
		found, _ := m["ThemeLayout"].(map[string]*themeLayoutNote)
		return m, found
	}
	options := storedLayoutSettings(t)
	options.ThemeSettings = map[string]model.ThemeSettings{"arranged": {IgnoreLayout: []string{model.LayoutTitleSize}}}
	m, found := notes(options)
	specified := make([]string, 0, len(found))
	for option := range found {
		specified = append(specified, option)
	}
	slices.Sort(specified)
	require.Equal(t, []string{model.LayoutDesktopColumns, model.LayoutHideAppDescriptions, model.LayoutTitleSize}, specified)
	require.Equal(t, "arranged", m["ThemeLayoutTheme"])
	require.Equal(t, false, m["ThemeLayoutOff"])
	require.Equal(t, "The selected theme sets: Homepage title size, Application descriptions, Desktop columns. Each of these options says so below and can be switched back to your setting on its own.", m["ThemeLayoutSummary"])
	require.Equal(t, themeLayoutNote{Field: "theme-layout-ignore-DesktopColumns", Text: "The selected theme sets this to “6”.", UseMine: "Use my setting"}, *found[model.LayoutDesktopColumns])
	require.Equal(t, themeLayoutNote{Field: "theme-layout-ignore-TitleSize", Text: "The selected theme sets this to “Compact”.", UseMine: "Use my setting", Ignored: true}, *found[model.LayoutTitleSize])
	require.Equal(t, "The selected theme sets this to “Hide”.", found[model.LayoutHideAppDescriptions].Text)

	// Turned off as a whole, the options have no box of their own.
	options.Homepage.IgnoreThemeLayout = true
	m, found = notes(options)
	require.Equal(t, true, m["ThemeLayoutOff"])
	for option, note := range found {
		require.Empty(t, note.Field, option)
		require.False(t, note.Ignored, option)
		require.Contains(t, note.Text, "Your setting applies", option)
	}

	// A theme without a layout has nothing to describe. The page still gets
	// the remarks, empty: its template looks every option up in them.
	define.SetTheme("blackboard")
	m, found = notes(storedLayoutSettings(t))
	require.NotNil(t, found)
	require.Empty(t, found)
	require.Len(t, m, 1)
}
