package data

import (
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestLocaleNormalizationAndLegacyUpdates(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	t.Cleanup(func() { invalidateFileCache("config") })
	for _, tc := range []struct{ input, want string }{{"en-US", "en"}, {"zh-CN", "zh"}, {"EN_gb", "en"}} {
		require.True(t, UpdateAppearance(model.Application{Locale: tc.input}))
		got, err := GetAllSettingsOptions()
		require.NoError(t, err)
		require.Equal(t, tc.want, got.Locale)
	}
	require.True(t, UpdateAppearance(model.Application{Locale: "en"}))
	require.True(t, UpdateAppearance(model.Application{Title: "legacy form"}))
	got, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "en", got.Locale)
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	for _, locale := range []string{"fr", "bogus", "<script>"} {
		require.False(t, UpdateAppearance(model.Application{Title: "must not save", Locale: locale}))
		after, err := os.ReadFile("config.yml")
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
	}
}
