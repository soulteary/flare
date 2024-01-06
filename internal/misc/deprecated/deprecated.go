package FlareDeprecated

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareDefine "github.com/soulteary/flare/config/define"
)

// TODO：样式优化
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

// 展示临时的落地页，在几个版本后，彻底取消路由
func RegisterRouting(router *gin.Engine) {
	const urlMDI = "/resources/mdi-cheat-sheets/"
	router.GET(urlMDI, func(c *gin.Context) {
		if FlareDefine.AppFlags.EnableDeprecatedNotice {
			c.Data(http.StatusOK, "text/html; charset=utf-8", makeLandingPage(urlMDI, FlareDefine.RegularPages.Icons.Path, "5"))
		} else {
			c.Redirect(http.StatusFound, FlareDefine.RegularPages.Icons.Path)
		}
		c.Abort()
	})
}
