package define

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

const (
	userThemeColors = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n"
	userThemeStyle  = template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;")
	blackboardStyle = template.CSS("--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;")
)

// userThemeDirectory runs a test in its own data directory with an empty
// themes directory and the given theme selected in config.yml, and leaves no
// theme file behind for the following tests.
func userThemeDirectory(t *testing.T, selected string) {
	t.Helper()
	restoreThemeState(t)
	t.Chdir(t.TempDir())
	// Registered after t.Chdir, so it runs before the directory is left.
	t.Cleanup(func() {
		themeMu.Lock()
		defer themeMu.Unlock()
		userThemeState, selectedThemeStamp = userThemeScan{}, themeStamp{}
	})
	require.NoError(t, os.Mkdir(UserThemeDirectory, 0700))
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: "+selected+"\n"), 0600))
	UpdatePagePalettes()
}

// writeUserTheme writes a file into the themes directory. The modification
// time is set well before the settle window, as it is for a file nobody is
// editing, unless the test says otherwise.
func writeUserTheme(t *testing.T, file, content string) {
	t.Helper()
	path := filepath.Join(UserThemeDirectory, file)
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	setUserThemeAge(t, file, time.Minute)
}

func setUserThemeAge(t *testing.T, file string, age time.Duration) {
	t.Helper()
	modified := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(filepath.Join(UserThemeDirectory, file), modified, modified))
}

func userThemeNames(themes []model.Theme) []string {
	names := make([]string, 0, len(themes))
	for _, theme := range themes {
		names = append(names, theme.Name)
	}
	return names
}

func TestUserThemeFilesAreRead(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "plain.yml", userThemeColors)
	writeUserTheme(t, "forest.yml", userThemeColors+
		"  muted: \"#9db8a6\"\n  hover: \"#ffffff1a\"\n  radius: 10px\n  font-family: Georgia, serif\n"+
		"name:\n  en: Forest at dusk\n  zh: 暮色森林\nversion: \"1.2\"\nauthor: Example Author\nlicense: MIT\n")
	// Entries that are not theme files are left alone without a remark.
	writeUserTheme(t, "README.md", "notes")
	writeUserTheme(t, ".forest.yml.swp", "editor state")
	writeUserTheme(t, "._forest.yml", "resource fork")
	writeUserTheme(t, "forest.yml~", "backup")
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, "assets"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(UserThemeDirectory, "assets", "nested.yml"), []byte(userThemeColors), 0600))

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"forest", "plain"}, userThemeNames(themes), "themes are listed in the order of their file names")
	require.Empty(t, problems)
	require.Empty(t, notices)

	forest := themes[0]
	require.Equal(t, model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}, forest.Colors)
	require.Equal(t, map[string]string{"muted": "#9db8a6", "hover": "#ffffff1a", "radius": "10px", "font-family": "Georgia, serif"}, forest.Tokens)
	require.Equal(t, "Forest at dusk", forest.DisplayName("en"))
	require.Equal(t, "暮色森林", forest.DisplayName("zh"))
	require.Equal(t, []string{"1.2", "Example Author", "MIT"}, []string{forest.Version, forest.Author, forest.License})
	require.Equal(t, "plain", themes[1].DisplayName("en"))

	for _, name := range []string{"forest", "plain"} {
		require.True(t, IsUserTheme(name), name)
		require.True(t, IsKnownTheme(name), name)
	}
	for _, name := range []string{"blackboard", model.ThemeCustomName, "nested", "assets", "README", "missing", ""} {
		require.False(t, IsUserTheme(name), name)
	}
	require.Equal(t, "#e8f1e4", GetThemePrimaryColor("forest"))

	SetTheme("forest")
	require.Equal(t, userThemeStyle+"--flare-muted:#9db8a6;--flare-hover:#ffffff1a;--flare-radius:10px;--flare-font-family:Georgia, serif;", GetAppBodyStyle())
	name, primary := ThemeSnapshot()
	require.Equal(t, "forest", name)
	require.Equal(t, "#e8f1e4", primary)
	require.Equal(t, forest.Colors, CurrentThemePalette())
}

