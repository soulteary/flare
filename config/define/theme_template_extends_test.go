package define

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/themetemplate"
)

const (
	// Templates that say which theme they are of.
	nightApplicationsSource = `<div class="night-applications">{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`
	nightAppSource          = `<div class="night-tile"><a href="{{.Href}}">{{.Name}}</a></div>`
)

// writeNightTheme writes a theme of one file that is based on zelda, and a
// directory that is based on that.
func writeNightTheme(t *testing.T) {
	t.Helper()
	writeUserTheme(t, "compact.yml", basedOnZelda+"name: Zelda Compact\ntokens:\n  radius: 2px\n")
	writeThemeDirectory(t, "night", "contract: 1\nextends: compact\ntokens:\n  accent: \"#00ff88\"\n")
}

func confirmed(themes ...string) model.Application {
	settings := model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{}}
	for _, theme := range themes {
		settings.ThemeSettings[theme] = model.ThemeSettings{Templates: true}
	}
	return settings
}

func pageOf(theme string, kind themetemplate.PageKind) ThemePage {
	page := homePage()
	page.Theme, page.Kind = theme, kind
	return page
}

// A theme that is based on another is shown with the templates of the
// themes below it for what it brings no template for. The nearest theme
// that has a template for a page or for the home page gives the template
// of the page, and an item has the nearest template of its kind.
func TestThemeTakesTheTemplatesOfTheThemesBelow(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeThemeTemplate(t, "zelda", "bookmark.html", bookmarkItemSource)
	writeNightTheme(t)
	writeThemeTemplate(t, "night", "applications.html", nightApplicationsSource)
	writeThemeTemplate(t, "night", "app.html", nightAppSource)
	ReloadUserThemes()
	require.Empty(t, templateRemarks(t))

	// A theme of one file has the templates of the directory below it.
	require.Equal(t, []string{"zelda/home.html", "zelda/app.html", "zelda/bookmark.html"}, ThemeTemplateFiles("compact"))
	// The template of applications of zelda is replaced, and is not listed.
	require.Equal(t, []string{"applications.html", "app.html", "zelda/home.html", "zelda/bookmark.html"}, ThemeTemplateFiles("night"))
	require.Equal(t, []string{"home.html", "app.html", "bookmark.html"}, ThemeTemplateFiles("zelda"))

	settings := confirmed("night", "compact")
	zelda := userTheme(t, "zelda").Hash
	// The assets a template names are those of the theme that brings it.
	home := renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome)))
	require.Contains(t, home, `class="shell" data-kind="home"`)
	require.Contains(t, home, `src="/theme/zelda/`+zelda+`/assets/wallpaper.jpg"`)
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageApplications))), `class="night-applications"`)
	// Neither night nor compact has a template for the bookmarks or for
	// the home page, so the one of the home page of zelda stands in.
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageBookmarks))), `class="shell" data-kind="bookmarks"`)
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("compact", themetemplate.PageApplications))), `class="shell" data-kind="applications"`)

	items := ThemeItemTemplates(settings, "night", false, "en", func(key string) string { return "<" + key + ">" })
	require.NotNil(t, items)
	require.Contains(t, renderThemeItem(t, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `class="night-tile"`)
	require.Contains(t, renderThemeItem(t, items.Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{})), `class="mark"`)
	require.Nil(t, items.Fragment(themetemplate.ItemGroup, themetemplate.ItemValues{Named: true}))
	below := ThemeItemTemplates(settings, "compact", false, "en", func(key string) string { return "<" + key + ">" })
	tile := renderThemeItem(t, below.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{}))
	require.Contains(t, tile, `class="tile"`)
	require.Contains(t, tile, `src="/theme/zelda/`+zelda+`/assets/wallpaper.jpg"`)
	require.NotEqual(t, items.State(), below.State())

	// The confirmation is that of the theme a page is shown with: the
	// templates of zelda are used for night without zelda being confirmed,
	// and not for a theme above that is not confirmed.
	require.Nil(t, ThemePageTemplate(confirmed("zelda"), pageOf("night", themetemplate.PageHome)))
	require.Nil(t, ThemeItemTemplates(confirmed("zelda"), "night", false, "en", nil))
	require.NotNil(t, ThemePageTemplate(confirmed("zelda"), pageOf("zelda", themetemplate.PageHome)))

	// A theme that brings a template of the home page has it stand in for
	// its other pages, before the templates of the themes below.
	writeThemeDirectory(t, "deep", "contract: 1\nextends: night\n")
	writeThemeTemplate(t, "deep", "home.html", `<div class="deep-home">`+everyPagePart+`</div>`)
	ReloadUserThemes()
	require.Equal(t, []string{"home.html", "night/app.html", "zelda/bookmark.html"}, ThemeTemplateFiles("deep"))
	require.Contains(t, renderThemePage(t, ThemePageTemplate(confirmed("deep"), pageOf("deep", themetemplate.PageApplications))), `class="deep-home"`)

	// A theme that is based on a bundled theme has its own templates.
	writeThemeDirectory(t, "soft", "contract: 1\nextends: tron\n")
	writeThemeTemplate(t, "soft", "bookmark.html", bookmarkItemSource)
	ReloadUserThemes()
	require.Equal(t, []string{"bookmark.html"}, ThemeTemplateFiles("soft"))
}

