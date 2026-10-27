package data

import (
	"errors"
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestEditorValidationReportsExactFields(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		doc         EditorDocument
	}{
		{"blank category id", "categories.0.ID", EditorDocument{Categories: []model.Category{{Name: "A"}}}},
		{"duplicate category id", "categories.1.ID", EditorDocument{Categories: []model.Category{{ID: "a", Name: "A"}, {ID: "a", Name: "B"}}}},
		{"blank app name", "apps.0.Name", EditorDocument{Apps: []model.Bookmark{{URL: "http://nas"}}}},
		{"blank bookmark url", "bookmarks.0.URL", EditorDocument{Bookmarks: []model.Bookmark{{Name: "A"}}}},
		{"app category", "apps.0.Category", EditorDocument{Apps: []model.Bookmark{{Name: "A", URL: "http://nas", Category: "a"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEditorDocument(tc.doc)
			var invalid *EditorValidationError
			require.ErrorAs(t, err, &invalid)
			require.Equal(t, tc.field, invalid.Field)
			require.Equal(t, invalid.Message, err.Error())
		})
	}
}

func TestEditorAuthorizationFailurePreservesFiles(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(map[int]string{1: "entry", 2: "after staging"}[failAt], func(t *testing.T) {
			doc := editorFixture(t)
			original, err := os.ReadFile("apps.yml")
			require.NoError(t, err)
			doc.Apps[0].Name = "Changed"
			denied := errors.New("session revoked")
			calls := 0
			_, err = CommitEditorDocumentChecked(doc, func() error {
				calls++
				if calls == failAt {
					return denied
				}
				return nil
			})
			require.ErrorIs(t, err, denied)
			require.Equal(t, failAt, calls)
			raw, err := os.ReadFile("apps.yml")
			require.NoError(t, err)
			require.Equal(t, original, raw)
		})
	}
}

func TestEditorSnapshotRejectsBrokenNavigation(t *testing.T) {
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		t.Run(name, func(t *testing.T) {
			doc := editorFixture(t)
			require.NoError(t, os.WriteFile(name, []byte("links: [broken"), 0600))
			_, err := EditorSnapshot()
			require.Error(t, err)
			doc.Apps[0].Name = "Changed"
			_, err = CommitEditorDocument(doc)
			require.Error(t, err)
		})
	}
}

func TestEditorStagingAndRecoveryRejectUnsafePaths(t *testing.T) {
	t.Chdir(t.TempDir())
	require.Error(t, stageEditorFile(&stagedFile{name: "apps"}))
	require.NoError(t, os.Mkdir("apps.yml", 0700))
	require.Error(t, stageEditorFile(&stagedFile{name: "apps"}))
	require.NoError(t, os.Remove("apps.yml"))
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: []\n"), 0400))
	require.ErrorIs(t, stageEditorFile(&stagedFile{name: "apps"}), ErrBookmarkReadOnly)
	require.NoError(t, os.Symlink(t.TempDir(), ".flare-editor-recovery"))
	_, err := prepareEditorRecovery(nil)
	require.ErrorContains(t, err, "恢复目录无效")
}

func TestSettingsReadErrorsNeverReplaceInvalidConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := []byte("Title: [invalid")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	require.Empty(t, GetThemeName())
	require.False(t, UpdateThemeName("blackboard"))
	require.False(t, UpdateLocation("Beijing"))
	require.False(t, UpdateWeatherAndLocation(true, "Beijing"))
	require.False(t, UpdateSearch(true, false))
	require.False(t, UpdateSelectedWeather(true, nil))
	require.False(t, UpdateAppearance(model.Application{Title: "New"}))
	_, err := RawBookmarkSnapshot("bookmarks")
	require.Error(t, err)
	require.Error(t, UpdateSearchOptions(true, false, nil))
	require.False(t, MigrateWeatherLocation("Beijing", &model.WeatherLocation{ID: "id", Province: "北京", City: "北京"}))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
	_, err = initAppConfig("config.yml")
	require.Error(t, err)
	require.False(t, saveAppConfigToYamlFile("config", model.Application{Title: "New"}))
	require.False(t, createAppConfig("config", "config.yml", model.Application{Title: "New"}))
	require.False(t, saveConfigAtomically("missing/config.yml", []byte("Title: New")))
	require.False(t, saveConfigAtomically(".", []byte("Title: New")))
}
