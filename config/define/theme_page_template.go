package define

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/themetemplate"
)

// maximumCompiledThemePages is how many compiled templates are kept, those
// of pages and those of items. A template is compiled once for each set of
// values it can branch on. Those values are settings and what an entry has,
// so an instance has few sets. Everything kept is dropped when the number is
// reached.
const maximumCompiledThemePages = 256

// compiledThemePage is a page template compiled for one set of values, or
// the reason it does not compile for them.
type compiledThemePage struct {
	fragment *themetemplate.Fragment
	problem  ThemeProblem
	// theme is the theme the template was compiled for, and source the
	// template in the state it had.
	theme  string
	source model.ThemeTemplate
}

var compiledThemePages = struct {
	sync.Mutex
	entries map[string]compiledThemePage
}{entries: map[string]compiledThemePage{}}

// ThemePage is a page that asks for the template of a theme.
type ThemePage struct {
	// Theme is the theme the page is shown with.
	Theme string
	// Preview reports a page that shows a theme without selecting it. Its
	// templates are used before they are confirmed.
	Preview bool
	// Kind is the page.
	Kind themetemplate.PageKind
	// Lang is the language of the interface, and T gives its texts.
	Lang string
	T    func(key string) string
	// Searching reports a page that shows the results of a search.
	Searching bool
	// Modules says which parts the page shows, by the name of the part
	// without its prefix.
	Modules map[string]bool
}

// ThemeTemplatesAvailable reports whether this instance uses templates of
// themes at all. That a template runs no script is finally enforced by the
// Content-Security-Policy of the navigation pages, so an instance that sends
// none uses no template; and safe mode leaves the themes directory out.
func ThemeTemplatesAvailable() bool {
	return !AppFlags.DisableCSP && !AppFlags.ThemeSafeMode
}

// ThemePageTemplate returns the template of a theme for a page, compiled
// for the values of that page, or nil for a page that is rendered with the
// markup of the application: where the instance or the site does not allow
// templates, where the templates of the theme are not confirmed, where the
// theme has no template for the page, and where the template does not
// compile for these values. The application page and the bookmark page take
// the template of the home page from a theme that has none for them.
//
// The settings are those of the request, with the layout in effect.
func ThemePageTemplate(options model.Application, page ThemePage) *themetemplate.Fragment {
	definition, chosen, used := themeTemplatesInUse(options, page.Theme, page.Preview)
	if !used {
		return nil
	}
	source, found := themePageSource(definition.Templates, page.Kind)
	if !found {
		return nil
	}
	values := themetemplate.PageValues{
		Lang: page.Lang, Kind: page.Kind, Searching: page.Searching, Modules: page.Modules,
		Layout: options.Homepage.LayoutValues(), Options: make(map[string]any, len(definition.Options)),
	}
	var key strings.Builder
	key.WriteString(definition.Name + "\x00" + definition.Hash + "\x00" + source.From.Name + "\x00" + source.From.Hash + "\x00" + source.Name + "\x00" + source.Hash + "\x00" + string(page.Kind) + "\x00" + page.Lang + "\x00" + strconv.FormatBool(page.Searching))
	for _, name := range themetemplate.PageSlotNames {
		key.WriteString("\x00" + strconv.FormatBool(page.Modules[name[len(themetemplate.ReservedPrefix):]]))
	}
	for _, option := range model.ThemeLayoutOptions {
		key.WriteString("\x00" + values.Layout[option])
	}
	for _, option := range definition.Options {
		value := option.Value(chosen.Options[option.ID])
		values.Options[option.ID] = value
		key.WriteString("\x00" + value)
	}

	funcs := themeTemplateFuncs(definition, source, page.T)
	compiledThemePages.Lock()
	defer compiledThemePages.Unlock()
	if compiled, kept := compiledThemePages.entries[key.String()]; kept {
		return compiled.fragment
	}
	if len(compiledThemePages.entries) >= maximumCompiledThemePages {
		clear(compiledThemePages.entries)
	}
	// The parts a template has to place are those of the page it is
	// rendered for: the template of the home page places every part on the
	// home page, and what the other pages show where it stands in for them.
	fragment, err := themetemplate.CompilePage(source.Source, page.Kind, values, funcs)
	compiled := compiledThemePage{fragment: fragment, theme: definition.Name, source: source}
	if err != nil {
		compiled.problem = themeTemplateProblem(source.From.Name, source.Name, err)
		logger.GetLogger().Warn("A template of the theme does not compile for the settings of this page; the page keeps the markup of the application", "file", compiled.problem.File, "reason", compiled.problem.Reason, "subject", compiled.problem.Subject)
	}
	compiledThemePages.entries[key.String()] = compiled
	return fragment
}