// A template that is not accepted is left out, and what it was for is
// looked for further below. A theme says what is to say about its own
// templates, so a template of a theme below is remarked on once; one that
// is never come to is not looked at.
func TestTemplatesOfThemesBelowThatAreNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", homeTemplateSource)
	writeThemeTemplate(t, "zelda", "bookmarks.html", `<div><script>x()</script></div>`)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeNightTheme(t)
	// The assets of zelda are not those of night.
	writeThemeTemplate(t, "night", "app.html", nightAppSource+`<img src="{{asset "wallpaper.jpg"}}" alt="">`)
	writeThemeTemplate(t, "night", "home.html", `<div>`)
	ReloadUserThemes()

	require.Equal(t, []string{"zelda/home.html", "zelda/app.html"}, ThemeTemplateFiles("night"))
	remarks := templateRemarks(t)
	require.Len(t, remarks, 3)
	require.Contains(t, remarks, ThemeProblem{File: "zelda/templates/bookmarks.html", Reason: "template_element", Subject: "script"})
	files := []string{remarks[0].File, remarks[1].File, remarks[2].File}
	require.ElementsMatch(t, []string{"zelda/templates/bookmarks.html", "night/templates/home.html", "night/templates/app.html"}, files)

	// A template of the theme below that the theme replaces is not
	// compiled for it: this one compiles only with the default of zelda.
	writeThemeDirectory(t, "framed", basedOnZelda+"options:\n  - id: frames\n    type: boolean\n    default: true\n")
	writeThemeTemplate(t, "zelda", "applications.html", applicationsTemplateSource)
	writeThemeTemplate(t, "zelda", "bookmarks.html", applicationsTemplateSourceFor("bookmarks"))
	writeThemeTemplate(t, "framed", "applications.html", nightApplicationsSource)
	writeThemeTemplate(t, "night", "app.html", nightAppSource)
	writeThemeTemplate(t, "night", "home.html", `<div class="night-home">`+everyPagePart+`</div>`)
	ReloadUserThemes()
	require.Equal(t, "true", userTheme(t, "framed").Options[0].Default)
	// With the default of framed, the template of the bookmarks of zelda
	// places no part: it is remarked on, and the home page stands in.
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/bookmarks.html", Reason: "template_slot_missing", Subject: "flare/subpage-header"}}, templateRemarks(t))
	require.Equal(t, []string{"applications.html", "zelda/home.html", "zelda/app.html"}, ThemeTemplateFiles("framed"))
	require.Equal(t, []string{"home.html", "applications.html", "bookmarks.html", "app.html"}, ThemeTemplateFiles("zelda"))
}

