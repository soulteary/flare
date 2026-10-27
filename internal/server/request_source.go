package server

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/soulteary/flare/internal/logger"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
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
		reject := func(status int, code string) error {
			raw := make([]byte, 12)
			_, _ = rand.Read(raw)
			id := hex.EncodeToString(raw)
			c.Set("X-Request-ID", id)
			c.Set("Cache-Control", "private, no-store")
			logger.GetLogger().Warn("request source rejected", "code", code, "status", status, "request_id", id)
			return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": code}, "request_id": id})
		}
		if _, err := policy.External(c); err != nil {
			return reject(400, "INVALID_INPUT")
		}
		switch c.Method() {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return c.Next()
		}
		if !policy.Valid(c) {
			return reject(403, "SOURCE_REJECTED")
		}
		return c.Next()
	}, nil
}
