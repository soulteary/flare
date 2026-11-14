package define

import (
	"errors"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// userThemeBackground is the background of the theme files of these tests.
const userThemeBackground = "background:\n  image: assets/wallpaper.jpg\n  dim: 60\n"

// backgroundRule is the rule a page gets for a background image under an
// address, dimmed by a number of percent.
func backgroundRule(address string, dim string) template.CSS {
	layers := "url(" + address + ")"
	if dim != "" {
		veil := "color-mix(in srgb,var(--flare-background) " + dim + "%,transparent)"
		layers = "linear-gradient(" + veil + "," + veil + ")," + layers
	}
	return template.CSS("body{background-image:" + layers + ";background-size:cover;background-position:center;background-repeat:no-repeat;background-attachment:fixed}")
}

func TestThemeBackgroundIsRead(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "walled", userThemeColors+userThemeBackground)
	writeThemeAsset(t, "walled", "wallpaper.jpg", assetJPEG)
	// The image as it is, without a dim.
	writeThemeDirectory(t, "plain-image", userThemeColors+"background:\n  image: assets/Wall.PNG\n  dim: 0\n")
	writeThemeAsset(t, "plain-image", "Wall.PNG", assetPNG)
	// A theme with a stylesheet and with values for light and dark as well.
	writeThemeDirectory(t, "full", userThemeColors+"background:\n  image: assets/a.png\n  dim: 100\nmodes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n")
	writeThemeStylesheet(t, "full", userThemeStylesheetText)
	writeThemeAsset(t, "full", "a.png", assetPNG)
	// Themes without a background image.
	writeThemeDirectory(t, "bare", userThemeColors)
	writeThemeAsset(t, "bare", "a.png", assetPNG)
	writeUserTheme(t, "single.yml", userThemeColors)

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"bare", "full", "plain-image", "single", "walled"}, userThemeNames(themes))
	require.Empty(t, problems)
	require.Empty(t, notices)

	// The theme keeps the name of the image and how much it is dimmed.
	walled := userTheme(t, "walled")
	require.Equal(t, "wallpaper.jpg", walled.BackgroundImage)
	require.Equal(t, 60, walled.BackgroundDim)
	require.NotEmpty(t, walled.Hash)
	for _, name := range []string{"bare", "single"} {
		require.Empty(t, userTheme(t, name).BackgroundImage, name)
	}

	// A page gets a rule with the address the image is served at. The image
	// is served under it like any asset.
	page, known := GetPageThemeFor("walled")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Background: backgroundRule("/theme/walled/"+walled.Hash+"/assets/wallpaper.jpg", "60")}, page)
	content, asset, served := readThemeAsset(t, "walled", walled.Hash, "wallpaper.jpg")
	require.True(t, served)
	require.Equal(t, assetJPEG, content)
	require.Equal(t, "image/jpeg", asset.Type)

	// Without a dim the image is the only layer.
	plain := userTheme(t, "plain-image")
	page, known = GetPageThemeFor("plain-image")
	require.True(t, known)
	require.Equal(t, backgroundRule("/theme/plain-image/"+plain.Hash+"/assets/Wall.PNG", ""), page.Background)

	// The rule is the same for light and for dark: the color over the image
	// is that of the token, which the mode sets. It stands beside the
	// stylesheet, under the same hash.
	full := userTheme(t, "full")
	page, known = GetPageThemeFor("full")
	require.True(t, known)
	require.Empty(t, page.BodyStyle)
	require.NotEmpty(t, page.Rules)
	require.Equal(t, "/theme/full/"+full.Hash+"/theme.css", page.Stylesheet)
	require.Equal(t, backgroundRule("/theme/full/"+full.Hash+"/assets/a.png", "100"), page.Background)

	// A theme without a background image has no rule, and neither have a
	// bundled theme and the custom colors.
	for _, name := range []string{"bare", "single", "blackboard"} {
		page, known = GetPageThemeFor(name)
		require.True(t, known, name)
		require.Empty(t, page.Background, name)
	}
	require.Empty(t, GetPageTheme().Background)
	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.Empty(t, GetPageTheme().Background)

	// The pages take the rule of the selected theme, and none where the
	// themes of the directory are left out.
	SetTheme("walled")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Background: backgroundRule("/theme/walled/"+walled.Hash+"/assets/wallpaper.jpg", "60")}, GetPageTheme())
	require.Equal(t, PageTheme{}, GetPageThemeWithoutDirectory())
	SetTheme("bare")
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())
}

