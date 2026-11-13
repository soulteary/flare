package requestsource

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type diagnosis struct {
	authority, source *Diagnostic
	external          string
	externalErr       error
}

// diagnose sends one request through a policy and returns what it decided. The
// Fiber test adapter reports the unspecified address as the peer.
func diagnose(t *testing.T, p *Policy, method, host string, headers http.Header) diagnosis {
	t.Helper()
	var result diagnosis
	app := fiber.New()
	app.All("/write", func(c fiber.Ctx) error {
		result.authority = p.DiagnoseAuthority(c)
		result.source = p.Diagnose(c)
		if u, err := p.External(c); err != nil {
			result.externalErr = err
		} else {
			result.external = u.String()
		}
		return c.SendStatus(http.StatusNoContent)
	})
	req := httptest.NewRequest(method, "http://flare.test/write", nil)
	req.Host = host
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	return result
}

func requireReason(t *testing.T, want string, got *Diagnostic, headers http.Header) {
	t.Helper()
	if want == "" {
		require.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	require.Equal(t, want, got.Reason)
	require.NotEmpty(t, got.Message)
	require.NotEmpty(t, got.Hint)
	// Diagnostics are fixed texts: no header value may be echoed.
	for _, values := range headers {
		for _, value := range values {
			if len(value) > 8 {
				require.NotContains(t, got.Message, value)
				require.NotContains(t, got.Hint, value)
			}
		}
	}
}

func TestFetchMetadataDecidesBrowserWrites(t *testing.T) {
	for _, tc := range []struct {
		name, host, reason string
		secure             bool
		headers            http.Header
	}{
		// The browser computed these from the public URL, so a scheme, Host or
		// port rewritten by a reverse proxy does not matter.
		{name: "TLS terminated upstream", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"https://flare.test"}}},
		{name: "host rewritten by proxy", host: "backend:5005", headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"https://flare.test"}}},
		{name: "public port dropped by proxy", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"https://flare.test:8443"}}},
		{name: "opaque origin from referrer policy", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"null"}}},
		{name: "referer is not consulted", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Referer": {"https://other.test/page"}}},
		{name: "user initiated", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {"none"}}},
		{name: "explicit HTTPS policy", host: "flare.test", secure: true, headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"https://flare.test"}}},
		// Browsers also send fetch metadata to loopback HTTP; their verdict stands.
		{name: "loopback HTTP under explicit HTTPS policy", host: "localhost:5005", secure: true, headers: http.Header{"Sec-Fetch-Site": {"same-origin"}, "Origin": {"http://localhost:5005"}}},
		{name: "cross-site", host: "flare.test", reason: "FETCH_SITE_NOT_SAME_ORIGIN", headers: http.Header{"Sec-Fetch-Site": {"cross-site"}, "Origin": {"https://flare.test"}}},
		{name: "same-site sibling", host: "flare.test", reason: "FETCH_SITE_NOT_SAME_ORIGIN", headers: http.Header{"Sec-Fetch-Site": {"same-site"}}},
		{name: "unknown value", host: "flare.test", reason: "FETCH_SITE_NOT_SAME_ORIGIN", headers: http.Header{"Sec-Fetch-Site": {"Same-Origin"}}},
		{name: "duplicate metadata", host: "flare.test", reason: "FETCH_METADATA_AMBIGUOUS", headers: http.Header{"Sec-Fetch-Site": {"same-origin", "same-origin"}}},
		// An empty value is handled like an absent header.
		{name: "empty metadata, foreign origin", host: "flare.test", reason: "ORIGIN_HOST_MISMATCH", headers: http.Header{"Sec-Fetch-Site": {""}, "Origin": {"http://evil.test"}}},
		{name: "empty metadata, matching origin", host: "flare.test", headers: http.Header{"Sec-Fetch-Site": {""}, "Origin": {"http://flare.test"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New("", "", "", tc.secure)
			require.NoError(t, err)
			result := diagnose(t, p, http.MethodPost, tc.host, tc.headers)
			require.Nil(t, result.authority)
			requireReason(t, tc.reason, result.source, tc.headers)
			require.Equal(t, tc.reason == "", p.validForTest(t, tc.host, tc.headers))
		})
	}
}

