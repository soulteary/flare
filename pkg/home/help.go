package home

import (
	"html/template"

	FlareModel "github.com/soulteary/flare/model"
	FlareMDI "github.com/soulteary/flare/pkg/mdi"
	FlareState "github.com/soulteary/flare/state"
)

func GenerateHelpTemplate() template.HTML {
	apps := []FlareModel.Bookmark{
		{
			Name: "程序首页",
			URL:  FlareState.RegularPages.Home.Path,
			Icon: "homeCircle",
			Desc: "",
		},
		{
			Name: "帮助页面",
			URL:  FlareState.RegularPages.Help.Path,
			Icon: "helpCircle",
			Desc: "",
		},
		{
			Name: "程序设置",
			URL:  FlareState.RegularPages.Settings.Path,
			Icon: "fireCircle",
			Desc: "",
		},
		{
			Name: "向导页面",
			URL:  FlareState.RegularPages.Guide.Path,
			Icon: "radioactiveCircleOutline",
			Desc: "",
		},
		{
			Name: "图标挑选",
			URL:  FlareState.RegularPages.Icons.Path,
			Icon: "heartCircle",
			Desc: "",
		},
		{
			Name: "内容编辑",
			URL:  FlareState.RegularPages.Editor.Path,
			Icon: "pencilCircle",
			Desc: "",
		},
		{
			Name: "主题设置",
			URL:  FlareState.SettingPages.Theme.Path,
			Icon: "starCircle",
			Desc: "",
		},
		// {
		// 	Name: "主题预览",
		// 	URL:  "/preview",
		// 	Icon: "incognitoCircle",
		// 	Desc: "",
		// },
		{
			Name: "天气设置",
			URL:  FlareState.SettingPages.Weather.Path,
			Icon: "leafCircle",
			Desc: "",
		},
		{
			Name: "搜索设置",
			URL:  FlareState.SettingPages.Search.Path,
			Icon: "lightningBoltCircle",
			Desc: "",
		},
		{
			Name: "界面设置",
			URL:  FlareState.SettingPages.Appearance.Path,
			Icon: "leafCircle",
			Desc: "",
		},
		{
			Name: "程序版本",
			URL:  FlareState.SettingPages.Others.Path,
			Icon: "commaCircle",
			Desc: "",
		},
		{
			Name: "问题反馈",
			URL:  "https://github.com/soulteary/docker-flare/issues",
			Icon: "crownCircle",
			Desc: "GitHub Issues",
		},
	}

	tpl := ""

	for _, app := range apps {

		desc := ""
		if app.Desc == "" {
			desc = app.URL
		} else {
			desc = app.Desc
		}

		tpl = tpl + `
			<div class="app-container" data-id="` + app.Icon + `">
			<a href="` + app.URL + `" class="app-item" title="` + app.Name + `">
			  <div class="app-icon">` + FlareMDI.GetIconByName(app.Icon) + `</div>
			  <div class="app-text">
				<p class="app-title">` + app.Name + `</p>
				<p class="app-desc">` + desc + `</p>
			  </div>
			</a>
			</div>
			`
	}
	return template.HTML(tpl)
}
