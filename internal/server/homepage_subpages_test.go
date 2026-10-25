package server

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNavigationSubpagesShareAppearanceClasses(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.Homepage = &model.HomepageOptions{MobileBookmarkLayout: "compact", NavigationTextSize: "large", DesktopColumns: "6", HideAppDescriptions: true}
	for _, keepCase := range []bool{false, true} {
		options.KeepLetterCase = keepCase
		require.True(t, data.UpdateAppearance(options))
		for _, path := range []string{"/", "/bookmarks", "/applications"} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, 200, rec.Code)
			for _, class := range []string{"mobile-bookmarks-compact", "navigation-text-large", "desktop-columns-6", "hide-app-descriptions"} {
				require.Regexp(t, `<body[^>]*class=[^>]*`+class, rec.Body.String())
			}
			if keepCase {
				require.NotRegexp(t, `<body[^>]*class=[^>]*app-content-uppercase`, rec.Body.String())
			} else {
				require.Regexp(t, `<body[^>]*class=[^>]*app-content-uppercase`, rec.Body.String())
			}
		}
	}
}
