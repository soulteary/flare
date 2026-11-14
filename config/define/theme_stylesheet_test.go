package define

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

const userThemeStylesheetText = "/* Rounded tiles. */\n.app-item { border-radius: calc(var(--flare-radius) * 2); }\n"

// writeThemeStylesheet writes the theme.css of a theme that is a directory.
func writeThemeStylesheet(t *testing.T, name, content string) {
	t.Helper()
	writeUserTheme(t, filepath.Join(name, userThemeStylesheet), content)
}

func TestThemeStylesheetIsRead(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	// A theme without a theme.css has no stylesheet, and neither has one
	// with an empty theme.css.
	writeThemeDirectory(t, "bare", userThemeColors)
	writeThemeDirectory(t, "blank", userThemeColors)
	writeThemeStylesheet(t, "blank", "")
	// A stylesheet belongs to a theme that is a directory: beside no
	// theme.yml it is not read, whatever it holds.
	writeUserTheme(t, "plain.yml", userThemeColors)
	makeThemesSubdirectory(t, "plain")
	writeThemeStylesheet(t, "plain", "body { background: url(a.png) }")
	// Other stylesheets of the directory are not read.
	writeUserTheme(t, filepath.Join("styled", "other.css"), "body { background: url(a.png) }")
	makeThemesSubdirectory(t, filepath.Join("styled", "assets"))
	writeUserTheme(t, filepath.Join("styled", "assets", "theme.css"), "body { background: url(a.png) }")

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"bare", "blank", "plain", "styled"}, userThemeNames(themes))
	require.Empty(t, problems)
	require.Empty(t, notices)
	for _, theme := range themes[:3] {
		require.Empty(t, theme.Stylesheet, theme.Name)
		require.Empty(t, theme.Hash, theme.Name)
		page, known := GetPageThemeFor(theme.Name)
		require.True(t, known, theme.Name)
		require.Empty(t, page.Stylesheet, theme.Name)
	}

	// The stylesheet is kept as it is in the file, under a hash of it.
	styled := themes[3]
	require.Equal(t, userThemeStylesheetText, styled.Stylesheet)
	require.Regexp(t, regexp.MustCompile(`^[0-9a-f]{16}$`), styled.Hash)
	address := "/theme/styled/" + styled.Hash + "/theme.css"
	page, known := GetPageThemeFor("styled")
	require.True(t, known)
	require.Equal(t, address, page.Stylesheet)
	require.Equal(t, userThemeStyle, page.BodyStyle)

	// It is handed out by the name and the hash of that address only.
	content, known := ThemeStylesheet("styled", styled.Hash)
	require.True(t, known)
	require.Equal(t, userThemeStylesheetText, content)
	for _, wrong := range [][2]string{
		{"styled", ""}, {"styled", strings.Repeat("0", 16)}, {"styled", strings.ToUpper(styled.Hash)},
		{"styled", styled.Hash + "0"}, {"bare", ""}, {"bare", styled.Hash}, {"blank", ""},
		{"plain", styled.Hash}, {"blackboard", ""}, {model.ThemeCustomName, ""}, {"", ""},
		{"missing", styled.Hash}, {"../styled", styled.Hash}, {"styled/other", styled.Hash},
	} {
		_, known = ThemeStylesheet(wrong[0], wrong[1])
		require.False(t, known, "%q %q", wrong[0], wrong[1])
	}

	// The pages take the stylesheet of the selected theme, and none with a
	// bundled theme or custom colors.
	require.Empty(t, GetPageTheme().Stylesheet)
	SetTheme("styled")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Stylesheet: address}, GetPageTheme())
	SetTheme("bare")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.Empty(t, GetPageTheme().Stylesheet)

	// With different values for light and dark the stylesheet stays.
	writeThemeDirectory(t, "styled", userThemeColors+"modes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n")
	SetTheme("styled")
	page = GetPageTheme()
	require.Empty(t, page.BodyStyle)
	require.NotEmpty(t, page.Rules)
	require.Equal(t, address, page.Stylesheet)
}

