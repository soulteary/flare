package home

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/templates"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func fragmentFixture(tb testing.TB, size int) {
	tb.Helper()
	fixture := model.Bookmarks{Categories: []model.Category{{ID: "public", Name: "Public"}, {ID: "secret", Name: "Secret category", Private: true}}}
	for i := 0; i < size; i++ {
		fixture.Items = append(fixture.Items, model.Bookmark{Name: fmt.Sprintf("public-%d", i), URL: fmt.Sprintf("https://example.com/%d", i), Category: "public"})
	}
	fixture.Items = append(fixture.Items, model.Bookmark{Name: "PRIVATE-FRAGMENT-SENTINEL", URL: "https://secret.example", Category: "secret"})
	apps := model.Bookmarks{Items: append([]model.Bookmark(nil), fixture.Items...)}
	for i := range apps.Items {
		apps.Items[i].Private = apps.Items[i].Category == "secret"
		apps.Items[i].Category = ""
	}
	require.True(tb, data.SaveFavoriteBookmarks(apps))
	require.True(tb, data.SaveNormalBookmarks(fixture))
	tb.Cleanup(func() {
		require.True(tb, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(tb, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
}

func fragmentApp(tb testing.TB, disabled bool, size int) (*fiber.App, *security.Store, *http.Cookie) {
	tb.Helper()
	tb.Chdir(tb.TempDir())
	original, oldCache := define.AppFlags, navigationFragments
	tb.Cleanup(func() { define.AppFlags = original; navigationFragments = oldCache })
	navigationFragments = newNavigationHTMLCache()
	define.AppFlags = model.Flags{Port: 5005, CookieName: "fragment-test", DisableLoginMode: disabled, EnableOfflineMode: true}
	require.NoError(tb, os.WriteFile("config.yml", []byte("ShowApps: true\nShowBookmarks: true\nShowWeather: false\nLocale: en\nTheme: blackboard\n"), 0600))
	fragmentFixture(tb, size)
	store, err := security.Open(tb.TempDir())
	require.NoError(tb, err)
	tb.Cleanup(func() { require.NoError(tb, store.Close()) })
	require.NoError(tb, store.Initialize("admin", "fragment test password", "local"))
	token, err := store.CreateSession()
	require.NoError(tb, err)
	views, err := templates.NewRenderer()
	require.NoError(tb, err)
	app := fiber.New(fiber.Config{Views: views, Immutable: true})
	require.NoError(tb, auth.RequestHandle(app, store))
	RegisterRouting(app)
	cookie := &http.Cookie{Name: auth.RequestHandleSessionName("fragment-test", 5005) + "_" + store.Info().InstanceID, Value: token}
	return app, store, cookie
}

func fragmentRequest(tb testing.TB, app *fiber.App, path string, cookie *http.Cookie) (string, int) {
	tb.Helper()
	req := httptest.NewRequest("GET", "http://flare.test"+path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	response, err := app.Test(req)
	require.NoError(tb, err)
	defer response.Body.Close()
	require.Equal(tb, "private, no-store", response.Header.Get("Cache-Control"))
	raw, err := io.ReadAll(response.Body)
	require.NoError(tb, err)
	return string(raw), response.StatusCode
}

func TestNavigationFragmentsHTTPIdentitiesAndInvalidation(t *testing.T) {
	for _, mode := range []string{"disabled", "anonymous", "authenticated"} {
		t.Run(mode, func(t *testing.T) {
			app, store, token := fragmentApp(t, mode == "disabled", 10)
			var cookie *http.Cookie
			if mode == "authenticated" {
				cookie = token
			}
			// Warm administrator HTML first: anonymous requests must never reuse it.
			if mode == "anonymous" {
				body, status := fragmentRequest(t, app, "/", token)
				require.Equal(t, 200, status)
				require.Contains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
			}
			for _, path := range []string{"/", "/applications", "/bookmarks"} {
				for round := 0; round < 2; round++ {
					body, status := fragmentRequest(t, app, path, cookie)
					require.Equal(t, 200, status)
					require.Contains(t, body, "public-0")
					if mode == "anonymous" {
						require.NotContains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
						require.NotContains(t, body, "Secret category")
					} else {
						require.Contains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
					}
					require.NotContains(t, body, `target="_blank"`)
				}
			}
			require.NotEmpty(t, navigationFragments.entries)
			// External config changes must invalidate even when navigation is unchanged.
			require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: true\nShowBookmarks: true\nShowWeather: false\nOpenAppNewTab: true\nOpenBookmarkNewTab: true\nLocale: en\n"), 0600))
			body, status := fragmentRequest(t, app, "/", cookie)
			require.Equal(t, 200, status)
			require.Contains(t, body, `target="_blank"`)
			// Change both content and its ACL via the normal save/invalidation path.
			require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "NEW-PRIVATE-ONLY", URL: "https://new.example", Private: true}}}))
			require.True(t, data.SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "NEW-PUBLIC", URL: "https://public.example"}}}))
			body, status = fragmentRequest(t, app, "/", cookie)
			require.Equal(t, 200, status)
			require.NotContains(t, body, "public-0")
			require.NotContains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
			require.Contains(t, body, "NEW-PUBLIC")
			if mode == "anonymous" {
				require.NotContains(t, body, "NEW-PRIVATE-ONLY")
			} else {
				require.Contains(t, body, "NEW-PRIVATE-ONLY")
			}
			if mode == "authenticated" {
				require.NoError(t, store.RevokeAll())
				body, status = fragmentRequest(t, app, "/", cookie)
				require.Equal(t, 200, status)
				require.NotContains(t, body, "NEW-PRIVATE-ONLY")
			}
		})
	}
}

