package define

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// How files of the kinds a theme can bring start.
const (
	assetPNG   = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	assetJPEG  = "\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01"
	assetWOFF2 = "wOF2\x00\x01\x00\x00\x00\x00\x00\x00"
)

const assetStylesheetText = ".app-item { background: url(assets/a.png) }\n"

// writeThemeAsset writes a file into the assets of a theme. The file and the
// directory get a modification time well before the settle window, as they
// have when nobody is working on the theme.
func writeThemeAsset(t *testing.T, theme, name, content string) {
	t.Helper()
	assets := filepath.Join(theme, model.ThemeAssetDirectory)
	require.NoError(t, os.MkdirAll(filepath.Join(UserThemeDirectory, assets), 0700))
	writeUserTheme(t, filepath.Join(assets, name), content)
	setUserThemeAge(t, assets, time.Minute)
}

// readThemeAsset reads a file of the assets by the address it is served at.
func readThemeAsset(t *testing.T, theme, hash, name string) (string, model.ThemeAsset, bool) {
	t.Helper()
	file, asset, known := OpenThemeAsset(theme, hash, name)
	if !known {
		require.Nil(t, file)
		return "", asset, false
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(content), asset, true
}

func userTheme(t *testing.T, name string) model.Theme {
	t.Helper()
	themes, _, _ := UserThemes()
	for _, theme := range themes {
		if theme.Name == name {
			return theme
		}
	}
	require.Failf(t, "not in use", "theme %q", name)
	return model.Theme{}
}

func TestThemeAssetsAreRead(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	stylesheet := "@font-face { font-family: Brand; src: url(\"assets/Brand-Regular.woff2\") format(\"woff2\") }\n" +
		"body { font-family: Brand, sans-serif; background: url(assets/paper.png) }\n"
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", stylesheet)
	writeThemeAsset(t, "styled", "paper.png", assetPNG+"paper")
	writeThemeAsset(t, "styled", "Brand-Regular.woff2", assetWOFF2+"brand")
	// A file the stylesheet does not refer to, with its extension in
	// upper case.
	writeThemeAsset(t, "styled", "photo.JPG", assetJPEG)
	// Entries that are not meant as assets are left alone without a remark:
	// hidden ones, files of other kinds, directories, and what a file manager
	// or a storage device puts there.
	for _, name := range []string{".DS_Store", ".hidden.png", "._paper.png", "notes.txt", "OFL.txt", "logo.svg", "Thumbs.db", "theme.css", "theme.yml", "paper.png~", "paper.png.bak", "png"} {
		writeThemeAsset(t, "styled", name, assetPNG)
	}
	makeThemesSubdirectory(t, filepath.Join("styled", "assets", "@eaDir"))
	makeThemesSubdirectory(t, filepath.Join("styled", "assets", "more"))
	writeUserTheme(t, filepath.Join("styled", "assets", "more", "nested.png"), assetPNG)
	require.NoError(t, os.Symlink("paper.png", filepath.Join(UserThemeDirectory, "styled", "assets", "link")))
	// Files of these kinds elsewhere in the directory of the theme are not
	// assets.
	writeUserTheme(t, filepath.Join("styled", "preview.png"), assetPNG)
	setUserThemeAge(t, filepath.Join("styled", "assets"), time.Minute)

	// A theme can have assets and no stylesheet, a stylesheet and no assets,
	// an empty directory of assets, and a file where that directory would be.
	writeThemeDirectory(t, "only-assets", userThemeColors)
	writeThemeAsset(t, "only-assets", "a.png", assetPNG)
	writeThemeDirectory(t, "only-style", userThemeColors)
	writeThemeStylesheet(t, "only-style", userThemeStylesheetText)
	writeThemeDirectory(t, "empty", userThemeColors)
	makeThemesSubdirectory(t, filepath.Join("empty", "assets"))
	writeThemeDirectory(t, "file", userThemeColors)
	writeUserTheme(t, filepath.Join("file", "assets"), assetPNG)
	// Assets belong to a theme that is a directory: beside no theme.yml
	// they are not read, whatever they hold.
	writeUserTheme(t, "plain.yml", userThemeColors)
	makeThemesSubdirectory(t, "plain")
	writeThemeAsset(t, "plain", "a b.png", "not an image")

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"empty", "file", "only-assets", "only-style", "plain", "styled"}, userThemeNames(themes))
	require.Empty(t, problems)
	require.Empty(t, notices)

	// The assets are the images and fonts of the directory, in the order of
	// their names, with the type each is served as and its state.
	styled := userTheme(t, "styled")
	names := make([]string, 0, len(styled.Assets))
	for _, asset := range styled.Assets {
		names = append(names, asset.Name+" "+asset.Type)
		info, err := os.Stat(filepath.Join(UserThemeDirectory, "styled", "assets", asset.Name))
		require.NoError(t, err)
		require.Equal(t, info.Size(), asset.Size, asset.Name)
		require.True(t, info.ModTime().Equal(asset.ModTime), asset.Name)
	}
	require.Equal(t, []string{"Brand-Regular.woff2 font/woff2", "paper.png image/png", "photo.JPG image/jpeg"}, names)
	require.Equal(t, stylesheet, styled.Stylesheet)
	for _, name := range []string{"empty", "file", "only-style", "plain"} {
		require.Empty(t, userTheme(t, name).Assets, name)
	}
	require.Empty(t, userTheme(t, "empty").Hash)
	require.Empty(t, userTheme(t, "file").Hash)

	// The hash of the addresses covers the stylesheet and the assets. A
	// theme without assets has the hash of the content of its stylesheet.
	hash := regexp.MustCompile(`^[0-9a-f]{16}$`)
	require.Regexp(t, hash, styled.Hash)
	sum := sha256.Sum256([]byte(stylesheet))
	require.NotEqual(t, hex.EncodeToString(sum[:8]), styled.Hash)
	sum = sha256.Sum256([]byte(userThemeStylesheetText))
	require.Equal(t, hex.EncodeToString(sum[:8]), userTheme(t, "only-style").Hash)
	onlyAssets := userTheme(t, "only-assets")
	require.Regexp(t, hash, onlyAssets.Hash)
	require.Empty(t, onlyAssets.Stylesheet)

	// The stylesheet is linked under that hash, and a theme without a
	// stylesheet links none.
	page, known := GetPageThemeFor("styled")
	require.True(t, known)
	require.Equal(t, "/theme/styled/"+styled.Hash+"/theme.css", page.Stylesheet)
	page, known = GetPageThemeFor("only-assets")
	require.True(t, known)
	require.Empty(t, page.Stylesheet)

	// Reading the directory again in the same state gives the same hash.
	ReloadUserThemes()
	require.Equal(t, styled.Hash, userTheme(t, "styled").Hash)
	require.Equal(t, onlyAssets.Hash, userTheme(t, "only-assets").Hash)
}

