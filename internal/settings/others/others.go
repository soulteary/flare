package others

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlareVersion "github.com/soulteary/flare/internal/version"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareDefine.SettingPages.Others.Path, pageOthers)
}

func pageOthers(c *gin.Context) {
	options := FlareData.GetAllSettingsOptions()

	isLogined := false

	if !FlareDefine.AppFlags.DisableLoginMode {
		isLogined = FlareAuth.CheckUserIsLogin(c)
	} else {
		isLogined = true
	}

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":        FlareDefine.AppFlags.DebugMode,
			"DisableLoginMode": FlareDefine.AppFlags.DisableLoginMode,
			"UserIsLogin":      isLogined,
			"UserName":         FlareAuth.GetUserName(c),
			"LoginDate":        FlareAuth.GetUserLoginDate(c),

			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),
			"PageAppearance":  FlareDefine.GetAppBodyStyle(),
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,
			"LoginURI":        FlareDefine.MiscPages.Login.Path,
			"LogoutURI":       FlareDefine.MiscPages.Logout.Path,

			"PageName":     "Others",
			"SettingPages": FlareDefine.SettingPages,
			"OptionTitle":  options.Title,
			"Version":      FlareVersion.Version,
			"BuildDate":    FlareVersion.BuildDate,
			"COMMIT":       FlareVersion.Commit,

			"OptionFooter": template.HTML(options.Footer),
		},
	)
}
