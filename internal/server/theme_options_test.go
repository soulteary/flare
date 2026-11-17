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

// themeOptionsText declares one option of each kind, with labels in two
// languages and one label that looks like markup.
const themeOptionsText = `name: Zelda
options:
  - id: density
    type: enum
    label: { en: Density, zh: 密度 }
    choices:
      - value: normal
        label: { en: Normal, zh: 标准 }
      - value: compact
        label: { en: Compact, zh: 紧凑 }
        tokens: { radius: 2px, font-scale: 0.9 }
        layout: { HideAppDescriptions: true }
  - id: frames
    type: boolean
    label: "<b>Frames</b>"
    tokens: { border: "#ffffff" }
  - id: highlight
    type: color
    label: { en: Highlight, zh: 强调色 }
    token: accent
`

// themeOptionsPage is the part of the theme page about the options of the
// selected theme, as a browser reads it.
type themeOptionsPage struct {
	title string
	// fields holds the fields of the form in order: the name, the kind of
	// control, the value it has, and its label.
	fields [][4]string
	// values holds the values a list offers, by the name of the field, with
	// their labels.
	values map[string][]string
	status []string
	// actions holds the hidden actions of the two forms, and themes the
	// theme each of them names.
	actions []string
	themes  []string
	// notes holds what the cards of the themes say about them, by the name
	// of the theme.
	notes map[string][]string
}

func readThemeOptionsPage(t *testing.T, page string) themeOptionsPage {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	read := themeOptionsPage{values: map[string][]string{}, notes: map[string][]string{}}
	labels := map[string]string{}
	form := func(root *html.Node) {
		require.Equal(t, "POST", nodeAttribute(root, "method"))
		walkNodes(root, func(node *html.Node) {
			if node.Type != html.ElementNode {
				return
			}
			name := nodeAttribute(node, "name")
			switch {
			case node.Data == "h2":
				read.title = nodeText(node)
			case node.Data == "input" && name == "action":
				read.actions = append(read.actions, nodeAttribute(node, "value"))
			case node.Data == "input" && name == "theme-options-theme":
				require.Equal(t, "hidden", nodeAttribute(node, "type"))
				read.themes = append(read.themes, nodeAttribute(node, "value"))
			case node.Data == "label" && nodeAttribute(node, "for") != "":
				labels[nodeAttribute(node, "for")] = strings.TrimSpace(nodeText(node))
			case node.Data == "input" && nodeAttribute(node, "type") == "checkbox":
				value := "off"
				for _, attribute := range node.Attr {
					if attribute.Key == "checked" {
						value = "on"
					}
				}
				require.Equal(t, "true", nodeAttribute(node, "value"))
				read.fields = append(read.fields, [4]string{name, "checkbox", value, strings.TrimSpace(nodeText(node.Parent))})
			case node.Data == "input" && nodeAttribute(node, "type") == "color":
				read.fields = append(read.fields, [4]string{name, "color", nodeAttribute(node, "value"), nodeAttribute(node, "id")})
			case node.Data == "select":
				selected := ""
				for option := node.FirstChild; option != nil; option = option.NextSibling {
					if option.Type != html.ElementNode {
						continue
					}
					require.Equal(t, "option", option.Data)
					read.values[name] = append(read.values[name], nodeAttribute(option, "value")+"="+strings.TrimSpace(nodeText(option)))
					for _, attribute := range option.Attr {
						if attribute.Key == "selected" {
							selected = nodeAttribute(option, "value")
						}
					}
				}
				read.fields = append(read.fields, [4]string{name, "select", selected, nodeAttribute(node, "id")})
			case nodeHasClass(node, "theme-notice") && nodeAttribute(node, "role") == "status":
				read.status = append(read.status, nodeText(node))
			}
		})
	}
	walkNodes(document, func(node *html.Node) {
		switch {
		case nodeHasClass(node, "theme-options-form"), nodeHasClass(node, "theme-options-reset-form"):
			form(node)
		case nodeHasClass(node, "theme-file"):
			name := ""
			walkNodes(node, func(part *html.Node) {
				if nodeHasClass(part, "theme-choose") {
					name = nodeAttribute(part, "value")
					read.notes[name] = []string{}
				}
				if nodeHasClass(part, "theme-about") {
					read.notes[name] = append(read.notes[name], nodeText(part))
				}
			})
		}
	})
	// A list and a color have their labels beside them, by their ids.
	for i, field := range read.fields {
		if field[1] != "checkbox" {
			read.fields[i][3] = labels[field[3]]
		}
	}
	return read
}

