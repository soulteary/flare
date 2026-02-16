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
			Path:  "/editor",
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

		Appearance: model.Page{
			Name:  "Appearance",
			Title: "界面",
			Path:  "/settings/appearance",
		},

		Others: model.Page{
			Name:  "Others",
			Title: "其他",
			Path:  "/settings/application",
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
