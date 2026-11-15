package themetemplate

import "html/template"

// ItemKind is what a theme can bring an item template for: an application,
// a bookmark, and a group of bookmarks.
type ItemKind string

const (
	ItemApp      ItemKind = "app"
	ItemBookmark ItemKind = "bookmark"
	ItemGroup    ItemKind = "group"
)

// ItemKinds lists the kinds in the order their templates are looked at.
var ItemKinds = []ItemKind{ItemApp, ItemBookmark, ItemGroup}

// File is the name of the template of a kind in the templates of a theme.
func (kind ItemKind) File() string { return string(kind) + ".html" }

// MaximumItemBytes is what an item template can write.
const MaximumItemBytes = 64 << 10

// The parts of the application that an item template places.
const (
	// SlotIcon is the icon of an application or a bookmark.
	SlotIcon = "flare/icon"
	// SlotItems are the bookmarks of a group.
	SlotItems = "flare/items"
)

// The fields an item template can show.
const (
	FieldName        = "Name"
	FieldDescription = "Description"
	FieldHost        = "Host"
	FieldInitial     = "Initial"
)

// ItemValues is what an item template can branch on. None of it is a text
// of an entry: an entry has an icon or has none, and so on.
type ItemValues struct {
	// Lang is the language of the interface.
	Lang string
	// Icon reports an entry that has an icon.
	Icon bool
	// Description reports an application whose description is shown.
	Description bool
	// NewTab reports entries that open in a new window.
	NewTab bool
	// Named reports a group that has a name. The bookmarks of a site
	// without categories are one group without a name.
	Named bool
	// Collapsible reports a group that the reader opens and closes.
	Collapsible bool
	// Options are the values of the options of the theme, by their ids.
	Options map[string]any
}

// CompileItem compiles the template of an item for one set of values.
//
// An application and a bookmark are about one address: the template has
// exactly one link to it, and shows the name as text, so that an entry can
// be opened and is read out under every theme. A bookmark is an item of a
// list, so its template writes one li, and the template of a group places
// the bookmarks directly in a ul or an ol. A group that is opened and closed
// is written with details and summary, which do that without script.
func CompileItem(source string, kind ItemKind, values ItemValues, funcs PageFuncs) (*Fragment, error) {
	spec := Spec{
		Funcs:       template.FuncMap{"T": funcs.T, "asset": funcs.Asset},
		AssetPrefix: funcs.AssetPrefix,
		Limit:       MaximumItemBytes,
	}
	site := map[string]any{"Lang": values.Lang}
	theme := map[string]any{"Options": values.Options}
	switch kind {
	case ItemGroup:
		spec.Texts = []string{FieldName}
		spec.Slots = []Slot{{Name: SlotItems, Required: true, Within: []string{"ul", "ol"}}}
		if values.Named {
			spec.RequiredTexts = []string{FieldName}
		}
		if values.Collapsible {
			spec.Elements = []string{"details", "summary"}
		}
		return Compile(source, spec, func(fields Fields) any {
			return map[string]any{
				"Name": fields.Text(FieldName), "Has": map[string]any{"Name": values.Named},
				"Collapsible": values.Collapsible, "Site": site, "Theme": theme,
			}
		})
	case ItemBookmark:
		spec.Texts = []string{FieldName, FieldHost, FieldInitial}
		spec.Root = "li"
	default:
		spec.Texts = []string{FieldName, FieldDescription, FieldHost, FieldInitial}
	}
	spec.RequiredTexts = []string{FieldName}
	spec.Link = true
	spec.Slots = []Slot{{Name: SlotIcon, Inline: true}}
	return Compile(source, spec, func(fields Fields) any {
		model := map[string]any{
			"Name": fields.Text(FieldName), "Host": fields.Text(FieldHost), "Initial": fields.Text(FieldInitial),
			"Href": fields.Address(), "Has": map[string]any{"Icon": values.Icon}, "NewTab": values.NewTab,
			"Site": site, "Theme": theme,
		}
		if kind == ItemApp {
			model["Description"] = fields.Text(FieldDescription)
			model["Has"] = map[string]any{"Icon": values.Icon, "Description": values.Description}
		}
		return model
	})
}

// ItemValueSets lists the sets of values an item of a kind can have, apart
// from the language and the options of the theme: every combination of
// what an entry has or has not. A template is checked for each of them.
func ItemValueSets(kind ItemKind) []ItemValues {
	if kind == ItemGroup {
		return []ItemValues{{Named: true}, {Named: true, Collapsible: true}, {}}
	}
	var sets []ItemValues
	for _, icon := range []bool{true, false} {
		for _, newTab := range []bool{false, true} {
			if kind == ItemBookmark {
				sets = append(sets, ItemValues{Icon: icon, NewTab: newTab})
				continue
			}
			for _, description := range []bool{true, false} {
				sets = append(sets, ItemValues{Icon: icon, Description: description, NewTab: newTab})
			}
		}
	}
	return sets
}
