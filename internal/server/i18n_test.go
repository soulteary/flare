package server

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSiteLocaleAcrossPagesAndSettings(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() {
		define.AppFlags = original
		data.SaveFavoriteBookmarks(model.Bookmarks{})
		data.SaveNormalBookmarks(model.Bookmarks{})
	})
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, EnableEditor: true})
	require.NoError(t, err)
	for _, locale := range []string{"en", "zh"} {
		options, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		options.Locale = locale
		require.True(t, data.UpdateAppearance(options))
		for _, page := range []struct{ path, en, zh string }{
			{"/settings/appearance", "General", "通用设置"},
			{"/editor", "Save changes", "保存更改"},
			{"/editor/transfer", "Import", "导入"},
		} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, page.path, nil))
			require.Equal(t, 200, rec.Code, page.path)
			want := page.en
			lang := "en"
			if locale == "zh" {
				want = page.zh
				lang = "zh-CN"
			}
			require.Contains(t, rec.Body.String(), want, page.path)
			require.True(t, strings.Contains(rec.Body.String(), `lang="`+lang+`"`) || strings.Contains(rec.Body.String(), `lang=`+lang+`>`), page.path)
		}
	}
	for _, input := range []string{"en-US", "fr", "invalid"} {
		form := url.Values{"locale": {input}}
		req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if input == "en-US" {
			require.Equal(t, 200, rec.Code)
		} else {
			require.Equal(t, 400, rec.Code)
		}
		got, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		require.Equal(t, "en", got.Locale)
	}
}
