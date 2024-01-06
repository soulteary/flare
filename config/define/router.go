package FlareDefine

import (
	FlareModel "github.com/soulteary/flare/config/model"
)

func getRegularPages() FlareModel.RouteMaps {
	return FlareModel.RouteMaps{
		// 应用首页
		Home: FlareModel.Page{
			Name:  "Home",
			Title: "Home",
			Path:  "/",
		},
		// 设置页面
		Settings: FlareModel.Page{
			Name:  "Settings",
			Title: "Settings",
			Path:  "/settings",
		},
		// 应用页面
		Applications: FlareModel.Page{
			Name:  "Applications",
			Title: "Applications",
			Path:  "/applications",
		},
		// 书签页面
		Bookmarks: FlareModel.Page{
			Name:  "Bookmarks",
			Title: "Bookmarks",
			Path:  "/bookmarks",
		},
		// 帮助页面
		Help: FlareModel.Page{
			Name:  "Help",
			Title: "Help",
			Path:  "/help",
		},
		// 使用向导
		Guide: FlareModel.Page{
			Name:  "Guide",
			Title: "Guide",
			Path:  "/guide",
		},
		// 使用向导
		Editor: FlareModel.Page{
			Name:  "Editor",
			Title: "Editor",
			Path:  "/editor",
		},
		// MDI
		Icons: FlareModel.Page{
			Name:  "Icons",
			Title: "MDI",
			Path:  "/icons",
		},
	}
}

var RegularPages = getRegularPages()

func getSettingPages() FlareModel.RouteMaps {
	return FlareModel.RouteMaps{
		Theme: FlareModel.Page{
			Name:  "Theme",
			Title: "主题",
			Path:  "/settings/theme",
		},
		Weather: FlareModel.Page{
			Name:  "Weather",
			Title: "天气",
			Path:  "/settings/weather",
		},

		Search: FlareModel.Page{
			Name:  "Search",
			Title: "搜索",
			Path:  "/settings/search",
		},

		Appearance: FlareModel.Page{
			Name:  "Appearance",
			Title: "界面",
			Path:  "/settings/appearance",
		},

		Others: FlareModel.Page{
			Name:  "Others",
			Title: "其他",
			Path:  "/settings/application",
		},
	}
}

var SettingPages = getSettingPages()

func getSettingAPIs() FlareModel.RouteMaps {
	return FlareModel.RouteMaps{
		WeatherTest: FlareModel.API{
			Name: "Weather Tester",
			Path: "/settings/weather/test",
		},
	}
}

var SettingPagesAPI = getSettingAPIs()

func getMiscPages() FlareModel.RouteMaps {
	return FlareModel.RouteMaps{
		HealthCheck: FlareModel.API{
			Name: "HealthCheck",
			Path: "/ping",
		},

		RedirHome: FlareModel.Page{
			Title: "正在跳转...",
			Name:  "Redir",
			Path:  "/redir",
		},
		RedirHelper: FlareModel.API{
			Name: "RedirHelper",
			Path: "/redir/url",
		},

		Login: FlareModel.API{
			Name: "Login",
			Path: "/login",
		},
		Logout: FlareModel.API{
			Name: "Logout",
			Path: "/logout",
		},
	}
}

var MiscPages = getMiscPages()
