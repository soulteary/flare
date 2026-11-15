package theme

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/define"
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
	for _, card := range themeFileCards("en", themes) {
		notes[card.Name] = card.Templates
	}
	want := "Brings page templates: home.html, applications.html. Templates of themes are turned off, so the pages keep the markup of the application."
	if len(notes) != 2 || notes["zelda"] != want || notes["plain"] != "" {
		t.Fatalf("notes of the cards: %q", notes)
	}
	if card := themeFileCards("zh", themes)[1]; !strings.Contains(card.Templates, "home.html, applications.html") || card.Templates == want {
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
		themetemplate.ReasonLinkCount, themetemplate.ReasonTextMissing,
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
