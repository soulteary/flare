package themetemplate

import (
	"html/template"
)

// PageKind is a navigation page a theme can bring a template for.
type PageKind string

const (
	PageHome         PageKind = "home"
	PageApplications PageKind = "applications"
	PageBookmarks    PageKind = "bookmarks"
)

// PageKinds lists the pages in the order their templates are looked at.
var PageKinds = []PageKind{PageHome, PageApplications, PageBookmarks}

// File is the name of the template of a page in the templates of a theme.
func (kind PageKind) File() string { return string(kind) + ".html" }

// PageData is the key under which a page hands the compiled template of its
// theme to the renderer, as a *Fragment. A page without one is rendered with
// the markup of the application.
const PageData = "PageTemplate"

// MaximumPageBytes is what a page template can write.
const MaximumPageBytes = 256 << 10

// The parts of the application that a page template places.
const (
	SlotSearch        = "flare/search"
	SlotClock         = "flare/clock"
	SlotGreeting      = "flare/greeting"
	SlotWeather       = "flare/weather"
	SlotSubpageHeader = "flare/subpage-header"
	SlotApps          = "flare/apps"
	SlotBookmarks     = "flare/bookmarks"
	SlotToolbar       = "flare/toolbar"
	SlotFooter        = "flare/footer"
)

// PageSlotNames lists the parts in the order the application's own pages
// have them.
var PageSlotNames = []string{
	SlotSearch, SlotClock, SlotGreeting, SlotWeather, SlotSubpageHeader, SlotApps, SlotBookmarks, SlotToolbar, SlotFooter,
}

// PageSlots are the parts a template of a page can place, and which of them
// it has to. What a page shows is for the administrator to decide: a part
// renders nothing while its module is turned off, and a template cannot
// leave out a part that the page has.
//
// The template of the home page stands in for the two other pages where a
// theme brings none for them, so it places every part. A template for the
// application page or the bookmark page has to place what that page shows:
// its header, its list, the toolbar and the footer.
func PageSlots(kind PageKind) []Slot {
	required := map[string]bool{SlotSubpageHeader: true, SlotToolbar: true, SlotFooter: true}
	switch kind {
	case PageApplications:
		required[SlotApps] = true
	case PageBookmarks:
		required[SlotBookmarks] = true
	default:
		for _, name := range PageSlotNames {
			required[name] = true
		}
	}
	slots := make([]Slot, 0, len(PageSlotNames))
	for _, name := range PageSlotNames {
		slots = append(slots, Slot{Name: name, Required: required[name]})
	}
	return slots
}

// PageValues is what a page template can branch on. Nothing in it is an
// entry, a setting that is not shown on the page anyway, or a value of the
// request other than which page it is for.
type PageValues struct {
	// Lang is the language of the interface.
	Lang string
	// Kind is the page the template is rendered for. The template of the
	// home page is also rendered for the two other pages.
	Kind PageKind
	// Searching reports a page that shows the results of a search.
	Searching bool
	// Modules says for each part with a switch whether it is turned on, by
	// the name of the part without its prefix.
	Modules map[string]bool
	// Layout is the layout in effect, by the names of the options under
	// Homepage in config.yml.
	Layout map[string]string
	// Options are the values of the options of the theme, by their ids.
	Options map[string]any
}

// PageFuncs are the functions of a page template besides the built-in ones:
// T for a text of the interface and asset for the address of a file among
// the assets of the theme.
type PageFuncs struct {
	// T returns the text of a key in the language of the page.
	T func(key string) string
	// Asset returns the address of an asset by its name, or an error for a
	// name that is no asset of the theme.
	Asset func(name string) (string, error)
	// AssetPrefix begins every address Asset returns.
	AssetPrefix string
}

// CompilePage compiles the template of a page for one set of values. The
// title of the site is the one field a page can show; everything else it is
// given decides which way the template goes and is not part of the page.
func CompilePage(source string, kind PageKind, values PageValues, funcs PageFuncs) (*Fragment, error) {
	spec := Spec{
		Texts:       []string{"Title"},
		Slots:       PageSlots(kind),
		Funcs:       template.FuncMap{"T": funcs.T, "asset": funcs.Asset},
		AssetPrefix: funcs.AssetPrefix,
		Limit:       MaximumPageBytes,
	}
	return Compile(source, spec, func(fields Fields) any {
		return map[string]any{
			"Site":    map[string]any{"Title": fields.Text("Title"), "Lang": values.Lang},
			"Page":    map[string]any{"Kind": string(values.Kind), "Searching": values.Searching},
			"Modules": values.Modules,
			"Layout":  values.Layout,
			"Theme":   map[string]any{"Options": values.Options},
		}
	})
}