// The theme files are read at startup with the selection of config.yml.
func TestUserThemeSelectedAtStartup(t *testing.T) {
	userThemeDirectory(t, "forest")
	require.Empty(t, GetAppBodyStyle(), "a selection without a file has no theme")
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 10px\n")
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())
	require.True(t, IsKnownTheme("forest"))
}

func TestUserThemeFilesNotInUse(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	outside := filepath.Join(t.TempDir(), "outside.yml")
	require.NoError(t, os.WriteFile(outside, []byte(userThemeColors), 0600))

	writeUserTheme(t, "usable.yml", userThemeColors)
	writeUserTheme(t, "notes.yaml", userThemeColors)
	writeUserTheme(t, "Mixed.yml", userThemeColors)
	writeUserTheme(t, "under_score.yml", userThemeColors)
	writeUserTheme(t, "two words.yml", userThemeColors)
	writeUserTheme(t, strings.Repeat("a", 33)+".yml", userThemeColors)
	writeUserTheme(t, "blackboard.yml", userThemeColors)
	writeUserTheme(t, "onedark.yml", userThemeColors)
	writeUserTheme(t, "custom.yml", userThemeColors)
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, "folder.yml"), 0700))
	require.NoError(t, os.Symlink(outside, filepath.Join(UserThemeDirectory, "link-out.yml")))
	require.NoError(t, os.Symlink("usable.yml", filepath.Join(UserThemeDirectory, "link-in.yml")))
	require.NoError(t, os.Symlink("missing.yml", filepath.Join(UserThemeDirectory, "link-dangling.yml")))
	writeUserTheme(t, "at-limit.yml", userThemeColors+"#"+strings.Repeat("x", MaximumThemeFileBytes-len(userThemeColors)-1))
	writeUserTheme(t, "over-limit.yml", userThemeColors+"#"+strings.Repeat("x", MaximumThemeFileBytes-len(userThemeColors)))
	writeUserTheme(t, "syntax.yml", "contract: 1\ntokens:\n  background: \"#10241b\"\n   primary: \"#e8f1e4\"\n")
	writeUserTheme(t, "list.yml", "- contract: 1\n")
	writeUserTheme(t, "empty.yml", "")
	writeUserTheme(t, "shape.yml", "contract: 1\ntokens: red\n")
	writeUserTheme(t, "no-contract.yml", strings.TrimPrefix(userThemeColors, "contract: 1\n"))
	writeUserTheme(t, "later-contract.yml", strings.Replace(userThemeColors, "contract: 1", "contract: 2", 1))
	writeUserTheme(t, "value.yml", userThemeColors+"  radius: 4em\n")
	writeUserTheme(t, "injection.yml", userThemeColors+"  font-family: \"x;background:url(//example.test/)\"\n")
	writeUserTheme(t, "color.yml", "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n")
	writeUserTheme(t, "contrast.yml", "contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#f2b84b\"\n")
	writeUserTheme(t, "text.yml", userThemeColors+"author: \""+strings.Repeat("a", 65)+"\"\n")

	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"at-limit", "usable"}, userThemeNames(themes))
	require.Empty(t, notices)

	reported := map[string]ThemeProblem{}
	files := make([]string, 0, len(problems))
	for _, problem := range problems {
		_, twice := reported[problem.File]
		require.False(t, twice, "%s is reported twice", problem.File)
		reported[problem.File] = problem
		files = append(files, problem.File)
	}
	require.IsIncreasing(t, files, "problems are listed in the order of the file names")
	want := map[string]ThemeProblem{
		"notes.yaml":                     {Reason: ThemeProblemExtension},
		"Mixed.yml":                      {Reason: ThemeProblemName},
		"under_score.yml":                {Reason: ThemeProblemName},
		"two words.yml":                  {Reason: ThemeProblemName},
		strings.Repeat("a", 33) + ".yml": {Reason: ThemeProblemName},
		"blackboard.yml":                 {Reason: ThemeProblemReserved},
		"onedark.yml":                    {Reason: ThemeProblemReserved},
		"custom.yml":                     {Reason: ThemeProblemReserved},
		"folder.yml":                     {Reason: ThemeProblemType},
		"link-out.yml":                   {Reason: ThemeProblemType},
		"link-in.yml":                    {Reason: ThemeProblemType},
		"link-dangling.yml":              {Reason: ThemeProblemType},
		"over-limit.yml":                 {Reason: ThemeProblemSize},
		"list.yml":                       {Reason: ThemeProblemSyntax},
		"empty.yml":                      {Reason: ThemeProblemSyntax, Subject: "the file is empty"},
		"no-contract.yml":                {Reason: ThemeProblemContract},
		"later-contract.yml":             {Reason: ThemeProblemContract},
		"value.yml":                      {Reason: ThemeProblemValue, Subject: "tokens.radius"},
		"injection.yml":                  {Reason: ThemeProblemValue, Subject: "tokens.font-family"},
		"color.yml":                      {Reason: ThemeProblemColor, Subject: "tokens.accent"},
		"contrast.yml":                   {Reason: ThemeProblemContrast},
		"text.yml":                       {Reason: ThemeProblemText, Subject: "author"},
		"syntax.yml":                     {Reason: ThemeProblemSyntax},
		"shape.yml":                      {Reason: ThemeProblemSyntax},
	}
	require.Len(t, reported, len(want))
	for file, expected := range want {
		problem, found := reported[file]
		require.True(t, found, "%s is not reported", file)
		require.Equal(t, expected.Reason, problem.Reason, file)
		switch file {
		case "syntax.yml":
			require.Contains(t, problem.Subject, "yaml: line ", file)
		case "list.yml":
			require.NotEmpty(t, problem.Subject, file)
		case "shape.yml":
			require.True(t, strings.HasPrefix(problem.Subject, "tokens: "), "%s: %q", file, problem.Subject)
		default:
			require.Equal(t, expected.Subject, problem.Subject, file)
		}
	}

	// None of the files that are not in use can be selected, and nothing of
	// a rejected file reaches a page.
	for file := range want {
		name := strings.TrimSuffix(strings.TrimSuffix(file, ".yml"), ".yaml")
		require.False(t, IsUserTheme(name), name)
		SetTheme(name)
		style := GetAppBodyStyle()
		if name == "blackboard" || name == "onedark" {
			require.Contains(t, string(style), "--color-background:", name)
			require.NotContains(t, string(style), "#10241b", "the bundled theme %s took the colors of a file", name)
			continue
		}
		require.Empty(t, style, name)
	}
}

func TestUnreadableUserThemeFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions do not apply to this user")
	}
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "locked.yml", userThemeColors)
	path := filepath.Join(UserThemeDirectory, "locked.yml")
	require.NoError(t, os.Chmod(path, 0))
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Empty(t, themes)
	require.Equal(t, []ThemeProblem{{File: "locked.yml", Reason: ThemeProblemRead}}, problems)
}

// An instance without the directory, or with something else under its name,
// has no theme files and no remarks.
func TestThemesDirectoryIsOptional(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for _, prepare := range []func(){
		func() { require.NoError(t, os.Remove(UserThemeDirectory)) },
		func() { require.NoError(t, os.WriteFile(UserThemeDirectory, []byte(userThemeColors), 0600)) },
	} {
		prepare()
		ReloadUserThemes()
		themes, problems, notices := UserThemes()
		require.Empty(t, themes)
		require.Empty(t, problems)
		require.Empty(t, notices)
		require.Equal(t, blackboardStyle, GetAppBodyStyle())
	}
}

func TestUserThemeFileLimit(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for i := range MaximumUserThemeFiles + 6 {
		writeUserTheme(t, fmt.Sprintf("t%02d.yml", i), userThemeColors)
	}
	// Entries that are not theme files do not count.
	writeUserTheme(t, "a-note.md", "notes")
	writeUserTheme(t, ".a-hidden.yml", userThemeColors)
	writeUserTheme(t, "zz-after-the-limit.yaml", userThemeColors)

	ReloadUserThemes()
	themes, problems, _ := UserThemes()
	require.Len(t, themes, MaximumUserThemeFiles)
	require.Equal(t, "t00", themes[0].Name)
	require.Equal(t, fmt.Sprintf("t%02d", MaximumUserThemeFiles-1), themes[len(themes)-1].Name)
	// The files after the limit are reported once, by the first of them.
	require.Equal(t, []ThemeProblem{{File: fmt.Sprintf("t%02d.yml", MaximumUserThemeFiles), Reason: ThemeProblemLimit}}, problems)
	require.False(t, IsUserTheme(fmt.Sprintf("t%02d", MaximumUserThemeFiles)))

	// A file that is reported instead of used counts as one of the files
	// looked at, so neither the work nor the list of the settings page
	// depends on what the directory holds.
	writeUserTheme(t, "t03.yml", "contract: 1\n")
	writeUserTheme(t, "Upper.yml", userThemeColors)
	writeUserTheme(t, "a-note.yaml", userThemeColors)
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Len(t, themes, MaximumUserThemeFiles-3)
	require.Equal(t, []ThemeProblem{
		{File: "Upper.yml", Reason: ThemeProblemName},
		{File: "a-note.yaml", Reason: ThemeProblemExtension},
		{File: "t03.yml", Reason: ThemeProblemColor, Subject: "tokens.background"},
		{File: fmt.Sprintf("t%02d.yml", MaximumUserThemeFiles-2), Reason: ThemeProblemLimit},
	}, problems)

	// However many files there are, the list stays within the limit.
	for i := range 3 * MaximumUserThemeFiles {
		writeUserTheme(t, fmt.Sprintf("a%03d.yaml", i), userThemeColors)
	}
	ReloadUserThemes()
	themes, problems, _ = UserThemes()
	require.Empty(t, themes)
	require.Len(t, problems, MaximumUserThemeFiles+1)
	require.Equal(t, ThemeProblemLimit, problems[len(problems)-1].Reason)
}

