package guide

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/memfs"

	FlareDefine "github.com/soulteary/flare/config/define"
	FlareFn "github.com/soulteary/flare/internal/fn"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/guide"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

//go:embed guide-assets
var IntroAssets embed.FS

func Init() {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)

	if err != nil {
		panic(err)
	}
}

func RegisterRouting(router *gin.Engine) {
	introAssets, _ := fs.Sub(IntroAssets, "guide-assets")
	router.StaticFS(_ASSETS_WEB_URI, http.FS(introAssets))

	router.GET(FlareDefine.RegularPages.Guide.Path, render)
}

func render(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(getUserHomePage()))
	c.Abort()
}

func getUserHomePage() string {
	port := strconv.Itoa(FlareDefine.AppFlags.Port)

	body, err := FlareFn.GetHTML("http://localhost:" + port + "/")
	if err != nil {
		return ""
	}

	ruleHead := regexp.MustCompile("</head>")
	ruleBody := regexp.MustCompile("</body>")
	rulePageView := regexp.MustCompile(`class="pageview"`)
	content := ruleHead.ReplaceAllString(body, `<link rel="stylesheet" href="/assets/guide/introjs.min.css"><link rel="stylesheet" href="/assets/guide/app.css"><script src="/assets/guide/intro.min.js"></script></head>`)
	content = ruleBody.ReplaceAllString(content, `<script src="/assets/guide/app.js"></script></head>`)
	content = rulePageView.ReplaceAllString(content, `class="pageview" style="position:inherit;"`)

	return content
}
