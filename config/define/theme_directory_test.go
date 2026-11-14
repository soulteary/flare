package define

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// writeThemeDirectory writes the theme.yml of a theme that is a directory,
// and makes the directory if it is not there.
func writeThemeDirectory(t *testing.T, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(UserThemeDirectory, name), 0700))
	writeUserTheme(t, filepath.Join(name, userThemeManifest), content)
}

func makeThemesSubdirectory(t *testing.T, name string) {
	t.Helper()
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, name), 0700))
}

func TestThemeDirectoryIsRead(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "zelda", userThemeColors+"  radius: 10px\nname:\n  en: Zelda\n  zh: 塞尔达\nversion: \"2.0\"\nlayout:\n  DesktopColumns: 6\n")
	writeUserTheme(t, "plain.yml", userThemeColors)
	// Of the directory of a theme, theme.yml is read and nothing else.
	writeUserTheme(t, filepath.Join("zelda", "theme.css"), "body { color: red }")
	writeUserTheme(t, filepath.Join("zelda", "other.yml"), userThemeColors+"  radius: 4em\n")
	makeThemesSubdirectory(t, filepath.Join("zelda", "assets"))
	writeUserTheme(t, filepath.Join("zelda", "assets", "theme.yml"), userThemeColors)
	// A directory without a theme.yml is not a theme, whatever it holds, and
	// a hidden directory is left alone like a hidden file.
	makeThemesSubdirectory(t, "empty")
	makeThemesSubdirectory(t, "notes")
	writeUserTheme(t, filepath.Join("notes", "forest.yml"), userThemeColors)
	makeThemesSubdirectory(t, ".hidden")
	writeUserTheme(t, filepath.Join(".hidden", "theme.yml"), userThemeColors)

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"plain", "zelda"}, userThemeNames(themes), "themes are listed in the order of the names of their entries")
	require.Empty(t, problems)
	require.Empty(t, notices)

	require.False(t, themes[0].Directory)
	zelda := themes[1]
	require.True(t, zelda.Directory)
	require.Equal(t, model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}, zelda.Colors)
	require.Equal(t, map[string]string{"radius": "10px"}, zelda.Tokens)
	require.Equal(t, map[string]string{"DesktopColumns": "6"}, zelda.Layout)
	require.Equal(t, "Zelda", zelda.DisplayName("en"))
	require.Equal(t, "塞尔达", zelda.DisplayName("zh"))
	require.Equal(t, "2.0", zelda.Version)

	require.True(t, IsUserTheme("zelda"))
	require.True(t, IsKnownTheme("zelda"))
	for _, name := range []string{"empty", "notes", "forest", "hidden", ".hidden", "assets", "other", "theme", "zelda/theme", "zelda/assets"} {
		require.False(t, IsUserTheme(name), name)
		require.False(t, IsKnownTheme(name), name)
	}

	// The theme is selected, shown and previewed by its name like any other.
	SetTheme("zelda")
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())
	name, layout := SelectedThemeLayout()
	require.Equal(t, "zelda", name)
	require.Equal(t, map[string]string{"DesktopColumns": "6"}, layout)
	SetTheme("blackboard")
	page, known := GetPageThemeFor("zelda")
	require.True(t, known)
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", page.BodyStyle)
	require.Equal(t, "塞尔达", ThemeTitle("zelda", "zh"))
}

// The theme of a directory is selected at startup like the theme of a file.
func TestThemeDirectorySelectedAtStartup(t *testing.T) {
	userThemeDirectory(t, "zelda")
	require.Empty(t, GetAppBodyStyle(), "a selection without a theme has no style")
	writeThemeDirectory(t, "zelda", userThemeColors+"  radius: 10px\n")
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())
	require.True(t, IsKnownTheme("zelda"))
}

