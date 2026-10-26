package server

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestCategoryNavigationHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("categories:\n- id: alpha\n  title: Alpha\n- id: beta\n  title: Beta\nlinks:\n- name: Alpha match\n  link: https://alpha\n  category: alpha\n- name: Beta match\n  link: https://beta\n  category: beta\n"), 0600))
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.Homepage = &model.HomepageOptions{CategoryNavigation: "tabs"}
	require.True(t, data.UpdateAppearance(options))
	get := func(path string) string {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 200, rec.Code)
		require.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'none'")
		return rec.Body.String()
	}
	output := get("/?category=beta&search=match")
	require.Contains(t, output, ">Beta match<")
	require.NotContains(t, output, ">Alpha match<")
	require.Regexp(t, `name="?category"? value="?beta"?`, output)
	// Category navigation links must not count as matching search results.
	output = get("/?category=beta&search=no-such-entry")
	require.Contains(t, output, "没有找到匹配的书签或应用")
	require.NotContains(t, output, ">Beta match<")
	require.Regexp(t, `name="?category"? value="?beta"?`, output)
	output = get("/bookmarks?category=alpha")
	require.Contains(t, output, ">Alpha match<")
	require.NotContains(t, output, ">Beta match<")
	form := url.Values{"search": {"match"}, "category": {"alpha"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), ">Alpha match<")
	require.NotContains(t, rec.Body.String(), ">Beta match<")
}
