package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// protectWriteSource rejects browser cross-origin writes, including same-site
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
		return c.Status(http.StatusForbidden).SendString("Write request rejected. Use the Flare page on this origin and retry.")
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
		raw := string(values[0])
		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil || raw == "" || strings.TrimSpace(raw) != raw || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || !strings.EqualFold(parsed.Host, string(c.Request().Host())) {
			return reject()
		}
		if header == "Origin" && (parsed.Path != "" || parsed.RawQuery != "" || parsed.ForceQuery) {
			return reject()
		}
	}
	return c.Next()
}
