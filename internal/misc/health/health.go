package FlareHealth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareDefine "github.com/soulteary/flare/config/define"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareDefine.MiscPages.HealthCheck.Path, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
}
