package others

import (
	"html/template"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
	version "github.com/soulteary/version-kit/v4"
)

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Others.Path, pageOthers)
	e.Get(define.SettingPages.About.Path, auth.AuthRequired, pageAbout)
}

// pageOthers is the account page of an instance that signs in without a
// security store: who is signed in, with the way out, or the form to sign
// in. An instance that needs no sign-in has no account, and the address
// leads to what it used to show besides, the page about the application.
func pageOthers(c fiber.Ctx) error {
	if define.AppFlags.DisableLoginMode {
		return c.Redirect().Status(http.StatusFound).To(define.SettingPages.About.Path)
	}
	return renderOthers(c, define.SettingPages.Others.Name)
}

// pageAbout is the page about the application: what it is, where it comes
// from, and its version unless the site hides that.
func pageAbout(c fiber.Ctx) error {
	return renderOthers(c, define.SettingPages.About.Name)
}

func renderOthers(c fiber.Ctx, page string) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	isLogined := false
	if !define.AppFlags.DisableLoginMode {
		isLogined = auth.CheckUserIsLogin(c)
	} else {
		isLogined = true
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["DisableLoginMode"] = define.AppFlags.DisableLoginMode
	m["UserIsLogin"] = isLogined
	m["UserName"] = auth.GetUserName(c)
	m["LoginDate"] = auth.GetUserLoginDate(c)
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	pageTheme := define.GetPageTheme()
	m["PageAppearance"] = pageTheme.BodyStyle
	m["PageThemeRules"] = pageTheme.Rules
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["LoginURI"] = define.MiscPages.Login.Path
	m["LogoutURI"] = define.MiscPages.Logout.Path
	m["PageName"] = page
	m["SettingPages"] = define.SettingPages
	m["OptionTitle"] = options.Title
	m["OptionHideVersion"] = options.HideVersion
	if !options.HideVersion {
		m["Version"] = version.Version
		m["BuildDate"] = version.BuildDate
		m["COMMIT"] = version.Commit
	}
	m["OptionFooter"] = template.HTML(options.Footer)
	return c.Status(http.StatusOK).Render("settings.html", m)
}
