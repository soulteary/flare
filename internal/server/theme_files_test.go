package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/resources/assets"
)

const (
	themeFileColors = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n"
	themeFileStyle  = "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;"
	forestThemeFile = themeFileColors + "  muted: \"#9db8a6\"\n  hover: \"#ffffff1a\"\n  radius: 10px\n  font-family: Georgia, serif\n" +
		"name:\n  en: Forest at dusk\n  zh: 暮色森林\nversion: \"1.2\"\nauthor: Example Author\nlicense: MIT\n"
	forestThemeStyle = themeFileStyle + "--flare-muted:#9db8a6;--flare-hover:#ffffff1a;--flare-radius:10px;--flare-font-family:Georgia, serif;"
)

// writeThemeFile writes a file of the themes directory that was last modified
// a minute ago, as a file nobody is editing. A file in a directory of the
// themes directory has that directory made for it.
func writeThemeFile(t *testing.T, file, content string) {
	t.Helper()
	path := filepath.Join(define.UserThemeDirectory, file)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	modified := time.Now().Add(-time.Minute)
	require.NoError(t, os.Chtimes(path, modified, modified))
}

// themeFilesRouter starts an instance in its own data directory with the
// given theme files, and leaves no theme file behind for the following tests.
func themeFilesRouter(t *testing.T, config string, files map[string]string) http.Handler {
	t.Helper()
	customThemeDirectory(t, config)
	// Runs before the cleanup of customThemeDirectory reads the directory again.
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	for file, content := range files {
		writeThemeFile(t, file, content)
	}
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	return handler
}

func getPage(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code, path)
	return rec.Body.String()
}

// The theme page as a browser reads it.
type themeFilePage struct {
	cards     []themeFilePageCard
	lines     [][]string
	lineTexts []string
	alerts    []string
	help      []string
	// elements counts the elements inside the cards and the lines, by name.
	elements map[string]int
}

type themeFilePageCard struct {
	value, label, title, about string
	colors                     []string
}

func nodeAttribute(node *html.Node, key string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == key {
			return attribute.Val
		}
	}
	return ""
}

func nodeHasClass(node *html.Node, class string) bool {
	return node.Type == html.ElementNode && strings.Contains(" "+nodeAttribute(node, "class")+" ", " "+class+" ")
}

func nodeText(node *html.Node) string {
	if node.Type == html.TextNode {
		return node.Data
	}
	var text strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		text.WriteString(nodeText(child))
	}
	return text.String()
}

func walkNodes(node *html.Node, visit func(*html.Node)) {
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkNodes(child, visit)
	}
}

func readThemeFilePage(t *testing.T, page string) themeFilePage {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	read := themeFilePage{elements: map[string]int{}}
	count := func(root *html.Node) {
		walkNodes(root, func(node *html.Node) {
			if node.Type == html.ElementNode && node != root {
				read.elements[node.Data]++
			}
		})
	}
	walkNodes(document, func(node *html.Node) {
		switch {
		case nodeHasClass(node, "theme-file"):
			card := themeFilePageCard{}
			walkNodes(node, func(part *html.Node) {
				switch {
				case nodeHasClass(part, "theme-choose"):
					card.value, card.label = nodeAttribute(part, "value"), nodeAttribute(part, "aria-label")
					require.Equal(t, "theme", nodeAttribute(part, "name"))
					require.Equal(t, "submit", nodeAttribute(part, "type"))
				case nodeHasClass(part, "theme-title"):
					card.title = nodeText(part)
				case nodeHasClass(part, "theme-about"):
					card.about = nodeText(part)
				case nodeHasClass(part, "theme-color"):
					card.colors = append(card.colors, nodeAttribute(part, "style"))
				}
			})
			read.cards = append(read.cards, card)
			count(node)
		case nodeHasClass(node, "theme-file-lines"):
			for item := node.FirstChild; item != nil; item = item.NextSibling {
				if item.Type != html.ElementNode {
					continue
				}
				require.Equal(t, "li", item.Data)
				var codes []string
				for part := item.FirstChild; part != nil; part = part.NextSibling {
					if part.Type == html.ElementNode && part.Data == "code" {
						codes = append(codes, nodeText(part))
					}
				}
				read.lines = append(read.lines, codes)
				read.lineTexts = append(read.lineTexts, strings.Join(strings.Fields(nodeText(item)), " "))
			}
			count(node)
		case nodeHasClass(node, "theme-notice") && nodeAttribute(node, "role") == "alert":
			read.alerts = append(read.alerts, nodeText(node))
		case nodeHasClass(node, "theme-help"):
			read.help = append(read.help, nodeText(node))
		}
	})
	return read
}

func TestThemeFilesHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\nShowApps: true\n", map[string]string{
		"forest.yml":   forestThemeFile,
		"later.yml":    themeFileColors + "  space: 4px\noptions:\n  - id: density\n",
		"rejected.yml": themeFileColors + "  radius: 4em\n",
		"Mixed.yml":    themeFileColors,
		"notes.yaml":   themeFileColors,
		"README.md":    "notes",
	})

	// The page lists the usable files as themes after the bundled ones, and
	// says for every other theme file why it is not in use.
	raw := getPage(t, handler, "/settings/theme")
	require.Contains(t, raw, blackboardThemeStyle)
	page := readThemeFilePage(t, raw)
	swatches := []string{"background-color: #10241b;", "background-color: #e8f1e4;", "background-color: #f2b84b;"}
	require.Equal(t, []themeFilePageCard{
		{value: "forest", label: "Forest at dusk", title: "Forest at dusk", about: "1.2 · Example Author · MIT", colors: swatches},
		{value: "later", label: "later", title: "later", colors: swatches},
	}, page.cards)
	require.Equal(t, [][]string{
		{"Mixed.yml"},
		{"later.yml", "options, tokens.space"},
		{"notes.yaml"},
		{"rejected.yml", "tokens.radius"},
	}, page.lines)
	require.Equal(t, []string{
		"Mixed.yml " + i18n.T("en", "theme_file_problem_name"),
		"later.yml " + i18n.T("en", "theme_file_unused") + " options, tokens.space",
		"notes.yaml " + i18n.T("en", "theme_file_problem_extension"),
		"rejected.yml " + i18n.T("en", "theme_file_problem_value") + " tokens.radius",
	}, page.lineTexts)
	require.Empty(t, page.alerts)
	require.Contains(t, page.help, i18n.T("en", "theme_files_help"))
	require.Contains(t, raw, i18n.T("en", "theme_files"))

	// The name of a theme is shown in the language of the interface.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: blackboard\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, "暮色森林", page.cards[0].title)
	require.Equal(t, "暮色森林", page.cards[0].label)
	require.Equal(t, "forest", page.cards[0].value)
	require.Contains(t, page.help, i18n.T("zh", "theme_files_help"))
	require.Equal(t, "rejected.yml "+i18n.T("zh", "theme_file_problem_value")+" tokens.radius", page.lineTexts[3])
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: en\nTheme: blackboard\nShowApps: true\n"), 0600))

	// Selecting a theme file stores its name and applies every token it
	// sets to all themed pages. It has no generated favicon of its own.
	selected := themeRequest(handler, url.Values{"theme": {"forest"}})
	require.Equal(t, http.StatusOK, selected.Code)
	require.Equal(t, forestThemeStyle, bodyStyle(t, selected.Body.String()))
	require.Contains(t, selected.Body.String(), `href="`+assets.FaviconURL()+`"`)
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(stored), "Theme: forest")
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help", "/settings/theme", "/settings/appearance"} {
		require.Equal(t, forestThemeStyle, bodyStyle(t, getPage(t, handler, path)), path)
	}
	require.Contains(t, getPage(t, handler, "/"), `href="`+assets.FaviconURL()+`"`)

	// A file with parts this release does not use can be selected; a file
	// that is not in use cannot, and neither can anything that is no name.
	for _, name := range []string{"rejected", "Mixed", "notes", "README", "missing", "forest.yml", "../config", "themes/forest"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		require.Equal(t, forestThemeStyle, bodyStyle(t, getPage(t, handler, "/")), name)
	}
	unchanged, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(unchanged))
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"later"}}).Code)
	require.Equal(t, themeFileStyle, bodyStyle(t, getPage(t, handler, "/")))
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"forest"}}).Code)

	// An edit of the selected file is applied by the next page, without a
	// restart and without visiting the settings.
	writeThemeFile(t, "forest.yml", strings.Replace(forestThemeFile, "radius: 10px", "radius: 2px", 1))
	edited := strings.Replace(forestThemeStyle, "--flare-radius:10px;", "--flare-radius:2px;", 1)
	for _, path := range []string{"/", "/help", "/settings/theme"} {
		require.Equal(t, edited, bodyStyle(t, getPage(t, handler, path)), path)
	}

	// A file that is rejected leaves the pages without a theme, so they use
	// the colors of the stylesheet. The settings say which theme that is and
	// what is wrong with its file.
	writeThemeFile(t, "forest.yml", strings.Replace(forestThemeFile, "  primary: \"#e8f1e4\"\n", "", 1))
	for _, path := range []string{"/", "/help", "/settings/appearance"} {
		require.Empty(t, bodyStyle(t, getPage(t, handler, path)), path)
	}
	raw = getPage(t, handler, "/settings/theme")
	require.Empty(t, bodyStyle(t, raw))
	page = readThemeFilePage(t, raw)
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "forest"})}, page.alerts)
	require.Len(t, page.cards, 1)
	require.Equal(t, "later", page.cards[0].value)
	require.Contains(t, page.lines, []string{"forest.yml", "tokens.primary"})
	// The colors of a rejected file are not shown anywhere on the page.
	require.NotContains(t, raw, "#9db8a6")

	// Removing the file is reported in the same way, and putting a usable
	// file back brings the theme back.
	require.NoError(t, os.Remove(filepath.Join(define.UserThemeDirectory, "forest.yml")))
	require.Empty(t, bodyStyle(t, getPage(t, handler, "/")))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Len(t, page.alerts, 1)
	require.NotContains(t, page.lines, []string{"forest.yml", "tokens.primary"})
	writeThemeFile(t, "forest.yml", forestThemeFile)
	require.Equal(t, forestThemeStyle, bodyStyle(t, getPage(t, handler, "/")))
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)

	// A bundled theme can be selected again and has its own favicon.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"cloud"}}).Code)
	home := getPage(t, handler, "/")
	require.Equal(t, "--color-background:#f1f2f0;--color-primary:#35342f;--color-accent:#37bbe4;", bodyStyle(t, home))
	require.NotContains(t, home, `href="`+assets.FaviconURL()+`"`)
}

