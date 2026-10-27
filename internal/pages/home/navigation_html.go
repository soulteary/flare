package home

import (
	"html"
	"strings"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/mdi"
	"net/url"
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
		info := firstRequestInfo(requestInfo)
		id := data.OpaqueNavigationHandle(raw, func(template string) bool { return fn.ParseDynamicUrlWith(template, info) == raw })
		if id != "" {
			link := "/redir/url?id=" + id
			if info != nil {
				link += "&from=" + url.QueryEscape(strings.TrimPrefix(info.Href, info.Origin))
			}
			return link
		}
		return fn.EncodedNavigationURL(raw, info)
	}
	return raw
}

// navigationIcon returns markup only from the trusted bundled SVG renderer or a
// validated and attribute-escaped image URL. Names/descriptions remain plain text.
func navigationIcon(icon, destination, mode string, private ...bool) string {
	if fn.IsSafeImageURL(icon) {
		return `<img referrerpolicy="no-referrer" src="` + html.EscapeString(icon) + `"/>`
	}
	if icon != "" {
		return mdi.GetIconByName(icon)
	}
	if mode == "FILLING" && (len(private) == 0 || !private[0]) {
		return fn.GetYandexFavicon(destination, "")
	}
	return ""
}
