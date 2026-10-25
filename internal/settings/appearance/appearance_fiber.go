//go:build fiber

package appearance

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Appearance.Path, auth.AuthRequired, pageAppearance)
	e.Post(define.SettingPages.Appearance.Path, auth.AuthRequired, updateAppearanceOptions)
}

func updateAppearanceOptions(c fiber.Ctx) error {
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
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	var update model.Application
	// Missing fields from older forms must not erase existing clock preferences.
	if c.FormValue("clock-timezone-mode") != "" {
		dynamic, seconds := c.FormValue("clock-dynamic"), c.FormValue("clock-seconds")
		if (dynamic != "0" && dynamic != "1") || (seconds != "0" && seconds != "1") {
			return c.Status(http.StatusBadRequest).SendString("invalid clock options")
		}
		showSeconds := seconds == "1"
		update.Clock = &model.ClockOptions{Dynamic: dynamic == "1", Seconds: &showSeconds, TimezoneMode: c.FormValue("clock-timezone-mode"), Timezone: strings.TrimSpace(c.FormValue("clock-timezone"))}
		if err := clock.Validate(*update.Clock); err != nil {
			return c.Status(http.StatusBadRequest).SendString(err.Error())
		}
	}

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
	if !data.UpdateAppearance(update) {
		return c.Status(http.StatusInternalServerError).SendString("unable to save appearance settings")
	}
	return pageAppearance(c)
}

func pageAppearance(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
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
	resolved := clock.Resolve(options.Clock, options.ShowDateTime)
	m["ClockDynamic"] = resolved.Options.Dynamic
	m["ClockSeconds"] = resolved.Options.ShowSeconds()
	m["ClockMode"] = resolved.Options.TimezoneMode
	m["ClockTimezone"] = resolved.Options.Timezone
	m["ClockServerZone"] = time.Local.String()
	m["ClockServerTime"] = time.Now().Format("2006-01-02 15:04:05 MST")
	if !define.AppFlags.DisableCSP {
		c.Set("Content-Security-Policy", clock.SettingsPolicy())
	}
	m["OptionLocale"] = options.Locale
	m["Locale"] = options.Locale
	return c.Status(http.StatusOK).Render("settings.html", m)
}
