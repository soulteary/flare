package mdi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
)

func TestCheatSheetRelativeAssets(t *testing.T) {
	e := newIconTestRouter(t, false)
	page := define.RegularPages.Icons.Path
	redirect := requestIcon(e, http.MethodGet, page, "", "")
	if redirect.Code != http.StatusFound || redirect.Header().Get("Location") != page+"/" {
		t.Fatalf("expected directory redirect, got %d %q", redirect.Code, redirect.Header().Get("Location"))
	}
	index := requestIcon(e, http.MethodGet, page+"/", "", "")
	if index.Code != http.StatusOK {
		t.Fatalf("index status: %d", index.Code)
	}
	if !strings.Contains(index.Body.String(), `href="css/materialdesignicons.min.css"`) {
		t.Fatal("missing relative stylesheet link")
	}
	base, err := url.Parse("http://example.test" + redirect.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range []string{"css/materialdesignicons.min.css", "fonts/materialdesignicons-webfont.woff2"} {
		reference, err := url.Parse(asset)
		if err != nil {
			t.Fatal(err)
		}
		response := requestIcon(e, http.MethodGet, base.ResolveReference(reference).String(), "", "")
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("asset %s: status %d, bytes %d", asset, response.Code, response.Body.Len())
		}
	}
}

func TestCheatSheetNamesRenderInFlare(t *testing.T) {
	newIconTestRouter(t, false)
	for _, name := range []string{"accessPoint", "accountBoxEditOutline", "account"} {
		t.Run(name, func(t *testing.T) {
			if GetIconByName(name) == _EMPTY_ICON {
				t.Fatalf("copied name %q did not render on the homepage", name)
			}
			if !strings.Contains(GetIconSVGByName(name), "<path") {
				t.Fatalf("copied name %q did not render an editor preview", name)
			}
		})
	}
}

func TestCheatSheetBundledVersion(t *testing.T) {
	const version = "7.4.47"
	index, err := MdiExampleAssets.ReadFile("mdi-cheat-sheets/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`<span class="version">` + version + `</span>`, `icon.version === '` + version + `'`, `name:"account-box-edit-outline"`} {
		if !strings.Contains(string(index), marker) {
			t.Fatalf("cheat sheet missing current version marker: %s", marker)
		}
	}
	css, err := MdiExampleAssets.ReadFile("mdi-cheat-sheets/css/materialdesignicons.min.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "?v="+version) || !strings.Contains(string(css), ".mdi-account-box-edit-outline::before") {
		t.Fatal("cheat sheet stylesheet does not match the current icon release")
	}
}

// newIconTestRouter isolates the package's startup state and registers the real routes.
// These tests intentionally do not run in parallel because the application flags are global.
func newIconTestRouter(t *testing.T, minimumRequest bool) *fiber.App {
	t.Helper()
	originalFlags := define.AppFlags
	originalTheme := define.ThemeCurrent
	originalColor := define.ThemePrimaryColor
	t.Cleanup(func() {
		define.AppFlags = originalFlags
		define.ThemeCurrent = originalTheme
		define.ThemePrimaryColor = originalColor
	})
	define.AppFlags.EnableMinimumRequest = minimumRequest
	define.ThemeCurrent = "pumpkin"
	define.ThemePrimaryColor = "#123456"
	if err := Init(); err != nil {
		t.Fatalf("initialize MDI resources: %v", err)
	}
	e := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true})
	RegisterRouting(e)
	return e
}

// iconURL extracts the URL a browser would request from the rendered icon
// markup: the file referenced by <use>, without the fragment that selects the
// path inside it.
func iconURL(markup string) (string, error) {
	const prefix = `<use href="`
	_, reference, found := strings.Cut(markup, prefix)
	if !found || !strings.HasPrefix(markup, "<svg ") {
		return "", fmt.Errorf("expected an SVG that references an icon file, got %q", markup)
	}
	reference, _, found = strings.Cut(reference, `"`)
	if !found {
		return "", fmt.Errorf("icon reference has no closing quote: %q", markup)
	}
	url, fragment, found := strings.Cut(reference, "#")
	if !found || fragment != iconFragment {
		return "", fmt.Errorf("icon reference does not select the path: %q", reference)
	}
	return url, nil
}

// generatedIconURL renders an existing bundled icon and returns its request URL.
func generatedIconURL(t *testing.T) string {
	t.Helper()
	url, err := iconURL(GetIconByName("serverNetwork"))
	require.NoError(t, err)
	return url
}

