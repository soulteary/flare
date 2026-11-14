package define

import (
	"os"
	"path/filepath"
	"testing"

	version "github.com/soulteary/version-kit/v4"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// setApplicationVersion runs a test as an application of a given version.
func setApplicationVersion(t *testing.T, release string) {
	t.Helper()
	previous := version.Version
	version.Version = release
	t.Cleanup(func() { version.Version = previous })
}

// A theme that names a later release than the one that runs is not in use,
// as a file and as a directory, and the theme page is told which release it
// needs.
func TestThemeThatNeedsALaterRelease(t *testing.T) {
	setApplicationVersion(t, "0.7.0")
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "earlier.yml", userThemeColors+"requires: 0.6.0\n")
	writeUserTheme(t, "now.yml", userThemeColors+"requires: 0.7.0\n")
	writeUserTheme(t, "later.yml", userThemeColors+"requires: 0.7.1\n")
	writeUserTheme(t, "none.yml", userThemeColors)
	writeUserTheme(t, "written-wrong.yml", userThemeColors+"requires: \">=0.7.0\"\n")
	// A file for a later release can hold what this release does not accept
	// or does not know. It is reported for the release alone.
	writeUserTheme(t, "later-values.yml", userThemeColors+"  radius: 4em\noptions:\n  - id: density\nrequires: 0.8.0\n")
	writeThemeDirectory(t, "later-styled", userThemeColors+"requires: 1.0.0\n")
	writeThemeStylesheet(t, "later-styled", "@import 'a.css';")
	writeThemeDirectory(t, "now-styled", userThemeColors+"requires: 0.7.0\n")
	writeThemeStylesheet(t, "now-styled", userThemeStylesheetText)

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"earlier", "none", "now-styled", "now"}, userThemeNames(themes))
	require.Empty(t, notices)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "later.yml", Reason: ThemeProblemRequires, Subject: "0.7.1"},
		{File: "later-values.yml", Reason: ThemeProblemRequires, Subject: "0.8.0"},
		{File: "later-styled/theme.yml", Reason: ThemeProblemRequires, Subject: "1.0.0"},
		{File: "written-wrong.yml", Reason: ThemeProblemRequiresValue, Subject: ">=0.7.0"},
	}, problems)
	for _, name := range []string{"later", "later-values", "later-styled", "written-wrong"} {
		require.False(t, IsUserTheme(name), name)
		SetTheme(name)
		require.Equal(t, PageTheme{}, GetPageTheme(), name)
	}
	SetTheme("now-styled")
	require.NotEmpty(t, GetPageTheme().Stylesheet)

	// A later release uses the themes that need it.
	setApplicationVersion(t, "0.8.0")
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Equal(t, []string{"earlier", "later", "none", "now-styled", "now"}, userThemeNames(themes))
	require.ElementsMatch(t, []ThemeProblem{
		{File: "later-values.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
		{File: "later-styled/theme.yml", Reason: ThemeProblemRequires, Subject: "1.0.0"},
		{File: "written-wrong.yml", Reason: ThemeProblemRequiresValue, Subject: ">=0.7.0"},
	}, problems)

	// A build that is not a release, as during development, meets every
	// requirement, and still wants the release written as one.
	setApplicationVersion(t, "dev")
	ReloadUserThemes()
	_, problems, _ = UserThemes()
	require.ElementsMatch(t, []ThemeProblem{
		{File: "later-values.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
		{File: "later-styled/theme.css", Reason: ThemeProblemStyleRule, Subject: "line 1: @import"},
		{File: "written-wrong.yml", Reason: ThemeProblemRequiresValue, Subject: ">=0.7.0"},
	}, problems)
}

// The selected theme is held to the release it needs like any other.
func TestSelectedThemeThatNeedsALaterRelease(t *testing.T) {
	setApplicationVersion(t, "0.7.0")
	userThemeDirectory(t, "forest")
	writeUserTheme(t, "forest.yml", userThemeColors+"requires: 0.7.0\n")
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle, GetAppBodyStyle())

	// An edit that names a later release leaves the selection without a theme.
	writeUserTheme(t, "forest.yml", userThemeColors+"requires: 0.7.10\n")
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "forest.yml", Reason: ThemeProblemRequires, Subject: "0.7.10"}}, problems)
}

// A theme file that needs a later release is not stored either.
func TestStoreUserThemeThatNeedsALaterRelease(t *testing.T) {
	setApplicationVersion(t, "0.7.0")
	userThemeDirectory(t, "blackboard")
	require.ErrorIs(t, StoreUserTheme("later", []byte(userThemeColors+"requires: 0.7.1\n"), false), model.ErrThemeRequires)
	require.ErrorIs(t, StoreUserTheme("wrong", []byte(userThemeColors+"requires: v0.7.0\n"), false), model.ErrThemeRequiresValue)
	require.Empty(t, themesDirectoryEntries(t))
	problem := ThemeFileProblem(StoreUserTheme("later", []byte(userThemeColors+"requires: 0.7.1\n"), false))
	require.Equal(t, ThemeProblem{Reason: ThemeProblemRequires, Subject: "0.7.1"}, problem)

	raw := userThemeColors + "requires: 0.7.0\n"
	require.NoError(t, StoreUserTheme("now", []byte(raw), false))
	stored, err := os.ReadFile(filepath.Join(UserThemeDirectory, "now.yml"))
	require.NoError(t, err)
	require.Equal(t, raw, string(stored))
	require.True(t, IsUserTheme("now"))
}