// A file of the assets is handed out by the name of the theme, the hash and
// the name of the file of its address, and by nothing else.
func TestThemeAssetIsOpenedByItsAddress(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", assetStylesheetText)
	writeThemeAsset(t, "styled", "a.png", assetPNG+"first")
	writeThemeAsset(t, "styled", "font.woff2", assetWOFF2)
	writeThemeAsset(t, "styled", "notes.txt", "notes")
	writeThemeAsset(t, "styled", "logo.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>")
	writeThemeAsset(t, "styled", ".hidden.png", assetPNG)
	makeThemesSubdirectory(t, filepath.Join("styled", "assets", "more"))
	writeUserTheme(t, filepath.Join("styled", "assets", "more", "nested.png"), assetPNG)
	setUserThemeAge(t, filepath.Join("styled", "assets"), time.Minute)
	writeThemeDirectory(t, "other", userThemeColors)
	writeThemeAsset(t, "other", "b.png", assetPNG+"other")
	writeThemeDirectory(t, "bare", userThemeColors)
	writeUserTheme(t, "plain.yml", userThemeColors)
	ReloadUserThemes()
	_, problems, _ := UserThemes()
	require.Empty(t, problems)
	hash, other := userTheme(t, "styled").Hash, userTheme(t, "other").Hash

	content, asset, known := readThemeAsset(t, "styled", hash, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"first", content)
	require.Equal(t, "a.png", asset.Name)
	require.Equal(t, "image/png", asset.Type)
	require.Equal(t, int64(len(content)), asset.Size)
	content, asset, known = readThemeAsset(t, "styled", hash, "font.woff2")
	require.True(t, known)
	require.Equal(t, assetWOFF2, content)
	require.Equal(t, "font/woff2", asset.Type)
	content, _, known = readThemeAsset(t, "other", other, "b.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"other", content)

	for _, wrong := range [][3]string{
		// Another hash, and the hash of another theme.
		{"styled", "", "a.png"}, {"styled", strings.Repeat("0", 16), "a.png"}, {"styled", strings.ToUpper(hash), "a.png"},
		{"styled", hash + "0", "a.png"}, {"styled", other, "a.png"}, {"other", hash, "b.png"},
		// A file of another theme, and a name that is no theme in use.
		{"styled", hash, "b.png"}, {"other", other, "a.png"}, {"bare", "", "a.png"}, {"bare", hash, "a.png"},
		{"plain", "", "a.png"}, {"missing", hash, "a.png"}, {"", hash, "a.png"}, {"blackboard", "", "a.png"},
		{model.ThemeCustomName, "", "a.png"}, {"../styled", hash, "a.png"}, {"styled/assets", hash, "a.png"},
		// Files of the directory that are not assets, and names that lead
		// elsewhere.
		{"styled", hash, "notes.txt"}, {"styled", hash, "logo.svg"}, {"styled", hash, ".hidden.png"}, {"styled", hash, "more"},
		{"styled", hash, "more/nested.png"}, {"styled", hash, "nested.png"}, {"styled", hash, ""}, {"styled", hash, "."},
		{"styled", hash, ".."}, {"styled", hash, "../theme.yml"}, {"styled", hash, "../theme.css"}, {"styled", hash, "../../plain.yml"},
		{"styled", hash, "../../../config.yml"}, {"styled", hash, "/etc/hosts"}, {"styled", hash, "a.png/"}, {"styled", hash, "./a.png"},
		{"styled", hash, "A.PNG"}, {"styled", hash, "a.PNG"}, {"styled", hash, "a.png\x00"}, {"styled", hash, "a.png "},
		{"styled", hash, "a%2Epng"}, {"styled", hash, `..\theme.yml`}, {"styled", hash, "../../other/assets/b.png"},
	} {
		_, _, known = readThemeAsset(t, wrong[0], wrong[1], wrong[2])
		require.False(t, known, "%q", wrong)
	}

	// The address stands for the file in the state it was read in. A file
	// that has another size or another modification time since is not handed
	// out under it, and has another address once the directory is read again.
	path := filepath.Join("styled", "assets", "a.png")
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, path), []byte(assetPNG+"second, longer"), 0600))
	setUserThemeAge(t, path, time.Minute)
	_, _, known = readThemeAsset(t, "styled", hash, "a.png")
	require.False(t, known, "another size")
	_, _, known = readThemeAsset(t, "styled", hash, "font.woff2")
	require.True(t, known, "a file that has not changed is handed out")
	ReloadUserThemes()
	second := userTheme(t, "styled").Hash
	require.NotEqual(t, hash, second)
	_, _, known = readThemeAsset(t, "styled", hash, "a.png")
	require.False(t, known, "the earlier address")
	_, _, known = readThemeAsset(t, "styled", hash, "font.woff2")
	require.False(t, known, "the earlier address")
	content, _, known = readThemeAsset(t, "styled", second, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"second, longer", content)

	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, path), []byte(assetPNG+"third, longer!"), 0600))
	setUserThemeAge(t, path, 2*time.Minute)
	_, _, known = readThemeAsset(t, "styled", second, "a.png")
	require.False(t, known, "the same size and another modification time")
	ReloadUserThemes()
	third := userTheme(t, "styled").Hash
	require.NotEqual(t, second, third)
	content, _, known = readThemeAsset(t, "styled", third, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"third, longer!", content)

	// Another size with the modification time the file had, as a copy that
	// keeps the times of a file leaves it.
	absolute := filepath.Join(UserThemeDirectory, path)
	before, err := os.Stat(absolute)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(absolute, []byte(assetPNG+"fourth"), 0600))
	require.NoError(t, os.Chtimes(absolute, before.ModTime(), before.ModTime()))
	_, _, known = readThemeAsset(t, "styled", third, "a.png")
	require.False(t, known, "another size and the same modification time")
	ReloadUserThemes()
	latest := userTheme(t, "styled").Hash
	require.NotEqual(t, third, latest)
	content, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"fourth", content)

	// A file that is replaced by a link or a directory, or removed, is not
	// handed out, also not where the link leads to a file of the same state.
	require.NoError(t, os.Rename(absolute, absolute+".kept"))
	require.NoError(t, os.Symlink("a.png.kept", absolute))
	_, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.False(t, known, "a link")
	require.NoError(t, os.Remove(absolute))
	require.NoError(t, os.Mkdir(absolute, 0700))
	_, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.False(t, known, "a directory")
	require.NoError(t, os.Remove(absolute))
	_, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.False(t, known, "removed")
	require.NoError(t, os.Rename(absolute+".kept", absolute))
	_, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.True(t, known, "the file as it was read")

	// Without the themes directory nothing is handed out.
	require.NoError(t, os.Rename(UserThemeDirectory, UserThemeDirectory+".kept"))
	_, _, known = readThemeAsset(t, "styled", latest, "a.png")
	require.False(t, known)
	require.NoError(t, os.Rename(UserThemeDirectory+".kept", UserThemeDirectory))
}

