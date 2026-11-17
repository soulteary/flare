package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

// The control of a page a visitor chooses a theme with.
type choiceControl struct {
	action, back, label, submit, token string
	// options are the values the control offers with the name each is shown
	// under, in its order, and selected is the value that is marked.
	options  [][2]string
	selected []string
	// afterFooter reports that the control follows the footer of the page.
	afterFooter bool
}

// readChoiceControl returns the control of a page, or nil when it has none.
func readChoiceControl(t *testing.T, page string) *choiceControl {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var control *choiceControl
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "form" || !nodeHasClass(node, "theme-choice") {
			return
		}
		require.Nil(t, control, "the page has the control twice")
		require.Equal(t, "POST", nodeAttribute(node, "method"))
		control = &choiceControl{action: nodeAttribute(node, "action")}
		for before := node.PrevSibling; before != nil; before = before.PrevSibling {
			if before.Type == html.ElementNode {
				control.afterFooter = nodeHasClass(before, "footer-container")
				break
			}
		}
		walkNodes(node, func(part *html.Node) {
			if part.Type != html.ElementNode {
				return
			}
			switch part.Data {
			case "input":
				switch nodeAttribute(part, "name") {
				case "back":
					control.back = nodeAttribute(part, "value")
				case "csrf-token":
					control.token = nodeAttribute(part, "value")
				}
			case "label":
				require.Equal(t, "theme-choice", nodeAttribute(part, "for"))
				control.label = nodeText(part)
			case "select":
				require.Equal(t, "theme-choice", nodeAttribute(part, "id"))
				require.Equal(t, "theme", nodeAttribute(part, "name"))
			case "option":
				control.options = append(control.options, [2]string{nodeAttribute(part, "value"), nodeText(part)})
				for _, attribute := range part.Attr {
					if attribute.Key == "selected" {
						control.selected = append(control.selected, nodeAttribute(part, "value"))
					}
				}
			case "button":
				control.submit = nodeText(part)
			}
		})
	})
	return control
}

func TestPagesOfferTheThemesToChooseHTTP(t *testing.T) {
	handler := themeFilesRouter(t, previewConfig+"ThemeChoices: [markup, tron, gone, forest]\n", map[string]string{
		"forest.yml": previewForestFile,
		"markup.yml": themeFileColors + "name: '<b>Bold</b> & \"quoted\"'\n",
		"hidden.yml": themeFileColors + "  radius: 2px\n",
	})
	offered := [][2]string{
		{"", i18n.Format("en", "theme_choice_site", map[string]any{"name": "blackboard"})},
		{"markup", `<b>Bold</b> & "quoted"`}, {"tron", "tron"}, {"forest", "Forest at dusk"},
	}
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
		for _, chosen := range []string{"", "forest", "tron", "markup"} {
			page, _ := getWithChoice(t, handler, path, chosen)
			control := readChoiceControl(t, page)
			require.NotNil(t, control, "%s with %s", path, chosen)
			require.Equal(t, choiceControl{
				action: "/theme-choice", back: path, label: i18n.T("en", "theme_choice_label"), submit: i18n.T("en", "theme_choice_apply"),
				options: offered, selected: []string{chosen}, afterFooter: true,
			}, *control, "%s with %s", path, chosen)
			// The name of a theme is text, whatever it holds.
			require.NotContains(t, page, "<b>Bold</b>", "%s with %s", path, chosen)
		}
		// A cookie that names no choice leaves the theme of the site marked.
		page, _ := getWithChoice(t, handler, path, "hidden")
		require.Equal(t, []string{""}, readChoiceControl(t, page).selected, path)

		// What the control submits is taken, and leads back to the page.
		rec, cookie := postChoice(t, handler, url.Values{"theme": {"forest"}, "back": {path}})
		require.Equal(t, http.StatusSeeOther, rec.Code, path)
		require.Equal(t, path, rec.Header().Get("Location"), path)
		require.Equal(t, "forest", cookie.Value, path)
	}
	// A search is answered by the home page, and a choice leads back there.
	page, _ := getWithChoice(t, handler, "/?search=example", "")
	require.Equal(t, "/", readChoiceControl(t, page).back)

	// The pages of the settings have no control.
	for _, path := range []string{"/settings/theme", "/settings/layout"} {
		page, _ = getWithChoice(t, handler, path, "forest")
		require.Nil(t, readChoiceControl(t, page), path)
	}

	// When another theme is selected, the control names it as the theme of
	// the site and no longer among the choices.
	require.NoError(t, data.SaveThemeName("forest"))
	define.SetTheme("forest")
	page, _ = getWithChoice(t, handler, "/", "tron")
	control := readChoiceControl(t, page)
	require.Equal(t, [][2]string{
		{"", i18n.Format("en", "theme_choice_site", map[string]any{"name": "Forest at dusk"})},
		{"markup", `<b>Bold</b> & "quoted"`}, {"tron", "tron"},
	}, control.options)
	require.Equal(t, []string{"tron"}, control.selected)
}

