package fn

import (
	"net/url"
	"strings"
	"unicode"
)

// IsSafeNavigationURL permits web/relative links and the application protocols
// supported by bookmark imports. It does not normalize or rewrite saved values.
// Keep executable/document schemes out even when links use the redirect helper.
func IsSafeNavigationURL(raw string) bool {
	if raw == "" || strings.Contains(raw, `\`) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Hostname() != "" && u.Opaque == ""
	case "", "ftp", "ftps", "mailto", "tel", "ssh", "smb", "rdp", "vnc", "magnet", "steam", "obsidian", "chrome-extension":
		return true
	default:
		return false
	}
}

// IsSafeImageURL only permits absolute HTTP(S) image resources. Other icon
// values are treated as bundled icon identifiers, never arbitrary HTML or data.
func IsSafeImageURL(raw string) bool {
	// Reject built-in icon names and non-web protocols before URL parsing. The
	// shared navigation validator then parses an actual image URL exactly once.
	httpPrefix, httpsPrefix := "http://", "https://"
	web := len(raw) >= len(httpPrefix) && strings.EqualFold(raw[:len(httpPrefix)], httpPrefix)
	web = web || len(raw) >= len(httpsPrefix) && strings.EqualFold(raw[:len(httpsPrefix)], httpsPrefix)
	return web && IsSafeNavigationURL(raw)
}
