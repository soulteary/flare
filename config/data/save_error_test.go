package data

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestSettingsSavesReportWhyTheyFailed(t *testing.T) {
	alias := "ShowSearchComponent: true\nSearch: &shared\n  Mode: select\n  DefaultTarget: local\nOtherData: *shared\n"
	engines := "ShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n  Engines:\n    - ID: old\n      Name: Old\n      URLTemplate: 'https://old.example/?q={query}'\n      Enabled: true\n      Extension: keep\n"
	renamed := &model.SearchOptions{Mode: "select", DefaultTarget: "local", Engines: []model.SearchEngine{{ID: "new", Name: "Old", URLTemplate: "https://old.example/?q={query}", Enabled: true}}}
	write := func(raw string) func(*testing.T) {
		return func(t *testing.T) { require.NoError(t, os.WriteFile("config.yml", []byte(raw), 0600)) }
	}
	linked := func(t *testing.T) {
		require.NoError(t, os.WriteFile("target.yml", []byte("Title: target\nShowSearchComponent: true\n"), 0600))
		require.NoError(t, os.Symlink("target.yml", "config.yml"))
	}
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T)
		save    func() error
		want    SaveFailureReason
	}{
		{"theme: newer version", write("AppVersion: 3.0.0\nTitle: newer\n"), func() error { return SaveThemeName("changed") }, SaveFailureNewerVersion},
		{"search: newer version", write("AppVersion: 3.0.0\nShowSearchComponent: true\n"), func() error { return SaveSearch(false, false) }, SaveFailureNewerVersion},
		{"appearance: aliases", write(alias), func() error { return SaveAppearance(model.Application{Title: "changed"}) }, SaveFailureSharedNodes},
		{"search: aliases", write(alias), func() error { return UpdateSearchOptions(false, false, nil) }, SaveFailureSharedNodes},
		{"weather: symbolic link", linked, func() error { return SaveSelectedWeather(false, nil) }, SaveFailureNotRegularFile},
		{"search: symbolic link", linked, func() error { return SaveSearch(false, false) }, SaveFailureNotRegularFile},
		{"theme: dangling link", func(t *testing.T) { require.NoError(t, os.Symlink("missing.yml", "config.yml")) }, func() error { return saveAppConfig("config", model.Application{Title: "new"}) }, SaveFailureWrite},
		{"theme: invalid YAML", write("Title: [invalid\n"), func() error { return SaveThemeName("changed") }, SaveFailureConfigUnreadable},
		{"appearance: invalid YAML", write("Title: [invalid\n"), func() error { return SaveAppearance(model.Application{Title: "changed"}) }, SaveFailureConfigUnreadable},
		{"weather: invalid YAML", write("Title: [invalid\n"), func() error { return SaveSelectedWeather(true, nil) }, SaveFailureConfigUnreadable},
		{"search: invalid YAML", write("Title: [invalid\n"), func() error { return SaveSearch(false, false) }, SaveFailureConfigUnreadable},
		{"search options: invalid YAML", write("Title: [invalid\n"), func() error { return UpdateSearchOptions(false, false, nil) }, SaveFailureConfigUnreadable},
		{"direct: duplicate keys", write("Title: one\nTitle: two\n"), func() error { return saveAppConfig("config", model.Application{Title: "new"}) }, SaveFailureConfigUnreadable},
		{"direct: stale revision", write("Title: current\n"), func() error {
			return saveAppConfig("config", model.Application{Title: "new", ConfigRevision: "revision-of-an-older-document"})
		}, SaveFailureConfigChanged},
		{"search options: stale revision", write("ShowSearchComponent: true\n"), func() error {
			return UpdateSearchOptions(false, false, nil, "revision-of-an-older-document")
		}, SaveFailureConfigChanged},
		{"appearance: invalid layout", write("Title: current\n"), func() error {
			return SaveAppearance(model.Application{Homepage: &model.HomepageOptions{DesktopColumns: "seven"}})
		}, SaveFailureInvalidInput},
		{"appearance: invalid clock", write("Title: current\n"), func() error {
			return SaveAppearance(model.Application{Clock: &model.ClockOptions{TimezoneMode: "nowhere"}})
		}, SaveFailureInvalidInput},
		{"appearance: invalid locale", write("Title: current\n"), func() error { return SaveAppearance(model.Application{Locale: "tlh"}) }, SaveFailureInvalidInput},
		{"search options: invalid engine", write("ShowSearchComponent: true\n"), func() error {
			return UpdateSearchOptions(true, false, &model.SearchOptions{Mode: "select", DefaultTarget: "local", Engines: []model.SearchEngine{{ID: "bad", Name: "Bad", URLTemplate: "javascript:alert({query})", Enabled: true}}})
		}, SaveFailureInvalidInput},
		{"search options: engine extension", write(engines), func() error { return UpdateSearchOptions(true, false, renamed) }, SaveFailureEngineExtension},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			t.Chdir(t.TempDir())
			tc.prepare(t)
			before := snapshotConfigTree(t)
			err := tc.save()
			require.Error(t, err)
			require.Equal(t, tc.want, SaveFailureReasonOf(err), err.Error())
			require.Equal(t, before, snapshotConfigTree(t), "a refused save must not change any file")
		})
	}
}

func TestSaveErrorKeepsUnderlyingTextAndInnermostReason(t *testing.T) {
	cause := errors.New("disk detail")
	err := saveError(SaveFailureWrite, cause)
	require.EqualError(t, err, "disk detail")
	require.ErrorIs(t, err, cause)
	// A caller that classifies again must not replace the more specific reason.
	require.Equal(t, SaveFailureWrite, SaveFailureReasonOf(saveError(SaveFailureConfigUnreadable, fmt.Errorf("wrapped: %w", err))))
	require.Equal(t, SaveFailureReason(""), SaveFailureReasonOf(cause))
	require.Equal(t, SaveFailureReason(""), SaveFailureReasonOf(nil))
}

func TestBooleanSaveFunctionsFollowTheClassifiedOnes(t *testing.T) {
	withApplicationVersion(t, "2.0.0")
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("config.yml", []byte("AppVersion: 2.0.0\nTitle: current\nShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n"), 0600))
	require.NoError(t, SaveThemeName("cloud"))
	require.NoError(t, SaveAppearance(model.Application{Title: "renamed", Locale: "en"}))
	require.NoError(t, SaveSearch(false, true))
	location := &model.WeatherLocation{Province: "北京", City: "北京"}
	require.NoError(t, SaveSelectedWeather(true, location))
	require.True(t, UpdateThemeName("blackboard"))
	require.True(t, UpdateSearch(true, false))
	options, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "blackboard", options.Theme)
	require.Equal(t, "renamed", options.Title)
	require.Equal(t, "en", options.Locale)
	require.True(t, options.ShowSearchComponent)
	require.False(t, options.DisabledSearchAutoFocus)
	require.True(t, options.ShowWeather)
	require.Equal(t, location.String(), options.Location)

	require.NoError(t, os.WriteFile("config.yml", []byte("AppVersion: 3.0.0\nTitle: newer\nShowSearchComponent: true\n"), 0600))
	require.False(t, UpdateThemeName("cloud"))
	require.False(t, UpdateAppearance(model.Application{Title: "changed"}))
	require.False(t, UpdateSearch(false, false))
	require.False(t, UpdateSelectedWeather(false, nil))
}
