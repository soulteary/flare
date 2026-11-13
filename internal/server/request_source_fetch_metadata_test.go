package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pages/home"
	"github.com/soulteary/flare/internal/requestsource"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

var sourceTestToken = regexp.MustCompile(`name="?csrf-token"? value="?([A-Za-z0-9_.-]+)`)

// sourceBrowser drives the complete production stack (persistent store, source
// protection, authentication and handlers) with explicit request headers.
type sourceBrowser struct {
	t       *testing.T
	app     *fiber.App
	host    string
	cookies map[string]*http.Cookie
}

type sourceResponse struct {
	status      int
	contentType string
	requestID   string
	cache       string
	body        string
}

func newSourceBrowser(t *testing.T, locale string, configure func(*model.Flags)) (*sourceBrowser, *security.Store) {
	t.Helper()
	originalFlags := define.AppFlags
	originalTheme, originalColor := define.ThemeCurrent, define.ThemePrimaryColor
	t.Cleanup(func() {
		define.AppFlags = originalFlags
		define.ThemeCurrent, define.ThemePrimaryColor = originalTheme, originalColor
	})
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: "+locale+"\nShowSearchComponent: true\nShowWeather: false\nTheme: blackboard\nTitle: original\n"), 0600))
	flags := &model.Flags{Port: 5005, CookieName: "source", Visibility: "DEFAULT", StartupStatus: "ready", DisableLoginMode: true, EnableOfflineMode: true}
	if configure != nil {
		configure(flags)
	}
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	if flags.DisableLoginMode {
		require.NoError(t, store.Initialize("", "", security.ModeTrusted))
	} else {
		require.NoError(t, store.Initialize("admin", "source test password", "local"))
	}
	app, err := NewFiberApp(flags, store)
	require.NoError(t, err)
	return &sourceBrowser{t: t, app: app, host: "flare.example", cookies: map[string]*http.Cookie{}}, store
}