func TestThemeDirectoriesNotInUse(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, userThemeManifest), []byte(userThemeColors), 0600))
	link := func(target string, path ...string) {
		t.Helper()
		require.NoError(t, os.Symlink(target, filepath.Join(append([]string{UserThemeDirectory}, path...)...)))
	}

	writeThemeDirectory(t, "usable", userThemeColors)
	// The name of the directory is the name of the theme, with the rules of
	// the name of a file.
	writeThemeDirectory(t, "Mixed", userThemeColors)
	writeThemeDirectory(t, "under_score", userThemeColors)
	writeThemeDirectory(t, "two words", userThemeColors)
	writeThemeDirectory(t, strings.Repeat("b", 33), userThemeColors)
	writeThemeDirectory(t, "blackboard", userThemeColors)
	writeThemeDirectory(t, "onedark", userThemeColors)
	writeThemeDirectory(t, "custom", userThemeColors)
	// theme.yml gets the checks of every theme file.
	writeThemeDirectory(t, "at-limit", userThemeColors+"#"+strings.Repeat("x", MaximumThemeFileBytes-len(userThemeColors)-1))
	writeThemeDirectory(t, "over-limit", userThemeColors+"#"+strings.Repeat("x", MaximumThemeFileBytes-len(userThemeColors)))
	writeThemeDirectory(t, "empty-file", "")
	writeThemeDirectory(t, "no-contract", strings.TrimPrefix(userThemeColors, "contract: 1\n"))
	writeThemeDirectory(t, "value", userThemeColors+"  radius: 4em\n")
	writeThemeDirectory(t, "injection", userThemeColors+"  font-family: \"x;background:url(//example.test/)\"\n")
	writeThemeDirectory(t, "contrast", "contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#f2b84b\"\n")
	// theme.yml has to be a regular file of the directory itself.
	makeThemesSubdirectory(t, "folder")
	makeThemesSubdirectory(t, filepath.Join("folder", userThemeManifest))
	makeThemesSubdirectory(t, "link-to-outside")
	link(filepath.Join(outside, userThemeManifest), "link-to-outside", userThemeManifest)
	makeThemesSubdirectory(t, "link-to-inside")
	link(filepath.Join("..", "usable", userThemeManifest), "link-to-inside", userThemeManifest)
	makeThemesSubdirectory(t, "link-to-nothing")
	link("missing.yml", "link-to-nothing", userThemeManifest)
	// The other extension of YAML is reported in a directory as it is for a file.
	makeThemesSubdirectory(t, "yaml")
	writeUserTheme(t, filepath.Join("yaml", "theme.yaml"), userThemeColors)
	// A link in place of a directory is not followed, wherever it leads.
	link(outside, "outside")
	link("usable", "inside")
	link("missing", "dangling")

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"at-limit", "usable"}, userThemeNames(themes))
	require.Empty(t, notices)

	reported := map[string]ThemeProblem{}
	for _, problem := range problems {
		_, twice := reported[problem.File]
		require.False(t, twice, "%s is reported twice", problem.File)
		reported[problem.File] = problem
	}
	want := map[string]ThemeProblem{
		"Mixed/theme.yml":                      {Reason: ThemeProblemName},
		"under_score/theme.yml":                {Reason: ThemeProblemName},
		"two words/theme.yml":                  {Reason: ThemeProblemName},
		strings.Repeat("b", 33) + "/theme.yml": {Reason: ThemeProblemName},
		"blackboard/theme.yml":                 {Reason: ThemeProblemReserved},
		"onedark/theme.yml":                    {Reason: ThemeProblemReserved},
		"custom/theme.yml":                     {Reason: ThemeProblemReserved},
		"over-limit/theme.yml":                 {Reason: ThemeProblemSize},
		"empty-file/theme.yml":                 {Reason: ThemeProblemSyntax, Subject: "the file is empty"},
		"no-contract/theme.yml":                {Reason: ThemeProblemContract},
		"value/theme.yml":                      {Reason: ThemeProblemValue, Subject: "tokens.radius"},
		"injection/theme.yml":                  {Reason: ThemeProblemValue, Subject: "tokens.font-family"},
		"contrast/theme.yml":                   {Reason: ThemeProblemContrast},
		"folder/theme.yml":                     {Reason: ThemeProblemType},
		"link-to-outside/theme.yml":            {Reason: ThemeProblemType},
		"link-to-inside/theme.yml":             {Reason: ThemeProblemType},
		"link-to-nothing/theme.yml":            {Reason: ThemeProblemType},
		"yaml/theme.yaml":                      {Reason: ThemeProblemExtension},
		"outside":                              {Reason: ThemeProblemLink},
		"inside":                               {Reason: ThemeProblemLink},
		"dangling":                             {Reason: ThemeProblemLink},
	}
	require.Len(t, reported, len(want))
	for file, expected := range want {
		problem, found := reported[file]
		require.True(t, found, "%s is not reported", file)
		expected.File = file
		require.Equal(t, expected, problem, file)
	}

	// None of them can be selected, and nothing of them reaches a page.
	for file := range want {
		name, _, _ := strings.Cut(file, "/")
		require.False(t, IsUserTheme(name), name)
		SetTheme(name)
		style := GetAppBodyStyle()
		if name == "blackboard" || name == "onedark" {
			require.Contains(t, string(style), "--color-background:", name)
			require.NotContains(t, string(style), "#10241b", "the bundled theme %s took the colors of a directory", name)
			continue
		}
		require.Empty(t, style, name)
	}
}

func TestUnreadableThemeDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not apply to this user")
	}
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "locked", userThemeColors)
	writeThemeDirectory(t, "sealed", userThemeColors)
	file := filepath.Join(UserThemeDirectory, "locked", userThemeManifest)
	directory := filepath.Join(UserThemeDirectory, "sealed")
	require.NoError(t, os.Chmod(file, 0))
	require.NoError(t, os.Chmod(directory, 0))
	t.Cleanup(func() {
		_ = os.Chmod(file, 0600)
		_ = os.Chmod(directory, 0700)
	})
	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Empty(t, themes)
	require.Equal(t, []ThemeProblem{
		{File: "locked/theme.yml", Reason: ThemeProblemRead},
		{File: "sealed/theme.yml", Reason: ThemeProblemRead},
	}, problems)
}

// A name has one theme. A directory with a theme.yml is the theme of its
// name, and the file of that name is then not read.
func TestThemeOfOneNameAsFileAndAsDirectory(t *testing.T) {
	userThemeDirectory(t, "forest")
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 2px\n")
	UpdatePagePalettes()
	fileStyle := userThemeStyle + "--flare-radius:2px;"
	require.Equal(t, fileStyle, GetAppBodyStyle())

	// A directory of the name without a theme.yml is not a theme.
	makeThemesSubdirectory(t, "forest")
	writeUserTheme(t, filepath.Join("forest", "theme.css"), "body { color: red }")
	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Equal(t, []string{"forest"}, userThemeNames(themes))
	require.False(t, themes[0].Directory)
	require.Empty(t, problems)
	require.Equal(t, fileStyle, GetAppBodyStyle())

	// With a theme.yml the directory is the theme, and the file is reported.
	writeThemeDirectory(t, "forest", userThemeColors+"  radius: 12px\n")
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Equal(t, []string{"forest"}, userThemeNames(themes))
	require.True(t, themes[0].Directory)
	require.Equal(t, []ThemeProblem{{File: "forest.yml", Reason: ThemeProblemDuplicate}}, problems)
	require.Equal(t, userThemeStyle+"--flare-radius:12px;", GetAppBodyStyle())

	// A theme.yml that is not usable does not bring the file back: the name
	// belongs to the directory, and the selection is without a theme.
	writeThemeDirectory(t, "forest", userThemeColors+"  radius: 4em\n")
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Empty(t, themes)
	require.Equal(t, []ThemeProblem{
		{File: "forest/theme.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"},
		{File: "forest.yml", Reason: ThemeProblemDuplicate},
	}, problems)
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))

	// Without the theme.yml the file is the theme again.
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "forest", userThemeManifest)))
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Equal(t, []string{"forest"}, userThemeNames(themes))
	require.False(t, themes[0].Directory)
	require.Empty(t, problems)
	require.Equal(t, fileStyle, GetAppBodyStyle())

	// A file is reported for its name before it is reported as a second one.
	writeUserTheme(t, "blackboard.yml", userThemeColors)
	writeThemeDirectory(t, "blackboard", userThemeColors)
	writeUserTheme(t, "Mixed.yml", userThemeColors)
	writeThemeDirectory(t, "Mixed", userThemeColors)
	ReloadUserThemes()
	_, problems, _ = UserThemes()
	require.ElementsMatch(t, []ThemeProblem{
		{File: "Mixed/theme.yml", Reason: ThemeProblemName},
		{File: "Mixed.yml", Reason: ThemeProblemName},
		{File: "blackboard/theme.yml", Reason: ThemeProblemReserved},
		{File: "blackboard.yml", Reason: ThemeProblemReserved},
	}, problems)
}