// A file that is meant as an asset and not accepted leaves the theme out of
// use: a theme is not applied without a file it brings.
func TestThemeAssetsNotAccepted(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	outside := filepath.Join(t.TempDir(), "outside.png")
	require.NoError(t, os.WriteFile(outside, []byte(assetPNG), 0600))
	long := strings.Repeat("a", 61) + ".png"
	for theme, files := range map[string]map[string]string{
		"space":     {"my photo.png": assetPNG},
		"hyphen":    {"-a.png": assetPNG},
		"long":      {long: assetPNG},
		"letters":   {"图.png": assetPNG},
		"percent":   {"a%20b.png": assetPNG},
		"content":   {"a.png": "<!doctype html><script>alert(1)</script>"},
		"other":     {"a.png": assetJPEG},
		"empty":     {"a.gif": ""},
		"short":     {"a.webp": "RIFF"},
		"too-large": {"a.png": assetPNG + strings.Repeat("\x00", model.MaximumThemeAssetBytes-len(assetPNG)+1)},
		// The first file by its name is the one that is reported.
		"first": {"a.png": "not an image", "b b.png": assetPNG, "c.png": assetPNG},
	} {
		writeThemeDirectory(t, theme, userThemeColors)
		for name, content := range files {
			writeThemeAsset(t, theme, name, content)
		}
	}
	// Themes that are in use: a file of the largest size, a name of the
	// greatest length, and as many files as a theme can bring.
	writeThemeDirectory(t, "at-limit", userThemeColors)
	writeThemeAsset(t, "at-limit", "a.png", assetPNG+strings.Repeat("\x00", model.MaximumThemeAssetBytes-len(assetPNG)))
	writeThemeAsset(t, "at-limit", long[1:], assetPNG)
	writeThemeDirectory(t, "full", userThemeColors)
	writeThemeDirectory(t, "too-many", userThemeColors)
	for i := range model.MaximumThemeAssets + 1 {
		if i < model.MaximumThemeAssets {
			writeThemeAsset(t, "full", fmt.Sprintf("%02d.png", i), assetPNG)
		}
		writeThemeAsset(t, "too-many", fmt.Sprintf("%02d.png", i), assetPNG)
	}
	// Files that are left alone do not count.
	for i := range 8 {
		writeThemeAsset(t, "full", fmt.Sprintf("%02d.txt", i), "notes")
	}

	// A file of the assets is a regular file of their directory, and the
	// directory is one of the theme.
	for _, theme := range []string{"folder", "link-inside", "link-outside", "link-nowhere", "assets-link"} {
		writeThemeDirectory(t, theme, userThemeColors)
	}
	makeThemesSubdirectory(t, filepath.Join("folder", "assets"))
	makeThemesSubdirectory(t, filepath.Join("folder", "assets", "a.png"))
	writeThemeAsset(t, "link-inside", "b.png", assetPNG)
	require.NoError(t, os.Symlink("b.png", filepath.Join(UserThemeDirectory, "link-inside", "assets", "a.png")))
	makeThemesSubdirectory(t, filepath.Join("link-outside", "assets"))
	require.NoError(t, os.Symlink(outside, filepath.Join(UserThemeDirectory, "link-outside", "assets", "a.png")))
	makeThemesSubdirectory(t, filepath.Join("link-nowhere", "assets"))
	require.NoError(t, os.Symlink("missing.png", filepath.Join(UserThemeDirectory, "link-nowhere", "assets", "a.png")))
	require.NoError(t, os.Symlink(filepath.Join("..", "full", "assets"), filepath.Join(UserThemeDirectory, "assets-link", "assets")))

	// What the stylesheet refers to has to be among the assets.
	for theme, stylesheet := range map[string]string{
		"missing":    "a { color: red }\n.app-item { background: url(assets/b.png) }\n",
		"not-served": ".app-item { background: url(assets/logo.svg) }\n",
		"hidden":     ".app-item { background: url(assets/.hidden.png) }\n",
		"nested":     ".app-item { background: url(assets/more/a.png) }\n",
		"case":       ".app-item { background: url(assets/A.png) }\n",
		"beside":     ".app-item { background: url(preview.png) }\n",
	} {
		writeThemeDirectory(t, theme, userThemeColors)
		writeThemeStylesheet(t, theme, stylesheet)
		writeThemeAsset(t, theme, "a.png", assetPNG)
		writeThemeAsset(t, theme, "logo.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"/>")
		writeThemeAsset(t, theme, ".hidden.png", assetPNG)
		writeUserTheme(t, filepath.Join(theme, "preview.png"), assetPNG)
	}
	makeThemesSubdirectory(t, filepath.Join("nested", "assets", "more"))
	writeUserTheme(t, filepath.Join("nested", "assets", "more", "a.png"), assetPNG)

	// The files are looked at in this order: theme.yml, the assets, the
	// stylesheet. What comes after the first that is not accepted is not
	// looked at.
	writeThemeDirectory(t, "file-first", userThemeColors+"  radius: 4em\n")
	writeThemeAsset(t, "file-first", "a.png", "not an image")
	writeThemeDirectory(t, "asset-first", userThemeColors)
	writeThemeAsset(t, "asset-first", "a.png", "not an image")
	writeThemeStylesheet(t, "asset-first", "@import 'a.css';")

	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Equal(t, []string{"at-limit", "full"}, userThemeNames(themes))
	require.Len(t, userTheme(t, "at-limit").Assets, 2)
	require.Len(t, userTheme(t, "full").Assets, model.MaximumThemeAssets)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "space/assets/my photo.png", Reason: ThemeProblemAssetName},
		{File: "hyphen/assets/-a.png", Reason: ThemeProblemAssetName},
		{File: "long/assets/" + long, Reason: ThemeProblemAssetName},
		{File: "letters/assets/图.png", Reason: ThemeProblemAssetName},
		{File: "percent/assets/a%20b.png", Reason: ThemeProblemAssetName},
		{File: "content/assets/a.png", Reason: ThemeProblemAssetContent},
		{File: "other/assets/a.png", Reason: ThemeProblemAssetContent},
		{File: "empty/assets/a.gif", Reason: ThemeProblemAssetContent},
		{File: "short/assets/a.webp", Reason: ThemeProblemAssetContent},
		{File: "too-large/assets/a.png", Reason: ThemeProblemAssetSize},
		{File: "first/assets/a.png", Reason: ThemeProblemAssetContent},
		{File: "too-many/assets", Reason: ThemeProblemAssetLimit},
		{File: "folder/assets/a.png", Reason: ThemeProblemType},
		{File: "link-inside/assets/a.png", Reason: ThemeProblemLink},
		{File: "link-outside/assets/a.png", Reason: ThemeProblemLink},
		{File: "link-nowhere/assets/a.png", Reason: ThemeProblemLink},
		{File: "assets-link/assets", Reason: ThemeProblemLink},
		{File: "missing/theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 2: url(assets/b.png)"},
		{File: "not-served/theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 1: url(assets/logo.svg)"},
		{File: "hidden/theme.css", Reason: ThemeProblemStyleURL, Subject: "line 1: url()"},
		{File: "nested/theme.css", Reason: ThemeProblemStyleURL, Subject: "line 1: url()"},
		{File: "case/theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 1: url(assets/A.png)"},
		{File: "beside/theme.css", Reason: ThemeProblemStyleURL, Subject: "line 1: url()"},
		{File: "file-first/theme.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
		{File: "asset-first/assets/a.png", Reason: ThemeProblemAssetContent},
	}, problems)

	// None of them can be selected, nothing of them reaches a page, and
	// none of their files is handed out.
	for _, problem := range problems {
		name, _, _ := strings.Cut(problem.File, "/")
		require.False(t, IsUserTheme(name), name)
		require.False(t, IsKnownTheme(name), name)
		_, known := GetPageThemeFor(name)
		require.False(t, known, name)
		SetTheme(name)
		require.Equal(t, PageTheme{}, GetPageTheme(), name)
		for _, file := range []string{"a.png", "b.png", "c.png", "00.png"} {
			_, _, known = readThemeAsset(t, name, "", file)
			require.False(t, known, name)
		}
	}
}