func TestNavigationFragmentHitStillChecksBackingFiles(t *testing.T) {
	for _, missing := range []string{"config.yml", "apps.yml", "bookmarks.yml"} {
		t.Run(missing, func(t *testing.T) {
			app, _, cookie := fragmentApp(t, false, 5)
			body, status := fragmentRequest(t, app, "/", cookie)
			require.Equal(t, 200, status)
			require.Contains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
			require.NoError(t, os.Remove(missing))
			body, status = fragmentRequest(t, app, "/", cookie)
			require.Equal(t, 500, status)
			require.NotContains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
			_, err := os.Stat(missing)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestRequestSettingsSnapshot(t *testing.T) {
	t.Chdir(t.TempDir())
	app := fiber.New()
	calls := 0
	app.Get("/", func(c fiber.Ctx) error {
		first, err := requestSettings(c)
		require.NoError(t, err)
		calls++
		if calls == 1 {
			require.NoError(t, os.WriteFile("config.yml", []byte("Title: changed\n"), 0600))
			second, err := requestSettings(c)
			require.NoError(t, err)
			require.Equal(t, first.ConfigRevision, second.ConfigRevision)
			require.Equal(t, "initial", second.Title)
		}
		return c.SendString(first.Title)
	})
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: initial\n"), 0600))
	for _, want := range []string{"initial", "changed"} {
		r, err := app.Test(httptest.NewRequest("GET", "/", nil))
		require.NoError(t, err)
		raw, err := io.ReadAll(r.Body)
		r.Body.Close()
		require.NoError(t, err)
		require.Equal(t, want, string(raw))
	}
}

func TestNavigationFragmentBypassesRequestDependentOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	original := navigationFragments
	navigationFragments = newNavigationHTMLCache()
	t.Cleanup(func() { navigationFragments = original })
	for _, encoded := range []bool{false, true} {
		for _, link := range []string{"https://{host}/{pathname}", "chrome-extension://abc/page", "https://example.com"} {
			require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Link", URL: link}}}))
			view, _, err := data.LoadNavigationViews(true, false, true, false)
			require.NoError(t, err)
			opts := &model.Application{EnableEncryptedLink: encoded}
			info := fn.DynamicURL{Host: "one.test", Pathname: "/one", Href: "http://one.test/one", Origin: "http://one.test"}
			one := navigationFragment(view, opts, true, "", "", "/", &info)
			info.Host = "two.test"
			info.Pathname = "/two"
			info.Href = "http://two.test/two"
			info.Origin = "http://two.test"
			two := navigationFragment(view, opts, true, "", "", "/", &info)
			if encoded || link != "https://example.com" {
				require.NotEqual(t, one, two)
			} else {
				require.Equal(t, one, two)
			}
		}
	}
	require.Len(t, navigationFragments.entries, 1, "only the unencoded static destination should be cached")
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
}

func TestNavigationHTMLCacheBoundsAndSingleBuild(t *testing.T) {
	cache := newNavigationHTMLCache()
	_, _, generation := define.ThemeRenderSnapshot()
	key := navigationFragmentKey{version: 1, themeGeneration: generation}
	var builds atomic.Int32
	start, release := make(chan struct{}), make(chan struct{})
	build := func() template.HTML { builds.Add(1); close(start); <-release; return "rendered" }
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); require.Equal(t, template.HTML("rendered"), cache.get(key, build)) }()
	}
	<-start
	close(release)
	workers.Wait()
	require.EqualValues(t, 1, builds.Load())
	for i := uint64(2); i < 100; i++ {
		key.version = i
		cache.get(key, func() template.HTML { return template.HTML(strings.Repeat("x", 300<<10)) })
	}
	require.LessOrEqual(t, len(cache.entries), navigationCacheEntries)
	require.LessOrEqual(t, cache.bytes, navigationCacheBytes)
	key.version = 101
	cache.get(key, func() template.HTML { return template.HTML(strings.Repeat("x", navigationCacheEntryBytes+1)) })
	_, retained := cache.entries[key]
	require.False(t, retained)
}

