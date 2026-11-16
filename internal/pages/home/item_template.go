package home

import (
	"html/template"
	"io"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/themetemplate"
)

// requestThemeItems returns the item templates the entries of this response
// are written with, or nil for the markup of the application. The theme is
// the one the page is shown with, as for the template of the page: the
// selected one once its templates are confirmed, and a previewed one before
// that. Safe mode uses none, and neither does the guide.
func requestThemeItems(c fiber.Ctx, options model.Application, locale string) *define.ThemeItems {
	if !options.ThemeTemplates || requestSafeMode(c) || guideAssetsURI(c) != "" {
		return nil
	}
	theme, preview, _ := requestTemplateTheme(c, options)
	return define.ThemeItemTemplates(options, theme, preview, locale, func(key string) string { return i18n.T(locale, key) })
}

// entryContent fills the template of an application or a bookmark in.
type entryContent struct {
	name, description, host, initial string
	// href is the address the link leads to, as navigationHref checked it,
	// and icon the markup navigationIcon wrote.
	href, icon string
	newTab     bool
}

func (e *entryContent) Text(field string) string {
	switch field {
	case themetemplate.FieldName:
		return e.name
	case themetemplate.FieldDescription:
		return e.description
	case themetemplate.FieldHost:
		return e.host
	case themetemplate.FieldInitial:
		return e.initial
	}
	return ""
}

func (e *entryContent) Address() string { return e.href }

// LinkAttributes are those the link of an entry has in the markup of the
// application.
func (e *entryContent) LinkAttributes() template.HTMLAttr {
	if e.newTab {
		return ` target="_blank" rel="noopener"`
	}
	return ` rel="noopener"`
}

func (e *entryContent) Slot(name string) template.HTML {
	if name == themetemplate.SlotIcon {
		// #nosec G203 -- the markup navigationIcon wrote: a bundled icon or an image with a checked and escaped address.
		return template.HTML(e.icon)
	}
	return ""
}

// groupContent fills the template of a group in. The bookmarks are written
// where the template places them, straight into the list.
type groupContent struct {
	name      string
	bookmarks func()
}

func (g *groupContent) Text(string) string                { return g.name }
func (g *groupContent) Address() string                   { return "" }
func (g *groupContent) LinkAttributes() template.HTMLAttr { return "" }
func (g *groupContent) Slot(string) template.HTML         { return "" }

func (g *groupContent) WriteSlot(_ io.Writer, name string) {
	if name == themetemplate.SlotItems {
		g.bookmarks()
	}
}

// entryHost is the host an entry leads to, for an address of the web, and
// empty for any other.
func entryHost(raw string) string {
	address, err := url.Parse(raw)
	if err != nil || (address.Scheme != "http" && address.Scheme != "https") {
		return ""
	}
	return address.Hostname()
}

// entryInitial is the first character of a name.
func entryInitial(name string) string {
	for _, first := range strings.TrimSpace(name) {
		return string(first)
	}
	return ""
}