// The selection of config.yml is applied when the instance starts.
func TestThemeFileSelectedAtStartup(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: forest\n", map[string]string{"forest.yml": forestThemeFile})
	for _, path := range []string{"/", "/help", "/settings/theme"} {
		require.Equal(t, forestThemeStyle, bodyStyle(t, getPage(t, handler, path)), path)
	}
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)
}

// A selection without a usable file starts with the colors of the stylesheet
// and says so on the theme page. The file can be added while the instance
// runs.
func TestThemeFileMissingAtStartup(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: forest\n", nil)
	require.Empty(t, bodyStyle(t, getPage(t, handler, "/")))
	page := readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "forest"})}, page.alerts)
	require.Empty(t, page.cards)
	require.Empty(t, page.lines)

	writeThemeFile(t, "forest.yml", forestThemeFile)
	require.Equal(t, forestThemeStyle, bodyStyle(t, getPage(t, handler, "/")))

	// Without the directory nothing is listed and nothing is reported.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"blackboard"}}).Code)
	require.NoError(t, os.RemoveAll(define.UserThemeDirectory))
	raw := getPage(t, handler, "/settings/theme")
	page = readThemeFilePage(t, raw)
	require.Empty(t, page.cards)
	require.Empty(t, page.lines)
	require.Empty(t, page.alerts)
	require.Contains(t, page.help, i18n.T("en", "theme_files_help"))
	require.Contains(t, raw, blackboardThemeStyle)
}

