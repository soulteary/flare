// Package requestsource resolves the external request authority without trusting
// forwarding headers from arbitrary clients.
package requestsource

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type Policy struct {
	external *url.URL
	hosts    []string
	proxies  []*net.IPNet
	secure   bool
}

// Diagnostic is a stable, safe explanation for a rejected browser source. It
// deliberately avoids echoing request headers, which may contain attacker input
// or sensitive Referer paths. Callers can expose these fields to users and logs.
type Diagnostic struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func diagnostic(reason, message, hint string) *Diagnostic {
	return &Diagnostic{Reason: reason, Message: message, Hint: hint}
}

// AuthorityError reports a request authority that cannot be resolved or is not
// allowed. Its Diagnostic follows the same rule as every other Diagnostic: it
// never echoes request input.
type AuthorityError struct{ Diagnostic *Diagnostic }

func (e *AuthorityError) Error() string { return e.Diagnostic.Message }

func forwardedAmbiguous() *Diagnostic {
	return diagnostic("FORWARDED_HEADER_AMBIGUOUS", "The trusted proxy sent duplicated or multi-valued forwarding headers.", "Make the last proxy overwrite X-Forwarded-Proto and X-Forwarded-Host with single values and send one X-Forwarded-For header.")
}

func forwardedForInvalid() *Diagnostic {
	return diagnostic("FORWARDED_FOR_INVALID", "The X-Forwarded-For chain from the trusted proxy is malformed.", "Ensure X-Forwarded-For holds at most 32 comma-separated IP addresses.")
}