// themeTemplatesInUse returns a theme whose templates a response uses, with
// what the administrator has set for it: the instance and the site allow
// templates, and those of the theme are confirmed or the response is a
// preview of it.
func themeTemplatesInUse(options model.Application, theme string, preview bool) (model.Theme, model.ThemeSettings, bool) {
	if !ThemeTemplatesAvailable() || !options.ThemeTemplates || theme == "" {
		return model.Theme{}, model.ThemeSettings{}, false
	}
	chosen := options.ThemeSettings[theme]
	if !preview && !chosen.Templates {
		return model.Theme{}, model.ThemeSettings{}, false
	}
	themeMu.RLock()
	definition, known := themeDefinition(theme)
	themeMu.RUnlock()
	return definition, chosen, known && len(definition.Templates) > 0
}

// ThemesShownWith names the themes in use that can be shown with templates
// of a theme: the theme itself and every theme of the themes directory that
// has it below, in the order of their names.
func ThemesShownWith(name string) []string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	var themes []string
	for _, theme := range userThemeState.themes {
		for below, steps := theme, 0; steps <= model.MaximumThemeBases; steps++ {
			if below.Name == name {
				themes = append(themes, theme.Name)
				break
			}
			if below, _ = themeDefinition(below.Extends); below.Name == "" {
				break
			}
		}
	}
	slices.Sort(themes)
	return themes
}

// ThemeLayoutUnsupported lists the layout options of the site that the
// selected theme does not support, in the order of the appearance page, or
// nothing where every option works: where no template of the theme is in
// use, as for ThemePageTemplate, so the pages have the markup of the
// application, and where the theme does not say what it supports.
func ThemeLayoutUnsupported(options model.Application) []string {
	theme, _ := ThemeSnapshot()
	definition, chosen, used := themeTemplatesInUse(options, theme, false)
	if !used {
		return nil
	}
	// An option of the theme can give a layout option its value.
	return model.ThemeLayoutUnsupported(definition.Supports, definition.WithOptions(chosen.Options).Layout)
}

// ThemeItems are the item templates of a theme for the entries one response
// shows. A list is rendered by one request, so it is not for several
// goroutines.
type ThemeItems struct {
	theme   model.Theme
	lang    string
	t       func(key string) string
	options map[string]any
	// hideDescriptions is the layout option of the site: an application is
	// then reported as one without a description.
	hideDescriptions bool
	state            string
	// compiled holds what was asked for before, by the kind and the values.
	compiled map[themeItemSet]*themetemplate.Fragment
}

// themeItemSet is a kind of item with what an entry of it has.
type themeItemSet struct {
	kind                                          themetemplate.ItemKind
	icon, description, newTab, named, collapsible bool
}

// ThemeItemTemplates returns the item templates of the theme a response is
// shown with, or nil where the entries have the markup of the application:
// where no template of the theme is in use, as for ThemePageTemplate, and
// where the theme brings no template for an item. The settings are those of
// the request, with the layout in effect.
func ThemeItemTemplates(options model.Application, theme string, preview bool, lang string, t func(key string) string) *ThemeItems {
	definition, chosen, used := themeTemplatesInUse(options, theme, preview)
	if !used {
		return nil
	}
	items := &ThemeItems{
		theme: definition, lang: lang, t: t, options: make(map[string]any, len(definition.Options)),
		hideDescriptions: options.Homepage != nil && options.Homepage.HideAppDescriptions,
		compiled:         map[themeItemSet]*themetemplate.Fragment{},
	}
	var state strings.Builder
	for _, template := range definition.Templates {
		if _, isItem := themeItemKind(template.Name); isItem {
			state.WriteString("\x00" + template.From.Name + "\x00" + template.From.Hash + "\x00" + template.Name + "\x00" + template.Hash)
		}
	}
	if state.Len() == 0 {
		return nil
	}
	for _, option := range definition.Options {
		value := option.Value(chosen.Options[option.ID])
		items.options[option.ID] = value
		state.WriteString("\x00" + value)
	}
	items.state = definition.Name + "\x00" + definition.Hash + "\x00" + lang + "\x00" + strconv.FormatBool(items.hideDescriptions) + state.String()
	return items
}

// State stands for everything the markup of an entry depends on besides the
// entry: the theme, its templates, the values of its options, the language
// and whether descriptions are hidden. It is empty for no templates.
func (items *ThemeItems) State() string {
	if items == nil {
		return ""
	}
	return items.state
}

// HideDescriptions reports whether an application is to be shown as one
// without a description.
func (items *ThemeItems) HideDescriptions() bool { return items != nil && items.hideDescriptions }