// bodyAttributes returns what a page writes on its body for the stylesheet
// of a theme, in the order of the page, and the classes of the body.
func bodyAttributes(t *testing.T, page string) ([]string, string) {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var attributes []string
	class := ""
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "body" {
			return
		}
		for _, attribute := range node.Attr {
			if strings.HasPrefix(attribute.Key, "data-") {
				attributes = append(attributes, attribute.Key+"="+attribute.Val)
			}
		}
		class = nodeAttribute(node, "class")
	})
	return attributes, class
}

func storedThemeOptions(t *testing.T, theme string) map[string]string {
	t.Helper()
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	return options.ThemeSettings[theme].Options
}

// A theme can offer options. The theme page has a form for those of the
// selected theme, and their values reach the pages.
func TestThemeOptionsHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: zelda\nShowApps: true\n", map[string]string{
		"zelda.yml":        themeFileColors + "  radius: 10px\n" + themeOptionsText,
		"styled/theme.yml": themeFileColors + "options:\n  - id: frames\n    type: boolean\n    default: true\n",
		"styled/theme.css": "body[data-theme-frames=\"true\"] .app-item { outline: 1px solid }\n",
		"plain.yml":        themeFileColors,
		"refused.yml":      themeFileColors + "options:\n  - id: density\n    type: enum\n",
	})
	navigation := []string{"/", "/applications", "/bookmarks", "/help"}
	settings := []string{"/settings/theme", "/settings/layout", "/settings/search"}
	post := func(form url.Values) *httptest.ResponseRecorder { return themeRequest(handler, form) }
	// about returns a form of the options as the page of a theme has it:
	// with the name of that theme.
	about := func(theme string, form url.Values) url.Values {
		form.Set("theme-options-theme", theme)
		return form
	}

	// The theme page offers the options of the selected theme with the
	// values they have: the first value, off, and the accent of the theme.
	// The cards say which themes have options, and a file whose options are
	// not accepted is named with the part.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeOptionsPage(t, raw)
	require.Equal(t, "Options of Zelda", page.title)
	require.Equal(t, [][4]string{
		{"option-density", "select", "normal", "Density"},
		{"option-frames", "checkbox", "off", "<b>Frames</b>"},
		{"option-highlight", "color", "#f2b84b", "Highlight"},
	}, page.fields)
	require.Equal(t, map[string][]string{"option-density": {"normal=Normal", "compact=Compact"}}, page.values)
	require.Equal(t, []string{"save-options", "reset-options"}, page.actions)
	require.Equal(t, []string{"zelda", "zelda"}, page.themes)
	require.Empty(t, page.status)
	hasOptions := i18n.T("en", "theme_options_note")
	require.Equal(t, map[string][]string{"plain": {}, "styled": {i18n.T("en", "theme_stylesheet_note"), hasOptions}, "zelda": {hasOptions}}, page.notes)
	require.NotContains(t, raw, "<b>Frames</b>")
	require.Contains(t, readThemeFilePage(t, raw).lineTexts, "refused.yml "+i18n.T("en", "theme_file_problem_option")+" options.density.choices")

	// With the defaults the pages have the theme as its file has it, and
	// the values of the options on the body of the navigation pages.
	for _, path := range navigation {
		body := getPage(t, handler, path)
		attributes, class := bodyAttributes(t, body)
		require.Equal(t, []string{"data-theme-density=normal", "data-theme-frames=false"}, attributes, path)
		require.NotContains(t, class, "hide-app-descriptions", path)
		require.Equal(t, themeFileStyle+"--flare-radius:10px;", bodyStyle(t, body), path)
	}
	// The theme sets no layout of its own, and the appearance page names
	// none.
	require.Empty(t, readLayoutPage(t, getPage(t, handler, "/settings/layout")).notes)

	// Values that are chosen are stored for the theme and applied: tokens
	// on every themed page, layout and attributes on the navigation pages.
	saved := post(about("zelda", url.Values{"action": {"save-options"}, "option-density": {"compact"}, "option-frames": {"true"}, "option-highlight": {"#00ff88"}}))
	require.Equal(t, http.StatusOK, saved.Code)
	page = readThemeOptionsPage(t, saved.Body.String())
	require.Equal(t, []string{i18n.T("en", "theme_options_saved")}, page.status)
	require.Equal(t, [][4]string{
		{"option-density", "select", "compact", "Density"},
		{"option-frames", "checkbox", "on", "<b>Frames</b>"},
		{"option-highlight", "color", "#00ff88", "Highlight"},
	}, page.fields)
	require.Equal(t, map[string]string{"density": "compact", "frames": "true", "highlight": "#00ff88"}, storedThemeOptions(t, "zelda"))
	chosen := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;--flare-border:#ffffff;--flare-radius:2px;--flare-font-scale:0.9;"
	for _, path := range navigation {
		body := getPage(t, handler, path)
		attributes, class := bodyAttributes(t, body)
		require.Equal(t, []string{"data-theme-density=compact", "data-theme-frames=true"}, attributes, path)
		require.Equal(t, chosen, bodyStyle(t, body), path)
		if path != "/help" {
			require.Contains(t, class, "hide-app-descriptions", path)
		}
	}
	for _, path := range settings {
		body := getPage(t, handler, path)
		attributes, _ := bodyAttributes(t, body)
		require.Empty(t, attributes, path)
		require.Equal(t, chosen, bodyStyle(t, body), path)
	}
	// The appearance page names the layout a value sets as it names the
	// layout of a theme, with the box that turns it off.
	require.Equal(t, map[string]string{note("theme_layout_note", "Hide"): "theme-layout-ignore-HideAppDescriptions"},
		readLayoutPage(t, getPage(t, handler, "/settings/layout")).notes)

	// Only what differs from the defaults of the theme is stored: an option
	// that is put back follows the theme again.
	require.Equal(t, http.StatusOK, post(about("zelda", url.Values{"action": {"save-options"}, "option-density": {"normal"}, "option-frames": {"true"}, "option-highlight": {"#f2b84b"}})).Code)
	require.Equal(t, map[string]string{"frames": "true"}, storedThemeOptions(t, "zelda"))
	attributes, _ := bodyAttributes(t, getPage(t, handler, "/"))
	require.Equal(t, []string{"data-theme-density=normal", "data-theme-frames=true"}, attributes)

	// A value an option cannot have is refused, and nothing changes. A
	// list and a color are always submitted, so one that is missing is
	// refused as well.
	for name, form := range map[string]url.Values{
		"a value that is not listed": {"action": {"save-options"}, "option-density": {"wide"}, "option-highlight": {"#f2b84b"}},
		"a value of another case":    {"action": {"save-options"}, "option-density": {"Compact"}, "option-highlight": {"#f2b84b"}},
		"a color that is a name":     {"action": {"save-options"}, "option-density": {"normal"}, "option-highlight": {"red"}},
		"a color with a declaration": {"action": {"save-options"}, "option-density": {"normal"}, "option-highlight": {"#00ff88;color:red"}},
		"a list that is not sent":    {"action": {"save-options"}, "option-highlight": {"#f2b84b"}},
		"a color that is not sent":   {"action": {"save-options"}, "option-density": {"normal"}},
		"nothing but the action":     {"action": {"save-options"}},
	} {
		refused := post(about("zelda", form))
		require.Equal(t, http.StatusBadRequest, refused.Code, name)
		require.NotContains(t, refused.Body.String(), "<", name)
		require.Equal(t, map[string]string{"frames": "true"}, storedThemeOptions(t, "zelda"), name)
	}

	// Values in config.yml that are written by hand are taken by the next
	// page, and one that is no value of the option is not used.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: en\nTheme: zelda\nShowApps: true\nThemeSettings:\n    zelda:\n        Options:\n            density: compact\n            frames: perhaps\n            highlight: \"\\\"><script>alert(1)</script>\"\n"), 0600))
	body := getPage(t, handler, "/")
	attributes, _ = bodyAttributes(t, body)
	require.Equal(t, []string{"data-theme-density=compact", "data-theme-frames=false"}, attributes)
	require.Equal(t, themeFileStyle+"--flare-radius:2px;--flare-font-scale:0.9;", bodyStyle(t, body))
	require.NotContains(t, body, "alert(1)")
	page = readThemeOptionsPage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, [][4]string{
		{"option-density", "select", "compact", "Density"},
		{"option-frames", "checkbox", "off", "<b>Frames</b>"},
		{"option-highlight", "color", "#f2b84b", "Highlight"},
	}, page.fields)

	// The defaults of the theme are put back in one step.
	reset := post(about("zelda", url.Values{"action": {"reset-options"}}))
	require.Equal(t, http.StatusOK, reset.Code)
	require.Equal(t, []string{i18n.T("en", "theme_options_saved")}, readThemeOptionsPage(t, reset.Body.String()).status)
	require.Empty(t, storedThemeOptions(t, "zelda"))
	attributes, _ = bodyAttributes(t, getPage(t, handler, "/"))
	require.Equal(t, []string{"data-theme-density=normal", "data-theme-frames=false"}, attributes)

	// A preview has the options of the previewed theme with the values
	// stored for that theme, and the selected theme keeps its own.
	previewed := getPage(t, handler, "/?theme-preview=styled")
	attributes, _ = bodyAttributes(t, previewed)
	require.Equal(t, []string{"data-theme-frames=true"}, attributes)
	require.Len(t, pageStyles(t, previewed), 2)
	attributes, _ = bodyAttributes(t, getPage(t, handler, "/?theme-safe-mode"))
	require.Empty(t, attributes)

	// The labels are those of the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: zelda\nShowApps: true\n"), 0600))
	page = readThemeOptionsPage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, "Zelda 的选项", page.title)
	require.Equal(t, [][4]string{
		{"option-density", "select", "normal", "密度"},
		{"option-frames", "checkbox", "off", "<b>Frames</b>"},
		{"option-highlight", "color", "#f2b84b", "强调色"},
	}, page.fields)
	require.Equal(t, map[string][]string{"option-density": {"normal=标准", "compact=紧凑"}}, page.values)

	// A theme without options has no form, and nothing is stored for it.
	require.Equal(t, http.StatusOK, post(url.Values{"theme": {"plain"}}).Code)
	page = readThemeOptionsPage(t, getPage(t, handler, "/settings/theme"))
	require.Empty(t, page.fields)
	require.Empty(t, page.actions)
	require.Equal(t, http.StatusBadRequest, post(about("plain", url.Values{"action": {"save-options"}, "option-density": {"compact"}})).Code)
	require.Equal(t, http.StatusBadRequest, post(about("plain", url.Values{"action": {"reset-options"}})).Code)
	require.Empty(t, storedThemeOptions(t, "plain"))
	attributes, _ = bodyAttributes(t, getPage(t, handler, "/"))
	require.Empty(t, attributes)

	// Neither has a bundled theme.
	require.Equal(t, http.StatusOK, post(url.Values{"theme": {"blackboard"}}).Code)
	require.Empty(t, readThemeOptionsPage(t, getPage(t, handler, "/settings/theme")).fields)
	require.Equal(t, http.StatusBadRequest, post(about("blackboard", url.Values{"action": {"save-options"}})).Code)

	// The values of a theme are kept while another theme is selected.
	require.Equal(t, http.StatusOK, post(url.Values{"theme": {"styled"}}).Code)
	require.Equal(t, []string{"styled", "styled"}, readThemeOptionsPage(t, getPage(t, handler, "/settings/theme")).themes)
	require.Equal(t, http.StatusOK, post(about("styled", url.Values{"action": {"save-options"}})).Code)
	require.Equal(t, map[string]string{"frames": "false"}, storedThemeOptions(t, "styled"))

	// A form is about the theme it was shown for. One that was shown for
	// another theme, in a tab that was left open, is refused, and so is one
	// that names no theme: their fields are not read as values for the
	// options of the theme that is selected now. Both themes have an option
	// of the name frames.
	for name, form := range map[string]url.Values{
		"the form of another theme":  about("zelda", url.Values{"action": {"save-options"}, "option-density": {"normal"}, "option-frames": {"true"}, "option-highlight": {"#f2b84b"}}),
		"the reset of another theme": about("zelda", url.Values{"action": {"reset-options"}}),
		"a form without a theme":     {"action": {"save-options"}, "option-frames": {"true"}},
		"a reset without a theme":    {"action": {"reset-options"}},
		"a theme that is no name":    about("../styled", url.Values{"action": {"save-options"}, "option-frames": {"true"}}),
	} {
		refused := post(form)
		require.Equal(t, http.StatusBadRequest, refused.Code, name)
		require.NotContains(t, refused.Body.String(), "<", name)
		require.Equal(t, map[string]string{"frames": "false"}, storedThemeOptions(t, "styled"), name)
		require.Empty(t, storedThemeOptions(t, "zelda"), name)
	}
	require.Equal(t, http.StatusOK, post(url.Values{"theme": {"zelda"}}).Code)
	require.Equal(t, map[string]string{"frames": "false"}, storedThemeOptions(t, "styled"))
	require.True(t, define.IsUserTheme("styled"))
}

