package theme

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/themetemplate"
)

const everyPagePart = `{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
	`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}`

func writeThemeTemplate(t *testing.T, theme, file, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(define.UserThemeDirectory, theme, "templates"), 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, filepath.Join(theme, "templates", file), content)
}

// The card of a theme names the page templates it brings and says that they
// are not applied, and a template that is not accepted is listed with the
// reason and what it is about, while the theme can be selected.
func TestThemeTemplatesOnTheThemePage(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeDirectory(t, "zelda", themeFileColors)
	writeThemeTemplate(t, "zelda", "home.html", `<div class="shell">`+everyPagePart+`</div>`)
	writeThemeTemplate(t, "zelda", "applications.html", `<div>{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`)
	writeThemeTemplate(t, "zelda", "bookmarks.html", `<div><iframe src="/"></iframe></div>`)
	writeThemeFile(t, "plain.yml", themeFileColors)

	if status, page := postTheme(t, app, url.Values{"theme": {"zelda"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, page)
	}
	if name, _ := define.ThemeSnapshot(); name != "zelda" {
		t.Fatalf("active theme %q", name)
	}
	themes, problems, notices := define.UserThemes()
	notes := map[string]string{}
	for _, card := range themeFileCards("en", themes, model.Application{}) {
		notes[card.Name] = card.Templates
	}
	want := "Brings page templates: home.html, applications.html. Templates of themes are turned off, so the pages keep the markup of the application."
	if len(notes) != 2 || notes["zelda"] != want || notes["plain"] != "" {
		t.Fatalf("notes of the cards: %q", notes)
	}
	if card := themeFileCards("zh", themes, model.Application{})[1]; !strings.Contains(card.Templates, "home.html, applications.html") || card.Templates == want {
		t.Fatalf("the note in the other language: %q", card.Templates)
	}
	lines := themeFileLines("en", problems, notices)
	if len(lines) != 1 || lines[0] != (themeFileLine{File: "zelda/templates/bookmarks.html", Text: "The template is not used: an element a template cannot hold.", Detail: "iframe"}) {
		t.Fatalf("lines: %+v", lines)
	}
}

// Every reason the compiler can give, and the one for a template that is no
// text, has a text of its own in both languages.
func TestThemeTemplateReasonsHaveTexts(t *testing.T) {
	reasons := []themetemplate.Reason{
		themetemplate.ReasonParse, themetemplate.ReasonReservedName, themetemplate.ReasonExecute, themetemplate.ReasonTooLarge,
		themetemplate.ReasonMarkup, themetemplate.ReasonElement, themetemplate.ReasonAttribute, themetemplate.ReasonUnbalanced,
		themetemplate.ReasonFieldPosition, themetemplate.ReasonAddressPosition, themetemplate.ReasonComputed, themetemplate.ReasonLink,
		themetemplate.ReasonResource, themetemplate.ReasonSlotPosition, themetemplate.ReasonSlotMissing, themetemplate.ReasonSlotRepeated,
		themetemplate.ReasonLinkCount, themetemplate.ReasonTextMissing, themetemplate.ReasonRoot, themetemplate.ReasonElementMissing,
	}
	keys := []string{"theme_file_problem_" + define.ThemeProblemTemplateEncoding}
	for _, reason := range reasons {
		keys = append(keys, "theme_file_problem_"+define.ThemeProblemTemplate+string(reason))
	}
	seen := map[string]string{}
	for _, locale := range []string{"en", "zh"} {
		for _, key := range keys {
			text := i18n.T(locale, key)
			if text == "" || text == key {
				t.Errorf("%s has no text in %s", key, locale)
			}
			if other, twice := seen[locale+text]; twice {
				t.Errorf("%s and %s have the same text in %s", key, other, locale)
			}
			seen[locale+text] = key
		}
	}
}

// The note on the card of a theme with templates says whether they are
// used: not while the site does not allow templates, not before they are
// confirmed for that theme, and not on an instance that uses none.
func TestThemeTemplateNoteSaysWhetherTheTemplatesAreUsed(t *testing.T) {
	themes := []model.Theme{{Name: "zelda", Templates: []model.ThemeTemplate{{Name: "home.html"}, {Name: "bookmarks.html"}}}}
	confirmed := map[string]model.ThemeSettings{"zelda": {Templates: true}}
	note := func(key string) string {
		return strings.ReplaceAll(i18n.T("en", key), "{files}", "home.html, bookmarks.html")
	}
	for _, tc := range []struct {
		name    string
		options model.Application
		want    string
	}{
		{"nothing is allowed", model.Application{}, "theme_templates_note"},
		{"confirmed, but the site does not allow templates", model.Application{ThemeSettings: confirmed}, "theme_templates_note"},
		{"allowed, not confirmed", model.Application{ThemeTemplates: true}, "theme_templates_note_unconfirmed"},
		{"allowed, another theme confirmed", model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{"other": {Templates: true}}}, "theme_templates_note_unconfirmed"},
		{"allowed and confirmed", model.Application{ThemeTemplates: true, ThemeSettings: confirmed}, "theme_templates_note_used"},
	} {
		if got := themeFileCards("en", themes, tc.options)[0].Templates; got != note(tc.want) {
			t.Errorf("%s: %q", tc.name, got)
		}
	}
	define.AppFlags.DisableCSP = true
	t.Cleanup(func() { define.AppFlags.DisableCSP = false })
	if got := themeFileCards("en", themes, model.Application{ThemeTemplates: true, ThemeSettings: confirmed})[0].Templates; got != note("theme_templates_note_unavailable") {
		t.Errorf("without the policy: %q", got)
	}
	texts := map[string]bool{}
	for _, key := range []string{"theme_templates_note", "theme_templates_note_unconfirmed", "theme_templates_note_used", "theme_templates_note_unavailable"} {
		for _, locale := range []string{"en", "zh"} {
			text := i18n.T(locale, key)
			if text == key || texts[text] || !strings.Contains(text, "{files}") {
				t.Errorf("%s in %s: %q", key, locale, text)
			}
			texts[text] = true
		}
	}
}

// The switch and the confirmation are stored with the checks of every
// write of the theme page: a session that is revoked after the request was
// let in stores nothing.
func TestThemeTemplatesAreNotAllowedOrConfirmedForARevokedSession(t *testing.T) {
	trusted := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeDirectory(t, "zelda", themeFileColors)
	writeThemeTemplate(t, "zelda", "home.html", `<div>`+everyPagePart+`</div>`)
	if status, body := postTheme(t, trusted, url.Values{"theme": {"zelda"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, body)
	}

	guarded := newRevocableTheme(t)
	submit := func(form url.Values, revoked bool) (int, string) {
		t.Helper()
		return guarded.post(func(csrf string) (io.Reader, string) {
			form.Set("csrf-token", csrf)
			return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"
		}, revoked)
	}
	stored := func() (bool, bool) {
		t.Helper()
		options, e := data.GetAllSettingsOptions()
		if e != nil {
			t.Fatal(e)
		}
		return options.ThemeTemplates, options.ThemeSettings["zelda"].Templates
	}
	allow := url.Values{"action": {actionSaveTemplates}, templatesAllowed: {"1"}}
	confirm := url.Values{"action": {actionConfirmTemplates}, templatesTheme: {"zelda"}, templatesFiles: {"home.html"}}
	withdraw := url.Values{"action": {actionWithdrawTemplates}, templatesTheme: {"zelda"}}

	if status, body := submit(allow, true); status != http.StatusUnauthorized || body != "AUTH_REQUIRED" {
		t.Fatalf("allow with a revoked session: %d %s", status, body)
	}
	if allowed, _ := stored(); allowed {
		t.Fatal("a revoked session allowed templates")
	}
	if status, body := submit(allow, false); status != http.StatusOK {
		t.Fatalf("allow: %d %s", status, body)
	}
	if status, body := submit(confirm, true); status != http.StatusUnauthorized || body != "AUTH_REQUIRED" {
		t.Fatalf("confirm with a revoked session: %d %s", status, body)
	}
	if allowed, confirmed := stored(); !allowed || confirmed {
		t.Fatalf("stored: %v %v", allowed, confirmed)
	}
	if status, body := submit(confirm, false); status != http.StatusOK {
		t.Fatalf("confirm: %d %s", status, body)
	}
	for _, form := range []url.Values{withdraw, {"action": {actionSaveTemplates}}} {
		if status, body := submit(form, true); status != http.StatusUnauthorized || body != "AUTH_REQUIRED" {
			t.Fatalf("%s with a revoked session: %d %s", form.Get("action"), status, body)
		}
	}
	if allowed, confirmed := stored(); !allowed || !confirmed {
		t.Fatalf("a revoked session took something back: %v %v", allowed, confirmed)
	}

	// A form that was shown for another theme is not about the selected
	// one, in either direction.
	for _, form := range []url.Values{
		{"action": {actionWithdrawTemplates}, templatesTheme: {"other"}},
		{"action": {actionWithdrawTemplates}},
		{"action": {actionConfirmTemplates}, templatesTheme: {"other"}, templatesFiles: {"home.html"}},
	} {
		if status, _ := submit(form, false); status != http.StatusBadRequest {
			t.Errorf("%v: %d", form, status)
		}
	}
	if allowed, confirmed := stored(); !allowed || !confirmed {
		t.Fatalf("stored: %v %v", allowed, confirmed)
	}
	if status, body := submit(withdraw, false); status != http.StatusOK {
		t.Fatalf("withdraw: %d %s", status, body)
	}
	if _, confirmed := stored(); confirmed {
		t.Fatal("the confirmation was not withdrawn")
	}
}
