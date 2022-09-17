package settings

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareState "github.com/soulteary/flare/internal/state"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareState.RegularPages.Settings.Path, pageHome)
	router.GET(FlareState.RegularPages.Settings.Path+"/", pageHome)
}

func pageHome(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, FlareState.SettingPages.Theme.Path)
}
