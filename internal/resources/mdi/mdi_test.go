//go:build !fiber

package mdi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/define"
)

// newIconTestRouter isolates the package's startup state and registers the real routes.
// These tests intentionally do not run in parallel because the application flags are global.
func newIconTestRouter(t *testing.T, minimumRequest bool) *echo.Echo {
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
	e := echo.New()
	RegisterRouting(e)
	return e
}

// iconURL extracts the URL a browser would request from the rendered icon markup.
func iconURL(markup string) (string, error) {
	const prefix = `<img src="`
	if !strings.HasPrefix(markup, prefix) {
		return "", fmt.Errorf("expected an image URL, got %q", markup)
	}
	url, _, found := strings.Cut(strings.TrimPrefix(markup, prefix), `"`)
	if !found {
		return "", fmt.Errorf("image URL has no closing quote: %q", markup)
	}
	return url, nil
}

// generatedIconURL renders an existing bundled icon and returns its request URL.
func generatedIconURL(t *testing.T) string {
	t.Helper()
	url, err := iconURL(GetIconByName("serverNetwork"))
	if err != nil {
		t.Fatal(err)
	}
	return url
}

// requestIcon exercises routing and error handling with an optional request header.
func requestIcon(e *echo.Echo, method, url, header, value string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	if header != "" {
		req.Header.Set(header, value)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
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
		if got := head.Header().Get("Content-Type"); got != "image/svg+xml" {
			t.Fatalf("HEAD content type: %q", got)
		}
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

func TestIconRoute_MissingAndInvalidPaths(t *testing.T) {
	e := newIconTestRouter(t, false)
	url := generatedIconURL(t)
	etag := requestIcon(e, http.MethodGet, url, "", "").Header().Get("ETag")
	// Place real content outside the served subtree so traversal cannot pass by accident.
	if err := MemFs.WriteFile("assets/private.svg", []byte("private content"), 0600); err != nil {
		t.Fatalf("prepare outside-subtree file: %v", err)
	}
	paths := []string{
		"/assets/mdi/pumpkin-missing.svg",
		"/assets/mdi/",
		"/assets/mdi/.",
		"/assets/mdi/../private.svg",
		"/assets/mdi/%2e%2e/private.svg",
		"/assets/mdi/%2e%2e%2fprivate.svg",
		"/assets/mdi/..%5cprivate.svg",
		"/assets/mdi/%2Fprivate.svg",
		"/assets/mdi//pumpkin-serverNetwork.svg",
		"/assets/mdi/pumpkin-serverNetwork.svg/extra",
		"/assets/mdipumpkin-serverNetwork.svg",
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
	e := echo.New()
	var reported error
	defaultErrorHandler := echo.DefaultHTTPErrorHandler(false)
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		reported = err
		defaultErrorHandler(c, err)
	}
	e.GET(_ASSETS_WEB_URI+"/:name", iconFileHandler(failingIconFS{err: cause}))
	rec := requestIcon(e, http.MethodGet, "/assets/mdi/pumpkin-serverNetwork.svg", "", "")
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

func TestIconRoute_ThemeChangeKeepsIndependentContents(t *testing.T) {
	e := newIconTestRouter(t, false)
	firstURL := generatedIconURL(t)
	first := requestIcon(e, http.MethodGet, firstURL, "", "")
	define.ThemeCurrent = "night"
	define.ThemePrimaryColor = "#abcdef"
	secondURL := generatedIconURL(t)
	second := requestIcon(e, http.MethodGet, secondURL, "If-None-Match", first.Header().Get("ETag"))
	if firstURL == secondURL || second.Code != http.StatusOK {
		t.Fatalf("theme change did not create a separate icon: %q -> %q, status %d", firstURL, secondURL, second.Code)
	}
	if !strings.Contains(first.Body.String(), "fill:#123456;") || !strings.Contains(second.Body.String(), "fill:#abcdef;") {
		t.Fatalf("icons have incorrect theme colors: first %q, second %q", first.Body.String(), second.Body.String())
	}
	if first.Header().Get("ETag") == second.Header().Get("ETag") {
		t.Fatal("different SVG contents must have different ETags")
	}
	again := requestIcon(e, http.MethodGet, firstURL, "", "")
	if again.Code != http.StatusOK || again.Body.String() != first.Body.String() {
		t.Fatalf("theme switch corrupted the original cached icon: status %d, body %q", again.Code, again.Body.String())
	}
}

func TestGetIconByName_MinimumRequest(t *testing.T) {
	e := newIconTestRouter(t, true)
	markup := GetIconByName("serverNetwork")
	if !strings.HasPrefix(markup, "<svg ") || !strings.Contains(markup, "fill: var(--color-primary);") || strings.Contains(markup, "<img") {
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
	missing := requestIcon(e, http.MethodGet, "/assets/mdi/pumpkin-serverNetwork.svg", "", "")
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
					// The two spellings share icon data but use distinct cache/file keys.
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
