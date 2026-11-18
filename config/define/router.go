package define

import (
	"github.com/soulteary/flare/config/model"
)

func getRegularPages() model.RouteMaps {
	return model.RouteMaps{
		// 应用首页
		Home: model.Page{
			Name:  "Home",
			Title: "Home",
			Path:  "/",
		},
		// 设置页面
		Settings: model.Page{
			Name:  "Settings",
			Title: "Settings",
			Path:  "/settings",
		},
		// 应用页面
		Applications: model.Page{
			Name:  "Applications",
			Title: "Applications",
			Path:  "/applications",
		},
		// 书签页面
		Bookmarks: model.Page{
			Name:  "Bookmarks",
			Title: "Bookmarks",
			Path:  "/bookmarks",
		},
		// 帮助页面
		Help: model.Page{
			Name:  "Help",
			Title: "Help",
			Path:  "/help",
		},
		// 使用向导
		Guide: model.Page{
			Name:  "Guide",
			Title: "Guide",
			Path:  "/guide",
		},
		// 使用向导
		Editor: model.Page{
			Name:  "Editor",
			Title: "Editor",
			Path:  "/settings/content",
		},
		// MDI
		Icons: model.Page{
			Name:  "Icons",
			Title: "MDI",
			Path:  "/icons",
		},
	}
}

var RegularPages = getRegularPages()

func getSettingPages() model.RouteMaps {
	return model.RouteMaps{
		Theme: model.Page{
			Name:  "Theme",
			Title: "主题",
			Path:  "/settings/theme",
		},
		Weather: model.Page{
			Name:  "Weather",
			Title: "天气",
			Path:  "/settings/weather",
		},

		Search: model.Page{
			Name:  "Search",
			Title: "搜索",
			Path:  "/settings/search",
		},

		// The two pages below hold what the theme page held beside choosing
		// a theme: the files of the themes of the instance, and what the
		// administrator allows themes and visitors.
		ThemeLibrary: model.Page{
			Name:  "ThemeLibrary",
			Title: "主题库",
			Path:  "/settings/themes",
		},

		ThemeAccess: model.Page{
			Name:  "ThemeAccess",
			Title: "主题授权",
			Path:  "/settings/theme-access",
		},

		// The five pages below hold what one page, the appearance page,
		// held before.
		Modules: model.Page{
			Name:  "Modules",
			Title: "模块",
			Path:  "/settings/modules",
		},

		Clock: model.Page{
			Name:  "Clock",
			Title: "时钟",
			Path:  "/settings/clock",
		},

		Links: model.Page{
			Name:  "Links",
			Title: "链接",
			Path:  "/settings/links",
		},

		Layout: model.Page{
			Name:  "Layout",
			Title: "布局",
			Path:  "/settings/layout",
		},

		Site: model.Page{
			Name:  "Site",
			Title: "基本信息",
			Path:  "/settings/site",
		},

		// Others is the account page. Its address is the one sign-in leads
		// to and comes back to, so it keeps the address it always had.
		Others: model.Page{
			Name:  "Others",
			Title: "账号",
			Path:  "/settings/application",
		},

		About: model.Page{
			Name:  "About",
			Title: "关于",
			Path:  "/settings/about",
		},
	}
}

var SettingPages = getSettingPages()

func getSettingAPIs() model.RouteMaps {
	return model.RouteMaps{
		WeatherTest: model.API{
			Name: "Weather Tester",
			Path: "/settings/weather/test",
		},
	}
}

var SettingPagesAPI = getSettingAPIs()

func getMiscPages() model.RouteMaps {
	return model.RouteMaps{
		HealthCheck: model.API{
			Name: "HealthCheck",
			Path: "/ping",
		},

		RedirHome: model.Page{
			Title: "正在跳转...",
			Name:  "Redir",
			Path:  "/redir",
		},
		RedirHelper: model.API{
			Name: "RedirHelper",
			Path: "/redir/url",
		},

		Login: model.API{
			Name: "Login",
			Path: "/login",
		},
		Logout: model.API{
			Name: "Logout",
			Path: "/logout",
		},
	}
}

var MiscPages = getMiscPages()
