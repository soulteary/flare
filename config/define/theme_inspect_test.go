package define

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// An archive is described by its directory, without anything being written:
// what an import would store, and the entries it would leave out.
func TestInspectThemeArchive(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	inspect := func(entries ...archiveEntry) (ThemeArchiveContents, error) {
		packed := packArchive(t, entries...)
		return InspectThemeArchive(bytes.NewReader(packed), int64(len(packed)))
	}
	contents, err := inspect(zeldaArchive("packed/", append(templateEntries("packed/"),
		archiveEntry{name: "packed/templates/notes.txt", content: "notes"}, archiveEntry{name: "packed/NOTICE", content: "a notice"}, archiveEntry{name: "elsewhere.txt", content: "x"})...)...)
	require.NoError(t, err)
	require.Equal(t, ThemeArchiveContents{Stylesheet: true, Assets: 2, Documents: 2, Templates: templateFileNames, Others: []string{"packed/templates/notes.txt", "elsewhere.txt"}}, contents)
	require.Empty(t, allThemeEntries(t))

	contents, err = inspect(archiveEntry{name: "theme.yml", content: userThemeColors})
	require.NoError(t, err)
	require.Equal(t, ThemeArchiveContents{}, contents)
	// The theme file is not read here: whether the theme is accepted is
	// found when it is stored.
	_, err = inspect(archiveEntry{name: "theme.yml", content: "not a theme"})
	require.NoError(t, err)

	_, err = inspect(archiveEntry{name: "theme.css", content: "a{}"})
	require.ErrorIs(t, err, ErrThemeArchiveEmpty)
	_, err = inspect(archiveEntry{name: "a/theme.yml", content: userThemeColors}, archiveEntry{name: "b/theme.yml", content: userThemeColors})
	require.ErrorIs(t, err, ErrThemeArchiveSeveral)
	_, err = inspect(archiveEntry{name: "theme.yml", content: userThemeColors}, archiveEntry{name: "theme.yml", content: userThemeColors})
	require.ErrorIs(t, err, ErrThemeArchiveTwice)
	_, err = InspectThemeArchive(strings.NewReader("contract: 1\n"), 12)
	require.ErrorIs(t, err, ErrThemeArchiveRead)
	_, err = InspectThemeArchive(strings.NewReader(""), model.MaximumThemeArchiveBytes+1)
	require.ErrorIs(t, err, ErrThemeArchiveSize)
	many := make([]archiveEntry, 0, model.MaximumThemeArchiveEntries+1)
	many = append(many, archiveEntry{name: "theme.yml", content: userThemeColors})
	for len(many) <= model.MaximumThemeArchiveEntries {
		many = append(many, archiveEntry{name: "other-" + strings.Repeat("a", len(many)%7) + string(rune('a'+len(many)%26)) + string(rune('a'+len(many)/26%26)) + ".bin", content: "x"})
	}
	_, err = inspect(many...)
	require.ErrorIs(t, err, ErrThemeArchiveEntries)
}

// The content of a theme file is checked as it is when it is stored, and
// nothing is written.
func TestCheckUserThemeFile(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	require.NoError(t, CheckUserThemeFile([]byte(userThemeColors)))
	require.NoError(t, CheckUserThemeFile([]byte("contract: 1\nextends: absent\n")), "the theme below is looked for when the file is stored")
	require.ErrorIs(t, CheckUserThemeFile([]byte(strings.Repeat("a", MaximumThemeFileBytes+1))), ErrThemeStoreSize)
	require.ErrorIs(t, CheckUserThemeFile([]byte("contract: 2\n")), model.ErrThemeContract)
	require.ErrorIs(t, CheckUserThemeFile([]byte(userThemeColors+userThemeBackground)), model.ErrThemeBackgroundAsset)
	require.Empty(t, allThemeEntries(t))

	for name, accepted := range map[string]bool{"zelda": true, "night-2": true, "tron": false, "custom": false, "Zelda": false, "": false, "a/b": false} {
		require.Equal(t, accepted, IsUserThemeName(name), name)
	}
	// What the check accepts is stored, and what it refuses is not.
	require.NoError(t, StoreUserTheme("zelda", []byte(userThemeColors), false))
	require.ErrorIs(t, StoreUserTheme("large", []byte(strings.Repeat("a", MaximumThemeFileBytes+1)), false), ErrThemeStoreSize)
	require.ErrorIs(t, StoreUserTheme("walled", []byte(userThemeColors+userThemeBackground), false), model.ErrThemeBackgroundAsset)
	require.Equal(t, []string{"zelda.yml"}, allThemeEntries(t))
}