func New(external, hosts, proxies string, secure bool) (*Policy, error) {
	p := &Policy{secure: secure}
	if external != "" {
		u, e := url.Parse(external)
		if e != nil || !validURL(u) || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.ForceQuery {
			return nil, errors.New("FLARE_EXTERNAL_URL requires an HTTP(S) origin without path, credentials or query")
		}
		p.external = u
		p.secure = p.secure || u.Scheme == "https"
	}
	for _, host := range strings.Split(hosts, ",") {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		u, e := url.Parse("http://" + host)
		if e != nil || !validURL(u) || u.Path != "" || u.RawQuery != "" || u.ForceQuery {
			return nil, errors.New("FLARE_ALLOWED_HOSTS requires comma-separated authorities")
		}
		p.hosts = append(p.hosts, strings.ToLower(host))
	}
	for _, raw := range strings.Split(proxies, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if ip := net.ParseIP(raw); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			p.proxies = append(p.proxies, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, e := net.ParseCIDR(raw)
		if e != nil {
			return nil, errors.New("FLARE_TRUSTED_PROXIES requires IP addresses or CIDRs")
		}
		p.proxies = append(p.proxies, network)
	}
	return p, nil
}
func validURL(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" {
		return false
	}
	if strings.ContainsAny(u.Host, "\\ \t\r\n") {
		return false
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return false
		}
	}
	return true
}
func port(u *url.URL) string {
	if u.Port() != "" {
		return u.Port()
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
func sameAuthority(a, b *url.URL) bool {
	return strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// allowed reports whether the effective authority is the configured external
// authority or one of the FLARE_ALLOWED_HOSTS aliases.
func (p *Policy) allowed(effective *url.URL) bool {
	if p.external != nil && sameAuthority(effective, p.external) {
		return true
	}
	for _, alias := range p.hosts {
		if u, e := url.Parse(effective.Scheme + "://" + alias); e == nil && sameAuthority(effective, u) {
			return true
		}
	}
	return false
}
func (p *Policy) trusted(ip net.IP) bool {
	for _, network := range p.proxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
func single(c fiber.Ctx, name string) (string, bool) {
	v := c.Request().Header.PeekAll(name)
	if len(v) > 1 {
		return "", false
	}
	if len(v) == 0 {
		return "", true
	}
	s := string(v[0])
	return s, !strings.Contains(s, ",") && strings.TrimSpace(s) == s
}

// External returns the configured canonical origin for links and diagnostics.
// Allowed aliases affect request validation, never the canonical origin.
func (p *Policy) External(c fiber.Ctx) (*url.URL, error) {
	active, rejected := p.active(c)
	if rejected != nil {
		return nil, &AuthorityError{Diagnostic: rejected}
	}
	if p.external != nil {
		return p.external, nil
	}
	return active, nil
}

// DiagnoseAuthority validates the request authority alone. It applies to every
// method, including reads. A nil result means that the authority is acceptable.
func (p *Policy) DiagnoseAuthority(c fiber.Ctx) *Diagnostic {
	_, rejected := p.active(c)
	return rejected
}

func (p *Policy) active(c fiber.Ctx) (*url.URL, *Diagnostic) {
	scheme := "http"
	if p.secure || c.RequestCtx().IsTLS() {
		scheme = "https"
	}
	host := string(c.Request().Host())
	hasForwardedHost := false
	trusted := p.trusted(c.RequestCtx().RemoteIP())
	if trusted {
		forwardedIPs := c.Request().Header.PeekAll("X-Forwarded-For")
		if len(forwardedIPs) > 1 {
			return nil, forwardedAmbiguous()
		}
		if len(forwardedIPs) == 1 {
			parts := strings.Split(string(forwardedIPs[0]), ",")
			if len(parts) > 32 {
				return nil, forwardedForInvalid()
			}
			for _, part := range parts {
				if net.ParseIP(strings.TrimSpace(part)) == nil {
					return nil, forwardedForInvalid()
				}
			}
		}
		// One explicit authority is accepted. Forwarded is not mixed with X-*;
		// deployments using RFC Forwarded should normalize it at the last proxy.
		if c.Get("Forwarded") != "" {
			return nil, diagnostic("FORWARDED_HEADER_UNSUPPORTED", "The trusted proxy sent an RFC 7239 Forwarded header, which is not supported.", "Remove the Forwarded header at the last proxy and send X-Forwarded-Proto and X-Forwarded-Host instead.")
		}
		proto, ok := single(c, "X-Forwarded-Proto")
		if !ok {
			return nil, forwardedAmbiguous()
		}
		forwarded, ok := single(c, "X-Forwarded-Host")
		if !ok {
			return nil, forwardedAmbiguous()
		}
		if proto != "" {
			if proto != "https" && proto != "http" {
				return nil, diagnostic("FORWARDED_PROTO_INVALID", "X-Forwarded-Proto from the trusted proxy is neither http nor https.", "Send X-Forwarded-Proto with the value https or http from the last proxy.")
			}
			if p.secure && proto == "http" {
				return nil, diagnostic("FORWARDED_PROTO_CONFLICT", "The trusted proxy reported HTTP, but this instance is configured for HTTPS.", "Serve the public URL over HTTPS, or remove FLARE_COOKIE_SECURE and any https FLARE_EXTERNAL_URL if this deployment is intentionally HTTP-only.")
			}
			scheme = proto
		}
		if forwarded != "" {
			host = forwarded
			hasForwardedHost = true
		}
	}
	effective, e := url.Parse(scheme + "://" + host)
	if e != nil || !validURL(effective) || effective.Path != "" || effective.RawQuery != "" || effective.ForceQuery {
		return nil, diagnostic("HOST_INVALID", "The request Host is not a valid authority.", "Check the Host header sent by the client or reverse proxy, and X-Forwarded-Host when a trusted proxy supplies it.")
	}

	// Host allowlisting is opt-in through FLARE_ALLOWED_HOSTS. The external
	// authority, when configured, always belongs to that list. FLARE_EXTERNAL_URL
	// on its own names the public origin without restricting other authorities.
	if len(p.hosts) == 0 || p.allowed(effective) {
		return effective, nil
	}
	// A trusted proxy may rewrite Host to an internal backend. Without an
	// explicit forwarded host, the fixed external origin is authoritative.
	if p.external != nil && trusted && !hasForwardedHost {
		return p.external, nil
	}
	return nil, diagnostic("HOST_NOT_ALLOWED", "The request Host is not listed in FLARE_ALLOWED_HOSTS.", "Open Flare through an allowed hostname, add this hostname and port to FLARE_ALLOWED_HOSTS, or preserve the original Host header at the reverse proxy.")
}

// Diagnose validates browser source metadata and returns an actionable, safe
// explanation on failure. A nil result means that the request is acceptable.
//
// The decision follows net/http.CrossOriginProtection, reimplemented on a
// fiber.Ctx because that type is bound to net/http. Duplicated or malformed
// source headers are rejected here rather than ignored.
//
//   - Sec-Fetch-Site is computed by the browser from the address the user is
//     actually visiting. Page scripts cannot set Sec- prefixed headers, and
//     browsers send them only to trustworthy URLs (HTTPS and localhost). A
//     same-origin or user-initiated value therefore holds regardless of how a
//     reverse proxy rewrote the scheme or Host on the way to this process.
//   - Without it (plain HTTP, or browsers released before 2023), the Origin
//     authority is compared with the authorities this instance answers to. The
//     scheme is not compared: behind TLS termination this process sees HTTP
//     while the browser reports an HTTPS origin.
//   - Requests carrying none of Sec-Fetch-Site, Origin and Referer do not come
//     from a browser. Management writes remain protected by the CSRF token
//     checked by the caller.
func (p *Policy) Diagnose(c fiber.Ctx) *Diagnostic {
	effective, rejected := p.active(c)
	if rejected != nil {
		return rejected
	}
	sites := c.Request().Header.PeekAll("Sec-Fetch-Site")
	if len(sites) > 1 {
		return diagnostic("FETCH_METADATA_AMBIGUOUS", "The request contains duplicate browser source metadata.", "Check the reverse proxy for duplicated Sec-Fetch-Site headers and retry from a newly opened Flare page.")
	}
	// An empty value is treated like an absent header, as net/http does.
	if len(sites) == 1 && len(sites[0]) > 0 {
		if site := string(sites[0]); site == "same-origin" || site == "none" {
			return nil
		}
		return diagnostic("FETCH_SITE_NOT_SAME_ORIGIN", "The browser marked this write as cross-origin.", "Open Flare and submit the change from the same scheme, hostname and port; do not submit management forms through an embedded or sibling site.")
	}
	// Referer is consulted only when Origin is absent.
	for _, header := range []string{"Origin", "Referer"} {
		if values := c.Request().Header.PeekAll(header); len(values) > 0 {
			return p.diagnoseSource(c, effective, header, values)
		}
	}
	return nil
}

func (p *Policy) diagnoseSource(c fiber.Ctx, effective *url.URL, header string, values [][]byte) *Diagnostic {
	prefix := strings.ToUpper(header)
	if len(values) != 1 {
		return diagnostic(prefix+"_AMBIGUOUS", "The request contains duplicate "+header+" headers.", "Check the reverse proxy for duplicated source headers and retry from a newly opened Flare page.")
	}
	raw := string(values[0])
	if raw == "null" {
		return diagnostic(prefix+"_OPAQUE", "The browser sent an opaque "+header+" value, so Flare cannot verify the page source.", "Open the Flare page directly. If a proxy overrides Referrer-Policy, remove that override or use same-origin.")
	}
	u, e := url.Parse(raw)
	if e != nil || strings.TrimSpace(raw) != raw || !validURL(u) {
		return diagnostic(prefix+"_INVALID", "The request contains an invalid "+header+" value.", "Retry from a newly opened Flare page and check whether the reverse proxy rewrites browser source headers.")
	}
	if header == "Origin" && (u.Path != "" || u.RawQuery != "" || u.ForceQuery) {
		return diagnostic("ORIGIN_INVALID", "The Origin header contains a path or query and is not a valid browser origin.", "Check whether a proxy or browser extension rewrites the Origin header.")
	}
	// An explicit HTTPS policy issues Secure cookies, so a page opened over HTTP
	// could not authenticate its write. Say so instead of failing later.
	if p.secure && u.Scheme == "http" {
		return diagnostic(prefix+"_SCHEME_MISMATCH", "The page was opened over HTTP, but this instance is configured for HTTPS.", "FLARE_COOKIE_SECURE or FLARE_EXTERNAL_URL requires HTTPS. Open the HTTPS public URL, or disable FLARE_COOKIE_SECURE if this deployment is intentionally HTTP-only.")
	}
	hostMatched := false
	for _, candidate := range p.sourceAuthorities(c, effective) {
		if !strings.EqualFold(u.Hostname(), candidate.hostname) {
			continue
		}
		hostMatched = true
		if explicitPort(u.Port(), u.Scheme) == explicitPort(candidate.port, u.Scheme) {
			return nil
		}
	}
	if hostMatched {
		return diagnostic(prefix+"_PORT_MISMATCH", "The page port does not match the public port received by Flare.", "Preserve the public port in Host; for Nginx use $http_host rather than $host.")
	}
	return diagnostic(prefix+"_HOST_MISMATCH", "The page hostname does not match the Host received by Flare.", "Configure the reverse proxy with 'proxy_set_header Host $http_host;', or set FLARE_EXTERNAL_URL to the public origin.")
}

// authority is a hostname with an optional explicit port. An empty port stands
// for the default port of whichever scheme the compared source uses.
type authority struct{ hostname, port string }

// sourceAuthorities lists the authorities this instance answers to: the Host
// header as received, the effective authority (which reflects X-Forwarded-Host
// from a trusted proxy), the configured external origin and the allowed aliases.
func (p *Policy) sourceAuthorities(c fiber.Ctx, effective *url.URL) []authority {
	authorities := make([]authority, 0, len(p.hosts)+3)
	add := func(host string) {
		if u, e := url.Parse("http://" + host); e == nil && u.Hostname() != "" {
			authorities = append(authorities, authority{u.Hostname(), u.Port()})
		}
	}
	add(string(c.Request().Host()))
	add(effective.Host)
	if p.external != nil {
		add(p.external.Host)
	}
	for _, alias := range p.hosts {
		add(alias)
	}
	return authorities
}

// explicitPort drops a port that is the default for the given scheme, so that
// "example.com" and "example.com:443" compare equal for an https source.
func explicitPort(port, scheme string) string {
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		return ""
	}
	return port
}

func (p *Policy) Valid(c fiber.Ctx) bool {
	return p.Diagnose(c) == nil
}
func (p *Policy) Secure(c fiber.Ctx) bool {
	u, e := p.External(c)
	return p.secure || c.RequestCtx().IsTLS() || e == nil && u.Scheme == "https"
}
func (p *Policy) ClientIP(c fiber.Ctx) string {
	peer := c.RequestCtx().RemoteIP()
	if !p.trusted(peer) {
		return peer.String()
	}
	values := c.Request().Header.PeekAll("X-Forwarded-For")
	if len(values) != 1 {
		return peer.String()
	}
	parts := strings.Split(string(values[0]), ",")
	if len(parts) > 32 {
		return peer.String()
	}
	for i := len(parts) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(parts[i]))
		if ip == nil {
			return peer.String()
		}
		if !p.trusted(ip) {
			return ip.String()
		}
	}
	return peer.String()
}