// Texts and names from the themes directory are shown as text. Nothing in a
// theme file or in the name of a file becomes markup or a declaration.
func TestThemeFileContentIsShownAsText(t *testing.T) {
	markup := `<script>alert(1)</script>`
	breakout := `"><img src=x onerror=alert(1)>`
	longName := strings.Repeat("n", 120) + ".yml"
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", map[string]string{
		"texts.yml":    themeFileColors + "name: '" + markup + "'\nauthor: '" + breakout + "'\nlicense: \"' onmouseover='alert(1)\"\n",
		"<b>x.yml":     themeFileColors,
		"a\nb.yml":     themeFileColors,
		longName:       themeFileColors,
		"unused.yml":   themeFileColors + "  \"<i>token</i>\": \"1\"\n\"<u>key</u>\": 1\n",
		"declared.yml": themeFileColors + "  font-family: \"x;background:url(//example.test/)\"\n",
		"closing.yml":  themeFileColors + "  radius: \"4px\\\"><script>alert(1)</script>\"\n",
	})
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)

	require.Len(t, page.cards, 2)
	texts := page.cards[0]
	require.Equal(t, "texts", texts.value)
	require.Equal(t, markup, texts.title)
	require.Equal(t, markup, texts.label)
	require.Equal(t, breakout+" · ' onmouseover='alert(1)", texts.about)
	require.Equal(t, "unused", page.cards[1].value)

	require.Equal(t, [][]string{
		{"<b>x.yml"},
		{"a b.yml"},
		{"closing.yml", "tokens.radius"},
		{"declared.yml", "tokens.font-family"},
		{strings.Repeat("n", 80) + "…"},
		{"unused.yml", "<u>key</u>, tokens.<i>token</i>"},
	}, page.lines)

	// The cards and the lines hold only the elements of the template, and
	// the two links of a card lead to its preview and to its download.
	for element := range page.elements {
		require.Contains(t, []string{"div", "input", "p", "li", "code", "a"}, element)
	}
	require.Equal(t, 2*len(page.cards), page.elements["a"])
	require.Contains(t, raw, `href="/?theme-preview=texts"`)
	require.Contains(t, raw, `href="/settings/theme/export?name=texts"`)
	require.NotContains(t, raw, "<script>alert(1)")
	require.NotContains(t, raw, "<img src=x")
	require.NotContains(t, raw, "example.test")
	require.NotContains(t, raw, "onmouseover='alert")

	// Selecting the theme with the texts writes only its tokens into the pages.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"texts"}}).Code)
	for _, path := range []string{"/", "/settings/theme"} {
		body := getPage(t, handler, path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
		require.NotContains(t, body, "<script>alert(1)", path)
	}
}

// The font scale of a theme reaches the themed pages with its other tokens,
// and the stylesheet has the default it replaces.
func TestThemeFontScaleReachesThePages(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: scaled\nShowApps: true\n", map[string]string{
		"scaled.yml":   themeFileColors + "  radius: 8px\n  font-scale: 1.25\n",
		"too-big.yml":  themeFileColors + "  font-scale: 3\n",
		"declared.yml": themeFileColors + "  font-scale: \"1;zoom:2\"\n",
	})
	want := themeFileStyle + "--flare-radius:8px;--flare-font-scale:1.25;"
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help", "/settings/theme"} {
		page := getPage(t, handler, path)
		require.Equal(t, want, bodyStyle(t, page), path)
		require.True(t, strings.Contains(page, "--flare-font-scale: 1;"), "%s: the stylesheet has no default for the scale", path)
	}
	// A scale outside the accepted range, or one that is not a number,
	// rejects the file.
	lines := readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lines
	require.Contains(t, lines, []string{"too-big.yml", "tokens.font-scale"})
	require.Contains(t, lines, []string{"declared.yml", "tokens.font-scale"})
	for _, name := range []string{"too-big", "declared"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
	}
}