// requestIcon exercises routing and error handling with an optional request header.
func requestIcon(e *fiber.App, method, url, header, value string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	if header != "" {
		req.Header.Set(header, value)
	}
	rec := httptest.NewRecorder()
	response, err := e.Test(req)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	for key, values := range response.Header {
		rec.Header()[key] = values
	}
	rec.WriteHeader(response.StatusCode)
	if _, err := io.Copy(rec.Body, response.Body); err != nil {
		panic(err)
	}
	return rec
}

func TestIconRoute_ServesSVGAndHTTPRequests(t *testing.T) {
	e := newIconTestRouter(t, false)
	url := generatedIconURL(t)
	first := requestIcon(e, http.MethodGet, url, "", "")
	if first.Code != http.StatusOK {
		t.Fatalf("GET generated icon: status %d, body %s", first.Code, first.Body.String())
	}
	if got := first.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Fatalf("unexpected SVG content type: %q", got)
	}
	var svg struct {
		XMLName xml.Name
		Path    struct {
			Data string `xml:"d,attr"`
		} `xml:"path"`
	}
	// Parsing the complete response catches a truncated or consumed in-memory reader.
	if err := xml.Unmarshal(first.Body.Bytes(), &svg); err != nil || svg.XMLName.Local != "svg" || svg.Path.Data == "" {
		t.Fatalf("invalid SVG response: root %q, error %v", svg.XMLName.Local, err)
	}
	body := first.Body.String()
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("successful icon response must have a content ETag")
	}

	assertIconHTTPRequests(t, e, url, body, etag)
}

func TestIconRoute_MissingAndInvalidPaths(t *testing.T) {
	e := newIconTestRouter(t, false)
	url := generatedIconURL(t)
	etag := requestIcon(e, http.MethodGet, url, "", "").Header().Get("ETag")
	// Place real content outside the served subtree so traversal cannot pass by accident.
	if err := MemFs.WriteFile("assets/private.svg", []byte("private content"), 0600); err != nil {
		t.Fatalf("prepare outside-subtree file: %v", err)
	}
	paths := []string{
		"/assets/mdi/missing.svg",
		// Releases up to 0.6.15 wrote one file per theme. Those names are gone.
		"/assets/mdi/pumpkin-servernetwork.svg",
		"/assets/mdi/servernetwork",
		"/assets/mdi/servernetwork.SVG",
		"/assets/mdi/",
		"/assets/mdi/.",
		"/assets/mdi/../private.svg",
		"/assets/mdi/%2e%2e/private.svg",
		"/assets/mdi/%2e%2e%2fprivate.svg",
		"/assets/mdi/..%5cprivate.svg",
		"/assets/mdi/%2Fprivate.svg",
		"/assets/mdi//servernetwork.svg",
		"/assets/mdi/servernetwork.svg/extra",
		"/assets/mdiservernetwork.svg",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			rec := requestIcon(e, http.MethodGet, path, "If-None-Match", etag)
			if rec.Code != http.StatusNotFound {
				t.Errorf("invalid or absent icon must return 404, got %d: %q", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("ETag"); got != "" {
				t.Errorf("missing icon has an ETag: %q", got)
			}
			if strings.Contains(rec.Body.String(), "private content") {
				t.Error("request escaped the MDI subtree")
			}
		})
	}
}

// failingIconFS verifies that storage errors retain their cause through HTTP handling.
type failingIconFS struct {
	err error
}

// Open returns the configured error without exposing any file contents.
func (f failingIconFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: f.err}
}

func TestIconFileHandler_PreservesUnexpectedReadErrors(t *testing.T) {
	cause := errors.New("icon cache unavailable")
	var reported error
	e := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true, ErrorHandler: func(c fiber.Ctx, err error) error {
		reported = err
		return fiber.DefaultErrorHandler(c, err)
	}})
	e.Get(_ASSETS_WEB_URI+"/:name", iconFileHandler(failingIconFS{err: cause}))
	rec := requestIcon(e, http.MethodGet, "/assets/mdi/servernetwork.svg", "", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected read failure must remain a 500, got %d: %q", rec.Code, rec.Body.String())
	}
	if !errors.Is(reported, cause) {
		t.Fatalf("read failure lost its cause: %v", reported)
	}
	if got := rec.Header().Get("ETag"); got != "" {
		t.Fatalf("failed read must not have an ETag: %q", got)
	}
}

