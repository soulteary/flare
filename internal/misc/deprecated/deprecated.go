package deprecated

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
)

func makeLandingPage(originURL string, currentURL string, delay string) []byte {
	tpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="Refresh" content="` + delay + `; url='` + currentURL + `'" />
    <title>URL Deprecated</title>
</head>
<body>
    <p>由于程序升级，<a href="` + currentURL + `"><code>` + originURL + `</code></a>变更为<a href="` + currentURL + `"><code>` + currentURL + `</code></a>，页面将在` + delay + `秒后自动跳转。</p>
	<p>你也可以直接点击<a href="` + currentURL + `"><code>这里</code></a>，前往新的页面</p>
</body>
</html>`
	return []byte(tpl)
}

func RegisterRouting(e *fiber.App) {
	const urlMDI = "/resources/mdi-cheat-sheets/"
	e.Get(urlMDI, func(c fiber.Ctx) error {
		if define.AppFlags.EnableDeprecatedNotice {
			return c.Status(http.StatusOK).Type("html", "utf-8").Send(makeLandingPage(urlMDI, define.RegularPages.Icons.Path, "5"))
		}
		return c.Redirect().Status(http.StatusFound).To(define.RegularPages.Icons.Path)
	})
}