func TestUserThemeFileWithUnusedParts(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "later.yml", userThemeColors+"  space: 4px\n  radius: 8px\noptions:\n  - id: density\nmodes:\n  sepia:\n    background: \"#f4ecd8\"\n  dark:\n    glow: \"#ffffff\"\n")
	writeUserTheme(t, "rejected.yml", userThemeColors+"  radius: 4em\nbackground:\n  image: wallpaper.jpg\n")
	ReloadUserThemes()
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"later"}, userThemeNames(themes))
	require.Equal(t, map[string]string{"radius": "8px"}, themes[0].Tokens)
	require.Equal(t, []ThemeProblem{{File: "rejected.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"}}, problems)
	require.Equal(t, []ThemeNotice{
		{File: "later.yml", Unused: model.ThemeFileUnused{Keys: []string{"modes.sepia", "options"}, Tokens: []string{"modes.dark.glow", "tokens.space"}}},
		{File: "rejected.yml", Unused: model.ThemeFileUnused{Keys: []string{"background"}}},
	}, notices)

	// What UserThemes returns is the caller's to change.
	themes[0].Name, problems[0].File, notices[0].File = "changed", "changed", "changed"
	themes, problems, notices = UserThemes()
	require.Equal(t, "later", themes[0].Name)
	require.Equal(t, "rejected.yml", problems[0].File)
	require.Equal(t, "later.yml", notices[0].File)
}