// A stylesheet that is not accepted leaves the theme out of use: a theme is
// not applied without its stylesheet.
func TestThemeStylesheetNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	outside := filepath.Join(t.TempDir(), "outside.css")
	require.NoError(t, os.WriteFile(outside, []byte(userThemeStylesheetText), 0600))
	stylesheets := map[string]string{
		"address":   "a { color: red }\n.app-item { background: url(https://example.test/a.png) }\n",
		"import":    "@import \"https://example.test/a.css\";\n",
		"font":      "a { color: red }\n\n@font-face { font-family: a; src: url(//example.test/a.woff2) }\n",
		"missing":   "a { color: red }\n.app-item { background: url(assets/a.png) }\n",
		"function":  ".app-item { background: image-set(\"a.png\" 1x) }\n",
		"character": ".app-item { background: #×url(a.png) }\n",
		"encoding":  "a { color: red } \xff",
		"at-limit":  userThemeStylesheetText + strings.Repeat(" ", model.MaximumThemeStylesheetBytes-len(userThemeStylesheetText)),
		"too-large": userThemeStylesheetText + strings.Repeat(" ", model.MaximumThemeStylesheetBytes-len(userThemeStylesheetText)+1),
	}
	for name, stylesheet := range stylesheets {
		writeThemeDirectory(t, name, userThemeColors)
		writeThemeStylesheet(t, name, stylesheet)
	}
	// theme.css has to be a regular file of the directory.
	for _, name := range []string{"folder", "link-inside", "link-outside", "link-nowhere"} {
		writeThemeDirectory(t, name, userThemeColors)
	}
	makeThemesSubdirectory(t, filepath.Join("folder", userThemeStylesheet))
	writeThemeDirectory(t, "usable", userThemeColors)
	writeThemeStylesheet(t, "usable", userThemeStylesheetText)
	require.NoError(t, os.Symlink(filepath.Join("..", "usable", userThemeStylesheet), filepath.Join(UserThemeDirectory, "link-inside", userThemeStylesheet)))
	require.NoError(t, os.Symlink(outside, filepath.Join(UserThemeDirectory, "link-outside", userThemeStylesheet)))
	require.NoError(t, os.Symlink("missing.css", filepath.Join(UserThemeDirectory, "link-nowhere", userThemeStylesheet)))
	// A theme.yml that is not usable is reported, and the stylesheet beside
	// it is not looked at.
	writeThemeDirectory(t, "both", userThemeColors+"  radius: 4em\n")
	writeThemeStylesheet(t, "both", "@import 'a.css';")

	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Equal(t, []string{"at-limit", "usable"}, userThemeNames(themes))
	require.Len(t, themes[0].Stylesheet, model.MaximumThemeStylesheetBytes)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "address/theme.css", Reason: ThemeProblemStyleURL, Subject: "line 2: url()"},
		{File: "import/theme.css", Reason: ThemeProblemStyleRule, Subject: "line 1: @import"},
		{File: "font/theme.css", Reason: ThemeProblemStyleURL, Subject: "line 3: url()"},
		{File: "missing/theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 2: url(assets/a.png)"},
		{File: "function/theme.css", Reason: ThemeProblemStyleFunction, Subject: "line 1: image-set()"},
		{File: "character/theme.css", Reason: ThemeProblemStyleCharacter, Subject: "line 1: U+00D7"},
		{File: "encoding/theme.css", Reason: ThemeProblemStyleEncoding},
		{File: "too-large/theme.css", Reason: ThemeProblemStyleSize},
		{File: "folder/theme.css", Reason: ThemeProblemType},
		{File: "link-inside/theme.css", Reason: ThemeProblemType},
		{File: "link-outside/theme.css", Reason: ThemeProblemType},
		{File: "link-nowhere/theme.css", Reason: ThemeProblemType},
		{File: "both/theme.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
	}, problems)

	// None of them can be selected, and nothing of them reaches a page: no
	// colors and no stylesheet.
	for _, problem := range problems {
		name, _, _ := strings.Cut(problem.File, "/")
		require.False(t, IsUserTheme(name), name)
		require.False(t, IsKnownTheme(name), name)
		_, known := GetPageThemeFor(name)
		require.False(t, known, name)
		SetTheme(name)
		require.Equal(t, PageTheme{}, GetPageTheme(), name)
	}
}

func TestUnreadableThemeStylesheet(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not apply to this user")
	}
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "locked", userThemeColors)
	writeThemeStylesheet(t, "locked", userThemeStylesheetText)
	path := filepath.Join(UserThemeDirectory, "locked", userThemeStylesheet)
	require.NoError(t, os.Chmod(path, 0))
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Empty(t, themes)
	require.Equal(t, []ThemeProblem{{File: "locked/theme.css", Reason: ThemeProblemRead}}, problems)
}

