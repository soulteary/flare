package FlareFn

import (
	"fmt"
	"net/url"
)

func GetYandexFavicon(bookmarkLink string, fallback string) string {
	u, err := url.Parse(bookmarkLink)
	if err != nil {
		fmt.Println(err)
		return fallback
	}
	fmt.Println(`<img src="https://favicon.yandex.net/favicon/` + u.Hostname() + `/"/>`)

	return `<img src="https://favicon.yandex.net/favicon/` + u.Hostname() + `/"/>`
}