func TestUnreadableThemeAssets(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not apply to this user")
	}
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "locked-file", userThemeColors)
	writeThemeAsset(t, "locked-file", "a.png", assetPNG)
	file := filepath.Join(UserThemeDirectory, "locked-file", "assets", "a.png")
	require.NoError(t, os.Chmod(file, 0))
	t.Cleanup(func() { _ = os.Chmod(file, 0600) })
	writeThemeDirectory(t, "locked-directory", userThemeColors)
	writeThemeAsset(t, "locked-directory", "a.png", assetPNG)
	directory := filepath.Join(UserThemeDirectory, "locked-directory", "assets")
	require.NoError(t, os.Chmod(directory, 0))
	t.Cleanup(func() { _ = os.Chmod(directory, 0700) })

	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Empty(t, themes)
	require.ElementsMatch(t, []ThemeProblem{
		{File: "locked-file/assets/a.png", Reason: ThemeProblemRead},
		{File: "locked-directory/assets", Reason: ThemeProblemRead},
	}, problems)
}

// The pages follow the assets of the selected theme as they follow its theme
// file and its stylesheet: a change of a file gives the stylesheet another
// address, so that nothing a browser has kept stands for the file.
func TestSelectedThemeFollowsItsAssets(t *testing.T) {
	userThemeDirectory(t, "styled")
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", assetStylesheetText)
	writeThemeAsset(t, "styled", "a.png", assetPNG+"first")
	UpdatePagePalettes()
	asset := filepath.Join("styled", "assets", "a.png")
	assets := filepath.Join("styled", "assets")
	hash := func(address string) string { return strings.Split(address, "/")[3] }
	served := func(address, name string) (string, bool) {
		t.Helper()
		content, _, known := readThemeAsset(t, "styled", hash(address), name)
		return content, known
	}
	// A reading of the themes directory that finds the theme changed gives
	// it another generation.
	generation := func() uint64 {
		_, _, current := ThemeRenderSnapshot()
		return current
	}

	first := GetPageTheme().Stylesheet
	require.NotEmpty(t, first)
	content, known := served(first, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"first", content)

	// While the files keep their state the directory is not read.
	before := generation()
	writeUserTheme(t, "added.yml", userThemeColors)
	for range 3 {
		require.Equal(t, first, GetPageTheme().Stylesheet)
	}
	require.False(t, IsUserTheme("added"))
	require.Equal(t, before, generation())

	// A file that is edited in place has the state of its directory as it
	// was. The next page has another address, and the file is no longer
	// handed out under the earlier one.
	directory, err := os.Stat(filepath.Join(UserThemeDirectory, assets))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, asset), []byte(assetPNG+"second, longer"), 0600))
	setUserThemeAge(t, asset, time.Minute)
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, assets), directory.ModTime(), directory.ModTime()))
	second := GetPageTheme().Stylesheet
	require.NotEqual(t, first, second)
	require.True(t, IsUserTheme("added"), "the directory was read again")
	_, known = served(first, "a.png")
	require.False(t, known)
	content, known = served(second, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"second, longer", content)

	// An edit that keeps the size is seen by the modification time.
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, asset), []byte(assetPNG+"third, longer!"), 0600))
	setUserThemeAge(t, asset, 2*time.Minute)
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, assets), directory.ModTime(), directory.ModTime()))
	third := GetPageTheme().Stylesheet
	require.NotEqual(t, second, third)
	content, known = served(third, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"third, longer!", content)

	// An edit that keeps the modification time, as a copy that keeps the
	// times of a file does, is seen by the size.
	stated, err := os.Stat(filepath.Join(UserThemeDirectory, asset))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, asset), []byte(assetPNG+"of another size"), 0600))
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, asset), stated.ModTime(), stated.ModTime()))
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, assets), directory.ModTime(), directory.ModTime()))
	resized := GetPageTheme().Stylesheet
	require.NotEqual(t, third, resized)
	content, known = served(resized, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"of another size", content)
	third = resized

	// A file that was edited a moment ago is read again by every page,
	// because a second edit within the same step of the clock of a
	// filesystem could keep both its size and its modification time.
	setUserThemeAge(t, asset, 0)
	fourth := GetPageTheme().Stylesheet
	require.NotEqual(t, third, fourth)
	writeUserTheme(t, "later.yml", userThemeColors)
	require.Equal(t, fourth, GetPageTheme().Stylesheet)
	require.True(t, IsUserTheme("later"), "the directory was read again")
	setUserThemeAge(t, asset, 3*time.Minute)
	fourth = GetPageTheme().Stylesheet

	// A file that is added has another address for the stylesheet too, also
	// where the stylesheet does not refer to it: the address stands for all
	// of them.
	writeThemeAsset(t, "styled", "b.png", assetPNG+"b")
	fifth := GetPageTheme().Stylesheet
	require.NotEqual(t, fourth, fifth)
	content, known = served(fifth, "b.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"b", content)
	// A file that is renamed keeps its size and its modification time, and
	// has another address by its name.
	require.NoError(t, os.Rename(filepath.Join(UserThemeDirectory, assets, "b.png"), filepath.Join(UserThemeDirectory, assets, "d.png")))
	setUserThemeAge(t, assets, 90*time.Second)
	renamed := GetPageTheme().Stylesheet
	require.NotEqual(t, fifth, renamed)
	_, known = served(renamed, "b.png")
	require.False(t, known)
	content, known = served(renamed, "d.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"b", content)
	fifth = renamed
	// An entry that is left alone changes the state of the directory, which
	// is read again, and leaves the address as it is.
	writeThemeAsset(t, "styled", "notes.txt", "notes")
	require.Equal(t, fifth, GetPageTheme().Stylesheet)
	_, known = served(fifth, "notes.txt")
	require.False(t, known)

	// A file that is not accepted leaves the selection without a theme: no
	// colors and no stylesheet.
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, asset), []byte("<!doctype html>"), 0600))
	setUserThemeAge(t, asset, 4*time.Minute)
	require.Equal(t, PageTheme{}, GetPageTheme())
	require.False(t, IsKnownTheme("styled"))
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "styled/assets/a.png", Reason: ThemeProblemAssetContent}}, problems)
	_, known = served(fifth, "d.png")
	require.False(t, known)

	// Correcting the file in place brings the theme back: a file that was
	// looked at is watched, also where it was not accepted.
	directory, err = os.Stat(filepath.Join(UserThemeDirectory, assets))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, asset), []byte(assetPNG+"corrected"), 0600))
	setUserThemeAge(t, asset, 5*time.Minute)
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, assets), directory.ModTime(), directory.ModTime()))
	sixth := GetPageTheme()
	require.Equal(t, userThemeStyle, sixth.BodyStyle)
	require.NotEmpty(t, sixth.Stylesheet)
	content, known = served(sixth.Stylesheet, "a.png")
	require.True(t, known)
	require.Equal(t, assetPNG+"corrected", content)

	// A file the stylesheet refers to that is removed or renamed leaves the
	// theme out of use, and putting it back brings the theme back.
	require.NoError(t, os.Rename(filepath.Join(UserThemeDirectory, asset), filepath.Join(UserThemeDirectory, assets, "c.png")))
	setUserThemeAge(t, assets, 6*time.Minute)
	require.Equal(t, PageTheme{}, GetPageTheme())
	_, problems, _ = UserThemes()
	require.Equal(t, []ThemeProblem{{File: "styled/theme.css", Reason: ThemeProblemStyleAsset, Subject: "line 1: url(assets/a.png)"}}, problems)
	require.NoError(t, os.Rename(filepath.Join(UserThemeDirectory, assets, "c.png"), filepath.Join(UserThemeDirectory, asset)))
	setUserThemeAge(t, assets, 7*time.Minute)
	require.Equal(t, sixth, GetPageTheme())

	// A directory of assets that goes away leaves the theme without them,
	// and one that appears is read.
	writeThemeStylesheet(t, "styled", userThemeStylesheetText)
	require.NoError(t, os.RemoveAll(filepath.Join(UserThemeDirectory, assets)))
	bare := GetPageTheme()
	require.Equal(t, userThemeStyle, bare.BodyStyle)
	require.Empty(t, userTheme(t, "styled").Assets)
	writeThemeAsset(t, "styled", "a.png", assetPNG)
	with := GetPageTheme()
	require.NotEqual(t, bare.Stylesheet, with.Stylesheet)
	require.Len(t, userTheme(t, "styled").Assets, 1)
}

