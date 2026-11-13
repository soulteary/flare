package search

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/stretchr/testify/require"
)

type searchFormViews struct{}

func (searchFormViews) Load() error { return nil }
func (searchFormViews) Render(w io.Writer, _ string, binding any, _ ...string) error {
	return json.NewEncoder(w).Encode(binding)
}

func TestSearchFormActionsRetainWorkingState(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.DisableLoginMode = true
	app := fiber.New(fiber.Config{Views: searchFormViews{}})
	app.Post("/form", updateSearchOptions)
	post := func(v url.Values) (int, string) {
		req := httptest.NewRequest("POST", "/form", strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r, err := app.Test(req)
		require.NoError(t, err)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		return r.StatusCode, string(body)
	}
	base := func(action string) url.Values {
		return url.Values{"engine-count": {"1"}, "engine-0-id": {"custom"}, "engine-0-name": {"Custom"}, "engine-0-url": {"https://search.example/?q={query}"}, "engine-0-enabled": {"1"}, "search-mode": {"select"}, "default-target": {"local"}, "action": {action}}
	}
	for _, tc := range []struct{ action, contains string }{
		{"add", `"ID":""`}, {"preset:bing", `"ID":"bing"`}, {"delete:0", `"Engines":null`}, {"up:bad", "invalid engine row"}, {"down:4", "invalid engine row"}, {"test:-1", "invalid engine row"}, {"test:bad", "invalid engine row"}, {"unknown", "Unknown form action"},
	} {
		t.Run(tc.action, func(t *testing.T) {
			status, body := post(base(tc.action))
			require.Equal(t, 200, status)
			require.Contains(t, body, tc.contains)
		})
	}
	v := base("test:0")
	status, _ := post(v)
	require.Equal(t, 303, status)
	v.Set("engine-0-url", "javascript:alert({query})")
	status, body := post(v)
	require.Equal(t, 200, status)
	require.Contains(t, body, "SearchError")
	for _, count := range []string{"-1", "33", "bad"} {
		v = base("save")
		v.Set("engine-count", count)
		_, body = post(v)
		require.Contains(t, body, "at most 32")
	}
	v = base("add")
	v.Set("engine-count", "32")
	_, body = post(v)
	require.Contains(t, body, "at most 32")
	v = base("save")
	v.Set("search-mode", "hybrid")
	v.Del("engine-0-enabled")
	_, body = post(v)
	require.Contains(t, body, `"SearchSaved":false`)
	v = base("save")
	v.Set("engine-0-url", "javascript:alert(1)")
	_, body = post(v)
	require.Contains(t, body, `"SearchSaved":false`)
	// Legacy checkbox-only forms still save autofocus and visibility.
	_, body = post(url.Values{"show-search-component": {"1"}, "auto-focus-setting": {"1"}, "search-auto-focus": {"1"}})
	require.Contains(t, body, `"SearchSaved":true`)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.True(t, options.ShowSearchComponent)
	require.False(t, options.DisabledSearchAutoFocus)
	// Configuration read failures retain submitted rows instead of writing defaults.
	require.NoError(t, os.WriteFile("config.yml", []byte("search: [invalid"), 0600))
	v = base("save")
	v.Set("auto-focus-setting", "1")
	v.Set("config-revision", "pending-revision")
	_, body = post(v)
	require.Contains(t, body, "更改未保存。 无法读取 config.yml")
	require.Contains(t, body, "pending-revision")
	require.Contains(t, body, "Custom")
	v.Set("engine-count", "bad")
	_, body = post(v)
	require.Contains(t, body, "invalid submitted form")
}

func TestSearchRowEditingAndPresetLimits(t *testing.T) {
	for _, tc := range []struct {
		action string
		want   []string
	}{
		{"up:1", []string{"b", "a", "c"}}, {"down:0", []string{"b", "a", "c"}}, {"delete:1", []string{"a", "c"}}, {"up:0", []string{"a", "b", "c"}}, {"down:2", []string{"a", "b", "c"}},
	} {
		opts := model.SearchOptions{Engines: []model.SearchEngine{{ID: "a"}, {ID: "b"}, {ID: "c"}}}
		require.NoError(t, editSearchRow(&opts, tc.action))
		var ids []string
		for _, e := range opts.Engines {
			ids = append(ids, e.ID)
		}
		require.Equal(t, tc.want, ids)
	}
	opts := model.SearchOptions{}
	require.NoError(t, addSearchPreset(&opts, "bing"))
	require.Len(t, opts.Engines, 1)
	require.NoError(t, addSearchPreset(&opts, "bing"))
	require.Len(t, opts.Engines, 1)
	opts.Engines = nil
	for i := 0; i < 32; i++ {
		opts.Engines = append(opts.Engines, model.SearchEngine{ID: strconv.Itoa(i)})
	}
	require.Error(t, addSearchPreset(&opts, "bing"))
	require.Len(t, opts.Engines, 32)
}

func TestSearchSaveFailuresAreExplainedInTheSiteLanguage(t *testing.T) {
	for _, locale := range []string{"zh", "en"} {
		t.Run(locale, func(t *testing.T) {
			t.Chdir(t.TempDir())
			flags := define.AppFlags
			t.Cleanup(func() { define.AppFlags = flags })
			define.AppFlags.DisableLoginMode = true
			// Shared YAML nodes cannot be edited safely from the settings page.
			raw := []byte("Locale: " + locale + "\nShowSearchComponent: true\nSearch: &shared\n  Mode: select\n  DefaultTarget: local\nOtherData: *shared\n")
			require.NoError(t, os.WriteFile("config.yml", raw, 0600))
			app := fiber.New(fiber.Config{Views: searchFormViews{}})
			app.Post("/form", updateSearchOptions)
			want := i18n.T(locale, "settings_save_failed") + " " + i18n.T(locale, "settings_save_failed_yaml_aliases")
			for name, form := range map[string]url.Values{
				"visibility only": {"show-search-component": {"0"}},
				"full form":       {"engine-count": {"0"}, "search-mode": {"select"}, "default-target": {"local"}, "action": {"save"}},
			} {
				req := httptest.NewRequest("POST", "/form", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
				require.NoError(t, err, name)
				var page struct {
					SearchError string
					SearchSaved bool
				}
				require.NoError(t, json.NewDecoder(res.Body).Decode(&page), name)
				require.NoError(t, res.Body.Close())
				require.Equal(t, want, page.SearchError, name)
				require.False(t, page.SearchSaved, name)
			}
			after, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Equal(t, raw, after)
		})
	}
}
