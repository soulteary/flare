package templates

import (
	"embed"
	"html/template"

	"github.com/gin-gonic/gin"

	FlareDefine "github.com/soulteary/flare/config/define"
)

//go:embed html
var TPL embed.FS

func RegisterRouting(router *gin.Engine) {

	if FlareDefine.AppFlags.DebugMode {
		router.LoadHTMLGlob("embed/templates/*.html")
		return
	}

	templ := template.Must(template.New("").ParseFS(TPL, "html/*.html"))
	router.SetHTMLTemplate(templ)
}