// The directory of the assets and the files in it are watched with the
// theme.yml they belong to, and not beside a file or a directory that is no
// theme.
func TestThemeStampWatchesTheAssets(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "plain.yml", userThemeColors)
	makeThemesSubdirectory(t, "plain")
	writeThemeAsset(t, "plain", "a.png", assetPNG)
	stamp := statTheme("plain")
	require.True(t, stamp.file.exists)
	require.False(t, stamp.manifest.exists)
	require.False(t, stamp.assets.exists)

	writeThemeDirectory(t, "plain", userThemeColors)
	stamp = statTheme("plain")
	require.True(t, stamp.manifest.exists)
	require.True(t, stamp.assets.exists)
	require.Empty(t, stamp.files, "the files are those a reading of the directory looked at")

	now := time.Now()
	read := themeFileStamp{modTime: now.Add(-time.Minute), size: 100, exists: true}
	edited := themeFileStamp{modTime: now.Add(-time.Minute), size: 101, exists: true}
	for name, tc := range map[string]struct {
		read, found themeStamp
		current     bool
	}{
		"unchanged":             {themeStamp{manifest: read, assets: read}, themeStamp{manifest: read, assets: read}, true},
		"no assets":             {themeStamp{manifest: read}, themeStamp{manifest: read}, true},
		"assets appear":         {themeStamp{manifest: read}, themeStamp{manifest: read, assets: read}, false},
		"the directory changed": {themeStamp{manifest: read, assets: read}, themeStamp{manifest: read, assets: edited}, false},
		"the assets removed":    {themeStamp{manifest: read, assets: read}, themeStamp{manifest: read}, false},
	} {
		require.Equal(t, tc.current, tc.read.current(tc.found, now), name)
	}

	// The files a reading looked at are compared with the files as they are.
	path := filepath.Join(UserThemeDirectory, "plain", "assets", "a.png")
	info, err := os.Lstat(path)
	require.NoError(t, err)
	found := themeFileStamp{modTime: info.ModTime(), size: info.Size(), exists: true}
	for name, tc := range map[string]struct {
		files   []themeAssetStamp
		current bool
	}{
		"none":             {nil, true},
		"unchanged":        {[]themeAssetStamp{{name: "a.png", stamp: found}}, true},
		"another size":     {[]themeAssetStamp{{name: "a.png", stamp: themeFileStamp{modTime: found.modTime, size: found.size + 1, exists: true}}}, false},
		"another time":     {[]themeAssetStamp{{name: "a.png", stamp: themeFileStamp{modTime: found.modTime.Add(time.Second), size: found.size, exists: true}}}, false},
		"removed":          {[]themeAssetStamp{{name: "b.png", stamp: found}}, false},
		"one of two":       {[]themeAssetStamp{{name: "a.png", stamp: found}, {name: "b.png", stamp: found}}, false},
		"of another theme": {[]themeAssetStamp{{name: "a.png", stamp: found}}, true},
	} {
		require.Equal(t, tc.current, themeStamp{files: tc.files}.assetsCurrent("plain", now), name)
	}
	require.False(t, themeStamp{files: []themeAssetStamp{{name: "a.png", stamp: found}}}.assetsCurrent("other", now))
	// A file that was modified a moment ago is not trusted.
	setUserThemeAge(t, filepath.Join("plain", "assets", "a.png"), 0)
	info, err = os.Lstat(path)
	require.NoError(t, err)
	found = themeFileStamp{modTime: info.ModTime(), size: info.Size(), exists: true}
	require.False(t, themeStamp{files: []themeAssetStamp{{name: "a.png", stamp: found}}}.assetsCurrent("plain", time.Now()))

	// A reading notes the files of the selected theme it looked at, also
	// those of a theme that is not in use, and none that are left alone.
	writeThemeAsset(t, "plain", "b.png", "not an image")
	writeThemeAsset(t, "plain", "c.png", assetPNG)
	writeThemeAsset(t, "plain", "notes.txt", "notes")
	setUserThemeAge(t, filepath.Join("plain", "assets", "a.png"), time.Minute)
	SetTheme("plain")
	ReloadUserThemes()
	require.False(t, IsUserTheme("plain"))
	themeMu.RLock()
	watched := selectedThemeStamp.files
	themeMu.RUnlock()
	names := make([]string, 0, len(watched))
	for _, file := range watched {
		names = append(names, file.name)
	}
	require.Equal(t, []string{"a.png", "b.png"}, names, "the files up to the one that was not accepted")
}