func TestRequestSettingsCachesFailureWithinRequest(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: [broken"), 0600))
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		_, first := requestSettings(c)
		require.Error(t, first)
		require.NoError(t, os.WriteFile("config.yml", []byte("Title: repaired\n"), 0600))
		_, second := requestSettings(c)
		require.Equal(t, first, second)
		return c.SendStatus(500)
	})
	r, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 500, r.StatusCode)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "repaired", options.Title)
}

func TestNavigationCacheThemeChangeDuringBuildIsNotRetained(t *testing.T) {
	theme, _ := define.ThemeSnapshot()
	t.Cleanup(func() { define.SetTheme(theme) })
	_, _, generation := define.ThemeRenderSnapshot()
	cache := newNavigationHTMLCache()
	key := navigationFragmentKey{version: 1, themeGeneration: generation}
	value := cache.get(key, func() template.HTML { define.SetTheme("blues"); define.SetTheme(theme); return "old theme build" })
	require.Equal(t, template.HTML("old theme build"), value)
	require.Empty(t, cache.entries, "even returning to the original theme must not retain a mixed-generation build")
}

func TestNavigationFragmentsDoNotCacheCSRFOrBypassStoreFailure(t *testing.T) {
	app, store, cookie := fragmentApp(t, false, 5)
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nLocale: en\n"), 0600))
	first, status := fragmentRequest(t, app, "/", cookie)
	require.Equal(t, 200, status)
	secondToken, err := store.CreateSession()
	require.NoError(t, err)
	secondCookie := *cookie
	secondCookie.Value = secondToken
	second, status := fragmentRequest(t, app, "/", &secondCookie)
	require.Equal(t, 200, status)
	csrf := regexp.MustCompile(`name=csrf-token value="([^"]+)"`)
	a, b := csrf.FindStringSubmatch(first), csrf.FindStringSubmatch(second)
	require.Len(t, a, 2)
	require.Len(t, b, 2)
	require.NotEqual(t, a[1], b[1])
	for _, fragment := range navigationFragments.entries {
		require.NotContains(t, string(fragment), a[1])
		require.NotContains(t, string(fragment), b[1])
	}
	require.NoError(t, store.Close())
	body, status := fragmentRequest(t, app, "/", cookie)
	require.Equal(t, 503, status)
	require.NotContains(t, body, "PRIVATE-FRAGMENT-SENTINEL")
}

func TestNavigationCacheOldBuildCannotReplaceNewVersion(t *testing.T) {
	cache := newNavigationHTMLCache()
	_, _, generation := define.ThemeRenderSnapshot()
	oldKey := navigationFragmentKey{version: 1, themeGeneration: generation}
	newKey := navigationFragmentKey{version: 2, themeGeneration: generation}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		cache.get(oldKey, func() template.HTML { close(started); <-release; return "old" })
	}()
	<-started
	require.Equal(t, template.HTML("new"), cache.get(newKey, func() template.HTML { return "new" }))
	close(release)
	<-done
	require.Equal(t, template.HTML("new"), cache.get(newKey, func() template.HTML { t.Fatal("new version unexpectedly missed"); return "" }))
}