// validForTest exercises Valid through a separate request.
func (p *Policy) validForTest(t *testing.T, host string, headers http.Header) bool {
	t.Helper()
	valid := false
	app := fiber.New()
	app.Post("/write", func(c fiber.Ctx) error { valid = p.Valid(c); return c.SendStatus(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "http://flare.test/write", nil)
	req.Host = host
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return valid
}

func TestSourceHeadersWithoutFetchMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, host, reason string
		secure             bool
		headers            http.Header
	}{
		{name: "no browser headers", host: "flare.test"},
		{name: "duplicate origin", host: "flare.test", reason: "ORIGIN_AMBIGUOUS", headers: http.Header{"Origin": {"http://flare.test", "http://flare.test"}}},
		{name: "empty origin", host: "flare.test", reason: "ORIGIN_INVALID", headers: http.Header{"Origin": {""}}},
		{name: "unsupported origin scheme", host: "flare.test", reason: "ORIGIN_INVALID", headers: http.Header{"Origin": {"ftp://flare.test"}}},
		{name: "origin with query", host: "flare.test", reason: "ORIGIN_INVALID", headers: http.Header{"Origin": {"http://flare.test?x=1"}}},
		{name: "explicit default port on Host", host: "flare.test:443", headers: http.Header{"Origin": {"https://flare.test"}}},
		{name: "explicit default port on Origin", host: "flare.test", headers: http.Header{"Origin": {"https://flare.test:443"}}},
		{name: "other scheme default port is not implied", host: "flare.test:80", reason: "ORIGIN_PORT_MISMATCH", headers: http.Header{"Origin": {"https://flare.test"}}},
		{name: "referer on same host", host: "flare.test:5005", headers: http.Header{"Referer": {"http://flare.test:5005/settings/theme"}}},
		{name: "referer behind TLS termination", host: "flare.test", headers: http.Header{"Referer": {"https://flare.test/settings/search?saved=1"}}},
		{name: "foreign referer", host: "flare.test", reason: "REFERER_HOST_MISMATCH", headers: http.Header{"Referer": {"https://evil.test/"}}},
		{name: "referer wrong port", host: "flare.test:5005", reason: "REFERER_PORT_MISMATCH", headers: http.Header{"Referer": {"http://flare.test:5006/"}}},
		{name: "opaque referer", host: "flare.test", reason: "REFERER_OPAQUE", headers: http.Header{"Referer": {"null"}}},
		{name: "invalid referer", host: "flare.test", reason: "REFERER_INVALID", headers: http.Header{"Referer": {"//flare.test/settings"}}},
		{name: "duplicate referer", host: "flare.test", reason: "REFERER_AMBIGUOUS", headers: http.Header{"Referer": {"http://flare.test/", "http://flare.test/"}}},
		// Referer is consulted only when Origin is absent.
		{name: "valid origin with foreign referer", host: "flare.test", headers: http.Header{"Origin": {"http://flare.test"}, "Referer": {"https://evil.test/"}}},
		{name: "foreign origin with valid referer", host: "flare.test", reason: "ORIGIN_HOST_MISMATCH", headers: http.Header{"Origin": {"https://evil.test"}, "Referer": {"http://flare.test/"}}},
		// An explicit HTTPS policy issues Secure cookies; an HTTP page cannot write.
		{name: "HTTPS policy, HTTP origin", host: "flare.test", secure: true, reason: "ORIGIN_SCHEME_MISMATCH", headers: http.Header{"Origin": {"http://flare.test"}}},
		{name: "HTTPS policy, HTTP referer", host: "flare.test", secure: true, reason: "REFERER_SCHEME_MISMATCH", headers: http.Header{"Referer": {"http://flare.test/settings"}}},
		{name: "HTTPS policy, HTTPS origin", host: "flare.test", secure: true, headers: http.Header{"Origin": {"https://flare.test"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New("", "", "", tc.secure)
			require.NoError(t, err)
			result := diagnose(t, p, http.MethodPost, tc.host, tc.headers)
			require.Nil(t, result.authority)
			requireReason(t, tc.reason, result.source, tc.headers)
		})
	}
}

func TestConfiguredAuthoritiesAreKnownSources(t *testing.T) {
	p, err := New("https://flare.test:8443", "alias.test,lan.test:5005", "", false)
	require.NoError(t, err)
	for _, tc := range []struct{ name, host, origin, reason string }{
		{name: "external origin", host: "flare.test:8443", origin: "https://flare.test:8443"},
		{name: "alias with default port", host: "alias.test", origin: "https://alias.test"},
		{name: "alias with explicit port", host: "lan.test:5005", origin: "https://lan.test:5005"},
		{name: "external origin through another allowed Host", host: "alias.test", origin: "https://flare.test:8443"},
		{name: "external hostname with another port", host: "alias.test", origin: "https://flare.test", reason: "ORIGIN_PORT_MISMATCH"},
		{name: "foreign origin", host: "alias.test", origin: "https://evil.test", reason: "ORIGIN_HOST_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			headers := http.Header{"Origin": {tc.origin}}
			result := diagnose(t, p, http.MethodPost, tc.host, headers)
			require.Nil(t, result.authority)
			requireReason(t, tc.reason, result.source, headers)
			require.Equal(t, "https://flare.test:8443", result.external)
		})
	}
}