// The page fills the referenced path with the theme color, so the file and
// its URL must not depend on the theme or on its colors.
func TestIconFileIsIndependentOfTheme(t *testing.T) {
	e := newIconTestRouter(t, false)
	markup := GetIconByName("serverNetwork")
	url := generatedIconURL(t)
	if !strings.HasPrefix(url, "/assets/mdi/servernetwork.svg?v=") {
		t.Fatalf("unexpected icon URL: %q", url)
	}
	first := requestIcon(e, http.MethodGet, url, "", "")
	if first.Code != http.StatusOK {
		t.Fatalf("GET generated icon: status %d", first.Code)
	}
	body := first.Body.String()
	if !strings.Contains(body, `<path id="`+iconFragment+`" d="`) {
		t.Fatalf("the path cannot be referenced: %q", body)
	}
	for _, color := range []string{"fill", "style", "#123456", "pumpkin"} {
		if strings.Contains(body, color) {
			t.Fatalf("icon file carries theme data (%q): %q", color, body)
		}
	}

	// Another theme, then another color under the same theme name.
	define.ThemeCurrent = "night"
	define.ThemePrimaryColor = "#abcdef"
	if again := GetIconByName("serverNetwork"); again != markup {
		t.Fatalf("theme change altered the icon markup: %q", again)
	}
	define.ThemePrimaryColor = "#fedcba"
	if again := GetIconByName("serverNetwork"); again != markup {
		t.Fatalf("color change altered the icon markup: %q", again)
	}
	cached := requestIcon(e, http.MethodGet, url, "If-None-Match", first.Header().Get("ETag"))
	if cached.Code != http.StatusNotModified {
		t.Fatalf("theme change replaced the icon file: status %d", cached.Code)
	}
	iconCacheMu.RLock()
	entries := len(iconMarkupCache)
	iconCacheMu.RUnlock()
	if entries != 1 {
		t.Fatalf("theme changes created %d cache entries", entries)
	}
}

func TestGetIconByName_ReferencesIconFile(t *testing.T) {
	_ = newIconTestRouter(t, false)
	markup := GetIconByName("serverNetwork")
	pattern := regexp.MustCompile(`^<svg class="icon-ref" viewBox="0 0 24 24" width="68" height="68" style="fill: var\(--flare-icon\);" aria-hidden="true"><use href="/assets/mdi/servernetwork\.svg\?v=[0-9a-f]{64}#i"></use></svg>$`)
	if !pattern.MatchString(markup) {
		t.Fatalf("unexpected reference markup: %q", markup)
	}
	for _, name := range []string{"", "not-a-bundled-icon"} {
		if got := GetIconByName(name); got != "" {
			t.Errorf("invalid icon %q produced markup: %q", name, got)
		}
	}
}

func TestGetIconByName_MinimumRequest(t *testing.T) {
	e := newIconTestRouter(t, true)
	markup := GetIconByName("serverNetwork")
	if !strings.HasPrefix(markup, "<svg ") || !strings.Contains(markup, "fill: var(--flare-icon);") || strings.Contains(markup, "<img") {
		t.Fatalf("minimum-request mode must inline a theme-aware SVG, got %q", markup)
	}
	if again := GetIconByName("serverNetwork"); again != markup {
		t.Fatalf("repeated inline icon changed: %q", again)
	}
	for _, name := range []string{"", "not-a-bundled-icon"} {
		if got := GetIconByName(name); got != "" {
			t.Errorf("invalid icon %q produced markup: %q", name, got)
		}
	}
	missing := requestIcon(e, http.MethodGet, "/assets/mdi/servernetwork.svg", "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("inline rendering unexpectedly created an external SVG: status %d", missing.Code)
	}
}

func TestIconRoute_ConcurrentGenerationAndReads(t *testing.T) {
	for _, minimumRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("minimum_request=%t", minimumRequest), func(t *testing.T) {
			e := newIconTestRouter(t, minimumRequest)
			const workers = 32
			failures := make(chan error, workers)
			var workersDone sync.WaitGroup
			workersDone.Add(workers)
			for worker := range workers {
				go func() {
					defer workersDone.Done()
					// Name variants must share one cache entry and generated file.
					name := "serverNetwork"
					if worker%2 != 0 {
						name = "servernetwork"
					}
					for range 4 {
						markup := GetIconByName(name)
						if minimumRequest {
							if !strings.HasPrefix(markup, "<svg ") || !strings.HasSuffix(markup, "</svg>") {
								failures <- fmt.Errorf("incomplete inline SVG: %q", markup)
								return
							}
							continue
						}
						url, err := iconURL(markup)
						if err != nil {
							failures <- err
							return
						}
						rec := requestIcon(e, http.MethodGet, url, "", "")
						if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "<svg ") || !strings.HasSuffix(rec.Body.String(), "</svg>") {
							failures <- fmt.Errorf("incomplete external SVG: status %d, body %q", rec.Code, rec.Body.String())
							return
						}
					}
				}()
			}
			workersDone.Wait()
			close(failures)
			for err := range failures {
				t.Error(err)
			}
		})
	}
}