func (b *sourceBrowser) do(method, path string, form url.Values, headers http.Header) sourceResponse {
	b.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, "http://"+b.host+path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range b.cookies {
		req.AddCookie(cookie)
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	res, err := b.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(b.t, err)
	for _, cookie := range res.Cookies() {
		b.cookies[cookie.Name] = cookie
	}
	raw, err := io.ReadAll(res.Body)
	require.NoError(b.t, err)
	require.NoError(b.t, res.Body.Close())
	return sourceResponse{status: res.StatusCode, contentType: res.Header.Get("Content-Type"), requestID: res.Header.Get("X-Request-ID"), cache: res.Header.Get("Cache-Control"), body: string(raw)}
}

// token loads a management form and returns its CSRF token; the browser keeps
// the cookie that the token is bound to.
func (b *sourceBrowser) token() string {
	b.t.Helper()
	page := b.do(http.MethodGet, define.SettingPages.Appearance.Path, nil, nil)
	require.Equal(b.t, http.StatusOK, page.status, page.body)
	match := sourceTestToken.FindStringSubmatch(page.body)
	require.Len(b.t, match, 2, "management form must issue a token")
	return match[1]
}

// save submits the appearance form with a title that identifies the attempt.
func (b *sourceBrowser) save(token, title string, headers http.Header) sourceResponse {
	b.t.Helper()
	return b.do(http.MethodPost, define.SettingPages.Appearance.Path, url.Values{"csrf-token": {token}, "title": {title}}, headers)
}

func (b *sourceBrowser) saved(title string) bool {
	b.t.Helper()
	raw, err := os.ReadFile("config.yml")
	require.NoError(b.t, err)
	return strings.Contains(string(raw), title)
}

type sourceRejection struct {
	Error     struct{ Code, Reason, Message, Hint string } `json:"error"`
	RequestID string                                       `json:"request_id"`
}

func (b *sourceBrowser) requireRejected(response sourceResponse, status int, code, reason string) {
	b.t.Helper()
	require.Equal(b.t, status, response.status, response.body)
	require.Equal(b.t, "private, no-store", response.cache)
	require.NotEmpty(b.t, response.requestID)
	var rejection sourceRejection
	require.NoError(b.t, json.Unmarshal([]byte(response.body), &rejection), response.body)
	require.Equal(b.t, code, rejection.Error.Code)
	require.Equal(b.t, reason, rejection.Error.Reason)
	require.NotEmpty(b.t, rejection.Error.Message)
	require.NotEmpty(b.t, rejection.Error.Hint)
	require.Equal(b.t, response.requestID, rejection.RequestID)
}

// The reports behind this change: a TLS-terminating reverse proxy in front of a
// default configuration. The browser origin is HTTPS; this process sees HTTP.
func TestDefaultConfigurationAcceptsWritesBehindTLSTermination(t *testing.T) {
	for _, tc := range []struct {
		name, host, reason string
		headers            http.Header
	}{
		{name: "current browser, Host preserved", host: "flare.example", headers: http.Header{"Origin": {"https://flare.example"}, "Sec-Fetch-Site": {"same-origin"}}},
		{name: "browser without fetch metadata, Host preserved", host: "flare.example", headers: http.Header{"Origin": {"https://flare.example"}}},
		{name: "current browser, Host rewritten", host: "backend:5005", headers: http.Header{"Origin": {"https://flare.example"}, "Sec-Fetch-Site": {"same-origin"}}},
		{name: "current browser, public port dropped", host: "flare.example", headers: http.Header{"Origin": {"https://flare.example:8443"}, "Sec-Fetch-Site": {"same-origin"}}},
		{name: "browser without fetch metadata, Host rewritten", host: "backend:5005", reason: "ORIGIN_HOST_MISMATCH", headers: http.Header{"Origin": {"https://flare.example"}}},
		{name: "browser without fetch metadata, public port dropped", host: "flare.example", reason: "ORIGIN_PORT_MISMATCH", headers: http.Header{"Origin": {"https://flare.example:8443"}}},
		{name: "cross-site page", host: "flare.example", reason: "FETCH_SITE_NOT_SAME_ORIGIN", headers: http.Header{"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}}},
		{name: "sibling site", host: "flare.example", reason: "FETCH_SITE_NOT_SAME_ORIGIN", headers: http.Header{"Origin": {"https://other.flare.example"}, "Sec-Fetch-Site": {"same-site"}}},
		{name: "foreign origin without fetch metadata", host: "flare.example", reason: "ORIGIN_HOST_MISMATCH", headers: http.Header{"Origin": {"https://attacker.example"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			browser, _ := newSourceBrowser(t, "en", nil)
			browser.host = tc.host
			token := browser.token()
			response := browser.save(token, "written-by-test", tc.headers)
			if tc.reason == "" {
				require.Equal(t, http.StatusOK, response.status, response.body)
				require.True(t, browser.saved("written-by-test"))
				return
			}
			browser.requireRejected(response, http.StatusForbidden, "SOURCE_REJECTED", tc.reason)
			require.False(t, browser.saved("written-by-test"), "rejected write changed config")
			for _, values := range tc.headers {
				require.NotContains(t, response.body, values[0], "diagnostics must not echo request headers")
			}
		})
	}
}

// Accepting the browser's same-origin verdict does not replace the CSRF token.
func TestFetchMetadataDoesNotReplaceCSRFToken(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", nil)
	token := browser.token()
	sameOrigin := http.Header{"Origin": {"https://flare.example"}, "Sec-Fetch-Site": {"same-origin"}}
	for _, forged := range []string{"", "forged"} {
		response := browser.save(forged, "written-without-token", sameOrigin)
		require.Equal(t, http.StatusForbidden, response.status)
		require.Contains(t, response.body, "CSRF_INVALID")
	}
	require.False(t, browser.saved("written-without-token"))
	require.Equal(t, http.StatusOK, browser.save(token, "written-with-token", sameOrigin).status)
	require.True(t, browser.saved("written-with-token"))
}

// An explicit HTTPS policy issues Secure cookies. A page opened over HTTP is
// told so, unless the browser itself vouches for a same-origin request.
func TestExplicitHTTPSPolicyRejectsHTTPPages(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", func(flags *model.Flags) { flags.CookieSecure = true })
	token := browser.token()
	response := browser.save(token, "written-over-http", http.Header{"Origin": {"http://flare.example"}})
	browser.requireRejected(response, http.StatusForbidden, "SOURCE_REJECTED", "ORIGIN_SCHEME_MISMATCH")
	require.Contains(t, response.body, "FLARE_COOKIE_SECURE")
	require.False(t, browser.saved("written-over-http"))
	require.Equal(t, http.StatusOK, browser.save(token, "written-over-https", http.Header{"Origin": {"https://flare.example"}}).status)
	require.True(t, browser.saved("written-over-https"))
}

func TestExternalURLNamesTheOriginWithoutRestrictingHosts(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", func(flags *model.Flags) { flags.ExternalURL = "https://flare.example" })
	for _, host := range []string{"flare.example", "192.168.1.10:5005", "backend:5005"} {
		browser.host = host
		require.Equal(t, http.StatusOK, browser.do(http.MethodGet, define.RegularPages.Home.Path, nil, nil).status, host)
	}
	// The external origin is a known source even when a proxy rewrote Host.
	browser.host = "backend:5005"
	token := browser.token()
	require.Equal(t, http.StatusOK, browser.save(token, "written-through-proxy", http.Header{"Origin": {"https://flare.example"}}).status)
	require.True(t, browser.saved("written-through-proxy"))
	response := browser.save(token, "written-by-foreign-origin", http.Header{"Origin": {"https://attacker.example"}})
	browser.requireRejected(response, http.StatusForbidden, "SOURCE_REJECTED", "ORIGIN_HOST_MISMATCH")
	require.False(t, browser.saved("written-by-foreign-origin"))
}

func TestAllowedHostsRestrictEveryMethod(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", func(flags *model.Flags) {
		flags.ExternalURL = "https://flare.example"
		flags.AllowedHosts = "lan.example:5005"
	})
	for _, host := range []string{"flare.example", "lan.example:5005"} {
		browser.host = host
		require.Equal(t, http.StatusOK, browser.do(http.MethodGet, define.RegularPages.Home.Path, nil, nil).status, host)
	}
	browser.host = "192.168.1.10:5005"
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		response := browser.do(method, define.RegularPages.Home.Path, nil, nil)
		if method == http.MethodHead {
			require.Equal(t, http.StatusBadRequest, response.status)
			continue
		}
		browser.requireRejected(response, http.StatusBadRequest, "INVALID_INPUT", "HOST_NOT_ALLOWED")
		require.NotContains(t, response.body, "192.168.1.10")
	}
	// The read-only search routes skip the source diagnosis, not the allowlist.
	response := browser.do(http.MethodPost, home.SearchSuggestPath, url.Values{"search": {"flare"}}, nil)
	browser.requireRejected(response, http.StatusBadRequest, "INVALID_INPUT", "HOST_NOT_ALLOWED")
}

// Searching must not depend on the deployment. A cross-origin page cannot read
// the response, and neither route changes anything.
func TestSearchRoutesSkipSourceDiagnosis(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", nil)
	token := browser.token()
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	crossSite := http.Header{"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}}
	search := browser.do(http.MethodPost, define.RegularPages.Home.Path, url.Values{"search": {"flare"}}, crossSite)
	require.Equal(t, http.StatusOK, search.status, search.body)
	// Suggestions are disabled in this configuration; reaching the handler is the point.
	suggest := browser.do(http.MethodPost, home.SearchSuggestPath, url.Values{"search": {"flare"}}, crossSite)
	require.Equal(t, http.StatusNotFound, suggest.status, suggest.body)
	opaque := browser.do(http.MethodPost, define.RegularPages.Home.Path, url.Values{"search": {"flare"}}, http.Header{"Origin": {"null"}})
	require.Equal(t, http.StatusOK, opaque.status, opaque.body)
	// Only the exact POST routes are exempt.
	browser.requireRejected(browser.save(token, "written-cross-site", crossSite), http.StatusForbidden, "SOURCE_REJECTED", "FETCH_SITE_NOT_SAME_ORIGIN")
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
		browser.requireRejected(browser.do(method, define.RegularPages.Home.Path, nil, crossSite), http.StatusForbidden, "SOURCE_REJECTED", "FETCH_SITE_NOT_SAME_ORIGIN")
	}
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after, "search and rejected writes must not change settings")
}