func TestPagesWithoutAChoiceHaveNoControlHTTP(t *testing.T) {
	for name, config := range map[string]string{
		"no list":                 previewConfig,
		"an empty list":           previewConfig + "ThemeChoices: []\n",
		"only the selected theme": previewConfig + "ThemeChoices: [blackboard]\n",
		"only unknown themes":     previewConfig + "ThemeChoices: [gone]\n",
	} {
		t.Run(name, func(t *testing.T) {
			handler := themeFilesRouter(t, config, map[string]string{"forest.yml": previewForestFile})
			for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
				page, _ := getWithChoice(t, handler, path, "forest")
				require.Nil(t, readChoiceControl(t, page), path)
			}
		})
	}
}

// A theme that brings the markup of a page has the control with the footer
// it has to place.
func TestPageTemplateOfAThemeHasTheControlHTTP(t *testing.T) {
	config := "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\nThemeTemplates: true\n" +
		"ThemeChoices: [tron]\nThemeSettings:\n  zelda:\n    IgnoreLayout: []\n    Templates: true\n"
	handler := themeFilesRouter(t, config, map[string]string{
		"zelda/theme.yml": themeFileColors + "name: Zelda\n", "zelda/templates/home.html": themeShellTemplate,
	})
	for _, path := range []string{"/", "/applications", "/bookmarks"} {
		page, _ := getWithChoice(t, handler, path, "")
		require.Contains(t, page, `<div class="theme-shell"`, path)
		control := readChoiceControl(t, page)
		require.NotNil(t, control, path)
		require.True(t, control.afterFooter, path)
		require.Equal(t, [][2]string{{"", i18n.Format("en", "theme_choice_site", map[string]any{"name": "Zelda"})}, {"tron", "tron"}}, control.options, path)
		// A visitor who chose a theme without templates gets the markup of
		// the application, with the control to go back.
		page, _ = getWithChoice(t, handler, path, "tron")
		require.NotContains(t, page, "theme-shell", path)
		require.Equal(t, []string{"tron"}, readChoiceControl(t, page).selected, path)
	}
}

func TestThemePageStoresTheThemesToChooseHTTP(t *testing.T) {
	handler := themeFilesRouter(t, previewConfig, map[string]string{"forest.yml": previewForestFile, "alder.yml": themeFileColors})
	boxes := func(page string) (map[string]string, []string) {
		fields, shown := templateForm(t, page, "theme-choices-form")
		require.True(t, shown)
		require.Equal(t, "save-choices", fields["action"])
		require.Equal(t, i18n.T("en", "theme_custom_save"), fields["submit"])
		var ticked []string
		for name, state := range fields {
			if strings.HasPrefix(name, "theme-choice-") && state == " checked" {
				ticked = append(ticked, strings.TrimPrefix(name, "theme-choice-"))
			}
		}
		return fields, ticked
	}
	stored := func() []string {
		options, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		return options.ThemeChoices
	}

	// The form has a box for every theme that can be selected, and none is
	// ticked. The custom colors are not among them before any are stored.
	page := getPage(t, handler, "/settings/theme")
	fields, ticked := boxes(page)
	require.Empty(t, ticked)
	for _, theme := range define.ThemePalettes {
		require.Contains(t, fields, "theme-choice-"+theme.Name)
	}
	require.Contains(t, fields, "theme-choice-forest")
	require.Contains(t, fields, "theme-choice-alder")
	require.NotContains(t, fields, "theme-choice-custom")
	require.Len(t, fields, len(define.ThemePalettes)+2+2)
	require.Contains(t, page, i18n.Format("en", "theme_choices_selected", map[string]any{"name": "blackboard"}))
	require.NotContains(t, page, i18n.T("en", "theme_choices_saved"))
	require.Nil(t, readChoiceControl(t, getPage(t, handler, "/")))

	// The ticked themes are stored in the order of the page, whatever the
	// order of the request. A box for a name that is no theme stores nothing.
	rec := postThemeForm(t, handler, url.Values{
		"action": {"save-choices"}, "theme-choice-forest": {"1"}, "theme-choice-tron": {"1"}, "theme-choice-alder": {"1"},
		"theme-choice-gone": {"1"}, "theme-choice-../x": {"1"}, "theme-choice-": {"1"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), i18n.T("en", "theme_choices_saved"))
	require.Equal(t, []string{"tron", "alder", "forest"}, stored())
	_, ticked = boxes(rec.Body.String())
	require.ElementsMatch(t, []string{"tron", "alder", "forest"}, ticked)
	// The pages offer them at once.
	control := readChoiceControl(t, getPage(t, handler, "/"))
	require.NotNil(t, control)
	require.Equal(t, [][2]string{{"", i18n.Format("en", "theme_choice_site", map[string]any{"name": "blackboard"})}, {"tron", "tron"}, {"alder", "alder"}, {"forest", "Forest at dusk"}}, control.options)

	// The selected theme can be ticked: it is a choice once another theme
	// is selected.
	rec = postThemeForm(t, handler, url.Values{"action": {"save-choices"}, "theme-choice-blackboard": {"1"}, "theme-choice-tron": {"1"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []string{"blackboard", "tron"}, stored())
	require.Len(t, readChoiceControl(t, getPage(t, handler, "/")).options, 2)

	// A form with no box ticked stores an empty list, and the pages have no
	// control again.
	rec = postThemeForm(t, handler, url.Values{"action": {"save-choices"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, stored())
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(raw), "ThemeChoices: []")
	_, ticked = boxes(rec.Body.String())
	require.Empty(t, ticked)
	require.Nil(t, readChoiceControl(t, getPage(t, handler, "/")))

	// A name on the list that is no theme, as a file that went away leaves
	// one, has no box and is taken off the list by saving the form.
	require.NoError(t, data.SaveThemeChoices([]string{"gone", "tron"}))
	fields, ticked = boxes(getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{"tron"}, ticked)
	require.NotContains(t, fields, "theme-choice-gone")
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-choices"}, "theme-choice-tron": {"1"}}).Code)
	require.Equal(t, []string{"tron"}, stored())

	// The custom colors have a box once colors are stored for them.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-custom"}, "custom-background": {"#101010"}, "custom-primary": {"#f0f0f0"}, "custom-accent": {"#808080"}}).Code)
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"blackboard"}}).Code)
	fields, _ = boxes(getPage(t, handler, "/settings/theme"))
	require.Contains(t, fields, "theme-choice-custom")
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-choices"}, "theme-choice-custom": {"1"}}).Code)
	require.Equal(t, []string{"custom"}, stored())
	control = readChoiceControl(t, getPage(t, handler, "/"))
	require.Equal(t, [2]string{"custom", i18n.T("en", "theme_custom")}, control.options[1])
}

