package define

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

const (
	everyPagePart = `{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
		`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}`
	homeTemplateSource = `<div class="shell"><h1>{{.Site.Title}}</h1><img src="{{asset "wallpaper.jpg"}}" alt="{{T "apps"}}">` +
		`{{if eq .Theme.Options.frames "true"}}<p>frames</p>{{end}}` + everyPagePart + `</div>`
	// The options of the theme have their defaults when a template is
	// checked: this one places its parts only where the option is off.
	applicationsTemplateSource = `<div>{{if eq .Theme.Options.frames "false"}}{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/toolbar"}}{{template "flare/footer"}}{{end}}</div>`
)

// writeThemeTemplate writes a file into the templates of a theme that is a
// directory, last modified a minute ago.
func writeThemeTemplate(t *testing.T, theme, file, content string) {
	t.Helper()
	directory := filepath.Join(theme, model.ThemeTemplateDirectory)
	require.NoError(t, os.MkdirAll(filepath.Join(UserThemeDirectory, directory), 0700))
	writeUserTheme(t, filepath.Join(directory, file), content)
}

func templateNames(theme model.Theme) []string {
	names := make([]string, 0, len(theme.Templates))
	for _, template := range theme.Templates {
		names = append(names, template.Name)
	}
	return names
}

func templateRemarks(t *testing.T) []ThemeProblem {
	t.Helper()
	_, problems, _ := UserThemes()
	var remarks []ThemeProblem
	for _, problem := range problems {
		if strings.Contains(problem.File, "/"+model.ThemeTemplateDirectory) {
			remarks = append(remarks, problem)
		}
	}
	return remarks
}

// A theme that is a directory can bring a template for each navigation page.
// A template that compiles is kept with the theme; one that does not is
// remarked on, and the theme is in use without it.
func TestThemeBringsPageTemplates(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", homeTemplateSource)
	writeThemeTemplate(t, "zelda", "applications.html", applicationsTemplateSource)
	writeThemeTemplate(t, "zelda", "bookmarks.html", `<div><script>x()</script></div>`)
	// Other files of the directory are left alone.
	writeThemeTemplate(t, "zelda", "app.html", `<script>x()</script>`)
	writeThemeTemplate(t, "zelda", "notes.txt", "notes")
	ReloadUserThemes()

	zelda := userTheme(t, "zelda")
	require.Equal(t, []string{"home.html", "applications.html"}, templateNames(zelda))
	require.Equal(t, homeTemplateSource, zelda.Templates[0].Source)
	require.NotEmpty(t, zelda.Stylesheet)
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/bookmarks.html", Reason: "template_element", Subject: "script"}}, templateRemarks(t))

	// A theme of one file has no templates, and neither has a directory
	// without that directory.
	writeUserTheme(t, "plain.yml", userThemeColors)
	writeThemeDirectory(t, "bare", userThemeColors)
	ReloadUserThemes()
	require.Empty(t, userTheme(t, "plain").Templates)
	require.Empty(t, userTheme(t, "bare").Templates)

	// The functions of a template are those of its theme: its own assets
	// under the address they are served at, and no others.
	funcs := ThemeTemplateFuncs(zelda, func(key string) string { return "<" + key + ">" })
	address, err := funcs.Asset("wallpaper.jpg")
	require.NoError(t, err)
	require.Equal(t, "/theme/zelda/"+zelda.Hash+"/assets/wallpaper.jpg", address)
	require.Equal(t, "/theme/zelda/"+zelda.Hash+"/assets/", funcs.AssetPrefix)
	_, err = funcs.Asset("other.jpg")
	require.Error(t, err)
	require.Equal(t, "<apps>", funcs.T("apps"))
	bare := ThemeTemplateFuncs(userTheme(t, "bare"), nil)
	require.Empty(t, bare.AssetPrefix)
	_, err = bare.Asset("wallpaper.jpg")
	require.Error(t, err)
}

