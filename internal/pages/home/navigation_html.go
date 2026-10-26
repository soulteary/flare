package home

import (
	"html"
	"strings"

	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/mdi"
)

// navigationHref validates the original destination before hiding it behind the
// compatibility redirect. Encoding must never turn an executable URL into a link.
func navigationHref(raw string, encoded bool, requestInfo ...*fn.DynamicURL) string {
	if !fn.IsSafeNavigationURL(raw) {
		return "#"
	}
	const extensionScheme = "chrome-extension:"
	extension := len(raw) >= len(extensionScheme) && strings.EqualFold(raw[:len(extensionScheme)], extensionScheme)
	if encoded || extension {
		return fn.EncodedNavigationURL(raw, firstRequestInfo(requestInfo))
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
