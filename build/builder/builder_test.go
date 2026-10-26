package builder

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyAndWalkNestedResources(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, _CreateIfNotExists("src/nested", 0750))
	require.NoError(t, _CreateIfNotExists("src/nested", 0750))
	require.NoError(t, os.WriteFile("src/nested/page.html", []byte("<h1>keep</h1>"), 0600))
	require.NoError(t, os.WriteFile("src/asset.css", []byte("body{}"), 0600))
	_PrepareDirectory("dest")
	require.NoError(t, _CopyDirectoryWithoutSymlink("src", "dest"))
	got, err := os.ReadFile("dest/nested/page.html")
	require.NoError(t, err)
	require.Equal(t, "<h1>keep</h1>", string(got))
	matches, err := _WalkMatch("dest", "*.html")
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join("dest", "nested", "page.html")}, matches)
	require.Error(t, _CopyDirectoryWithoutSymlink("missing", "dest"))
	require.Error(t, _Copy("missing", "dest/missing"))
	require.Error(t, _Copy("src/asset.css", "missing/dest"))
	_, err = _WalkMatch("missing", "*.html")
	require.Error(t, err)
	_, err = _WalkMatch("src", "[")
	require.Error(t, err)
	// A broken resource link must fail rather than silently omit a resource.
	require.NoError(t, os.Symlink("missing", "src/broken"))
	require.Error(t, _CopyDirectoryWithoutSymlink("src", "dest"))
}

func TestTemplateBuildPreservesGoActionsAndReplacesStaleOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("src", 0700))
	require.NoError(t, os.Mkdir("dest", 0700))
	require.NoError(t, os.WriteFile("dest/stale.html", []byte("old"), 0600))
	require.NoError(t, os.WriteFile("src/page.html", []byte("<!doctype html>\n<html>\n<body> <!-- comment --> <h1>{{ .Title }}</h1> </body>\n</html>"), 0600))
	TaskForTemplates("src", "dest")
	_, err := os.Stat("dest/stale.html")
	require.ErrorIs(t, err, os.ErrNotExist)
	got, err := os.ReadFile("dest/page.html")
	require.NoError(t, err)
	require.Contains(t, string(got), "{{ .Title }}")
	require.Contains(t, string(got), "<html>")
	require.NotContains(t, string(got), "<!-- comment -->")
	require.NotContains(t, string(got), "\n")
}

func TestStyleBuildGeneratesParsableGo(t *testing.T) {
	t.Chdir(t.TempDir())
	paths := []string{"embed/assets/css/base.css", "embed/assets/css/home/apps.css", "embed/assets/css/home/bookmarks.css", "embed/assets/css/home/hero.css", "embed/assets/css/home/search.css", "embed/assets/css/home/toolbar.css", "embed/assets/css/settings/layout.css", "embed/assets/css/settings/sidebar.css", "embed/assets/css/settings/theme.css"}
	for _, path := range paths {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte("/* remove */\nbody {\n color: red;\n}\n"), 0600))
	}
	TaskForStyles("style.go")
	raw, err := os.ReadFile("style.go")
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "style.go", raw, parser.AllErrors)
	require.NoError(t, err)
	require.Contains(t, string(raw), "PAGE_INLINE_STYLE")
	require.Contains(t, string(raw), "body {color: red;}")
	require.NotContains(t, string(raw), "remove")
	require.Equal(t, "a\nb", string(RemoveCppStyleComments([]byte("a// comment\nb"))))
	require.Panics(t, func() { concatenateCSS([]string{"missing.css"}) })
}

func TestMDIBuildGeneratesIconsAndCopiesAssets(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir("src", 0700))
	require.NoError(t, os.WriteFile("src/icon.svg", []byte("<svg></svg>"), 0600))
	require.NoError(t, os.WriteFile("icons.js", []byte("exports.mdiTestIcon = \"M0 1L2 3\";\nexports.mdiHome = \"M1 2\";"), 0600))
	TaskForMdi("src", "dest", "icons.js", "icons.go")
	raw, err := os.ReadFile("icons.go")
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "icons.go", raw, parser.AllErrors)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"testicon": "M0 1L2 3"`)
	copied, err := os.ReadFile("dest/icon.svg")
	require.NoError(t, err)
	require.Equal(t, "<svg></svg>", string(copied))
}