// Pages, requests for a file of the assets and edits of that file at the same
// time: a page gets the theme with a stylesheet or no theme, and a file is
// handed out in the state its address stands for or not at all.
func TestThemeAssetsAreSafeForConcurrentPages(t *testing.T) {
	userThemeDirectory(t, "styled")
	writeThemeDirectory(t, "styled", userThemeColors)
	writeThemeStylesheet(t, "styled", assetStylesheetText)
	writeThemeAsset(t, "styled", "a.png", assetPNG)
	UpdatePagePalettes()
	path := filepath.Join(UserThemeDirectory, "styled", "assets", "a.png")
	address := regexp.MustCompile(`^/theme/styled/([0-9a-f]{16})/theme\.css$`)

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				page := GetPageTheme()
				if page == (PageTheme{}) {
					continue
				}
				found := address.FindStringSubmatch(page.Stylesheet)
				if page.BodyStyle != userThemeStyle || found == nil {
					t.Errorf("a page got %+v", page)
					return
				}
				file, asset, known := OpenThemeAsset("styled", found[1], "a.png")
				if !known {
					continue
				}
				info, err := file.Stat()
				_ = file.Close()
				if err != nil || info.Size() != asset.Size || !info.ModTime().Equal(asset.ModTime) || asset.Type != "image/png" {
					t.Errorf("a file was handed out in another state: %+v %v", asset, err)
					return
				}
			}
		})
	}
	for round := range 200 {
		content := assetPNG + strings.Repeat("x", round%7)
		switch round % 10 {
		case 3:
			// Not an image for a moment.
			content = "not an image"
		case 7:
			require.NoError(t, os.Remove(path))
			continue
		}
		// Written in place of the previous file in one step, as an editor
		// that saves through a temporary file does.
		next := path + ".tmp"
		require.NoError(t, os.WriteFile(next, []byte(content), 0600))
		require.NoError(t, os.Rename(next, path))
		if round%20 == 11 {
			ReloadUserThemes()
		}
	}
	close(stop)
	readers.Wait()
	require.NoError(t, os.WriteFile(path, []byte(assetPNG), 0600))
	setUserThemeAge(t, filepath.Join("styled", "assets", "a.png"), time.Minute)
	setUserThemeAge(t, filepath.Join("styled", "assets"), time.Minute)
	page := GetPageTheme()
	require.Equal(t, userThemeStyle, page.BodyStyle)
	require.Regexp(t, address, page.Stylesheet)
}