func TestThemeTemplatesThatAreNotAccepted(t *testing.T) {
	cases := []struct {
		name    string
		write   func(t *testing.T)
		file    string
		reason  string
		subject string
	}{
		{"larger than a template can be", func(t *testing.T) {
			writeThemeTemplate(t, "zelda", "home.html", "<div>"+everyPagePart+strings.Repeat(" ", MaximumThemeTemplateBytes)+"</div>")
		}, "zelda/templates/home.html", ThemeProblemSize, ""},
		{"not text", func(t *testing.T) { writeThemeTemplate(t, "zelda", "home.html", "<div>\xff"+everyPagePart+"</div>") },
			"zelda/templates/home.html", ThemeProblemTemplateEncoding, ""},
		{"a directory of the name", func(t *testing.T) {
			require.NoError(t, os.MkdirAll(filepath.Join(UserThemeDirectory, "zelda", "templates", "home.html"), 0700))
		}, "zelda/templates/home.html", ThemeProblemType, ""},
		{"a link", func(t *testing.T) {
			writeThemeTemplate(t, "zelda", "other.html", homeTemplateSource)
			require.NoError(t, os.Symlink("other.html", filepath.Join(UserThemeDirectory, "zelda", "templates", "home.html")))
		}, "zelda/templates/home.html", ThemeProblemType, ""},
		{"the directory is a link", func(t *testing.T) {
			require.NoError(t, os.MkdirAll(filepath.Join(UserThemeDirectory, "elsewhere"), 0700))
			require.NoError(t, os.Symlink(filepath.Join("..", "elsewhere"), filepath.Join(UserThemeDirectory, "zelda", "templates")))
		}, "zelda/templates", ThemeProblemLink, ""},
		{"an asset of another theme", func(t *testing.T) {
			writeThemeTemplate(t, "zelda", "home.html", `<div><img src="{{asset "other.png"}}">`+everyPagePart+`</div>`)
		}, "zelda/templates/home.html", "template_execute", ""},
		{"a part the page shows is left out", func(t *testing.T) {
			writeThemeTemplate(t, "zelda", "bookmarks.html", applicationsTemplateSource)
		}, "zelda/templates/bookmarks.html", "template_slot_missing", "flare/bookmarks"},
		{"a name of the application", func(t *testing.T) {
			writeThemeTemplate(t, "zelda", "applications.html", applicationsTemplateSource+`{{define "flare/apps"}}x{{end}}`)
		}, "zelda/templates/applications.html", "template_reserved_name", "flare/apps"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userThemeDirectory(t, "blackboard")
			writeZeldaTheme(t)
			tc.write(t)
			ReloadUserThemes()
			require.Empty(t, userTheme(t, "zelda").Templates, "the theme is in use without the template")
			remarks := templateRemarks(t)
			require.Len(t, remarks, 1)
			require.Equal(t, tc.file, remarks[0].File)
			require.Equal(t, tc.reason, remarks[0].Reason)
			if tc.subject != "" {
				require.Equal(t, tc.subject, remarks[0].Subject)
			}
		})
	}
	// A file named templates is no directory of templates.
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeUserTheme(t, filepath.Join("zelda", "templates"), "x")
	ReloadUserThemes()
	require.Empty(t, userTheme(t, "zelda").Templates)
	require.Empty(t, templateRemarks(t))
}

// The selected theme follows its templates as it follows its other files:
// one that is added, edited or removed is read by the next page.
func TestSelectedThemeFollowsItsTemplates(t *testing.T) {
	userThemeDirectory(t, "zelda")
	writeZeldaTheme(t)
	UpdatePagePalettes()
	current := func() []string {
		GetAppBodyStyle()
		return templateNames(userTheme(t, "zelda"))
	}
	require.Empty(t, current())

	writeThemeTemplate(t, "zelda", "home.html", homeTemplateSource)
	require.Equal(t, []string{"home.html"}, current())

	// Another template beside it is seen as well.
	writeThemeTemplate(t, "zelda", "applications.html", applicationsTemplateSource)
	require.Equal(t, []string{"home.html", "applications.html"}, current())

	// An edit in place keeps the state of the directory and changes that
	// of the file.
	edited := strings.Replace(homeTemplateSource, "shell", "frame", 1)
	writeThemeTemplate(t, "zelda", "home.html", edited)
	setUserThemeAge(t, filepath.Join("zelda", "templates", "home.html"), 2*time.Minute)
	require.Equal(t, []string{"home.html", "applications.html"}, current())
	require.Equal(t, edited, userTheme(t, "zelda").Templates[0].Source)

	// A template that stops compiling is dropped and remarked on.
	writeThemeTemplate(t, "zelda", "applications.html", `<div>`)
	setUserThemeAge(t, filepath.Join("zelda", "templates", "applications.html"), 3*time.Minute)
	require.Equal(t, []string{"home.html"}, current())
	require.Len(t, templateRemarks(t), 1)

	require.NoError(t, os.RemoveAll(filepath.Join(UserThemeDirectory, "zelda", "templates")))
	require.Empty(t, current())
	require.Empty(t, templateRemarks(t))
}
