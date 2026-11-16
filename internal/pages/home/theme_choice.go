package home

import (
	"net/http"
	"slices"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// ThemeChoicePath is where a visitor submits the theme the pages are to be
// shown with, in the field themeChoiceField. An empty value, or the name of
// the selected theme, gives the choice up. The field themeChoiceBack names
// the page to go back to.
const (
	ThemeChoicePath  = "/theme-choice"
	themeChoiceField = "theme"
	themeChoiceBack  = "back"
)

// themeChoiceCookie holds the name of the theme a visitor chose. It is a
// preference and nothing else: it identifies nobody, and it is honored only
// for a theme the administrator lets visitors choose.
const themeChoiceCookie = "flare_theme"

// themeChoiceMaxAge is how long a choice is kept without being made again.
const themeChoiceMaxAge = 365 * 24 * 60 * 60

// themeChoiceOption is one theme of the control a visitor chooses with.
type themeChoiceOption struct {
	// Name is what the form submits: the name of the theme, and nothing for
	// the theme of the site.
	Name     string
	Title    string
	Selected bool
}

// themeChoiceForm is the control of a page a visitor chooses a theme with.
type themeChoiceForm struct {
	Action    string
	Field     string
	BackField string
	// Back is the page the form was shown on, if a choice leads back to it.
	Back    string
	Label   string
	Submit  string
	Options []themeChoiceOption
}

// themeChoicePages are the pages a choice leads back to.
func themeChoicePages() []string {
	return []string{define.RegularPages.Home.Path, define.RegularPages.Applications.Path, define.RegularPages.Bookmarks.Path, define.RegularPages.Help.Path}
}

// themeChoiceTitle is the name a theme is offered under.
func themeChoiceTitle(name, locale string) string {
	if name == model.ThemeCustomName {
		return i18n.T(locale, "theme_custom")
	}
	return define.ThemeTitle(name, locale)
}

// themeChoiceControl returns the control for this response: the theme of
// the site first, then the themes that can be chosen, with the one the
// response is shown with marked. There is none while there is nothing to
// choose, and none on a response that is a preview or shown in safe mode,
// which is not the page a visitor gets.
func themeChoiceControl(c fiber.Ctx, locale string) *themeChoiceForm {
	if !requestChoices(c) || requestSafeMode(c) || requestPreview(c) != "" {
		return nil
	}
	options, err := requestSettings(c)
	if err != nil {
		return nil
	}
	chosen := requestChoice(c)
	selected, _ := define.ThemeSnapshot()
	form := &themeChoiceForm{
		Action: ThemeChoicePath, Field: themeChoiceField, BackField: themeChoiceBack, Back: define.RegularPages.Home.Path,
		Label: i18n.T(locale, "theme_choice_label"), Submit: i18n.T(locale, "theme_choice_apply"),
		Options: []themeChoiceOption{{
			Title:    i18n.Format(locale, "theme_choice_site", map[string]any{"name": themeChoiceTitle(selected, locale)}),
			Selected: chosen == "",
		}},
	}
	if path := c.Path(); slices.Contains(themeChoicePages(), path) {
		form.Back = path
	}
	for _, name := range define.ThemeChoices(options) {
		form.Options = append(form.Options, themeChoiceOption{Name: name, Title: themeChoiceTitle(name, locale), Selected: name == chosen})
	}
	return form
}

// chosenTheme returns the theme the cookie of the request names, if it is
// one a visitor can choose now.
func chosenTheme(c fiber.Ctx, options model.Application) string {
	if len(options.ThemeChoices) == 0 {
		return ""
	}
	name := c.Cookies(themeChoiceCookie)
	if name == "" || !slices.Contains(define.ThemeChoices(options), name) {
		return ""
	}
	return name
}

// requestChoice returns the theme the visitor has this response shown with,
// or an empty name. It is known once requestSettings has run.
func requestChoice(c fiber.Ctx) string {
	if snapshot, taken := c.Locals(requestSettingsKey{}).(settingsSnapshot); taken {
		return snapshot.choice
	}
	return ""
}

// requestChoices reports whether visitors can choose among themes. It is
// known once requestSettings has run.
func requestChoices(c fiber.Ctx) bool {
	snapshot, taken := c.Locals(requestSettingsKey{}).(settingsSnapshot)
	return taken && snapshot.choices
}

// chooseTheme stores the choice of a visitor in a cookie and sends the
// visitor back to the page the choice was made on. Nothing is stored on the
// instance. A name that is no choice is refused, and the cookie is left as
// it is.
func chooseTheme(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	options, err := requestSettings(c)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	name := c.FormValue(themeChoiceField)
	selected, _ := define.ThemeSnapshot()
	cookie := &fiber.Cookie{Name: themeChoiceCookie, Path: "/", HTTPOnly: true, Secure: c.Secure(), SameSite: "Lax"}
	choices := define.ThemeChoices(options)
	if name == "" || name == selected {
		cookie.MaxAge, cookie.Expires = -1, time.Unix(1, 0)
	} else if at := slices.Index(choices, name); at >= 0 {
		// The value is taken from the list, so nothing of the request is
		// written into the cookie.
		cookie.Value, cookie.MaxAge = choices[at], themeChoiceMaxAge
	} else {
		return c.Status(http.StatusBadRequest).SendString("This theme cannot be chosen")
	}
	c.Cookie(cookie)
	back := define.RegularPages.Home.Path
	for _, page := range themeChoicePages() {
		if c.FormValue(themeChoiceBack) == page {
			back = page
		}
	}
	c.Set("Location", back)
	return c.SendStatus(http.StatusSeeOther)
}
