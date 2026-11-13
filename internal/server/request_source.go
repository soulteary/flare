package server

import (
	"crypto/rand"
	"encoding/hex"
	"html"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/pages/home"
	"github.com/soulteary/flare/internal/requestsource"
)

// protectWriteSource rejects browser cross-origin management writes, including same-site
// sibling origins. Headerless API clients remain compatible; authentication and
// route-specific signed tokens/custom headers are still required independently.
// Host must be preserved by TLS-terminating proxies. Forwarding headers never
// supply trust and HTTP backend transport does not invalidate an HTTPS origin.
func protectWriteSource(c fiber.Ctx) error {
	switch c.Method() {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return c.Next()
	}
	reject := func() error {
		return c.Status(http.StatusForbidden).JSON(fiber.Map{
			"code":  "write_source_rejected",
			"error": "Write request rejected by origin protection. Open the Flare page on this origin and retry. If using a reverse proxy, preserve the original Host header including the public port. Disabling login does not disable origin protection.",
		})
	}
	sites := c.Request().Header.PeekAll("Sec-Fetch-Site")
	if len(sites) > 1 {
		return reject()
	}
	if len(sites) == 1 && string(sites[0]) != "same-origin" && string(sites[0]) != "none" {
		return reject()
	}
	for _, header := range []string{"Origin", "Referer"} {
		values := c.Request().Header.PeekAll(header)
		if len(values) == 0 {
			continue
		}
		if len(values) != 1 {
			return reject()
		}
		if !validWriteSource(string(values[0]), header, string(c.Request().Host())) {
			return reject()
		}
	}
	return c.Next()
}

func validWriteSource(raw, header, host string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || raw == "" || strings.TrimSpace(raw) != raw || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || !strings.EqualFold(parsed.Host, host) {
		return false
	}
	if header == "Origin" && hasOriginPath(parsed) {
		return false
	}
	return true
}

func hasOriginPath(parsed *url.URL) bool {
	return parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery
}

// NewWriteSourceProtection is used by persistent-security deployments. The
// legacy middleware above remains for old integration test fixtures only.
func NewWriteSourceProtection() (fiber.Handler, error) {
	policy, err := requestsource.New(define.AppFlags.ExternalURL, define.AppFlags.AllowedHosts, define.AppFlags.TrustedProxies, define.AppFlags.CookieSecure)
	if err != nil {
		return nil, err
	}
	return func(c fiber.Ctx) error {
		reject := func(status int, code string, diagnostic *requestsource.Diagnostic) error {
			raw := make([]byte, 12)
			_, _ = rand.Read(raw)
			id := hex.EncodeToString(raw)
			c.Set("X-Request-ID", id)
			c.Set("Cache-Control", "private, no-store")
			// Diagnostics are fixed texts chosen by the policy; request header
			// values never reach the log or the response.
			logger.GetLogger().Warn("request source rejected", "code", code, "reason", diagnostic.Reason, "status", status, "request_id", id)
			if documentNavigation(c) {
				return c.Status(status).Type("html", "utf-8").SendString(sourceRejectionPage(rejectionLocale(), status, diagnostic, id))
			}
			return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "reason": diagnostic.Reason, "message": diagnostic.Message, "hint": diagnostic.Hint}, "request_id": id})
		}
		if diagnostic := policy.DiagnoseAuthority(c); diagnostic != nil {
			return reject(http.StatusBadRequest, "INVALID_INPUT", diagnostic)
		}
		switch c.Method() {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return c.Next()
		}
		if readOnlyPost(c) {
			return c.Next()
		}
		if diagnostic := policy.Diagnose(c); diagnostic != nil {
			return reject(http.StatusForbidden, "SOURCE_REJECTED", diagnostic)
		}
		return c.Next()
	}, nil
}

// readOnlyPost reports whether the request targets one of the POST routes that
// only read: the homepage search form and its suggestions. A search must not
// depend on how the instance is deployed, and a cross-origin page gains nothing
// from submitting one, because it cannot read the response. Both handlers must
// remain free of side effects. With PRIVATE visibility they are still guarded by
// auth.AuthRequired, which validates the source and the CSRF token.
func readOnlyPost(c fiber.Ctx) bool {
	if c.Method() != http.MethodPost {
		return false
	}
	path := c.Path()
	return path == define.RegularPages.Home.Path || path == home.SearchSuggestPath
}

// documentNavigation reports whether the browser will display the response as a
// page, as opposed to script-initiated requests that parse the JSON body.
func documentNavigation(c fiber.Ctx) bool {
	if c.Get("Sec-Fetch-Mode") == "navigate" {
		return true
	}
	accept := c.Get(fiber.HeaderAccept)
	return strings.Contains(accept, fiber.MIMETextHTML) && !strings.Contains(accept, fiber.MIMEApplicationJSON)
}

// rejectionLocale reads the site language without creating a missing
// configuration file on behalf of a rejected request.
func rejectionLocale() string {
	if options, err := data.ReadExistingSettingsOptions(); err == nil {
		return options.Locale
	}
	return "en"
}

// rejectionKey maps a diagnostic reason to its translation key. Origin and
// Referer variants of the same failure share one entry.
func rejectionKey(reason string) string {
	for _, prefix := range []string{"ORIGIN_", "REFERER_"} {
		if suffix, ok := strings.CutPrefix(reason, prefix); ok {
			return "source.reason.header_" + strings.ToLower(suffix)
		}
	}
	return "source.reason." + strings.ToLower(reason)
}

// sourceRejectionPage renders a self-contained explanation for a rejected page
// navigation. It loads no external resource and runs no script.
func sourceRejectionPage(locale string, status int, diagnostic *requestsource.Diagnostic, id string) string {
	text := func(key, fallback string) string {
		if translated := i18n.T(locale, key); translated != key {
			return html.EscapeString(translated)
		}
		return html.EscapeString(fallback)
	}
	lead := text("source.rejected", "Flare did not accept this request because it could not confirm that it was sent from a page of this site. Nothing was changed.")
	if status == http.StatusBadRequest {
		lead = text("source.invalid", "Flare could not accept the address this request was sent to.")
	}
	key := rejectionKey(diagnostic.Reason)
	title := text("source.title", "Request not accepted")
	return `<!doctype html><html lang="` + html.EscapeString(i18n.HTMLLang(locale)) + `"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="robots" content="noindex"><title>` + title + `</title>` +
		`<style>body{font:1rem/1.6 system-ui,sans-serif;max-width:40rem;margin:2rem auto;padding:0 1rem;overflow-wrap:anywhere}dt{font-weight:600;margin-top:1rem}dd{margin:0}code{font-family:ui-monospace,monospace}</style></head>` +
		`<body><main><h1>` + title + `</h1><p>` + lead + `</p><p>` + text(key+".message", diagnostic.Message) + `</p><p>` + text(key+".hint", diagnostic.Hint) + `</p>` +
		`<dl><dt>` + text("source.reason_label", "Reason") + `</dt><dd><code>` + html.EscapeString(diagnostic.Reason) + `</code></dd>` +
		`<dt>` + text("source.request_label", "Request ID") + `</dt><dd><code>` + html.EscapeString(id) + `</code></dd></dl>` +
		`<p><a href="` + define.RegularPages.Home.Path + `">` + text("source.home", "Return to the home page") + `</a></p></main></body></html>`
}
