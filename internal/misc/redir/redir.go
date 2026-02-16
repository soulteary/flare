package redir

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *echo.Echo) {
	internalError := []byte(`<html><p>找不到匹配的跳转地址，请确认地址未被人为修改。</p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！</html>`)

	e.GET(define.MiscPages.RedirHome.Path, func(c *echo.Context) error {
		return c.Redirect(http.StatusFound, define.RegularPages.Home.Path)
	})

	e.GET(define.MiscPages.RedirHelper.Path, func(c *echo.Context) error {
		encoded := c.QueryParam("go")
		if len(encoded) < 1 {
			return c.HTMLBlob(http.StatusBadRequest, internalError)
		}
		decoded, err := data.Base64DecodeUrl(encoded)
		if err != nil {
			return c.HTMLBlob(http.StatusBadRequest, internalError)
		}
		decodeURL := string(decoded)
		appsData, errApps := data.LoadFavoriteBookmarks()
		if errApps == nil {
			for _, bookmark := range appsData.Items {
				if bookmark.URL == decodeURL {
					return c.Redirect(http.StatusFound, string(decoded))
				}
			}
		}
		bookmarksData, errBookmarks := data.LoadNormalBookmarks()
		if errBookmarks == nil {
			for _, bookmark := range bookmarksData.Items {
				if bookmark.URL == decodeURL {
					return c.Redirect(http.StatusFound, string(decoded))
				}
			}
		}
		return c.HTMLBlob(http.StatusOK, internalError)
	})
}
