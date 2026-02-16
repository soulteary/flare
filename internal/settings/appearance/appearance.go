package appearance

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(define.SettingPages.Appearance.Path, pageAppearance, auth.AuthRequired)
	e.POST(define.SettingPages.Appearance.Path, updateAppearanceOptions, auth.AuthRequired)
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
	var update model.Application
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
	data.UpdateAppearance(update)
	return pageAppearance(c)
}

func pageAppearance(c *echo.Context) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "config error")
	}
	IconModeDefault := options.IconMode == "DEFAULT"
	IconModeFilling := options.IconMode == "FILLING"
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageName"] = "Appearance"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["SettingsURI"] = define.RegularPages.Settings.Path
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