// Fragment returns the template of a kind of item, compiled for what an
// entry has, or nil for an entry that is rendered with the markup of the
// application: where the theme has no template for the kind, and where the
// template does not compile for these values.
func (items *ThemeItems) Fragment(kind themetemplate.ItemKind, values themetemplate.ItemValues) *themetemplate.Fragment {
	if items == nil {
		return nil
	}
	set := themeItemSet{kind: kind, icon: values.Icon, description: values.Description, newTab: values.NewTab, named: values.Named, collapsible: values.Collapsible}
	if fragment, asked := items.compiled[set]; asked {
		return fragment
	}
	fragment := items.compile(kind, set, values)
	items.compiled[set] = fragment
	return fragment
}

func (items *ThemeItems) compile(kind themetemplate.ItemKind, set themeItemSet, values themetemplate.ItemValues) *themetemplate.Fragment {
	source, found := themeItemSource(items.theme.Templates, kind)
	if !found {
		return nil
	}
	funcs := themeTemplateFuncs(items.theme, source, items.t)
	key := items.state + "\x00" + source.Name + fmt.Sprintf("\x00%t\x00%t\x00%t\x00%t\x00%t", set.icon, set.description, set.newTab, set.named, set.collapsible)
	compiledThemePages.Lock()
	defer compiledThemePages.Unlock()
	if compiled, kept := compiledThemePages.entries[key]; kept {
		return compiled.fragment
	}
	if len(compiledThemePages.entries) >= maximumCompiledThemePages {
		clear(compiledThemePages.entries)
	}
	values.Lang, values.Options = items.lang, items.options
	fragment, err := themetemplate.CompileItem(source.Source, kind, values, funcs)
	compiled := compiledThemePage{fragment: fragment, theme: items.theme.Name, source: source}
	if err != nil {
		compiled.problem = themeTemplateProblem(source.From.Name, source.Name, err)
		logger.GetLogger().Warn("A template of the theme does not compile for the settings of this page; the entries keep the markup of the application", "file", compiled.problem.File, "reason", compiled.problem.Reason, "subject", compiled.problem.Subject)
	}
	compiledThemePages.entries[key] = compiled
	return fragment
}

// themeTemplateFuncs are the functions a template a theme is shown with is
// compiled with: those of the theme that brings it, which is the theme
// itself or a theme below it.
func themeTemplateFuncs(theme model.Theme, template model.ThemeTemplate, t func(key string) string) themetemplate.PageFuncs {
	if template.From.Name != theme.Name {
		themeMu.RLock()
		theme, _ = themeDefinition(template.From.Name)
		themeMu.RUnlock()
	}
	return ThemeTemplateFuncs(theme, t)
}

// ThemeTemplateFiles names the templates a theme is shown with, in the
// order they are looked at: the name of the file, after the name of the
// theme that brings it where that is a theme below.
func ThemeTemplateFiles(theme string) []string {
	themeMu.RLock()
	definition, _ := themeDefinition(theme)
	themeMu.RUnlock()
	files := make([]string, 0, len(definition.Templates))
	for _, template := range definition.Templates {
		files = append(files, template.Label(definition.Name))
	}
	return files
}

// themeTemplateProblem names a template that does not compile, with the
// reason of the compiler and what it is about.
func themeTemplateProblem(theme, file string, err error) ThemeProblem {
	problem := ThemeProblem{File: theme + "/" + model.ThemeTemplateDirectory + "/" + file, Reason: ThemeProblemTemplate + string(themetemplate.ReasonExecute)}
	var refused *themetemplate.Error
	if errors.As(err, &refused) {
		problem.Reason, problem.Subject = ThemeProblemTemplate+string(refused.Reason), refused.Subject
	}
	return problem
}

// ThemePageTemplateProblems lists the templates of a theme that were
// compiled for a page since the start and did not compile for the values of
// that page, although they compile for the values they are checked with. A
// template is listed once for each reason, and no longer once it is edited.
func ThemePageTemplateProblems(theme string) []ThemeProblem {
	themeMu.RLock()
	definition, known := themeDefinition(theme)
	themeMu.RUnlock()
	if !known {
		return nil
	}
	compiledThemePages.Lock()
	defer compiledThemePages.Unlock()
	var problems []ThemeProblem
	for _, compiled := range compiledThemePages.entries {
		if compiled.fragment != nil || compiled.theme != definition.Name || !slices.Contains(definition.Templates, compiled.source) {
			continue
		}
		known := false
		for _, problem := range problems {
			known = known || problem == compiled.problem
		}
		if !known {
			problems = append(problems, compiled.problem)
		}
	}
	slices.SortFunc(problems, func(a, b ThemeProblem) int {
		return strings.Compare(a.File+"\x00"+a.Reason+"\x00"+a.Subject, b.File+"\x00"+b.Reason+"\x00"+b.Subject)
	})
	return problems
}