// The pages follow the selected theme in both forms: an edit of the file it
// is read from, and the other form taking its place.
func TestSelectedThemeFollowsItsDirectory(t *testing.T) {
	userThemeDirectory(t, "forest")
	writeThemeDirectory(t, "forest", userThemeColors)
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	_, _, generation := ThemeRenderSnapshot()
	radius := func(pixels int) string {
		return fmt.Sprintf("%s  radius: %dpx\n", userThemeColors, pixels)
	}
	styled := func(pixels int) template.CSS {
		return userThemeStyle + template.CSS(fmt.Sprintf("--flare-radius:%dpx;", pixels))
	}

	// While theme.yml keeps its state the directory is not read: a theme
	// added to it is not seen, and neither is another file of the theme.
	writeUserTheme(t, "added.yml", userThemeColors)
	writeUserTheme(t, filepath.Join("forest", "theme.css"), "body { color: red }")
	for range 3 {
		require.Equal(t, userThemeStyle, GetAppBodyStyle())
	}
	require.False(t, IsUserTheme("added"))
	_, _, unchanged := ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)

	// An edit of theme.yml is applied by the next page.
	writeThemeDirectory(t, "forest", radius(10))
	require.Equal(t, styled(10), GetAppBodyStyle())
	require.True(t, IsUserTheme("added"))

	// An edit that keeps the size is seen by the modification time.
	writeThemeDirectory(t, "forest", radius(12))
	setUserThemeAge(t, filepath.Join("forest", userThemeManifest), 2*time.Minute)
	require.Equal(t, styled(12), GetAppBodyStyle())

	// A file of the same name is noticed when it appears. The directory
	// stays the theme, and the file is reported.
	writeUserTheme(t, "forest.yml", radius(3))
	require.Equal(t, styled(12), GetAppBodyStyle())
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "forest.yml", Reason: ThemeProblemDuplicate}}, problems)

	// Without its theme.yml the directory is no theme, and the file is.
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "forest", userThemeManifest)))
	require.Equal(t, styled(3), GetAppBodyStyle())
	_, problems, _ = UserThemes()
	require.Empty(t, problems)

	// A theme.yml that appears takes the place of the file, which did not
	// change.
	writeThemeDirectory(t, "forest", radius(14))
	require.Equal(t, styled(14), GetAppBodyStyle())

	// One that is rejected leaves the selection without a theme.
	writeThemeDirectory(t, "forest", userThemeColors+"  radius: 4em\n")
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))

	// Removing the directory, then the file, and putting a directory back.
	require.NoError(t, os.RemoveAll(filepath.Join(UserThemeDirectory, "forest")))
	require.Equal(t, styled(3), GetAppBodyStyle())
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "forest.yml")))
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))
	writeThemeDirectory(t, "forest", radius(6))
	require.Equal(t, styled(6), GetAppBodyStyle())
}

func TestThemeStampWatchesBothForms(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	read := themeFileStamp{modTime: now.Add(-time.Minute), size: 100, exists: true}
	edited := themeFileStamp{modTime: now.Add(-time.Minute), size: 101, exists: true}
	for name, tc := range map[string]struct {
		read, found themeStamp
		current     bool
	}{
		"neither":                     {themeStamp{}, themeStamp{}, true},
		"a file, unchanged":           {themeStamp{file: read}, themeStamp{file: read}, true},
		"a directory, unchanged":      {themeStamp{manifest: read}, themeStamp{manifest: read}, true},
		"both, unchanged":             {themeStamp{file: read, manifest: read}, themeStamp{file: read, manifest: read}, true},
		"the file edited":             {themeStamp{file: read}, themeStamp{file: edited}, false},
		"theme.yml edited":            {themeStamp{manifest: read}, themeStamp{manifest: edited}, false},
		"theme.yml beside the file":   {themeStamp{file: read}, themeStamp{file: read, manifest: read}, false},
		"a file beside the directory": {themeStamp{manifest: read}, themeStamp{file: read, manifest: read}, false},
		"the file removed":            {themeStamp{file: read, manifest: read}, themeStamp{manifest: read}, false},
		"theme.yml removed":           {themeStamp{file: read, manifest: read}, themeStamp{file: read}, false},
		"the directory for the file":  {themeStamp{file: read}, themeStamp{manifest: read}, false},
	} {
		require.Equal(t, tc.current, tc.read.current(tc.found, now), name)
	}
}

