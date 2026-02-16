package guide

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/memfs"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/fn"
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

func RegisterRouting(e *echo.Echo) {
	if introAssets, err := fs.Sub(IntroAssets, "guide-assets"); err == nil {
		e.StaticFS(_ASSETS_WEB_URI, introAssets)
	}
	e.GET(define.RegularPages.Guide.Path, render)
}

func render(c *echo.Context) error {
	return c.HTMLBlob(http.StatusOK, []byte(getUserHomePage()))
}

func getUserHomePage() string {
	port := strconv.Itoa(define.AppFlags.Port)
	body, err := fn.GetHTML("http://localhost:" + port + "/")
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
