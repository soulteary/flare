package templates

import (
	"bytes"
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/internal/themetemplate"
)

func homeRenderer(t *testing.T) *Renderer {
	t.Helper()
	views, err := NewRenderer()
	require.NoError(t, err)
	return views.(*Renderer)
}

// The page that is put together from its parts and the skeleton is, byte for
// byte, the page that home.html renders as a whole.
func TestHomePageFromTheSkeletonIsThePageOfTheTemplate(t *testing.T) {
	renderer := homeRenderer(t)
	require.NotNil(t, renderer.home)
	sets := map[string]func() map[string]any{
		"every module on": homeData,
		"every module off": func() map[string]any {
			return map[string]any{"Locale": "en", "ClockMode": "server", "OptionHideSettingsButton": true, "OptionHideHelpButton": true}
		},
		"errors and notices": func() map[string]any {
			return map[string]any{
				"Locale": "zh", "ClockMode": "server", "OptionShowApps": true, "ApplicationsError": "apps <failed>",
				"OptionShowBookmarks": true, "BookmarksError": "bookmarks & more", "ShowSearchComponent": true, "SearchExternalFirst": true,
				"ThemeSafeMode": map[string]string{"Text": "safe mode", "Action": "/settings/theme", "Back": "back"}, "GuideAssetsURI": "/guide",
				"PageThemeStylesheet": "/theme/zelda/abc/theme.css", "BodyClassName": "a b",
			}
		},
	}
	for name, data := range sets {
		whole := execute(t, renderer.templates, homeTemplate, data())
		values := data()
		var out bytes.Buffer
		require.NoError(t, renderer.Render(&out, homeTemplate, values), name)
		require.Equal(t, whole, out.String(), name)
		require.NotContains(t, values, homeBody, "what was rendered for the skeleton does not stay in the data of the page")
		require.Greater(t, len(whole), 500, name)
	}
}

// The skeleton is the text between the parts, with one hole for each part in
// the order of the page, and it holds nothing a request decides.
func TestHomeSkeletonHasOneHoleForEachComponent(t *testing.T) {
	renderer := homeRenderer(t)
	holes := renderer.home.skeleton.Holes()
	require.Len(t, holes, len(HomeComponents))
	for i, name := range HomeComponents {
		require.Equal(t, themetemplate.Hole{Kind: themetemplate.HoleSlot, Name: name}, holes[i])
	}
	var empty bytes.Buffer
	renderer.home.skeleton.Render(&empty, noParts{})
	require.Contains(t, empty.String(), "id=hero-container")
	require.Contains(t, empty.String(), "id=plugin-datetime")
	require.Equal(t, renderer.home.skeleton.Size(), empty.Len())
	// The parts of a page are written where they stand and are nothing else.
	filled := &parts{}
	require.Empty(t, filled.Text("Name")+filled.Address()+string(filled.LinkAttributes())+string(filled.Slot("flare/search")))
}

type noParts struct{}

func (noParts) Text(string) string                { return "" }
func (noParts) Address() string                   { return "" }
func (noParts) LinkAttributes() template.HTMLAttr { return "" }
func (noParts) Slot(string) template.HTML         { return "" }

// A part that fails leaves nothing written, and data that is no map is
// rendered by the template as a whole.
func TestHomePageFailsAsAWholeAndFallsBack(t *testing.T) {
	renderer := homeRenderer(t)
	var out bytes.Buffer
	out.WriteString("already-written")
	data := homeData()
	data["ClockMode"] = 3
	require.Error(t, renderer.Render(&out, homeTemplate, data))
	require.Equal(t, "already-written", out.String())
	require.NotContains(t, data, homeBody)

	// So does the page around the skeleton, with its own failure.
	data = homeData()
	data["Locale"] = 3
	out.Reset()
	err := renderer.Render(&out, homeTemplate, data)
	require.Error(t, err)
	require.NotErrorIs(t, err, errHomeMark)
	require.Zero(t, out.Len())
	require.NotContains(t, data, homeBody)

	// Another page of the set is rendered by its own template.
	var direct bytes.Buffer
	require.NoError(t, renderer.templates.ExecuteTemplate(&direct, "settings-sidebar.html", map[string]any{"Locale": "en"}))
	require.NoError(t, renderer.Render(&out, "settings-sidebar.html", map[string]any{"Locale": "en"}))
	require.Equal(t, direct.String(), out.String())

	out.Reset()
	require.NoError(t, renderer.Render(&out, homeTemplate, map[string]string{"Locale": "en", "ClockMode": "server"}))
	require.Contains(t, out.String(), "class=footer-container")

	// A page without the place of its skeleton is not rendered in part.
	broken := *renderer.home
	broken.page = renderer.templates
	out.Reset()
	require.ErrorIs(t, (&Renderer{templates: renderer.templates, home: &broken}).Render(&out, homeTemplate, homeData()), errHomeMark)
	require.Zero(t, out.Len())

	// A set of templates without the page has no skeleton.
	_, err = prepareHome(template.Must(template.New("other").Parse("x")))
	require.Error(t, err)
}

// BenchmarkHomePage compares the page rendered as a whole with the page put
// together from its parts, for a small page and for one with many bookmarks.
func BenchmarkHomePage(b *testing.B) {
	views, err := NewRenderer()
	if err != nil {
		b.Fatal(err)
	}
	renderer := views.(*Renderer)
	whole := &Renderer{templates: renderer.templates}
	for _, size := range []struct {
		name      string
		bookmarks int
	}{{"small", 20}, {"large", 5000}} {
		data := homeData()
		var list bytes.Buffer
		for i := 0; i < size.bookmarks; i++ {
			list.WriteString(`<li><a class="bookmark" href="https://example.test/` + "page" + `" rel="noopener"><span>A bookmark with a name</span></a></li>`)
		}
		data["Bookmarks"] = template.HTML(list.String())
		var out bytes.Buffer
		b.Run(size.name+"/whole", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out.Reset()
				if err := whole.Render(&out, homeTemplate, data); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(size.name+"/parts", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				out.Reset()
				if err := renderer.Render(&out, homeTemplate, data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
