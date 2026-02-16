package home

import (
	"html/template"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/resources/mdi"
)

func GenerateHelpTemplate() template.HTML {
	apps := []model.Bookmark{}
	apps = append(apps, []model.Bookmark{
		{
			Name: "程序首页",
			URL:  define.RegularPages.Home.Path,
			Icon: "homeCircle",
			Desc: "",
		},
		{
			Name: "帮助页面",
			URL:  define.RegularPages.Help.Path,
			Icon: "helpCircle",
			Desc: "",
		},
		{
			Name: "程序设置",
			URL:  define.RegularPages.Settings.Path,
			Icon: "fireCircle",
			Desc: "",
		},
	}...)

	if define.AppFlags.EnableGuide {
		apps = append(apps, model.Bookmark{
			Name: "向导页面",
			URL:  define.RegularPages.Guide.Path,
			Icon: "radioactiveCircleOutline",
			Desc: "",
		})
	}

	if define.AppFlags.EnableEditor {
		apps = append(apps, model.Bookmark{
			Name: "内容编辑",
			URL:  define.RegularPages.Editor.Path,
			Icon: "pencilCircle",
			Desc: "",
		})
	}

	apps = append(apps, []model.Bookmark{
		{
			Name: "图标挑选",
			URL:  define.RegularPages.Icons.Path,
			Icon: "heartCircle",
			Desc: "",
		},
		{
			Name: "主题设置",
			URL:  define.SettingPages.Theme.Path,
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
			URL:  define.SettingPages.Weather.Path,
			Icon: "leafCircle",
			Desc: "",
		},
		{
			Name: "搜索设置",
			URL:  define.SettingPages.Search.Path,
			Icon: "lightningBoltCircle",
			Desc: "",
		},
		{
			Name: "界面设置",
			URL:  define.SettingPages.Appearance.Path,
			Icon: "leafCircle",
			Desc: "",
		},
		{
			Name: "程序版本",
			URL:  define.SettingPages.Others.Path,
			Icon: "commaCircle",
			Desc: "",
		},
		{
			Name: "问题反馈",
			URL:  "https://github.com/soulteary/docker-flare/issues",
			Icon: "crownCircle",
			Desc: "GitHub Issues",
		},
	}...)

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
			  <div class="app-icon">` + mdi.GetIconByName(app.Icon) + `</div>
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