// In safe mode the themes of the themes directory have no box, and saving
// the form does not take them off the list.
func TestThemeChoicesOfTheDirectoryOutlastSafeModeHTTP(t *testing.T) {
	customThemeDirectory(t, previewConfig+"ThemeChoices: [forest, tron, gazette]\n")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	writeThemeFile(t, "forest.yml", previewForestFile)
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, ThemeSafeMode: true})
	require.NoError(t, err)
	fields, shown := templateForm(t, getPage(t, handler, "/settings/theme"), "theme-choices-form")
	require.True(t, shown)
	require.NotContains(t, fields, "theme-choice-forest")
	require.Equal(t, " checked", fields["theme-choice-tron"])
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-choices"}, "theme-choice-gazette": {"1"}}).Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, []string{"gazette", "forest"}, options.ThemeChoices)
}

func TestControlOfAPrivateSiteCarriesTheTokenOfTheSession(t *testing.T) {
	for _, visibility := range []string{"DEFAULT", "PRIVATE"} {
		t.Run(visibility, func(t *testing.T) {
			customThemeDirectory(t, previewConfig+"ThemeChoices: [forest, tron]\n")
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
			require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
			writeThemeFile(t, "forest.yml", previewForestFile)
			store, err := security.Open(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			require.NoError(t, store.Initialize("admin", "legacy", "local"))
			app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: visibility, StartupStatus: "ready"}, store)
			require.NoError(t, err)
			token, err := store.Authenticate("admin", "legacy")
			require.NoError(t, err)
			session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
			var kept []*http.Cookie
			request := func(method, path string, values url.Values) (*http.Response, string) {
				var body io.Reader
				if values != nil {
					body = strings.NewReader(values.Encode())
				}
				req := httptest.NewRequest(method, "http://flare.test"+path, body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(session)
				for _, cookie := range kept {
					req.AddCookie(cookie)
				}
				res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
				require.NoError(t, e)
				raw, e := io.ReadAll(res.Body)
				res.Body.Close()
				require.NoError(t, e)
				kept = append(kept, res.Cookies()...)
				return res, string(raw)
			}
			for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
				_, page := request(http.MethodGet, path, nil)
				control := readChoiceControl(t, page)
				require.NotNil(t, control, path)
				// A public site takes the form from anyone, and hands no
				// token of a session to a page every visitor gets.
				require.Equal(t, visibility == "PRIVATE", control.token != "", path)
				fields := url.Values{"theme": {"forest"}, "back": {control.back}}
				if control.token != "" {
					fields.Set("csrf-token", control.token)
				}
				res, _ := request(http.MethodPost, control.action, fields)
				require.Equal(t, http.StatusSeeOther, res.StatusCode, path)
				require.Equal(t, path, res.Header.Get("Location"), path)
			}
			// A preview and safe mode for one page are not the page a
			// visitor gets, and have no control.
			for _, path := range []string{"/?theme-preview=tron", "/?theme-safe-mode"} {
				_, page := request(http.MethodGet, path, nil)
				require.Nil(t, readChoiceControl(t, page), path)
			}
		})
	}
}