// The pages follow the file of the selected theme. The check is the state of
// that one file: nothing is read while it has not changed.
func TestSelectedUserThemeFollowsItsFile(t *testing.T) {
	userThemeDirectory(t, "forest")
	writeUserTheme(t, "forest.yml", userThemeColors)
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	_, _, generation := ThemeRenderSnapshot()

	// While the file keeps its state the directory is not read: a file added
	// to it is not seen, and the pages are not rendered again.
	writeUserTheme(t, "added.yml", userThemeColors)
	for range 3 {
		require.Equal(t, userThemeStyle, GetAppBodyStyle())
	}
	require.False(t, IsUserTheme("added"))
	_, _, unchanged := ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)

	// An edit is applied by the next page, and the directory was read for it.
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 10px\n")
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())
	require.True(t, IsUserTheme("added"))
	_, _, edited := ThemeRenderSnapshot()
	require.NotEqual(t, generation, edited)

	// An edit that keeps the size is seen by the modification time.
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 12px\n")
	setUserThemeAge(t, "forest.yml", 2*time.Minute)
	require.Equal(t, userThemeStyle+"--flare-radius:12px;", GetAppBodyStyle())

	// A file written a moment ago is read again even if its size and time
	// are those of the last reading: the time can be too coarse to tell. The
	// time is set slightly ahead so that it stays recent while the test runs.
	path := filepath.Join(UserThemeDirectory, "forest.yml")
	recent := time.Now().Add(1500 * time.Millisecond)
	require.NoError(t, os.Chtimes(path, recent, recent))
	require.Equal(t, userThemeStyle+"--flare-radius:12px;", GetAppBodyStyle())
	require.NoError(t, os.WriteFile(path, []byte(userThemeColors+"  radius: 14px\n"), 0600))
	require.NoError(t, os.Chtimes(path, recent, recent))
	require.Equal(t, userThemeStyle+"--flare-radius:14px;", GetAppBodyStyle())

	// Writing the same theme again changes nothing that pages depend on.
	_, _, before := ThemeRenderSnapshot()
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 14px\n")
	setUserThemeAge(t, "forest.yml", 3*time.Minute)
	require.Equal(t, userThemeStyle+"--flare-radius:14px;", GetAppBodyStyle())
	_, _, after := ThemeRenderSnapshot()
	require.Equal(t, before, after)

	// A file that is rejected leaves the selection without a theme: no style,
	// and the color of the first bundled theme for what needs one.
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 4em\n")
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))
	name, primary := ThemeSnapshot()
	require.Equal(t, "forest", name)
	require.Equal(t, ThemePalettes[0].Colors.Primary, primary)
	require.Equal(t, ThemePalettes[0].Colors, CurrentThemePalette())
	_, problems, _ := UserThemes()
	require.Equal(t, []ThemeProblem{{File: "forest.yml", Reason: ThemeProblemValue, Subject: "tokens.radius"}}, problems)
	// The rejected file is not read again on every page either.
	writeUserTheme(t, "second.yml", userThemeColors)
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsUserTheme("second"))

	// Correcting the file, removing it and putting it back are all followed.
	writeUserTheme(t, "forest.yml", userThemeColors)
	setUserThemeAge(t, "forest.yml", 4*time.Minute)
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	require.True(t, IsUserTheme("second"))
	require.NoError(t, os.Remove(path))
	require.Empty(t, GetAppBodyStyle())
	require.False(t, IsKnownTheme("forest"))
	require.Empty(t, GetAppBodyStyle())
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 6px\n")
	require.Equal(t, userThemeStyle+"--flare-radius:6px;", GetAppBodyStyle())
}

// A bundled theme and the custom colors do not depend on the directory, so
// rendering a page with them looks at no file.
func TestOtherSelectionsReadNoThemeFile(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "added.yml", userThemeColors)
	require.Equal(t, blackboardStyle, GetAppBodyStyle())
	require.False(t, IsUserTheme("added"))

	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.Equal(t, template.CSS("--color-background:#ffffff;--color-primary:#222222;--color-accent:#888888;"), GetAppBodyStyle())
	require.False(t, IsUserTheme("added"))

	// Selecting a theme file makes the next page read the directory once.
	ReloadUserThemes()
	require.True(t, IsUserTheme("added"))
	SetTheme("added")
	writeUserTheme(t, "another.yml", userThemeColors)
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	require.True(t, IsUserTheme("another"))
	writeUserTheme(t, "third.yml", userThemeColors)
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	require.False(t, IsUserTheme("third"))
}

// A selection that cannot be the name of a theme file never becomes a path.
func TestThemeSelectionThatIsNoFileName(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	require.NoError(t, os.WriteFile("outside.yml", []byte(userThemeColors), 0600))
	writeUserTheme(t, "Mixed.yml", userThemeColors)
	for _, name := range []string{"", "../outside", "..", "Mixed", "a/b", "forest.yml", "blackboard", model.ThemeCustomName} {
		require.False(t, isUserThemeName(name), "%q", name)
		require.Equal(t, themeStamp{}, statTheme(name), "%q", name)
	}
	for _, name := range []string{"", "../outside", "Mixed"} {
		SetTheme(name)
		require.Empty(t, GetAppBodyStyle(), "%q", name)
		require.False(t, IsKnownTheme(name), "%q", name)
	}
}

