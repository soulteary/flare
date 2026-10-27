package redir

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/fn"
)

func RegisterRouting(e *fiber.App) {
	internalError := []byte(`<html><p>找不到匹配的跳转地址，请确认地址未被人为修改。</p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！</html>`)

	e.Get(define.MiscPages.RedirHome.Path, func(c fiber.Ctx) error {
		return c.Redirect().Status(http.StatusFound).To(define.RegularPages.Home.Path)
	})

	e.Get(define.MiscPages.RedirHelper.Path, func(c fiber.Ctx) error {
		c.Set("Cache-Control", "private, no-store")
		c.Set("Referrer-Policy", "no-referrer")
		if define.AppFlags.Visibility == "PRIVATE" && !auth.CheckUserIsLogin(c) {
			return c.Redirect().Status(302).To(define.SettingPages.Others.Path)
		}
		id := c.Query("id")
		if id != "" && c.Query("go") != "" {
			return c.SendStatus(http.StatusBadRequest)
		}
		var decodeURL string
		if id != "" {
			target, ok := data.MatchOpaqueNavigationTarget(id, auth.CheckUserIsLogin(c))
			if !ok {
				return c.Status(404).SendString("Not Found")
			}
			decodeURL = target
		} else {
			encoded := c.Query("go")
			if len(encoded) < 1 {
				return c.Status(http.StatusBadRequest).Type("html", "utf-8").Send(internalError)
			}
			decoded, err := data.Base64DecodeUrl(encoded)
			if err != nil {
				return c.Status(http.StatusBadRequest).Type("html", "utf-8").Send(internalError)
			}
			decodeURL = string(decoded)
		}
		request := http.Request{Host: string(c.Request().Host()), URL: &url.URL{Path: c.Path()}, RequestURI: c.OriginalURL()}
		if c.RequestCtx().IsTLS() {
			request.TLS = &tls.ConnectionState{}
		}
		info, valid := fn.ParseNavigationSource(&request, c.Query("from"))
		if id != "" {
			decodeURL = fn.ParseDynamicUrlWith(decodeURL, &info)
		}
		if !valid || !fn.IsSafeNavigationURL(decodeURL) {
			return c.Status(http.StatusBadRequest).Type("html", "utf-8").Send(internalError)
		}
		if id != "" {
			return c.Redirect().Status(http.StatusFound).To(decodeURL)
		}
		if data.MatchNavigationTargetFor(decodeURL, auth.CheckUserIsLogin(c), func(template string) bool { return fn.ParseDynamicUrlWith(template, &info) == decodeURL }) {
			return c.Redirect().Status(http.StatusFound).To(decodeURL)
		}

		return c.Status(http.StatusNotFound).SendString("Not Found")
	})
}
