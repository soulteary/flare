package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
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
