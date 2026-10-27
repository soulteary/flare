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
func same(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
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
	active, err := p.active(c)
	if err != nil {
		return nil, err
	}
	if p.external != nil {
		return p.external, nil
	}
	return active, nil
}
func (p *Policy) active(c fiber.Ctx) (*url.URL, error) {
	scheme := "http"
	if p.secure || c.RequestCtx().IsTLS() {
		scheme = "https"
	}
	host := string(c.Request().Host())
	hasForwardedHost := false
	if p.trusted(c.RequestCtx().RemoteIP()) {
		forwardedIPs := c.Request().Header.PeekAll("X-Forwarded-For")
		if len(forwardedIPs) > 1 {
			return nil, errors.New("ambiguous forwarded client addresses")
		}
		if len(forwardedIPs) == 1 {
			parts := strings.Split(string(forwardedIPs[0]), ",")
			if len(parts) > 32 {
				return nil, errors.New("forwarded chain is too long")
			}
			for _, part := range parts {
				if net.ParseIP(strings.TrimSpace(part)) == nil {
					return nil, errors.New("invalid forwarded client address")
				}
			}
		}
		// One explicit authority is accepted. Forwarded is not mixed with X-*;
		// deployments using RFC Forwarded should normalize it at the last proxy.
		if c.Get("Forwarded") != "" {
			return nil, errors.New("forwarded header is unsupported; configure external URL")
		}
		proto, ok := single(c, "X-Forwarded-Proto")
		if !ok {
			return nil, errors.New("ambiguous forwarded scheme")
		}
		forwarded, ok := single(c, "X-Forwarded-Host")
		if !ok {
			return nil, errors.New("ambiguous forwarded host")
		}
		if proto != "" {
			if proto != "https" && proto != "http" {
				return nil, errors.New("invalid forwarded scheme")
			}
			if p.secure && proto == "http" {
				return nil, errors.New("forwarded scheme contradicts configured HTTPS")
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
		return nil, errors.New("invalid request authority")
	}

	allowed := p.external != nil && same(effective, p.external)
	for _, alias := range p.hosts {
		u, parseErr := url.Parse(effective.Scheme + "://" + alias)
		if parseErr != nil || !validURL(u) {
			return nil, errors.New("invalid allowed host")
		}
		if same(effective, u) {
			allowed = true
			break
		}
	}
	if p.external != nil {
		if allowed {
			return effective, nil
		}
		// A trusted proxy may rewrite Host to an internal backend. Without an
		// explicit forwarded host, the fixed external origin is authoritative.
		if p.trusted(c.RequestCtx().RemoteIP()) && !hasForwardedHost {
			return p.external, nil
		}
		return nil, errors.New("request host is neither canonical nor an allowed alias")
	}
	if len(p.hosts) > 0 && !allowed {
		return nil, errors.New("request host is not allowed")
	}

	return effective, nil
}
func (p *Policy) Valid(c fiber.Ctx) bool {
	external, e := p.active(c)
	if e != nil {
		return false
	}
	sites := c.Request().Header.PeekAll("Sec-Fetch-Site")
	if len(sites) > 1 {
		return false
	}
	if len(sites) == 1 {
		site := string(sites[0])
		if site != "same-origin" && site != "none" {
			return false
		}
	}
	for _, header := range []string{"Origin", "Referer"} {
		values := c.Request().Header.PeekAll(header)
		if len(values) == 0 {
			continue
		}
		if len(values) != 1 {
			return false
		}
		raw := string(values[0])
		u, e := url.Parse(raw)
		if e != nil || strings.TrimSpace(raw) != raw || !validURL(u) || !same(u, external) {
			return false
		}
		if header == "Origin" && (u.Path != "" || u.RawQuery != "" || u.ForceQuery) {
			return false
		}
	}
	return true
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
