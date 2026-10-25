//go:build fiber

package guide

import (
	"embed"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	memfs "github.com/soulteary/memfs-kit"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/fn"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/guide"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

//go:embed guide-assets
var IntroAssets embed.FS

func Init() error {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		return err
	}
	return nil
}

func RegisterRouting(e *fiber.App) {
	if introAssets, err := fs.Sub(IntroAssets, "guide-assets"); err == nil {
		e.Use(_ASSETS_WEB_URI, static.New("", static.Config{FS: introAssets}))
	}
	e.Get(define.RegularPages.Guide.Path, render)
}

func render(c fiber.Ctx) error {
	return c.Status(http.StatusOK).Type("html", "utf-8").Send([]byte(getUserHomePage()))
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
