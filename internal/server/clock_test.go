package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestClockCSPAndSettingsHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	flags := model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, Visibility: "DEFAULT"}
	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 200, rec.Code)
		return rec
	}
	checkHash := func(rec *httptest.ResponseRecorder) {
		scripts := regexp.MustCompile(`<script>([\s\S]*?)</script>`).FindAllStringSubmatch(rec.Body.String(), -1)
		require.Len(t, scripts, 1)
		sum := sha256.Sum256([]byte(scripts[0][1]))
		policy := rec.Header().Get("Content-Security-Policy")
		require.Contains(t, policy, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		require.Contains(t, policy, "script-src-attr 'none'")
		require.NotContains(t, policy, "unsafe-inline")
	}
	rec := get("/")
	require.NotContains(t, rec.Body.String(), "<script>")
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'none'")
	opts, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	opts.Clock = &model.ClockOptions{Dynamic: true, TimezoneMode: "custom", Timezone: "Asia/Shanghai"}
	opts.Greetings = "morning;noon;afternoon;night"
	require.True(t, data.UpdateAppearance(opts))
	for _, path := range []string{"/", "/help"} {
		rec = get(path)
		checkHash(rec)
		require.Contains(t, rec.Body.String(), "data-zone=\"Asia/Shanghai\"")
	}
	rec = get("/")
	require.Contains(t, rec.Body.String(), "data-greetings=\"morning;noon;afternoon;night\"")
	rec = get("/help")
	require.NotContains(t, rec.Body.String(), "data-greetings=")
	rec = get(define.SettingPages.Appearance.Path)
	checkHash(rec)
	require.Regexp(t, `value=custom\s+selected`, rec.Body.String())
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	form := url.Values{"clock-timezone-mode": {"custom"}, "clock-timezone": {"Local"}, "clock-dynamic": {"1"}, "clock-seconds": {"0"}, "title": {"must not save"}}
	req := httptest.NewRequest(http.MethodPost, define.SettingPages.Appearance.Path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	opts.ShowDateTime = false
	require.True(t, data.UpdateAppearance(opts))
	rec = get("/")
	require.NotContains(t, rec.Body.String(), "id=\"clock\"")
	require.NotContains(t, rec.Body.String(), "<script>")
}

func TestClockDebugTemplates(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.Symlink(filepath.Join(root, "embed"), "embed"))
	flags := model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, DebugMode: true}
	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	opts, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	opts.ShowDateTime = true
	opts.Clock = &model.ClockOptions{Dynamic: true, TimezoneMode: "custom", Timezone: "UTC"}
	require.True(t, data.UpdateAppearance(opts))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, rec.Code)
	scripts := regexp.MustCompile(`<script>([\s\S]*?)</script>`).FindStringSubmatch(rec.Body.String())
	require.Len(t, scripts, 2)
	sum := sha256.Sum256([]byte(scripts[1]))
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), base64.StdEncoding.EncodeToString(sum[:]))
}
