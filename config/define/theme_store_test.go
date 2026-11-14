package define

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// themesDirectoryEntries lists the themes directory, hidden files included.
func themesDirectoryEntries(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(UserThemeDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func TestStoreUserTheme(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	require.NoError(t, os.Remove(UserThemeDirectory))

	// The directory is made when the first theme is stored, and the file is
	// written as given: comments and parts for later releases stay.
	raw := "# A comment of the author.\n" + userThemeColors + "  radius: 10px\noptions:\n  - id: density\n"
	require.NoError(t, StoreUserTheme("forest", []byte(raw), false))
	directory, err := os.Stat(UserThemeDirectory)
	require.NoError(t, err)
	require.True(t, directory.IsDir())
	require.Equal(t, os.FileMode(0700), directory.Mode().Perm())
	path := filepath.Join(UserThemeDirectory, "forest.yml")
	stored, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, raw, string(stored))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Equal(t, []string{"forest.yml"}, themesDirectoryEntries(t), "nothing but the theme file is left in the directory")

	// The theme is in use at once, without the directory being read by
	// anything else, and can be selected.
	require.True(t, IsUserTheme("forest"))
	themes, problems, notices := UserThemes()
	require.Equal(t, []string{"forest"}, userThemeNames(themes))
	require.Empty(t, problems)
	require.Equal(t, []ThemeNotice{{File: "forest.yml", Unused: model.ThemeFileUnused{Keys: []string{"options"}}}}, notices)
	SetTheme("forest")
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetAppBodyStyle())

	// A theme file of the same name is kept unless the caller asks to
	// replace it. A replacement keeps the permissions of the file, and the
	// pages follow it when the theme is the selected one.
	replacement := userThemeColors + "  radius: 2px\n"
	require.ErrorIs(t, StoreUserTheme("forest", []byte(replacement), false), ErrThemeStoreExists)
	stored, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, raw, string(stored))
	require.NoError(t, os.Chmod(path, 0640))
	require.NoError(t, StoreUserTheme("forest", []byte(replacement), true))
	stored, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, replacement, string(stored))
	info, err = os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0640), info.Mode().Perm())
	require.Equal(t, userThemeStyle+"--flare-radius:2px;", GetAppBodyStyle())
	require.Equal(t, []string{"forest.yml"}, themesDirectoryEntries(t))
}

// Nothing is written for a name that cannot be a theme file, for content
// that is not a usable theme, or for a file that is too large.
func TestStoreUserThemeRefusals(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	require.NoError(t, os.Remove(UserThemeDirectory))
	require.NoError(t, os.WriteFile("outside.yml", []byte("untouched"), 0600))

	for _, name := range []string{
		"", "Mixed", "under_score", "two words", "forest.yml", ".hidden", "../outside", "..", "a/b", `a\b`, "/etc/passwd",
		"forest\x00", strings.Repeat("a", 33), "blackboard", "onedark", model.ThemeCustomName,
	} {
		require.ErrorIs(t, StoreUserTheme(name, []byte(userThemeColors), true), ErrThemeStoreName, "%q", name)
	}

	var rejection *model.ThemeFileError
	for name, raw := range map[string]string{
		"not YAML":             "contract: 1\ntokens:\n  background: \"#10241b\"\n   primary: x\n",
		"no contract":          strings.TrimPrefix(userThemeColors, "contract: 1\n"),
		"value":                userThemeColors + "  radius: 4em\n",
		"declaration":          userThemeColors + "  font-family: \"x;background:url(//example.test/)\"\n",
		"missing color":        "contract: 1\ntokens:\n  background: \"#10241b\"\n",
		"contrast":             "contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#f2b84b\"\n",
		"empty":                "",
		"another kind of file": "<!doctype html><script>alert(1)</script>",
	} {
		err := StoreUserTheme("forest", []byte(raw), false)
		require.ErrorAs(t, err, &rejection, name)
	}

	large := userThemeColors + "#" + strings.Repeat("x", MaximumThemeFileBytes)
	require.ErrorIs(t, StoreUserTheme("forest", []byte(large), false), ErrThemeStoreSize)
	atLimit := userThemeColors + "#" + strings.Repeat("x", MaximumThemeFileBytes-len(userThemeColors)-1)
	require.Len(t, atLimit, MaximumThemeFileBytes)

	// None of the refusals made the directory or wrote anything.
	require.Nil(t, themesDirectoryEntries(t))
	untouched, err := os.ReadFile("outside.yml")
	require.NoError(t, err)
	require.Equal(t, "untouched", string(untouched))
	require.NoError(t, StoreUserTheme("forest", []byte(atLimit), false))
	require.Equal(t, []string{"forest.yml"}, themesDirectoryEntries(t))
}

