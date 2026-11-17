package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/i18n"
)

// The fields of the pages the settings of the appearance are spread over.
var splitPages = map[string][]string{
	"/settings/modules": {"greetings", "hide-help-button", "hide-settings-button", "hide-version", "show-apps", "show-bookmarks", "show-title"},
	"/settings/clock":   {"clock-dynamic", "clock-seconds", "clock-timezone", "clock-timezone-mode", "show-datetime"},
	"/settings/links":   {"enable-encrypted-link", "icon-mode", "keep-letter-case", "open-app-newtab", "open-bookmark-newtab"},
	"/settings/layout":  {"bookmark-layout", "category-navigation", "desktop-columns", "hide-app-descriptions", "mobile-bookmark-layout", "navigation-text-size", "title-size"},
	"/settings/site":    {"footer", "locale", "title"},
}

// What an administrator sets on each page, none of it a default.
var splitValues = map[string]url.Values{
	"/settings/modules": {"show-title": {"1"}, "greetings": {"Hello & <welcome>"}, "show-apps": {"1"}, "show-bookmarks": {"1"}, "hide-settings-button": {"1"}, "hide-help-button": {"1"}, "hide-version": {"1"}},
	"/settings/clock":   {"show-datetime": {"1"}, "clock-dynamic": {"1"}, "clock-seconds": {"0"}, "clock-timezone-mode": {"custom"}, "clock-timezone": {"Asia/Tokyo"}},
	"/settings/links":   {"open-app-newtab": {"1"}, "open-bookmark-newtab": {"1"}, "enable-encrypted-link": {"1"}, "keep-letter-case": {"1"}, "icon-mode": {"FILLING"}},
	"/settings/layout":  {"desktop-columns": {"6"}, "bookmark-layout": {"sections"}, "category-navigation": {"tabs"}, "mobile-bookmark-layout": {"compact"}, "navigation-text-size": {"large"}, "title-size": {"prominent"}, "hide-app-descriptions": {"1"}},
	"/settings/site":    {"locale": {"en"}, "title": {"Mine & <yours>"}, "footer": {"<b>foot</b>"}},
}

// readSettingsForm returns the address a page of the settings submits its
// form to, and the fields of the form with the values they are shown with.
func readSettingsForm(t *testing.T, page string) (string, url.Values) {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	action, fields, forms := "", url.Values{}, 0
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "form" {
			return
		}
		forms++
		action = nodeAttribute(node, "action")
		require.Equal(t, "POST", nodeAttribute(node, "method"))
		walkNodes(node, func(field *html.Node) {
			if field.Type != html.ElementNode {
				return
			}
			name := nodeAttribute(field, "name")
			switch field.Data {
			case "select":
				selected, options := "", 0
				walkNodes(field, func(option *html.Node) {
					if option.Type != html.ElementNode || option.Data != "option" {
						return
					}
					options++
					for _, attribute := range option.Attr {
						if attribute.Key == "selected" {
							require.Empty(t, selected, "%s has two values", name)
							selected = nodeAttribute(option, "value")
						}
					}
				})
				require.NotZero(t, options, name)
				require.NotEmpty(t, selected, "%s shows no value", name)
				fields.Add(name, selected)
			case "textarea":
				fields.Add(name, nodeText(field))
			case "input":
				if kind := nodeAttribute(field, "type"); kind != "submit" && name != "csrf-token" {
					fields.Add(name, nodeAttribute(field, "value"))
				}
			}
		})
	})
	require.Equal(t, 1, forms, "a page has one form")
	return action, fields
}

