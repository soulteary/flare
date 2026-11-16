package define

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/themetemplate"
)

// guideExample reads the files of the example theme from the guide for
// authors of templates: each is a block of code under a line that names the
// file in the themes directory.
func guideExample(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "theme-template-guide.md"))
	require.NoError(t, err)
	files := map[string]string{}
	block := regexp.MustCompile("(?s)\\*\\*`themes/harbor/([a-z./]+)`\\*\\*\n\n```[a-z]+\n(.*?)\n```\n")
	for _, found := range block.FindAllStringSubmatch(string(raw), -1) {
		require.NotContains(t, files, found[1])
		files[found[1]] = found[2] + "\n"
	}
	return files
}

// The example of the guide is a theme the application reads: every one of
// its templates is accepted, and compiles for each value of its option, on
// each page and for everything an entry can have.
func TestExampleOfTheTemplateGuideIsAccepted(t *testing.T) {
	files := guideExample(t)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	require.ElementsMatch(t, []string{"theme.yml", "theme.css", "templates/home.html", "templates/app.html", "templates/bookmark.html", "templates/group.html"}, names)

	userThemeDirectory(t, "harbor")
	resetCompiledThemePages(t)
	writeThemeDirectory(t, "harbor", files["theme.yml"])
	writeThemeStylesheet(t, "harbor", files["theme.css"])
	for name, content := range files {
		if dir, file := filepath.Split(name); dir == "templates/" {
			writeThemeTemplate(t, "harbor", file, content)
		}
	}
	ReloadUserThemes()
	_, problems, _ := UserThemes()
	require.Empty(t, problems)
	harbor := userTheme(t, "harbor")
	require.NotEmpty(t, harbor.Stylesheet)
	require.Equal(t, []string{"home.html", "app.html", "bookmark.html", "group.html"}, ThemeTemplateFiles("harbor"))
	require.Equal(t, []string{model.LayoutMobileBookmarkLayout, model.LayoutBookmarkLayout, model.LayoutDesktopColumns}, model.ThemeLayoutUnsupported(harbor.Supports, harbor.Layout))

	text := func(key string) string { return key }
	for _, column := range []string{"true", "false"} {
		settings := model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{"harbor": {Templates: true, Options: map[string]string{"column": column}}}}
		for _, kind := range themetemplate.PageKinds {
			for _, searching := range []bool{false, true} {
				modules := map[string]bool{}
				for _, name := range themetemplate.PageSlotNames {
					modules[name[len(themetemplate.ReservedPrefix):]] = true
				}
				page := ThemePage{Theme: "harbor", Kind: kind, Lang: "zh", T: text, Searching: searching, Modules: modules}
				shown := renderThemePage(t, ThemePageTemplate(settings, page))
				require.Contains(t, shown, `data-page="`+string(kind)+`"`)
				require.Equal(t, column == "true", regexp.MustCompile(`class="harbor harbor-with-column"`).MatchString(shown), column)
			}
		}
		items := ThemeItemTemplates(settings, "harbor", false, "zh", text)
		require.NotNil(t, items)
		for _, kind := range themetemplate.ItemKinds {
			for _, values := range themetemplate.ItemValueSets(kind) {
				require.NotNil(t, items.Fragment(kind, values), "%s %+v", kind, values)
			}
		}
	}
	require.Empty(t, ThemePageTemplateProblems("harbor"))
}

// The guide has a row for every reason a template is not used, in the
// words of the theme page.
func TestTemplateGuideNamesEveryReason(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "theme-template-guide.md"))
	require.NoError(t, err)
	guide := strings.ToLower(strings.ReplaceAll(string(raw), "`", ""))
	texts, err := os.ReadFile(filepath.Join("..", "..", "internal", "i18n", "locales", "en.json"))
	require.NoError(t, err)
	reasons := regexp.MustCompile(`"theme_file_problem_template_[a-z_]+": "The template is not used: ([^"]+)"`).FindAllStringSubmatch(string(texts), -1)
	require.Len(t, reasons, 21)
	for _, reason := range reasons {
		row := "| " + strings.TrimRight(strings.ToLower(reason[1]), ".:") + " |"
		require.Contains(t, guide, row)
	}
}
