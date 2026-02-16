package appearance

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(FlareDefine.SettingPages.Appearance.Path, pageAppearance, FlareAuth.AuthRequired)
	e.POST(FlareDefine.SettingPages.Appearance.Path, updateAppearanceOptions, FlareAuth.AuthRequired)
}

func updateAppearanceOptions(c *echo.Context) error {
	var body struct {
		OptionTitle              string `form:"title"`
		OptionFooter             string `form:"footer"`
		OptionOpenAppNewTab      bool   `form:"open-app-newtab"`
		OptionOpenBookmarkNewTab bool   `form:"open-bookmark-newtab"`
		OptionShowTitle          bool   `form:"show-title"`
		OptionGreetings          string `form:"greetings"`
		OptionShowDateTime       bool   `form:"show-datetime"`
		OptionShowApps           bool   `form:"show-apps"`
		OptionShowBookmarks      bool   `form:"show-bookmarks"`
		HideSettingsButton       bool   `form:"hide-settings-button"`
		HideHelpButton           bool   `form:"hide-help-button"`
		EnableEncryptedLink      bool   `form:"enable-encrypted-link"`
		IconMode                 string `form:"icon-mode"`
		KeepLetterCase           bool   `form:"keep-letter-case"`
		Locale                   string `form:"locale"`
		OptionCustomDay          string `form:"custom-day"`
		OptionCustomMonth        string `form:"custom-month"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	var update FlareModel.Application
	update.Title = body.OptionTitle
	update.Footer = body.OptionFooter
	update.OpenAppNewTab = body.OptionOpenAppNewTab
	update.OpenBookmarkNewTab = body.OptionOpenBookmarkNewTab
	update.ShowTitle = body.OptionShowTitle
	update.Greetings = body.OptionGreetings
	update.ShowDateTime = body.OptionShowDateTime
	update.ShowApps = body.OptionShowApps
	update.ShowBookmarks = body.OptionShowBookmarks
	update.HideSettingsButton = body.HideSettingsButton
	update.HideHelpButton = body.HideHelpButton
	update.EnableEncryptedLink = body.EnableEncryptedLink
	update.KeepLetterCase = body.KeepLetterCase
	requestIconMode := strings.ToUpper(body.IconMode)
	if requestIconMode != "DEFAULT" && requestIconMode != "FILLING" {
		update.IconMode = "DEFAULT"
	} else {
		update.IconMode = requestIconMode
	}
	if body.Locale != "" {
		update.Locale = body.Locale
	}
	FlareData.UpdateAppearance(update)
	return pageAppearance(c)
}

func pageAppearance(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	IconModeDefault := options.IconMode == "DEFAULT"
	IconModeFilling := options.IconMode == "FILLING"
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["PageName"] = "Appearance"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["OptionTitle"] = options.Title
	m["OptionFooter"] = template.HTML(options.Footer)
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowTitle"] = options.ShowTitle
	m["OptionGreetings"] = options.Greetings
	m["OptionShowDateTime"] = options.ShowDateTime
	m["OptionShowApps"] = options.ShowApps
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	m["OptionEnableEncryptedLink"] = options.EnableEncryptedLink
	m["OptionKeepLetterCase"] = options.KeepLetterCase
	m["OptionIconModeDefault"] = IconModeDefault
	m["OptionIconModeFilling"] = IconModeFilling
	m["OptionLocale"] = options.Locale
	m["Locale"] = options.Locale
	return c.Render(http.StatusOK, "settings.html", m)
}