// A template that does not compile for the values of a page is listed for
// the theme the page is shown with, by the file of the theme that brings
// it.
func TestTemplateOfThemeBelowThatDoesNotCompileForSomeValues(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", `<div>{{if eq .Theme.Options.frames "true"}}<script>x()</script>{{end}}`+everyPagePart+`</div>`)
	writeThemeTemplate(t, "zelda", "app.html", `<div><a href="{{.Href}}">{{.Name}}</a>{{if eq .Theme.Options.frames "true"}}<iframe></iframe>{{end}}</div>`)
	writeNightTheme(t)
	ReloadUserThemes()

	settings := confirmed("night")
	require.NotNil(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome)))
	settings.ThemeSettings["night"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	require.Nil(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome)))
	require.Nil(t, ThemeItemTemplates(settings, "night", false, "en", nil).Fragment(themetemplate.ItemApp, themetemplate.ItemValues{}))
	require.Equal(t, []ThemeProblem{
		{File: "zelda/templates/app.html", Reason: "template_element", Subject: "iframe"},
		{File: "zelda/templates/home.html", Reason: "template_element", Subject: "script"},
	}, ThemePageTemplateProblems("night"))
	require.Empty(t, ThemePageTemplateProblems("zelda"))
	require.Empty(t, ThemePageTemplateProblems("compact"))
}

// The selected theme follows the templates of the themes below it, and
// what is compiled from them follows the assets of the theme that brings
// them.
func TestSelectedThemeFollowsTheTemplatesBelow(t *testing.T) {
	userThemeDirectory(t, "night")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeNightTheme(t)
	UpdatePagePalettes()
	current := func() []string {
		GetAppBodyStyle()
		return ThemeTemplateFiles("night")
	}
	require.Empty(t, current())

	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	require.Equal(t, []string{"zelda/home.html"}, current())
	settings := confirmed("night")
	before := userTheme(t, "zelda").Hash
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome))), `src="/theme/zelda/`+before+`/assets/wallpaper.jpg"`)

	edited := strings.Replace(valuesTemplateSource, "shell", "frame", 1)
	writeThemeTemplate(t, "zelda", "home.html", edited)
	setUserThemeAge(t, filepath.Join("zelda", "templates", "home.html"), 2*time.Minute)
	require.Equal(t, []string{"zelda/home.html"}, current())
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome))), `class="frame"`)

	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG+"more")
	require.Equal(t, []string{"zelda/home.html"}, current())
	after := userTheme(t, "zelda").Hash
	require.NotEqual(t, before, after)
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome))), `src="/theme/zelda/`+after+`/assets/wallpaper.jpg"`)

	// A template of its own takes the place of the one below.
	writeThemeTemplate(t, "night", "home.html", `<div class="night-home">`+everyPagePart+`</div>`)
	require.Equal(t, []string{"home.html"}, current())
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, pageOf("night", themetemplate.PageHome))), `class="night-home"`)
}

// What the entries of a theme are kept under follows the assets of the
// theme below that brings their template.
func TestItemTemplatesBelowFollowTheAssetsOfTheirTheme(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeNightTheme(t)
	ReloadUserThemes()
	items := ThemeItemTemplates(confirmed("night"), "night", false, "en", func(key string) string { return key })
	before, night := userTheme(t, "zelda").Hash, userTheme(t, "night").Hash
	require.Contains(t, renderThemeItem(t, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `src="/theme/zelda/`+before+`/assets/wallpaper.jpg"`)

	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG+"more")
	ReloadUserThemes()
	after := userTheme(t, "zelda").Hash
	require.NotEqual(t, before, after)
	require.Equal(t, night, userTheme(t, "night").Hash, "the theme above has no files that changed")
	changed := ThemeItemTemplates(confirmed("night"), "night", false, "en", func(key string) string { return key })
	require.NotEqual(t, items.State(), changed.State())
	require.Contains(t, renderThemeItem(t, changed.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `src="/theme/zelda/`+after+`/assets/wallpaper.jpg"`)
}