func postSettingsForm(t *testing.T, handler http.Handler, path string, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// storedSettings reads the settings without what changes with every save.
func storedSettings(t *testing.T) model.Application {
	t.Helper()
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.ConfigRevision, options.AppVersion, options.ConfigVersionState = "", "", ""
	// An empty list is written out once the file is saved.
	if len(options.ThemeChoices) == 0 {
		options.ThemeChoices = nil
	}
	return options
}

func TestSettingsOfTheAppearanceAreOnFivePagesHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Locale: en\nTheme: blackboard\nShowTitle: true\n")
	seen := map[string]string{}
	for path, want := range splitPages {
		page := getPage(t, handler, path)
		action, fields := readSettingsForm(t, page)
		// A page submits to its own address, and has its own fields.
		require.Equal(t, path, action, path)
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
			require.Empty(t, seen[name], "%s is on %s and on %s", name, seen[name], path)
			seen[name] = path
		}
		sort.Strings(names)
		require.Equal(t, want, names, path)
		// Its title is its entry of the menu, with a sentence about it.
		key := strings.TrimPrefix(path, "/settings/")
		require.Contains(t, page, "<h2>"+i18n.T("en", "nav_"+key)+"</h2>", path)
		require.Contains(t, page, i18n.T("en", "settings_"+key+"_help"), path)
		// Only the page of the clock has a script of its own, and the
		// policy for it.
		rec := getResponse(handler, path)
		require.Equal(t, path == "/settings/clock", rec.Header().Get("Content-Security-Policy") == clock.SettingsPolicy(), path)
		require.Equal(t, path == "/settings/clock", strings.Contains(page, "clock-preview"), path)
		require.Equal(t, 1, strings.Count(page, "history.back()"), "%s leaves by the escape key once", path)
	}
	require.Len(t, seen, 27)
	// The page of the modules says where the date and time are set.
	require.Regexp(t, regexp.QuoteMeta(i18n.T("en", "settings_modules_clock"))+` <a href="?/settings/clock"?>`+i18n.T("en", "nav_clock")+`</a>`, getPage(t, handler, "/settings/modules"))
	// The greeting has a field only while the title is shown.
	require.Equal(t, http.StatusOK, postSettingsForm(t, handler, "/settings/modules", url.Values{"show-title": {"0"}}).Code)
	_, fields := readSettingsForm(t, getPage(t, handler, "/settings/modules"))
	require.NotContains(t, fields, "greetings")
}