// A background image that is not among the images of the assets leaves the
// theme out of use: a theme is not applied without its image.
func TestThemeBackgroundNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	// The image is not there.
	writeThemeDirectory(t, "missing", userThemeColors+"background:\n  image: assets/none.png\n")
	writeThemeAsset(t, "missing", "a.png", assetPNG)
	writeThemeDirectory(t, "no-assets", userThemeColors+userThemeBackground)
	writeThemeDirectory(t, "other-case", userThemeColors+"background:\n  image: assets/A.png\n")
	writeThemeAsset(t, "other-case", "a.png", assetPNG)
	writeThemeDirectory(t, "beside", userThemeColors+userThemeBackground)
	writeUserTheme(t, filepath.Join("beside", "wallpaper.jpg"), assetJPEG)
	writeThemeDirectory(t, "left-alone", userThemeColors+"background:\n  image: assets/.hidden.png\n")
	writeThemeAsset(t, "left-alone", ".hidden.png", assetPNG)
	// A theme that is a single file has no assets.
	writeUserTheme(t, "single.yml", userThemeColors+userThemeBackground)
	makeThemesSubdirectory(t, "single")
	writeThemeAsset(t, "single", "wallpaper.jpg", assetJPEG)
	// The background is not written in the accepted form.
	writeThemeDirectory(t, "form", userThemeColors+"background:\n  image: wallpaper.jpg\n")
	writeThemeAsset(t, "form", "wallpaper.jpg", assetJPEG)
	writeThemeDirectory(t, "font", userThemeColors+"background:\n  image: assets/brand.woff2\n")
	writeThemeAsset(t, "font", "brand.woff2", assetWOFF2)
	writeThemeDirectory(t, "dim", userThemeColors+"background:\n  image: assets/a.png\n  dim: 101\n")
	writeThemeAsset(t, "dim", "a.png", assetPNG)
	writeThemeDirectory(t, "fraction", userThemeColors+"background:\n  image: assets/a.png\n  dim: 0.6\n")
	writeThemeAsset(t, "fraction", "a.png", assetPNG)
	writeUserTheme(t, "shape.yml", userThemeColors+"background: assets/a.png\n")
	// The files are looked at in this order: theme.yml, the assets, the
	// background image, the stylesheet.
	writeThemeDirectory(t, "asset-first", userThemeColors+"background:\n  image: assets/a.png\n")
	writeThemeAsset(t, "asset-first", "a.png", "not an image")
	writeThemeDirectory(t, "image-first", userThemeColors+"background:\n  image: assets/none.png\n")
	writeThemeStylesheet(t, "image-first", "@import 'a.css';")
	// A part of the background for a later release is left out, and the
	// theme is in use.
	writeThemeDirectory(t, "later", userThemeColors+"background:\n  image: assets/a.png\n  size: contain\n")
	writeThemeAsset(t, "later", "a.png", assetPNG)

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"later"}, userThemeNames(themes))
	require.Equal(t, []ThemeNotice{{File: "later/theme.yml", Unused: model.ThemeFileUnused{Keys: []string{"background.size"}}}}, notices)
	syntax := -1
	for i, problem := range problems {
		if problem.File == "shape.yml" {
			require.Equal(t, ThemeProblemSyntax, problem.Reason)
			require.True(t, strings.HasPrefix(problem.Subject, "background: "), problem.Subject)
			syntax = i
		}
	}
	require.GreaterOrEqual(t, syntax, 0)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "missing/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/none.png"},
		{File: "no-assets/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"},
		{File: "other-case/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/A.png"},
		{File: "beside/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"},
		{File: "left-alone/theme.yml", Reason: ThemeProblemBackground, Subject: "background.image"},
		{File: "single.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"},
		{File: "form/theme.yml", Reason: ThemeProblemBackground, Subject: "background.image"},
		{File: "font/theme.yml", Reason: ThemeProblemBackground, Subject: "background.image"},
		{File: "dim/theme.yml", Reason: ThemeProblemBackground, Subject: "background.dim"},
		{File: "fraction/theme.yml", Reason: ThemeProblemBackground, Subject: "background.dim"},
		{File: "asset-first/assets/a.png", Reason: ThemeProblemAssetContent},
		{File: "image-first/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/none.png"},
	}, append(problems[:syntax:syntax], problems[syntax+1:]...))

	// None of them can be selected, and nothing of them reaches a page.
	for _, problem := range problems {
		name, _, _ := strings.Cut(strings.TrimSuffix(problem.File, ".yml"), "/")
		require.False(t, IsUserTheme(name), name)
		require.False(t, IsKnownTheme(name), name)
		_, known := GetPageThemeFor(name)
		require.False(t, known, name)
		SetTheme(name)
		require.Equal(t, PageTheme{}, GetPageTheme(), name)
	}
}