// The options of a theme are settings of the instance: choosing values and
// putting the defaults back need the session of the administrator and the
// token of the form, like every other write of the theme page.
func TestThemeOptionsRequireTheAdministrator(t *testing.T) {
	customThemeDirectory(t, "Locale: en\nTheme: zelda\n")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	writeThemeFile(t, "zelda.yml", themeFileColors+themeOptionsText)
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	send := func(req *http.Request, signedIn bool) (int, string) {
		if signedIn {
			req.AddCookie(session)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		return res.StatusCode, string(raw)
	}
	submit := func(action, csrf string, signedIn bool, origin string) int {
		form := url.Values{"action": {action}, "theme-options-theme": {"zelda"}, "option-density": {"compact"}, "option-highlight": {"#00ff88"}}
		if csrf != "" {
			form.Set("csrf-token", csrf)
		}
		req := httptest.NewRequest(http.MethodPost, "http://flare.test/settings/theme", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		status, _ := send(req, signedIn)
		return status
	}

	// Both forms carry the token of the session.
	status, page := send(httptest.NewRequest(http.MethodGet, "http://flare.test/settings/theme", nil), true)
	require.Equal(t, http.StatusOK, status)
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var tokens []string
	walkNodes(document, func(node *html.Node) {
		if nodeHasClass(node, "theme-options-form") || nodeHasClass(node, "theme-options-reset-form") {
			walkNodes(node, func(part *html.Node) {
				if part.Type == html.ElementNode && part.Data == "input" && nodeAttribute(part, "name") == "csrf-token" {
					tokens = append(tokens, nodeAttribute(part, "value"))
				}
			})
		}
	})
	require.Len(t, tokens, 2)
	require.NotEmpty(t, tokens[0])
	require.Equal(t, tokens[0], tokens[1])
	csrf := tokens[0]

	// A visitor does not get the page with the form at all.
	status, page = send(httptest.NewRequest(http.MethodGet, "http://flare.test/settings/theme", nil), false)
	require.NotEqual(t, http.StatusOK, status)
	require.NotContains(t, page, "theme-options-form")

	for _, action := range []string{"save-options", "reset-options"} {
		require.Equal(t, http.StatusUnauthorized, submit(action, csrf, false, ""), action+": no session")
		require.Equal(t, http.StatusForbidden, submit(action, "", true, ""), action+": no token")
		require.Equal(t, http.StatusForbidden, submit(action, csrf, true, "https://evil.example"), action+": foreign origin")
	}
	require.Empty(t, storedThemeOptions(t, "zelda"), "a refused request changed the settings")

	require.Equal(t, http.StatusOK, submit("save-options", csrf, true, ""))
	require.Equal(t, map[string]string{"density": "compact", "highlight": "#00ff88"}, storedThemeOptions(t, "zelda"))
	// The values show on the page of a visitor: they are part of the theme.
	status, page = send(httptest.NewRequest(http.MethodGet, "http://flare.test/", nil), false)
	require.Equal(t, http.StatusOK, status)
	attributes, _ := bodyAttributes(t, page)
	require.Equal(t, []string{"data-theme-density=compact", "data-theme-frames=false"}, attributes)
	require.Equal(t, http.StatusOK, submit("reset-options", csrf, true, ""))
	require.Empty(t, storedThemeOptions(t, "zelda"))
}
