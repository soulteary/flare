package home

import (
	"html"
	"net/url"
	"strings"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/mdi"
)

// navigationHref validates the original destination before hiding it behind the
// compatibility redirect. Encoding must never turn an executable URL into a link.
func navigationHref(raw string, encoded bool) string {
	if !fn.IsSafeNavigationURL(raw) {
		return "#"
	}
	parsed, _ := url.Parse(raw)
	if encoded || strings.EqualFold(parsed.Scheme, "chrome-extension") {
		return "/redir/url?go=" + data.Base64EncodeUrl(raw)
	}
	return raw
}

// navigationIcon returns markup only from the trusted bundled SVG renderer or a
// validated and attribute-escaped image URL. Names/descriptions remain plain text.
func navigationIcon(icon, destination, mode string) string {
	if fn.IsSafeImageURL(icon) {
		return `<img src="` + html.EscapeString(icon) + `"/>`
	}
	if icon != "" {
		return mdi.GetIconByName(icon)
	}
	if mode == "FILLING" {
		return fn.GetYandexFavicon(destination, "")
	}
	return ""
}