func TestIconNamesShareOneCanonicalCacheEntry(t *testing.T) {
	for _, inline := range []bool{false, true} {
		t.Run(fmt.Sprintf("inline=%t", inline), func(t *testing.T) {
			_ = newIconTestRouter(t, inline)
			first := GetIconByName("serverNetwork")
			for _, name := range []string{"servernetwork", "ServerNetwork", "SERVERNETWORK"} {
				if got := GetIconByName(name); got != first {
					t.Fatalf("name variant generated another resource: %s", name)
				}
			}
			iconCacheMu.RLock()
			entries := len(iconMarkupCache)
			iconCacheMu.RUnlock()
			if entries != 1 {
				t.Fatalf("canonical icon created %d cache entries", entries)
			}
		})
	}
}

func TestMixedCaseResourceURLUsesCanonicalIcon(t *testing.T) {
	app := newIconTestRouter(t, false)
	canonical := generatedIconURL(t)
	current := requestIcon(app, http.MethodGet, canonical, "", "")
	mixed := strings.Replace(canonical, "servernetwork.svg", "serverNetwork.svg", 1)
	cached := requestIcon(app, http.MethodGet, mixed, "If-None-Match", current.Header().Get("ETag"))
	if cached.Code != http.StatusNotModified {
		t.Fatalf("mixed-case asset failed: status=%d", cached.Code)
	}
}

func assertIconHTTPRequests(t *testing.T, e *fiber.App, url, body, etag string) {
	t.Helper()
	t.Run("repeated GET", func(t *testing.T) {
		repeated := requestIcon(e, http.MethodGet, url, "", "")
		if repeated.Code != http.StatusOK || repeated.Body.String() != body {
			t.Fatalf("repeated GET lost icon contents: status %d, body %q", repeated.Code, repeated.Body.String())
		}
		if got := repeated.Header().Get("ETag"); got != etag {
			t.Fatalf("unchanged icon ETag changed: %q != %q", got, etag)
		}
	})

	t.Run("HEAD", func(t *testing.T) {
		head := requestIcon(e, http.MethodHead, url, "", "")
		if head.Code != http.StatusOK || head.Body.Len() != 0 {
			t.Fatalf("HEAD must return headers without an SVG body: status %d, bytes %d", head.Code, head.Body.Len())
		}
		if got := head.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
			t.Fatalf("HEAD content length: %q, want %d", got, len(body))
		}
		require.Equal(t, "image/svg+xml", head.Header().Get("Content-Type"))
		if got := head.Header().Get("ETag"); got != etag {
			t.Fatalf("HEAD ETag: %q, want %q", got, etag)
		}
	})

	t.Run("Range", func(t *testing.T) {
		partial := requestIcon(e, http.MethodGet, url, "Range", "bytes=0-31")
		if partial.Code != http.StatusPartialContent || partial.Body.String() != body[:32] {
			t.Fatalf("Range did not return requested bytes: status %d, body %q", partial.Code, partial.Body.String())
		}
		if got, want := partial.Header().Get("Content-Range"), fmt.Sprintf("bytes 0-31/%d", len(body)); got != want {
			t.Fatalf("Content-Range: %q, want %q", got, want)
		}
	})

	t.Run("unsatisfiable Range", func(t *testing.T) {
		partial := requestIcon(e, http.MethodGet, url, "Range", fmt.Sprintf("bytes=%d-", len(body)+1))
		if partial.Code != http.StatusRequestedRangeNotSatisfiable {
			t.Fatalf("unsatisfiable range: status %d, body %q", partial.Code, partial.Body.String())
		}
	})

	t.Run("conditional GET", func(t *testing.T) {
		cached := requestIcon(e, http.MethodGet, url, "If-None-Match", etag)
		if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
			t.Fatalf("conditional GET: status %d, bytes %d", cached.Code, cached.Body.Len())
		}
		if got := cached.Header().Get("ETag"); got != etag {
			t.Fatalf("304 ETag: %q, want %q", got, etag)
		}
	})
}