// Directories and links are entries that are looked at, so they count for
// the number that is.
func TestThemeDirectoriesCountForTheLimit(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for i := range MaximumUserThemeFiles - 2 {
		makeThemesSubdirectory(t, fmt.Sprintf("d%02d", i))
	}
	require.NoError(t, os.Symlink("d00", filepath.Join(UserThemeDirectory, "e-link")))
	writeThemeDirectory(t, "f-theme", userThemeColors)
	writeThemeDirectory(t, "g-after", userThemeColors)
	writeUserTheme(t, "h-after.yml", userThemeColors)
	// What is left alone does not count.
	writeUserTheme(t, "a-note.md", "notes")
	makeThemesSubdirectory(t, ".a-hidden")

	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Equal(t, []string{"f-theme"}, userThemeNames(themes))
	require.Equal(t, []ThemeProblem{
		{File: "e-link", Reason: ThemeProblemLink},
		{File: "g-after", Reason: ThemeProblemLimit},
	}, problems)
	require.False(t, IsUserTheme("g-after"))
	require.False(t, IsUserTheme("h-after"))
}

// A theme file is not stored under a name whose theme is a directory: it
// would not be read, and an upload does not replace a directory.
func TestStoreUserThemeBesideADirectory(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "zelda", userThemeColors)
	writeThemeDirectory(t, "broken", userThemeColors+"  radius: 4em\n")
	makeThemesSubdirectory(t, "notes")
	before := themesDirectoryEntries(t)
	for _, name := range []string{"zelda", "broken"} {
		for _, replace := range []bool{false, true} {
			require.ErrorIs(t, StoreUserTheme(name, []byte(userThemeColors), replace), ErrThemeStoreDirectory, "%s, replace %v", name, replace)
		}
	}
	require.Equal(t, before, themesDirectoryEntries(t))
	zelda, err := os.ReadFile(filepath.Join(UserThemeDirectory, "zelda", userThemeManifest))
	require.NoError(t, err)
	require.Equal(t, userThemeColors, string(zelda))

	// A directory that is not a theme does not hold its name.
	require.NoError(t, StoreUserTheme("notes", []byte(userThemeColors+"  radius: 2px\n"), false))
	themes, _, _ := UserThemes()
	require.Equal(t, []string{"notes", "zelda"}, userThemeNames(themes))
	require.False(t, themes[0].Directory)
	require.True(t, themes[1].Directory)
}

// Directories and links count for a full themes directory as files do.
func TestStoreUserThemeIntoADirectoryFullOfDirectories(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for i := range MaximumUserThemeFiles - 1 {
		makeThemesSubdirectory(t, fmt.Sprintf("d%02d", i))
	}
	require.NoError(t, os.Symlink("d00", filepath.Join(UserThemeDirectory, "e-link")))
	require.ErrorIs(t, StoreUserTheme("one-more", []byte(userThemeColors), false), ErrThemeStoreFull)
	require.NoFileExists(t, filepath.Join(UserThemeDirectory, "one-more.yml"))
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "e-link")))
	require.NoError(t, StoreUserTheme("one-more", []byte(userThemeColors), false))
	require.True(t, IsUserTheme("one-more"))
}

// A theme that is a directory is not handed out as a single file.
func TestThemeDirectoryIsNotExported(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeThemeDirectory(t, "zelda", userThemeColors)
	writeUserTheme(t, "plain.yml", userThemeColors)
	ReloadUserThemes()
	_, _, known := ExportTheme("zelda")
	require.False(t, known)
	for _, name := range []string{"zelda/theme", "zelda/theme.yml", filepath.Join("zelda", "theme")} {
		_, _, known = ExportTheme(name)
		require.False(t, known, name)
	}
	file, raw, known := ExportTheme("plain")
	require.True(t, known)
	require.Equal(t, "plain.yml", file)
	require.Equal(t, userThemeColors, string(raw))
}