func TestThemeFileStampTellsWhenToReadAgain(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	settled := now.Add(-time.Minute)
	read := themeFileStamp{modTime: settled, size: 100, exists: true}
	for name, tc := range map[string]struct {
		read, found themeFileStamp
		current     bool
	}{
		"unchanged":                {read, read, true},
		"both missing":             {themeFileStamp{}, themeFileStamp{}, true},
		"appeared":                 {themeFileStamp{}, read, false},
		"removed":                  {read, themeFileStamp{}, false},
		"other size":               {read, themeFileStamp{modTime: settled, size: 101, exists: true}, false},
		"other time":               {read, themeFileStamp{modTime: settled.Add(time.Second), size: 100, exists: true}, false},
		"written a moment ago":     {themeFileStamp{modTime: now.Add(-time.Second), size: 100, exists: true}, themeFileStamp{modTime: now.Add(-time.Second), size: 100, exists: true}, false},
		"at the settle window":     {themeFileStamp{modTime: now.Add(-themeStampSettle), size: 100, exists: true}, themeFileStamp{modTime: now.Add(-themeStampSettle), size: 100, exists: true}, true},
		"time slightly ahead":      {themeFileStamp{modTime: now.Add(time.Second), size: 100, exists: true}, themeFileStamp{modTime: now.Add(time.Second), size: 100, exists: true}, false},
		"time far ahead is stable": {themeFileStamp{modTime: now.Add(time.Hour), size: 100, exists: true}, themeFileStamp{modTime: now.Add(time.Hour), size: 100, exists: true}, true},
	} {
		require.Equal(t, tc.current, tc.read.current(tc.found, now), name)
	}
}

// A file whose modification time lies ahead of the clock is not read on
// every page: the time alone says nothing about a recent edit.
func TestUserThemeFileWithATimeAhead(t *testing.T) {
	userThemeDirectory(t, "forest")
	writeUserTheme(t, "forest.yml", userThemeColors)
	setUserThemeAge(t, "forest.yml", -time.Hour)
	UpdatePagePalettes()
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	writeUserTheme(t, "added.yml", userThemeColors)
	require.Equal(t, userThemeStyle, GetAppBodyStyle())
	require.False(t, IsUserTheme("added"))
}

// Pages are rendered while the file of the selected theme is being edited
// and while themes are selected. Every style a page gets is one of the
// styles that were written, never a mix.
func TestUserThemeReloadIsSafeForConcurrentPages(t *testing.T) {
	userThemeDirectory(t, "forest")
	writeUserTheme(t, "forest.yml", userThemeColors)
	UpdatePagePalettes()
	path := filepath.Join(UserThemeDirectory, "forest.yml")
	valid := map[template.CSS]bool{"": true, blackboardStyle: true, userThemeStyle: true}
	contents := make([][]byte, 0, 8)
	for radius := range 8 {
		contents = append(contents, []byte(fmt.Sprintf("%s  radius: %dpx\n", userThemeColors, radius)))
		valid[userThemeStyle+template.CSS(fmt.Sprintf("--flare-radius:%dpx;", radius))] = true
	}

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
				if style := GetAppBodyStyle(); !valid[style] {
					t.Errorf("a page got the style %q", style)
					return
				}
				UserThemes()
				IsKnownTheme("forest")
				CurrentThemePalette()
			}
		})
	}
	for round := range 200 {
		// Written in place of the previous file in one step, as an editor
		// that saves through a temporary file does.
		next := path + ".tmp"
		require.NoError(t, os.WriteFile(next, contents[round%len(contents)], 0600))
		require.NoError(t, os.Rename(next, path))
		switch round % 20 {
		case 5:
			SetTheme("blackboard")
		case 6:
			ReloadUserThemes()
			SetTheme("forest")
		case 11:
			UpdatePagePalettes()
		}
	}
	close(stop)
	readers.Wait()
	SetTheme("forest")
	require.Equal(t, userThemeStyle+"--flare-radius:7px;", GetAppBodyStyle())
}
