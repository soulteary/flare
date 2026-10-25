package search

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
)

const searchCSRFCookie = "flare_search_csrf"
const searchCSRFField = "csrf-token"
const searchTokenLifetime = 8 * time.Hour

type searchSecurityKey struct{}
type searchSecurity struct {
	key        [32]byte
	cookieName string
}

func newSearchSecurity() *searchSecurity {
	guard := &searchSecurity{cookieName: fmt.Sprintf("%s_%d", searchCSRFCookie, define.AppFlags.Port)}
	_, _ = rand.Read(guard.key[:])
	return guard
}

// attach and protect are route-local so tokens are invalid across app instances.
func (s *searchSecurity) attach(c fiber.Ctx) error {
	c.Locals(searchSecurityKey{}, s)
	c.Set("Cache-Control", "no-store")
	return c.Next()
}

func (s *searchSecurity) protect(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if len(c.Request().Header.PeekAll("Sec-Fetch-Site")) > 1 {
		return rejectSearchRequest(c)
	}
	site := c.Get("Sec-Fetch-Site")
	if site != "" && site != "same-origin" && site != "none" {
		return rejectSearchRequest(c)
	}
	// A valid signed token supplies the scheme-independent proof needed behind
	// TLS termination. Only the actual Host is compared; proxy headers are ignored.
	if !validRequestSource(c, "Origin", false) || !validRequestSource(c, "Referer", true) {
		return rejectSearchRequest(c)
	}
	token := c.FormValue(searchCSRFField)
	cookie := s.singleCookie(c)
	if token == "" || cookie == "" || subtle.ConstantTimeCompare([]byte(token), []byte(cookie)) != 1 || !s.validToken(token, time.Now()) {
		return rejectSearchRequest(c)
	}
	return c.Next()
}

func rejectSearchRequest(c fiber.Ctx) error {
	return c.Status(http.StatusForbidden).SendString("Search settings request rejected. Reload the search settings page and try again.")
}

func validRequestSource(c fiber.Ctx, name string, referer bool) bool {
	values := c.Request().Header.PeekAll(name)
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 {
		return false
	}
	raw := string(values[0])
	if raw == "" || strings.TrimSpace(raw) != raw {
		return false
	}
	source, err := url.Parse(raw)
	if err != nil || source == nil || (source.Scheme != "http" && source.Scheme != "https") || source.User != nil || source.Host == "" || source.Opaque != "" || source.Fragment != "" {
		return false
	}
	if !referer && (source.Path != "" || source.RawQuery != "" || source.ForceQuery) {
		return false
	}
	// Compare an explicit effective port as well as the host. The request scheme
	// is intentionally not inferred from untrusted Forwarded/X-Forwarded-* headers.
	request, err := url.Parse(source.Scheme + "://" + string(c.Request().Host()))
	if err != nil || request.Host == "" || request.User != nil || request.Path != "" || request.RawQuery != "" {
		return false
	}
	return strings.EqualFold(source.Hostname(), request.Hostname()) && effectivePort(source) == effectivePort(request)
}

func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

func (s *searchSecurity) singleCookie(c fiber.Ctx) string {
	count := 0
	value := ""
	for key, raw := range c.Request().Header.Cookies() {
		if string(key) == s.cookieName {
			count++
			value = string(raw)
		}
	}
	if count != 1 {
		return ""
	}
	return value
}

func (s *searchSecurity) validToken(token string, now time.Time) bool {
	if len(token) > 256 {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(nonce) != 32 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expiry <= now.Unix() || expiry > now.Add(searchTokenLifetime).Unix() {
		return false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, s.key[:])
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	return hmac.Equal(signature, mac.Sum(nil))
}

func (s *searchSecurity) formToken(c fiber.Ctx) string {
	token := s.singleCookie(c)
	if !s.validToken(token, time.Now()) {
		nonce := make([]byte, 32)
		_, _ = rand.Read(nonce)
		payload := base64.RawURLEncoding.EncodeToString(nonce) + "." + strconv.FormatInt(time.Now().Add(searchTokenLifetime).Unix(), 10)
		mac := hmac.New(sha256.New, s.key[:])
		_, _ = mac.Write([]byte(payload))
		token = payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	c.Cookie(&fiber.Cookie{Name: s.cookieName, Value: token, Path: define.SettingPages.Search.Path, HTTPOnly: true, Secure: c.RequestCtx().IsTLS() || define.AppFlags.CookieSecure, SameSite: "Strict", MaxAge: int(searchTokenLifetime.Seconds())})
	c.Set("Cache-Control", "no-store")
	return token
}