// With PRIVATE visibility the search routes are guarded by auth.AuthRequired,
// which still validates the browser source together with the CSRF token.
func TestPrivateSearchStillValidatesSource(t *testing.T) {
	browser, store := newSourceBrowser(t, "en", func(flags *model.Flags) {
		flags.DisableLoginMode = false
		flags.Visibility = "PRIVATE"
	})
	session, err := store.CreateSession()
	require.NoError(t, err)
	name := auth.RequestHandleSessionName("source", 5005) + "_" + store.Info().InstanceID
	browser.cookies[name] = &http.Cookie{Name: name, Value: session}
	page := browser.do(http.MethodGet, define.RegularPages.Home.Path, nil, nil)
	require.Equal(t, http.StatusOK, page.status, page.body)
	match := sourceTestToken.FindStringSubmatch(page.body)
	require.Len(t, match, 2, "private search form must carry a token")
	form := url.Values{"search": {"flare"}, "csrf-token": {match[1]}}
	require.Equal(t, http.StatusOK, browser.do(http.MethodPost, define.RegularPages.Home.Path, form, http.Header{"Origin": {"https://flare.example"}, "Sec-Fetch-Site": {"same-origin"}}).status)
	for _, headers := range []http.Header{
		{"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}},
		{"Origin": {"https://attacker.example"}},
	} {
		response := browser.do(http.MethodPost, define.RegularPages.Home.Path, form, headers)
		require.Equal(t, http.StatusForbidden, response.status)
		require.Contains(t, response.body, "CSRF_INVALID")
	}
}

func TestRejectedNavigationRendersLocalizedPage(t *testing.T) {
	crossSite := http.Header{"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}}
	for _, tc := range []struct{ locale, lang, title, lead, message, hint, home string }{
		{locale: "en", lang: `lang="en"`, title: "Request not accepted", lead: "Nothing was changed.", message: "The browser marked this write as cross-origin.", hint: "Open Flare and submit the change", home: "Return to the home page"},
		{locale: "zh", lang: `lang="zh-CN"`, title: "请求未被接受", lead: "没有任何内容被修改。", message: "浏览器把这次写入标记为跨源请求。", hint: "请打开 Flare", home: "返回首页"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			browser, _ := newSourceBrowser(t, tc.locale, nil)
			token := browser.token()
			for _, navigation := range []http.Header{
				{"Sec-Fetch-Mode": {"navigate"}},
				{"Accept": {"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"}},
			} {
				headers := crossSite.Clone()
				for name, values := range navigation {
					headers[name] = values
				}
				response := browser.save(token, "written-cross-site", headers)
				require.Equal(t, http.StatusForbidden, response.status)
				require.True(t, strings.HasPrefix(response.contentType, "text/html"), response.contentType)
				require.Equal(t, "private, no-store", response.cache)
				require.NotEmpty(t, response.requestID)
				for _, want := range []string{tc.lang, tc.title, tc.lead, tc.message, tc.hint, tc.home, "FETCH_SITE_NOT_SAME_ORIGIN", response.requestID} {
					require.Contains(t, response.body, want)
				}
				require.NotContains(t, response.body, "attacker.example")
				require.NotContains(t, response.body, "<script")
				require.NotContains(t, response.body, "src=")
			}
			// Script-initiated requests keep the JSON body.
			for _, headers := range []http.Header{crossSite, {"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}, "Accept": {"text/html, application/json"}}} {
				browser.requireRejected(browser.save(token, "written-cross-site", headers), http.StatusForbidden, "SOURCE_REJECTED", "FETCH_SITE_NOT_SAME_ORIGIN")
			}
			require.False(t, browser.saved("written-cross-site"))
		})
	}
}

func TestRejectedAuthorityRendersPageForNavigation(t *testing.T) {
	browser, _ := newSourceBrowser(t, "en", func(flags *model.Flags) { flags.AllowedHosts = "flare.example" })
	browser.host = "other.example"
	response := browser.do(http.MethodGet, define.RegularPages.Home.Path, nil, http.Header{"Accept": {"text/html"}})
	require.Equal(t, http.StatusBadRequest, response.status)
	require.True(t, strings.HasPrefix(response.contentType, "text/html"), response.contentType)
	for _, want := range []string{"Flare could not accept the address this request was sent to.", "HOST_NOT_ALLOWED", "FLARE_ALLOWED_HOSTS", response.requestID} {
		require.Contains(t, response.body, want)
	}
	require.NotContains(t, response.body, "other.example")
}

func TestRejectionPageFallsBackWithoutConfiguration(t *testing.T) {
	browser, _ := newSourceBrowser(t, "zh", nil)
	token := browser.token()
	require.NoError(t, os.Remove("config.yml"))
	response := browser.save(token, "written-cross-site", http.Header{"Sec-Fetch-Site": {"cross-site"}, "Sec-Fetch-Mode": {"navigate"}})
	require.Equal(t, http.StatusForbidden, response.status)
	require.Contains(t, response.body, "Request not accepted")
	_, err := os.Stat("config.yml")
	require.True(t, os.IsNotExist(err), "a rejected request must not create configuration")
}

// Every reason has a translated explanation and action in both languages, and
// an unknown reason still renders the policy's own text.
func TestRejectionCatalogCoversEveryReason(t *testing.T) {
	reasons := []string{
		"HOST_INVALID", "HOST_NOT_ALLOWED", "FORWARDED_HEADER_UNSUPPORTED", "FORWARDED_HEADER_AMBIGUOUS",
		"FORWARDED_FOR_INVALID", "FORWARDED_PROTO_INVALID", "FORWARDED_PROTO_CONFLICT",
		"FETCH_METADATA_AMBIGUOUS", "FETCH_SITE_NOT_SAME_ORIGIN",
	}
	for _, header := range []string{"ORIGIN", "REFERER"} {
		for _, suffix := range []string{"AMBIGUOUS", "OPAQUE", "INVALID", "SCHEME_MISMATCH", "HOST_MISMATCH", "PORT_MISMATCH"} {
			reasons = append(reasons, header+"_"+suffix)
		}
	}
	for _, reason := range reasons {
		key := rejectionKey(reason)
		for _, locale := range []string{"en", "zh"} {
			for _, part := range []string{".message", ".hint"} {
				require.NotEqual(t, key+part, i18n.T(locale, key+part), "missing %s translation for %s", locale, reason)
			}
		}
	}
	require.Equal(t, rejectionKey("ORIGIN_HOST_MISMATCH"), rejectionKey("REFERER_HOST_MISMATCH"))
	page := sourceRejectionPage("en", http.StatusForbidden, &requestsource.Diagnostic{Reason: "FUTURE_<REASON>", Message: "Policy <message>", Hint: "Policy & hint"}, "id")
	require.Contains(t, page, "FUTURE_&lt;REASON&gt;")
	require.Contains(t, page, "Policy &lt;message&gt;")
	require.Contains(t, page, "Policy &amp; hint")
}
