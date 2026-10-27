package data

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExistingReadersNeverInitializeMissingYAML(t *testing.T) {
	prepareBookmarkHotPath(t)
	invalidateFileCache("config")
	_, settingsErr := ReadExistingSettingsOptions()
	require.Error(t, settingsErr)
	_, _, pairErr := ReadExistingVisibleNavigationPair(true, true, true)
	require.Error(t, pairErr)
	_, appsErr := ReadExistingVisibleFavoriteBookmarks(true)
	require.Error(t, appsErr)
	_, bookmarksErr := ReadExistingVisibleNormalBookmarks(true)
	require.Error(t, bookmarksErr)
	visible, categoryErr := ReadableExistingBookmarkCategory("missing", true)
	require.Error(t, categoryErr)
	require.False(t, visible)
	entries, readErr := os.ReadDir(".")
	require.NoError(t, readErr)
	require.Empty(t, entries, "read-only readers must not create YAML or helper directories")
	// The ordinary APIs retain their established first-use initialization.
	_, settingsErr = GetAllSettingsOptions()
	require.NoError(t, settingsErr)
	apps, bookmarks, pairErr := LoadVisibleNavigationPair(true, true, true)
	require.NoError(t, pairErr)
	require.NotEmpty(t, apps.Items)
	require.NotEmpty(t, bookmarks.Items)
}

func TestExistingReadersMatchAuthorizedViewsAndHandleRemoval(t *testing.T) {
	visibleNavigationFixture(t)
	invalidateFileCache("config")
	_, settingsErr := GetAllSettingsOptions()
	require.NoError(t, settingsErr)
	want, settingsErr := GetAllSettingsOptions()
	require.NoError(t, settingsErr)
	got, existingErr := ReadExistingSettingsOptions()
	require.NoError(t, existingErr)
	require.Equal(t, want, got)
	for _, administrator := range []bool{false, true} {
		apps, bookmarks, pairErr := LoadVisibleNavigationPair(true, true, administrator)
		require.NoError(t, pairErr)
		existingApps, existingBookmarks, existingPairErr := ReadExistingVisibleNavigationPair(true, true, administrator)
		require.NoError(t, existingPairErr)
		require.Equal(t, apps, existingApps)
		require.Equal(t, bookmarks, existingBookmarks)
		singleApps, appsErr := ReadExistingVisibleFavoriteBookmarks(administrator)
		require.NoError(t, appsErr)
		require.Equal(t, apps, singleApps)
		singleBookmarks, bookmarksErr := ReadExistingVisibleNormalBookmarks(administrator)
		require.NoError(t, bookmarksErr)
		require.Equal(t, bookmarks, singleBookmarks)
		visible, categoryErr := ReadableExistingBookmarkCategory("secret", administrator)
		require.NoError(t, categoryErr)
		require.Equal(t, administrator, visible)
	}
	for _, name := range []string{"config", "apps", "bookmarks"} {
		require.NoError(t, os.Remove(name+".yml"))
		invalidateFileCache(name)
	}
	_, existingErr = ReadExistingSettingsOptions()
	require.Error(t, existingErr)
	_, _, existingErr = ReadExistingVisibleNavigationPair(true, true, true)
	require.Error(t, existingErr)
	for _, name := range []string{"config.yml", "apps.yml", "bookmarks.yml"} {
		_, statErr := os.Stat(name)
		require.True(t, os.IsNotExist(statErr), "removed file must remain absent")
	}
}