func TestHostAllowlistIsOptIn(t *testing.T) {
	for _, tc := range []struct{ name, external, hosts, host, reason string }{
		{name: "default accepts any Host", host: "anything.test:5005"},
		{name: "external URL alone accepts any Host", external: "https://flare.test", host: "192.168.1.10:5005"},
		{name: "allowlist accepts alias", hosts: "flare.test", host: "flare.test"},
		{name: "allowlist rejects other Host", hosts: "flare.test", host: "192.168.1.10:5005", reason: "HOST_NOT_ALLOWED"},
		{name: "allowlist includes external authority", external: "https://flare.test", hosts: "alias.test", host: "flare.test"},
		{name: "allowlist with external rejects other Host", external: "https://flare.test", hosts: "alias.test", host: "192.168.1.10:5005", reason: "HOST_NOT_ALLOWED"},
		{name: "allowlist compares ports", hosts: "flare.test:8443", host: "flare.test", reason: "HOST_NOT_ALLOWED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.external, tc.hosts, "", false)
			require.NoError(t, err)
			// The authority applies to reads as well as writes.
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				result := diagnose(t, p, method, tc.host, nil)
				requireReason(t, tc.reason, result.authority, nil)
				requireReason(t, tc.reason, result.source, nil)
				if tc.reason == "" {
					require.NoError(t, result.externalErr)
					continue
				}
				var rejected *AuthorityError
				require.True(t, errors.As(result.externalErr, &rejected))
				require.Equal(t, tc.reason, rejected.Diagnostic.Reason)
				require.Equal(t, rejected.Diagnostic.Message, rejected.Error())
			}
		})
	}
}

