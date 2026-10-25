//go:build fiber

package redir

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *fiber.App) {
	internalError := []byte(`<html><p>找不到匹配的跳转地址，请确认地址未被人为修改。</p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！</html>`)

	e.Get(define.MiscPages.RedirHome.Path, func(c fiber.Ctx) error {
		return c.Redirect().Status(http.StatusFound).To(define.RegularPages.Home.Path)
	})

	e.Get(define.MiscPages.RedirHelper.Path, func(c fiber.Ctx) error {
		encoded := c.Query("go")
		if len(encoded) < 1 {
			return c.Status(http.StatusBadRequest).Type("html", "utf-8").Send(internalError)
		}
		decoded, err := data.Base64DecodeUrl(encoded)
		if err != nil {
			return c.Status(http.StatusBadRequest).Type("html", "utf-8").Send(internalError)
		}
		decodeURL := string(decoded)
		appsData, errApps := data.LoadFavoriteBookmarks()
		if errApps == nil {
			for _, bookmark := range appsData.Items {
				if bookmark.URL == decodeURL {
					return c.Redirect().Status(http.StatusFound).To(string(decoded))
				}
			}
		}
		bookmarksData, errBookmarks := data.LoadNormalBookmarks()
		if errBookmarks == nil {
			for _, bookmark := range bookmarksData.Items {
				if bookmark.URL == decodeURL {
					return c.Redirect().Status(http.StatusFound).To(string(decoded))
				}
			}
		}
		return c.Status(http.StatusOK).Type("html", "utf-8").Send(internalError)
	})
}
