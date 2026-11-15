package define

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/themetemplate"
)

// maximumCompiledThemePages is how many compiled page templates are kept.
// A template is compiled once for each set of values it can branch on, and
// those values are settings, so an instance has few sets. Everything kept is
// dropped when the number is reached.
const maximumCompiledThemePages = 128

// compiledThemePage is a page template compiled for one set of values, or
// the reason it does not compile for them.
type compiledThemePage struct {
	fragment *themetemplate.Fragment
	problem  ThemeProblem
	// theme and source name the template that was compiled, in the state
	// it had.
	theme  string
	source string
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
	if !ThemeTemplatesAvailable() || !options.ThemeTemplates || page.Theme == "" {
		return nil
	}
	chosen := options.ThemeSettings[page.Theme]
	if !page.Preview && !chosen.Templates {
		return nil
	}
	themeMu.RLock()
	definition, known := themeDefinition(page.Theme)
	themeMu.RUnlock()
	if !known {
		return nil
	}
	source, found := themePageSource(definition, page.Kind)
	if !found {
		return nil
	}
	values := themetemplate.PageValues{
		Lang: page.Lang, Kind: page.Kind, Searching: page.Searching, Modules: page.Modules,
		Layout: options.Homepage.LayoutValues(), Options: make(map[string]any, len(definition.Options)),
	}
	var key strings.Builder
	key.WriteString(definition.Name + "\x00" + definition.Hash + "\x00" + source.Name + "\x00" + source.Hash + "\x00" + string(page.Kind) + "\x00" + page.Lang + "\x00" + strconv.FormatBool(page.Searching))
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
	fragment, err := themetemplate.CompilePage(source.Source, page.Kind, values, ThemeTemplateFuncs(definition, page.T))
	compiled := compiledThemePage{fragment: fragment, theme: definition.Name, source: source.Name + "\x00" + source.Hash}
	if err != nil {
		compiled.problem = themeTemplateProblem(definition.Name, source.Name, err)
		logger.GetLogger().Warn("A template of the theme does not compile for the settings of this page; the page keeps the markup of the application", "file", compiled.problem.File, "reason", compiled.problem.Reason, "subject", compiled.problem.Subject)
	}
	compiledThemePages.entries[key.String()] = compiled
	return fragment
}

// ThemeTemplateFiles names the templates a theme brings that are accepted,
// in the order they are looked at.
func ThemeTemplateFiles(theme string) []string {
	themeMu.RLock()
	definition, _ := themeDefinition(theme)
	themeMu.RUnlock()
	files := make([]string, 0, len(definition.Templates))
	for _, template := range definition.Templates {
		files = append(files, template.Name)
	}
	return files
}

// themePageSource returns the template a theme renders a page with: its
// template for that page, and its template of the home page otherwise.
func themePageSource(theme model.Theme, kind themetemplate.PageKind) (model.ThemeTemplate, bool) {
	for _, file := range []string{kind.File(), themetemplate.PageHome.File()} {
		for _, template := range theme.Templates {
			if template.Name == file {
				return template, true
			}
		}
	}
	return model.ThemeTemplate{}, false
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
		if compiled.fragment != nil || compiled.theme != definition.Name || !slices.ContainsFunc(definition.Templates, func(template model.ThemeTemplate) bool {
			return template.Name+"\x00"+template.Hash == compiled.source
		}) {
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
