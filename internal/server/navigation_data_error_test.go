package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestHiddenNavigationModulesDoNotLoadOrCreateData(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Cleanup(func() { data.SaveFavoriteBookmarks(model.Bookmarks{}); data.SaveNormalBookmarks(model.Bookmarks{}) })
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.ShowApps = false
	options.ShowBookmarks = false
	require.True(t, data.UpdateAppearance(options))
	for _, path := range []string{"/", "/?search=needle", "/applications", "/bookmarks"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 200, recorder.Code)
	}
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		_, err := os.Stat(name)
		require.True(t, os.IsNotExist(err), "hidden module unexpectedly initialized %s", name)
	}
}

func TestMalformedNavigationDataIsVisibleWithoutInternalDetails(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	oldLogger := logger.GetLogger()
	defer logger.SetLogger(oldLogger)
	var logs bytes.Buffer
	logger.SetLogger(slog.New(logger.NewPrettyHandler(&logs, logger.PrettyHandlerOptions{})))
	dir := t.TempDir()
	t.Chdir(dir)
	t.Cleanup(func() { data.SaveFavoriteBookmarks(model.Bookmarks{}); data.SaveNormalBookmarks(model.Bookmarks{}) })
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.Locale = "en"
	options.ShowApps = true
	options.ShowBookmarks = true
	require.True(t, data.UpdateAppearance(options))
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		require.NoError(t, os.WriteFile(name, []byte("links: [sensitive-sentinel"), 0600))
	}
	for _, path := range []string{"/", "/?search=needle", "/applications", "/bookmarks"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 200, recorder.Code)
		require.Regexp(t, `class="?navigation-data-error"? role="?alert"?`, recorder.Body.String())
		require.NotContains(t, recorder.Body.String(), dir)
		require.NotContains(t, recorder.Body.String(), "sensitive-sentinel")
		require.NotContains(t, recorder.Body.String(), "yaml:")
		require.NotContains(t, recorder.Body.String(), "No matching bookmarks or applications.")
	}
	require.Contains(t, logs.String(), "Cannot render navigation module")
	require.Contains(t, logs.String(), `"module":"applications"`)
	require.Contains(t, logs.String(), `"module":"bookmarks"`)
	require.Contains(t, logs.String(), "解析配置文件")
	require.Contains(t, logs.String(), `"error":"`)
	require.Contains(t, logs.String(), "yaml:")
	require.NotContains(t, logs.String(), `"error":{}`)
}
