package settings

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareDefine "github.com/soulteary/flare/config/define"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareDefine.RegularPages.Settings.Path, pageHome)
	router.GET(FlareDefine.RegularPages.Settings.Path+"/", pageHome)
}

func pageHome(c *gin.Context) {
	c.Redirect(http.StatusFound, FlareDefine.SettingPages.Theme.Path)
}