// The pages follow the stylesheet of the selected theme as they follow its
// theme file.
func TestSelectedThemeFollowsItsStylesheet(t *testing.T) {
	userThemeDirectory(t, "styled")
	writeThemeDirectory(t, "styled", userThemeColors)
	UpdatePagePalettes()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
	address := func() string {
		t.Helper()
		themes, _, _ := UserThemes()
		for _, theme := range themes {
			if theme.Name == "styled" && theme.Stylesheet != "" {
				return "/theme/styled/" + theme.Hash + "/theme.css"
			}
		}
		return ""
	}

	// A stylesheet that appears is taken by the next page.
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	first := GetPageTheme().Stylesheet
	require.NotEmpty(t, first)
	require.Equal(t, address(), first)
	_, _, generation := ThemeRenderSnapshot()

	// While the files keep their state the directory is not read.
	writeUserTheme(t, "added.yml", userThemeColors)
	for range 3 {
		require.Equal(t, first, GetPageTheme().Stylesheet)
	}
	require.False(t, IsUserTheme("added"))
	_, _, unchanged := ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)

	// An edit has another address, and the content of the earlier one is
	// no longer handed out.
	edited := userThemeStylesheetText + ".app-title { letter-spacing: 1px }\n"
	writeThemeStylesheet(t, "styled", edited)
	second := GetPageTheme().Stylesheet
	require.NotEqual(t, first, second)
	require.Equal(t, address(), second)
	require.True(t, IsUserTheme("added"))
	hash := func(address string) string { return strings.Split(address, "/")[3] }
	_, known := ThemeStylesheet("styled", hash(first))
	require.False(t, known)
	content, known := ThemeStylesheet("styled", hash(second))
	require.True(t, known)
	require.Equal(t, edited, content)

	// An edit that keeps the size is seen by the modification time.
	writeThemeStylesheet(t, "styled", strings.Replace(edited, "1px", "2px", 1))
	setUserThemeAge(t, filepath.Join("styled", userThemeStylesheet), 2*time.Minute)
	third := GetPageTheme().Stylesheet
	require.NotEqual(t, second, third)

	// A stylesheet that is not accepted leaves the selection without a
	// theme: no colors and no stylesheet.
	writeThemeStylesheet(t, "styled", "@import 'a.css';")
	require.Equal(t, PageTheme{}, GetPageTheme())
	require.False(t, IsKnownTheme("styled"))
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "styled/theme.css", Reason: ThemeProblemStyleRule, Subject: "line 1: @import"}}, problems)

	// Correcting it brings the theme back, and removing it leaves the theme
	// without a stylesheet.
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	setUserThemeAge(t, filepath.Join("styled", userThemeStylesheet), 3*time.Minute)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Stylesheet: first}, GetPageTheme())
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "styled", userThemeStylesheet)))
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
}

// The stylesheet is watched with the theme.yml it belongs to, and not beside
// a file or a directory that is no theme.
func TestThemeStampWatchesTheStylesheet(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "plain.yml", userThemeColors)
	makeThemesSubdirectory(t, "plain")
	writeThemeStylesheet(t, "plain", userThemeStylesheetText)
	stamp := statTheme("plain")
	require.True(t, stamp.file.exists)
	require.False(t, stamp.manifest.exists)
	require.False(t, stamp.stylesheet.exists)

	writeThemeDirectory(t, "plain", userThemeColors)
	stamp = statTheme("plain")
	require.True(t, stamp.manifest.exists)
	require.True(t, stamp.stylesheet.exists)

	now := time.Now()
	read := themeFileStamp{modTime: now.Add(-time.Minute), size: 100, exists: true}
	edited := themeFileStamp{modTime: now.Add(-time.Minute), size: 101, exists: true}
	for name, tc := range map[string]struct {
		read, found themeStamp
		current     bool
	}{
		"unchanged":              {themeStamp{manifest: read, stylesheet: read}, themeStamp{manifest: read, stylesheet: read}, true},
		"no stylesheet":          {themeStamp{manifest: read}, themeStamp{manifest: read}, true},
		"a stylesheet appears":   {themeStamp{manifest: read}, themeStamp{manifest: read, stylesheet: read}, false},
		"the stylesheet edited":  {themeStamp{manifest: read, stylesheet: read}, themeStamp{manifest: read, stylesheet: edited}, false},
		"the stylesheet removed": {themeStamp{manifest: read, stylesheet: read}, themeStamp{manifest: read}, false},
	} {
		require.Equal(t, tc.current, tc.read.current(tc.found, now), name)
	}
}
