package define

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// setThemeSafeMode runs a test with safe mode for themes on or off.
func setThemeSafeMode(t *testing.T, on bool) {
	t.Helper()
	previous := AppFlags.ThemeSafeMode
	AppFlags.ThemeSafeMode = on
	t.Cleanup(func() { AppFlags.ThemeSafeMode = previous })
}

// In safe mode the themes directory is not read: its themes are not in use,
// nothing about it is reported, and nothing is stored in it. What it holds
// cannot be what keeps the instance from being used.
func TestThemeSafeModeLeavesTheThemesDirectoryAlone(t *testing.T) {
	userThemeDirectory(t, "styled")
	writeThemeDirectory(t, "styled", userThemeColors+"layout:\n  DesktopColumns: 6\n")
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	writeThemeAsset(t, "styled", "a.png", assetPNG)
	writeUserTheme(t, "plain.yml", userThemeColors)
	writeUserTheme(t, "rejected.yml", userThemeColors+"  radius: 4em\n")
	writeUserTheme(t, "notes.yaml", userThemeColors)
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	styled := GetPageTheme()
	require.NotEmpty(t, styled.Stylesheet)
	hash := filepath.Base(filepath.Dir(styled.Stylesheet))
	_, _, handedOut := readThemeAsset(t, "styled", hash, "a.png")
	require.True(t, handedOut)

	setThemeSafeMode(t, true)
	UpdatePagePalettes()
	themes, reported, notices := UserThemes()
	require.Empty(t, themes)
	require.Empty(t, reported, "the directory is not looked at, so nothing in it is reported")
	require.Empty(t, notices)
	for _, name := range []string{"styled", "plain", "rejected"} {
		require.False(t, IsUserTheme(name), name)
		require.False(t, IsKnownTheme(name), name)
		_, known := GetPageThemeFor(name)
		require.False(t, known, name)
		_, _, exported := ExportTheme(name)
		require.False(t, exported, name)
	}
	_, served := ThemeStylesheet("styled", hash)
	require.False(t, served)
	_, _, handedOut = readThemeAsset(t, "styled", hash, "a.png")
	require.False(t, handedOut)

	// The selection stays what it is, and the pages have no theme.
	name, _ := ThemeSnapshot()
	require.Equal(t, "styled", name)
	require.Equal(t, PageTheme{}, GetPageTheme())
	selected, layout := SelectedThemeLayout()
	require.Equal(t, "styled", selected)
	require.Empty(t, layout)
	site := model.Application{Homepage: &model.HomepageOptions{DesktopColumns: "4"}}
	require.Equal(t, "4", HomepageInEffect(site).DesktopColumns)

	// An edit of a file of the directory changes nothing, and reading the
	// directory again reads nothing.
	writeThemeDirectory(t, "styled", userThemeColors+"  radius: 10px\n")
	require.Equal(t, PageTheme{}, GetPageTheme())
	ReloadUserThemes()
	require.False(t, IsKnownTheme("styled"))

	// Nothing is stored in the directory.
	before := themesDirectoryEntries(t)
	require.ErrorIs(t, StoreUserTheme("added", []byte(userThemeColors), false), ErrThemeStoreSafeMode)
	require.ErrorIs(t, StoreUserTheme("plain", []byte(userThemeColors), true), ErrThemeStoreSafeMode)
	require.Equal(t, before, themesDirectoryEntries(t))

	// Bundled themes and custom colors work as always.
	SetTheme("blackboard")
	require.Equal(t, blackboardStyle, GetAppBodyStyle())
	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.NotEmpty(t, GetAppBodyStyle())
	_, _, exported := ExportTheme("blackboard")
	require.True(t, exported)

	// Without safe mode the directory is read again and the theme is back.
	setThemeSafeMode(t, false)
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: styled\n"), 0600))
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())
	_, reported, _ = UserThemes()
	require.Len(t, reported, 2)
}

// A directory that cannot even be read does not matter in safe mode.
func TestThemeSafeModeDoesNotOpenTheThemesDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not apply to this user")
	}
	setThemeSafeMode(t, true)
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "plain.yml", userThemeColors)
	require.NoError(t, os.Chmod(UserThemeDirectory, 0))
	t.Cleanup(func() { _ = os.Chmod(UserThemeDirectory, 0700) })
	UpdatePagePalettes()
	themes, reported, _ := UserThemes()
	require.Empty(t, themes)
	require.Empty(t, reported)
	require.Equal(t, blackboardStyle, GetAppBodyStyle())
}

// A page can be shown without the themes of the directory while the instance
// uses them: it keeps a bundled theme and the custom colors, and has no theme
// where the selection names a theme of the directory.
func TestPageThemeWithoutTheThemesDirectory(t *testing.T) {
	userThemeDirectory(t, "styled")
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	UpdatePagePalettes()
	require.NotEqual(t, PageTheme{}, GetPageTheme())
	require.Equal(t, PageTheme{}, GetPageThemeWithoutDirectory())
	// The selection and the pages that do not ask for it are untouched.
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	require.True(t, IsThemeOfDirectory("styled"))

	// A selection without a usable theme has none either way.
	SetTheme("missing")
	require.Equal(t, PageTheme{}, GetPageThemeWithoutDirectory())

	SetTheme("blackboard")
	require.Equal(t, GetPageTheme(), GetPageThemeWithoutDirectory())
	require.Equal(t, blackboardStyle, GetPageThemeWithoutDirectory().BodyStyle)
	require.False(t, IsThemeOfDirectory("blackboard"))
	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.Equal(t, GetPageTheme(), GetPageThemeWithoutDirectory())
	require.NotEmpty(t, GetPageThemeWithoutDirectory().BodyStyle)
	require.False(t, IsThemeOfDirectory(model.ThemeCustomName))
	for _, name := range []string{"", "Mixed", "../styled", "a b"} {
		require.False(t, IsThemeOfDirectory(name), "%q", name)
	}
}