func TestSavingOnePageOfTheSettingsLeavesTheOthersHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Theme: blackboard\n")
	paths := make([]string, 0, len(splitValues))
	for path := range splitValues {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	before := storedSettings(t)

	// Each page stores what it submits, and the page shows it afterwards.
	// The greeting is taken while the title is shown, which the same form
	// turns on.
	for _, path := range paths {
		rec := postSettingsForm(t, handler, path, splitValues[path])
		require.Equal(t, http.StatusOK, rec.Code, path)
		action, shown := readSettingsForm(t, rec.Body.String())
		require.Equal(t, path, action, "the page that is shown after saving")
		require.Equal(t, splitValues[path], shown, path)
	}
	stored := storedSettings(t)
	want := before
	want.Locale, want.Title, want.Footer = "en", "Mine & <yours>", "<b>foot</b>"
	want.ShowTitle, want.Greetings, want.ShowApps, want.ShowBookmarks = true, "Hello & <welcome>", true, true
	want.HideSettingsButton, want.HideHelpButton, want.HideVersion = true, true, true
	want.ShowDateTime = true
	seconds := false
	want.Clock = &model.ClockOptions{Dynamic: true, Seconds: &seconds, TimezoneMode: "custom", Timezone: "Asia/Tokyo"}
	want.OpenAppNewTab, want.OpenBookmarkNewTab, want.EnableEncryptedLink, want.KeepLetterCase, want.IconMode = true, true, true, true, "FILLING"
	homepage := model.HomepageOptions{}
	if before.Homepage != nil {
		homepage = *before.Homepage
	}
	homepage.DesktopColumns, homepage.BookmarkLayout, homepage.CategoryNavigation = "6", "sections", "tabs"
	homepage.MobileBookmarkLayout, homepage.NavigationTextSize, homepage.TitleSize, homepage.HideAppDescriptions = "compact", "large", "prominent", true
	want.Homepage = &homepage
	require.Equal(t, want, stored)

	// Saving a page as it is shown changes nothing, on that page or on any
	// other: every value is other than a default, so a field that a page
	// reset because its form lacks it would show.
	for _, path := range paths {
		action, shown := readSettingsForm(t, getPage(t, handler, path))
		require.Equal(t, splitValues[path], shown, path)
		require.Equal(t, http.StatusOK, postSettingsForm(t, handler, action, shown).Code, path)
		require.Equal(t, stored, storedSettings(t), "after saving %s", path)
	}

	// One field of a page changes that field. A form may also name fields
	// of another page: the address does not decide what is stored.
	changes := []struct {
		path   string
		fields url.Values
		apply  func(*model.Application)
	}{
		{"/settings/modules", url.Values{"show-apps": {"0"}}, func(o *model.Application) { o.ShowApps = false }},
		{"/settings/modules", url.Values{"greetings": {""}}, func(o *model.Application) { o.Greetings = "" }},
		{"/settings/clock", url.Values{"show-datetime": {"0"}}, func(o *model.Application) { o.ShowDateTime = false }},
		{"/settings/links", url.Values{"icon-mode": {"default"}}, func(o *model.Application) { o.IconMode = "DEFAULT" }},
		{"/settings/links", url.Values{"icon-mode": {"filling"}}, func(o *model.Application) { o.IconMode = "FILLING" }},
		{"/settings/links", url.Values{"icon-mode": {"other"}, "keep-letter-case": {"0"}}, func(o *model.Application) { o.IconMode, o.KeepLetterCase = "DEFAULT", false }},
		{"/settings/layout", url.Values{"desktop-columns": {"legacy"}}, func(o *model.Application) {
			changed := *o.Homepage
			changed.DesktopColumns = ""
			o.Homepage = &changed
		}},
		{"/settings/site", url.Values{"title": {""}}, func(o *model.Application) { o.Title = "" }},
		{"/settings/site", url.Values{"footer": {""}, "hide-version": {"0"}}, func(o *model.Application) { o.Footer, o.HideVersion = "", false }},
		{"/settings/site", url.Values{}, func(*model.Application) {}},
	}
	for _, change := range changes {
		expected := storedSettings(t)
		change.apply(&expected)
		require.Equal(t, http.StatusOK, postSettingsForm(t, handler, change.path, change.fields).Code, "%s %v", change.path, change.fields)
		require.Equal(t, expected, storedSettings(t), "%s %v", change.path, change.fields)
	}

	// What a page refuses is not stored.
	refused := storedSettings(t)
	for path, fields := range map[string]url.Values{
		"/settings/site":   {"locale": {"xx"}, "title": {"not stored"}},
		"/settings/clock":  {"clock-timezone-mode": {"custom"}, "clock-dynamic": {"1"}, "clock-seconds": {"1"}, "clock-timezone": {"/etc/passwd"}},
		"/settings/layout": {"desktop-columns": {"99"}},
	} {
		require.Equal(t, http.StatusBadRequest, postSettingsForm(t, handler, path, fields).Code, path)
		require.Equal(t, refused, storedSettings(t), path)
	}
}

// The address of the page that held these settings leads to the layout, and
// a form of that page, with all of its fields, is still taken.
func TestAddressOfTheAppearancePageHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Theme: blackboard\n")
	rec := getResponse(handler, "/settings/appearance")
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/settings/layout", rec.Header().Get("Location"))
	whole := url.Values{}
	for _, fields := range splitValues {
		for name, values := range fields {
			whole[name] = slices.Clone(values)
		}
	}
	rec = postSettingsForm(t, handler, "/settings/appearance", whole)
	require.Equal(t, http.StatusOK, rec.Code)
	action, shown := readSettingsForm(t, rec.Body.String())
	require.Equal(t, "/settings/layout", action)
	require.Equal(t, splitValues["/settings/layout"], shown)
	for path, fields := range splitValues {
		_, shown = readSettingsForm(t, getPage(t, handler, path))
		require.Equal(t, fields, shown, path)
	}
}