// The pages follow the background image of the selected theme as they follow
// its other files.
func TestSelectedThemeFollowsItsBackground(t *testing.T) {
	userThemeDirectory(t, "walled")
	writeThemeDirectory(t, "walled", userThemeColors+userThemeBackground)
	writeThemeAsset(t, "walled", "wallpaper.jpg", assetJPEG+"first")
	UpdatePagePalettes()
	rule := func() template.CSS {
		t.Helper()
		theme := userTheme(t, "walled")
		return backgroundRule("/theme/walled/"+theme.Hash+"/assets/"+theme.BackgroundImage, "60")
	}
	first := GetPageTheme()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Background: rule()}, first)

	// While the files keep their state the directory is not read.
	writeUserTheme(t, "added.yml", userThemeColors)
	for range 3 {
		require.Equal(t, first, GetPageTheme())
	}
	require.False(t, IsUserTheme("added"))

	// An image that is edited has another address on the next page.
	image := filepath.Join("walled", "assets", "wallpaper.jpg")
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, image), []byte(assetJPEG+"second, longer"), 0600))
	setUserThemeAge(t, image, 2*time.Minute)
	second := GetPageTheme()
	require.NotEqual(t, first.Background, second.Background)
	require.Equal(t, rule(), second.Background)
	require.True(t, IsUserTheme("added"))

	// The theme file is followed as before: another dim, another image, and
	// no background.
	writeThemeDirectory(t, "walled", userThemeColors+"background:\n  image: assets/wallpaper.jpg\n  dim: 20\n")
	setUserThemeAge(t, filepath.Join("walled", userThemeManifest), 2*time.Minute)
	require.Equal(t, strings.Replace(string(second.Background), " 60%", " 20%", 2), string(GetPageTheme().Background))
	writeThemeAsset(t, "walled", "other.png", assetPNG)
	writeThemeDirectory(t, "walled", userThemeColors+"background:\n  image: assets/other.png\n")
	setUserThemeAge(t, filepath.Join("walled", userThemeManifest), 3*time.Minute)
	// The page is asked first: it is what reads the directory again.
	other := GetPageTheme().Background
	require.Equal(t, backgroundRule("/theme/walled/"+userTheme(t, "walled").Hash+"/assets/other.png", ""), other)
	writeThemeDirectory(t, "walled", userThemeColors)
	setUserThemeAge(t, filepath.Join("walled", userThemeManifest), 4*time.Minute)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle}, GetPageTheme())

	// An image that goes away leaves the selection without a theme, and
	// putting it back brings the theme back.
	writeThemeDirectory(t, "walled", userThemeColors+userThemeBackground)
	setUserThemeAge(t, filepath.Join("walled", userThemeManifest), 5*time.Minute)
	require.NotEmpty(t, GetPageTheme().Background)
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, image)))
	setUserThemeAge(t, filepath.Join("walled", "assets"), 5*time.Minute)
	require.Equal(t, PageTheme{}, GetPageTheme())
	require.False(t, IsKnownTheme("walled"))
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "walled/theme.yml", Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"}}, problems)
	writeThemeAsset(t, "walled", "wallpaper.jpg", assetJPEG)
	back := GetPageTheme()
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle, Background: rule()}, back)
}

// A theme file that is stored in the themes directory is a theme of one file,
// which has no assets: one that names a background image is not stored.
func TestStoreUserThemeWithABackground(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	err := StoreUserTheme("added", []byte(userThemeColors+userThemeBackground), false)
	require.ErrorIs(t, err, model.ErrThemeBackgroundAsset)
	var rejection *model.ThemeFileError
	require.True(t, errors.As(err, &rejection))
	require.Equal(t, "assets/wallpaper.jpg", rejection.Subject)
	require.Equal(t, ThemeProblem{Reason: ThemeProblemBackgroundAsset, Subject: "assets/wallpaper.jpg"}, ThemeFileProblem(err))
	_, statErr := os.Stat(filepath.Join(UserThemeDirectory, "added.yml"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
	require.False(t, IsUserTheme("added"))

	// A background in a form that is not accepted is refused for that.
	err = StoreUserTheme("added", []byte(userThemeColors+"background:\n  image: wallpaper.jpg\n"), false)
	require.ErrorIs(t, err, model.ErrThemeBackground)
	require.Equal(t, ThemeProblem{Reason: ThemeProblemBackground, Subject: "background.image"}, ThemeFileProblem(err))

	// Without a background the same file is stored, also where it has a
	// part of a background for a later release and nothing else of one.
	require.NoError(t, StoreUserTheme("added", []byte(userThemeColors), false))
	require.True(t, IsUserTheme("added"))
	require.NoError(t, StoreUserTheme("empty", []byte(userThemeColors+"background:\n"), false))
	require.True(t, IsUserTheme("empty"))

	// Replacing a file is held to the same.
	require.ErrorIs(t, StoreUserTheme("added", []byte(userThemeColors+userThemeBackground), true), model.ErrThemeBackgroundAsset)
	raw, readErr := os.ReadFile(filepath.Join(UserThemeDirectory, "added.yml"))
	require.NoError(t, readErr)
	require.Equal(t, userThemeColors, string(raw))
}
