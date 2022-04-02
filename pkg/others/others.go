package others

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/data"
	FlareAuth "github.com/soulteary/flare/pkg/auth"
	FlareState "github.com/soulteary/flare/state"
	FlareVersion "github.com/soulteary/flare/version"
)

func RegisterRouting(router *gin.Engine) {

	router.GET(FlareState.SettingPages.Others.Path, pageHome)
	router.POST(FlareState.SettingPages.Others.Path, updateSearchOptions)

}

func pageHome(c *gin.Context) {

	render(c, "")

}

func updateSearchOptions(c *gin.Context) {
	render(c, "")
}

func render(c *gin.Context, testResult string) {
	options := FlareData.GetAllSettingsOptions()

	isLogined := false

	if !FlareState.AppFlags.DisableLoginMode {
		isLogined = FlareAuth.CheckUserIsLogin(c)
	} else {
		isLogined = true
	}

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":        FlareState.AppFlags.DebugMode,
			"DisableLoginMode": FlareState.AppFlags.DisableLoginMode,
			"UserIsLogin":      isLogined,
			"UserName":         FlareAuth.GetUserName(c),
			"LoginDate":        FlareAuth.GetUserLoginDate(c),

			"PageInlineStyle": FlareState.GetPageInlineStyle(),
			"PageAppearance":  FlareState.GetAppBodyStyle(),
			"SettingsURI":     FlareState.RegularPages.Settings.Path,
			"LoginURI":        FlareState.MiscPages.Login.Path,
			"LogoutURI":       FlareState.MiscPages.Logout.Path,

			"PageName":     "Others",
			"SettingPages": FlareState.SettingPages,
			"OptionTitle":  options.Title,
			"Version":      FlareVersion.Version,
			"BuildDate":    FlareVersion.BuildDate,
			"COMMIT":       FlareVersion.Commit,
		},
	)
}
