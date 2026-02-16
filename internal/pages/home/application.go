package home

import (
	"html/template"
	"strings"
	"sync"

	FlareData "github.com/soulteary/flare/config/data"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareFn "github.com/soulteary/flare/internal/fn"
	FlareMDI "github.com/soulteary/flare/internal/resources/mdi"
)

var builderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

func GenerateApplicationsTemplate(filter string, options *FlareModel.Application) template.HTML {
	if options == nil {
		op := FlareData.GetAllSettingsOptions()
		options = &op
	}
	appsData := FlareData.LoadFavoriteBookmarks()
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	defer builderPool.Put(b)

	n := len(appsData.Items)
	parseApps := make([]FlareModel.Bookmark, 0, n)
	for _, app := range appsData.Items {
		app.URL = FlareFn.ParseDynamicUrl(app.URL)
		parseApps = append(parseApps, app)
	}

	var apps []FlareModel.Bookmark
	if filter != "" {
		apps = make([]FlareModel.Bookmark, 0, n)
	}

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
		desc := app.Desc
		if desc == "" {
			desc = app.URL
		}
		templateURL := app.URL
		if strings.HasPrefix(app.URL, "chrome-extension://") || options.EnableEncryptedLink {
			templateURL = "/redir/url?go=" + FlareData.Base64EncodeUrl(app.URL)
		}
		templateIcon := FlareMDI.GetIconByName(app.Icon)
		if strings.HasPrefix(app.Icon, "http://") || strings.HasPrefix(app.Icon, "https://") {
			templateIcon = `<img src="` + app.Icon + `"/>`
		} else if app.Icon != "" {
			templateIcon = FlareMDI.GetIconByName(app.Icon)
		} else if options.IconMode == "FILLING" {
			templateIcon = FlareFn.GetYandexFavicon(app.URL, FlareMDI.GetIconByName(app.Icon))
		}
		if options.OpenAppNewTab {
			b.WriteString(`<div class="app-container" data-id="`)
			b.WriteString(app.Icon)
			b.WriteString(`"><a target="_blank" rel="noopener" href="`)
			b.WriteString(templateURL)
			b.WriteString(`" class="app-item" title="`)
			b.WriteString(app.Name)
			b.WriteString(`"><div class="app-icon">`)
			b.WriteString(templateIcon)
			b.WriteString(`</div><div class="app-text"><p class="app-title">`)
			b.WriteString(app.Name)
			b.WriteString(`</p><p class="app-desc">`)
			b.WriteString(desc)
			b.WriteString(`</p></div></a></div>`)
		} else {
			b.WriteString(`<div class="app-container" data-id="`)
			b.WriteString(app.Icon)
			b.WriteString(`"><a rel="noopener" href="`)
			b.WriteString(templateURL)
			b.WriteString(`" class="app-item" title="`)
			b.WriteString(app.Name)
			b.WriteString(`"><div class="app-icon">`)
			b.WriteString(templateIcon)
			b.WriteString(`</div><div class="app-text"><p class="app-title">`)
			b.WriteString(app.Name)
			b.WriteString(`</p><p class="app-desc">`)
			b.WriteString(desc)
			b.WriteString(`</p></div></a></div>`)
		}
	}
	return template.HTML(b.String())
}
