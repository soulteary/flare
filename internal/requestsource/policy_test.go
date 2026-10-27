package requestsource

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestCanonicalSourcePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, external, hosts, proxies, target, origin, forwardedHost, forwardedProto string
		secure                                                                        bool
		want                                                                          int
	}{
		{name: "default HTTP port", target: "http://flare.test/write", origin: "http://flare.test:80", want: 200},
		{name: "wrong scheme", target: "http://flare.test/write", origin: "https://flare.test", want: 403},
		{name: "secure default port", target: "http://flare.test/write", origin: "https://flare.test:443", secure: true, want: 200},
		{name: "secure downgrade", target: "http://flare.test/write", origin: "http://flare.test", secure: true, want: 403},
		{name: "ipv6", target: "http://[::1]:5005/write", origin: "http://[::1]:5005", want: 200},
		{name: "ipv6 wrong port", target: "http://[::1]:5005/write", origin: "http://[::1]", want: 403},
		{name: "spoofed headers", target: "http://flare.test/write", origin: "https://evil.test", forwardedHost: "evil.test", forwardedProto: "https", want: 403},
		{name: "explicit canonical", external: "https://flare.test", target: "http://flare.test/write", origin: "https://flare.test:443", want: 200},
		{name: "canonical arbitrary host", external: "https://flare.test", target: "http://evil.test/write", origin: "https://flare.test", want: 403},
		{name: "allowed host", hosts: "flare.test", target: "http://flare.test/write", origin: "http://flare.test", want: 200},
		{name: "unlisted host", hosts: "flare.test", target: "http://evil.test/write", origin: "http://evil.test", want: 403},
		{name: "opaque origin", target: "http://flare.test/write", origin: "null", want: 403},
		{name: "same-site rejected", target: "http://flare.test/write", origin: "http://sibling.test", want: 403},
		{name: "origin path", target: "http://flare.test/write", origin: "http://flare.test/path", want: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.external, tc.hosts, tc.proxies, tc.secure)
			require.NoError(t, err)
			app := fiber.New()
			app.Post("/write", func(c fiber.Ctx) error {
				if !p.Valid(c) {
					return c.SendStatus(403)
				}
				return c.SendStatus(200)
			})
			req := httptest.NewRequest("POST", tc.target, nil)
			req.Header.Set("Origin", tc.origin)
			if tc.forwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			}
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tc.want, resp.StatusCode)
		})
	}
}
func TestPolicyRejectsInvalidConfiguration(t *testing.T) {
	for _, external := range []string{"https://user@host", "https://host/path", "file:///tmp", "https://host?token=secret", "https://host:99999"} {
		_, err := New(external, "", "", false)
		require.Error(t, err)
	}
	for _, proxy := range []string{"all", "*", "10.0.0.0/99", "localhost"} {
		_, err := New("", "", proxy, false)
		require.Error(t, err)
	}
	_, err := New("https://flare.test", "flare.test,[::1]:5005", "127.0.0.1,10.0.0.0/8,::1", true)
	require.NoError(t, err)
}
func TestTrustedProxyResolution(t *testing.T) {
	// Fiber's Test adapter uses the unspecified remote address, so trust just that
	// exact address; production uses the real direct socket peer.
	policy, err := New("", "", "0.0.0.0/32", false)
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/write", func(c fiber.Ctx) error {
		if !policy.Valid(c) {
			return c.SendStatus(403)
		}
		return c.SendString(policy.ClientIP(c))
	})
	req := httptest.NewRequest("POST", "http://backend:5005/write", nil)
	req.Header.Set("Origin", "https://flare.test")
	req.Header.Set("X-Forwarded-Host", "flare.test")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	req.Header.Set("X-Forwarded-Host", "flare.test,evil.test")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode)
	req.Header.Set("X-Forwarded-Host", "flare.test")
	req.Header.Set("Forwarded", "host=evil.test;proto=http")
	resp, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode)
}

func TestFixedCanonicalOriginWithExplicitAliases(t *testing.T) {
	p, err := New("https://canonical.test", "alias.test,alias.test:8443", "", false)
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/write", func(c fiber.Ctx) error {
		canonical, e := p.External(c)
		if e != nil {
			return c.SendStatus(400)
		}
		require.Equal(t, "https://canonical.test", canonical.String())
		if !p.Valid(c) {
			return c.SendStatus(403)
		}
		return c.SendStatus(200)
	})
	for _, tc := range []struct {
		host, origin string
		want         int
	}{
		{"canonical.test", "https://canonical.test", 200},
		{"alias.test", "https://alias.test", 200},
		{"alias.test:8443", "https://alias.test:8443", 200},
		{"alias.test", "https://canonical.test", 403},
		{"canonical.test", "https://alias.test", 403},
		{"alias.test", "https://foreign.test", 403},
		{"alias.test", "http://alias.test", 403},
		{"wrong.test", "https://wrong.test", 400},
		{"alias.test:9999", "https://alias.test:9999", 400},
	} {
		req := httptest.NewRequest("POST", "http://"+tc.host+"/write", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-Forwarded-Host", "canonical.test") // untrusted and ignored
		resp, e := app.Test(req)
		require.NoError(t, e)
		require.Equal(t, tc.want, resp.StatusCode, "host=%s origin=%s", tc.host, tc.origin)
	}
}
func TestTrustedProxyAliasKeepsCanonicalOrigin(t *testing.T) {
	p, err := New("https://canonical.test", "alias.test", "0.0.0.0/32", false)
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/write", func(c fiber.Ctx) error {
		u, e := p.External(c)
		if e != nil {
			return c.SendStatus(400)
		}
		require.Equal(t, "https://canonical.test", u.String())
		if !p.Valid(c) {
			return c.SendStatus(403)
		}
		return c.SendStatus(200)
	})
	for _, tc := range []struct {
		forwarded, origin string
		want              int
	}{
		{"alias.test", "https://alias.test", 200},
		{"canonical.test", "https://canonical.test", 200},
		{"alias.test", "https://canonical.test", 403},
		{"evil.test", "https://evil.test", 400},
		{"", "https://canonical.test", 200},
	} {
		req := httptest.NewRequest("POST", "http://backend:5005/write", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-Forwarded-Proto", "https")
		if tc.forwarded != "" {
			req.Header.Set("X-Forwarded-Host", tc.forwarded)
		}
		resp, e := app.Test(req)
		require.NoError(t, e)
		require.Equal(t, tc.want, resp.StatusCode)
	}
}
