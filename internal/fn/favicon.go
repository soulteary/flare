package fn

import (
	"html"
	"net/url"
)

func GetYandexFavicon(bookmarkLink string, fallback string) string {
	u, err := url.Parse(bookmarkLink)
	if err != nil || !IsSafeImageURL(bookmarkLink) {
		return fallback
	}

	return `<img src="https://favicon.yandex.net/favicon/` + html.EscapeString(u.Hostname()) + `/"/>`
}