// What has the name of the theme file but is not a file is not for an
// upload to remove, and nothing outside the directory is written through it.
func TestStoreUserThemeKeepsWhatIsNotAThemeFile(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	outside := filepath.Join(t.TempDir(), "outside.yml")
	require.NoError(t, os.WriteFile(outside, []byte("untouched"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(UserThemeDirectory, "linked.yml")))
	require.NoError(t, os.Mkdir(filepath.Join(UserThemeDirectory, "folder.yml"), 0700))

	for _, name := range []string{"linked", "folder"} {
		for _, replace := range []bool{false, true} {
			require.ErrorIs(t, StoreUserTheme(name, []byte(userThemeColors), replace), ErrThemeStoreExists, "%s replace=%v", name, replace)
		}
	}
	untouched, err := os.ReadFile(outside)
	require.NoError(t, err)
	require.Equal(t, "untouched", string(untouched))
	link, err := os.Lstat(filepath.Join(UserThemeDirectory, "linked.yml"))
	require.NoError(t, err)
	require.NotZero(t, link.Mode()&os.ModeSymlink, "the link was replaced")
	require.Equal(t, []string{"folder.yml", "linked.yml"}, themesDirectoryEntries(t))

	// Something else under the name of the directory cannot be written into.
	require.NoError(t, os.RemoveAll(UserThemeDirectory))
	require.NoError(t, os.WriteFile(UserThemeDirectory, []byte("a file"), 0600))
	require.ErrorIs(t, StoreUserTheme("forest", []byte(userThemeColors), false), ErrThemeStoreWrite)
	content, err := os.ReadFile(UserThemeDirectory)
	require.NoError(t, err)
	require.Equal(t, "a file", string(content))
	require.NoError(t, os.Remove(UserThemeDirectory))
}

// A new theme file is stored only while it will be read.
func TestStoreUserThemeIntoAFullDirectory(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	for i := range MaximumUserThemeFiles - 1 {
		writeUserTheme(t, fmt.Sprintf("t%02d.yml", i), userThemeColors)
	}
	// Files that are reported instead of used count as well.
	writeUserTheme(t, "notes.yaml", userThemeColors)
	writeUserTheme(t, ".hidden.yml", userThemeColors)
	writeUserTheme(t, "README.md", "notes")

	require.ErrorIs(t, StoreUserTheme("one-more", []byte(userThemeColors), false), ErrThemeStoreFull)
	require.NoFileExists(t, filepath.Join(UserThemeDirectory, "one-more.yml"))
	// Replacing one of the files adds none.
	require.NoError(t, StoreUserTheme("t00", []byte(userThemeColors+"  radius: 2px\n"), true))
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "notes.yaml")))
	require.NoError(t, StoreUserTheme("one-more", []byte(userThemeColors), false))
	require.True(t, IsUserTheme("one-more"))
}

func TestExportTheme(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	raw := "# A comment of the author.\n" + userThemeColors + "  radius: 10px\n"
	writeUserTheme(t, "forest.yml", raw)
	ReloadUserThemes()

	// A theme file is handed out as it is.
	name, exported, known := ExportTheme("forest")
	require.True(t, known)
	require.Equal(t, "forest.yml", name)
	require.Equal(t, raw, string(exported))

	// A bundled theme is handed out as the file it is defined by, under a
	// name it can be stored under.
	for _, theme := range ThemePalettes {
		name, exported, known = ExportTheme(theme.Name)
		require.True(t, known, theme.Name)
		require.Equal(t, theme.Name+"-copy.yml", name)
		copyName := strings.TrimSuffix(name, ".yml")
		require.NoError(t, StoreUserTheme(copyName, exported, false), theme.Name)
		themeMu.RLock()
		copied := userThemeState.themes[copyName]
		themeMu.RUnlock()
		require.Equal(t, theme.Colors, copied.Colors, theme.Name)
		require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, name)))
	}

	// The custom colors are written as a theme file once there are any.
	_, _, known = ExportTheme(model.ThemeCustomName)
	require.False(t, known)
	palette := model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}
	require.NoError(t, SetCustomTheme(palette))
	name, exported, known = ExportTheme(model.ThemeCustomName)
	require.True(t, known)
	require.Equal(t, "custom-colors.yml", name)
	require.NoError(t, StoreUserTheme("custom-colors", exported, false))
	themeMu.RLock()
	copied := userThemeState.themes["custom-colors"]
	themeMu.RUnlock()
	require.Equal(t, palette, copied.Colors)

	// What is not a theme in use is not handed out, and neither is anything
	// that is no longer a file of the directory.
	for _, other := range []string{"", "missing", "../config", "forest.yml", "Forest", "config"} {
		_, _, known = ExportTheme(other)
		require.False(t, known, "%q", other)
	}
	path := filepath.Join(UserThemeDirectory, "forest.yml")
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\nSecret: value\n"), 0600))
	require.NoError(t, os.Symlink(filepath.Join("..", "config.yml"), path))
	_, exported, known = ExportTheme("forest")
	require.False(t, known, "a link was followed: %s", exported)
	// A link to another file of the directory is not followed either: the
	// theme file is the file itself.
	require.NoError(t, os.Remove(path))
	writeUserTheme(t, "inner.yml", userThemeColors)
	require.NoError(t, os.Symlink("inner.yml", path))
	_, exported, known = ExportTheme("forest")
	require.False(t, known, "a link inside the directory was followed: %s", exported)
}

// Themes are stored while pages are rendered. A page gets the style of one
// of the stored files, never a file that is half written.
func TestStoreUserThemeIsSafeForConcurrentPages(t *testing.T) {
	userThemeDirectory(t, "forest")
	require.NoError(t, StoreUserTheme("forest", []byte(userThemeColors), false))
	valid := map[string]bool{string(userThemeStyle): true}
	contents := make([][]byte, 0, 6)
	for radius := range 6 {
		contents = append(contents, []byte(fmt.Sprintf("%s  radius: %dpx\n", userThemeColors, radius)))
		valid[string(userThemeStyle)+fmt.Sprintf("--flare-radius:%dpx;", radius)] = true
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
				if style := string(GetAppBodyStyle()); !valid[style] {
					t.Errorf("a page got the style %q", style)
					return
				}
				ExportTheme("forest")
			}
		})
	}
	for round := range 120 {
		require.NoError(t, StoreUserTheme("forest", contents[round%len(contents)], true))
	}
	close(stop)
	readers.Wait()
	require.Equal(t, []string{"forest.yml"}, themesDirectoryEntries(t))
}