func TestAuthorityDiagnosticsNameTheFailedRule(t *testing.T) {
	const peer = "0.0.0.0/32"
	longChain := strings.TrimSuffix(strings.Repeat("198.51.100.1,", 33), ",")
	for _, tc := range []struct {
		name, external, hosts, proxies, host, reason string
		secure                                       bool
		headers                                      http.Header
	}{
		{name: "invalid Host", host: "flare.test:99999", reason: "HOST_INVALID"},
		{name: "invalid forwarded host", proxies: peer, host: "backend:5005", reason: "HOST_INVALID", headers: http.Header{"X-Forwarded-Host": {"flare.test:99999"}}},
		{name: "forwarded host outside the allowlist", hosts: "flare.test", proxies: peer, host: "backend:5005", reason: "HOST_NOT_ALLOWED", headers: http.Header{"X-Forwarded-Host": {"evil.test"}}},
		{name: "RFC 7239 header", proxies: peer, host: "backend:5005", reason: "FORWARDED_HEADER_UNSUPPORTED", headers: http.Header{"Forwarded": {"host=flare.test;proto=https"}}},
		{name: "comma-separated scheme", proxies: peer, host: "backend:5005", reason: "FORWARDED_HEADER_AMBIGUOUS", headers: http.Header{"X-Forwarded-Proto": {"https,http"}}},
		{name: "duplicated forwarded host", proxies: peer, host: "backend:5005", reason: "FORWARDED_HEADER_AMBIGUOUS", headers: http.Header{"X-Forwarded-Host": {"flare.test", "flare.test"}}},
		{name: "duplicated client chain", proxies: peer, host: "backend:5005", reason: "FORWARDED_HEADER_AMBIGUOUS", headers: http.Header{"X-Forwarded-For": {"198.51.100.1", "198.51.100.2"}}},
		{name: "client chain is not an address", proxies: peer, host: "backend:5005", reason: "FORWARDED_FOR_INVALID", headers: http.Header{"X-Forwarded-For": {"198.51.100.1, unknown"}}},
		{name: "client chain is too long", proxies: peer, host: "backend:5005", reason: "FORWARDED_FOR_INVALID", headers: http.Header{"X-Forwarded-For": {longChain}}},
		{name: "unsupported forwarded scheme", proxies: peer, host: "backend:5005", reason: "FORWARDED_PROTO_INVALID", headers: http.Header{"X-Forwarded-Proto": {"wss"}}},
		{name: "forwarded scheme contradicts HTTPS policy", proxies: peer, host: "backend:5005", secure: true, reason: "FORWARDED_PROTO_CONFLICT", headers: http.Header{"X-Forwarded-Proto": {"http"}}},
		// Forwarding headers from an untrusted peer are never interpreted.
		{name: "untrusted peer", host: "flare.test", headers: http.Header{"Forwarded": {"host=evil.test"}, "X-Forwarded-Proto": {"wss"}, "X-Forwarded-Host": {"a.test", "b.test"}, "X-Forwarded-For": {"unknown"}}},
		{name: "trusted peer with a valid chain", proxies: peer, host: "backend:5005", headers: http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"flare.test"}, "X-Forwarded-For": {"203.0.113.7, 198.51.100.1"}}},
		// Without a forwarded host, the fixed external origin stands in for an
		// internal Host that the allowlist would otherwise refuse.
		{name: "trusted peer rewrites Host", external: "https://flare.test", hosts: "alias.test", proxies: peer, host: "backend:5005"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := New(tc.external, tc.hosts, tc.proxies, tc.secure)
			require.NoError(t, err)
			result := diagnose(t, p, http.MethodGet, tc.host, tc.headers)
			requireReason(t, tc.reason, result.authority, tc.headers)
			requireReason(t, tc.reason, result.source, tc.headers)
			if tc.reason == "" {
				require.NoError(t, result.externalErr)
				return
			}
			var rejected *AuthorityError
			require.True(t, errors.As(result.externalErr, &rejected))
			require.Equal(t, tc.reason, rejected.Diagnostic.Reason)
		})
	}
}

func TestTrustedProxyAuthorityIsAKnownSource(t *testing.T) {
	p, err := New("", "", "0.0.0.0/32", false)
	require.NoError(t, err)
	forwarded := http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-Host": {"flare.test:8443"}}
	for _, tc := range []struct{ origin, reason string }{
		{origin: "https://flare.test:8443"},
		{origin: "https://flare.test", reason: "ORIGIN_PORT_MISMATCH"},
		{origin: "https://evil.test:8443", reason: "ORIGIN_HOST_MISMATCH"},
	} {
		headers := forwarded.Clone()
		headers.Set("Origin", tc.origin)
		result := diagnose(t, p, http.MethodPost, "backend:5005", headers)
		require.Nil(t, result.authority)
		requireReason(t, tc.reason, result.source, nil)
		require.Equal(t, "https://flare.test:8443", result.external)
	}
}
