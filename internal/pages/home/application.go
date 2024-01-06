package home

import (
	"html/template"
	"strings"

	FlareData "github.com/soulteary/flare/config/data"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareFn "github.com/soulteary/flare/internal/fn"
	FlareMDI "github.com/soulteary/flare/internal/resources/mdi"
)

func GenerateApplicationsTemplate(filter string) template.HTML {
	options := FlareData.GetAllSettingsOptions()
	appsData := FlareData.LoadFavoriteBookmarks()
	tpl := ""

	var parseApps []FlareModel.Bookmark
	for _, app := range appsData.Items {
		app.URL = FlareFn.ParseDynamicUrl(app.URL)
		parseApps = append(parseApps, app)
	}

	var apps []FlareModel.Bookmark

	if filter != "" {
		filterLower := strings.ToLower(filter)
		for _, bookmark := range parseApps {
			if strings.Contains(strings.ToLower(bookmark.Name), filterLower) || strings.Contains(strings.ToLower(bookmark.URL), filterLower) || strings.Contains(strings.ToLower(bookmark.Desc), filterLower) {
				apps = append(apps, bookmark)
			}
		}
	} else {
		apps = parseApps
	}

	for _, app := range apps {

		desc := ""
		if app.Desc == "" {
			desc = app.URL
		} else {
			desc = app.Desc
		}

		// 如果以 chrome-extension:// 协议开头
		// 则使用服务端 Location 方式打开链接
		templateURL := ""
		if strings.HasPrefix(app.URL, "chrome-extension://") {
			templateURL = "/redir/url?go=" + FlareData.Base64EncodeUrl(app.URL)
		} else {
			if options.EnableEncryptedLink {
				templateURL = "/redir/url?go=" + FlareData.Base64EncodeUrl(app.URL)
			} else {
				templateURL = app.URL
			}
		}

		templateIcon := ""
		if strings.HasPrefix(app.Icon, "http://") || strings.HasPrefix(app.Icon, "https://") {
			templateIcon = `<img src="` + app.Icon + `"/>`
		} else if app.Icon != "" {
			templateIcon = FlareMDI.GetIconByName(app.Icon)
		} else {
			if options.IconMode == "FILLING" {
				templateIcon = FlareFn.GetYandexFavicon(app.URL, FlareMDI.GetIconByName(app.Icon))
			} else {
				templateIcon = FlareMDI.GetIconByName(app.Icon)
			}
		}

		if options.OpenAppNewTab {
			tpl = tpl + `
			<div class="app-container" data-id="` + app.Icon + `">
			<a target="_blank" rel="noopener" href="` + templateURL + `" class="app-item" title="` + app.Name + `">
			  <div class="app-icon">` + templateIcon + `</div>
			  <div class="app-text">
				<p class="app-title">` + app.Name + `</p>
				<p class="app-desc">` + desc + `</p>
			  </div>
			</a>
			</div>
			`
		} else {
			tpl = tpl + `
			<div class="app-container" data-id="` + app.Icon + `">
			<a rel="noopener" href="` + templateURL + `" class="app-item" title="` + app.Name + `">
			  <div class="app-icon">` + templateIcon + `</div>
			  <div class="app-text">
				<p class="app-title">` + app.Name + `</p>
				<p class="app-desc">` + desc + `</p>
			  </div>
			</a>
			</div>
			`
		}
	}
	return template.HTML(tpl)
}
