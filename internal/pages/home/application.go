package home

import (
	"html/template"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/mdi"
)

var builderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

func GenerateApplicationsTemplate(filter string, options *model.Application) template.HTML {
	if options == nil {
		op, err := data.GetAllSettingsOptions()
		if err != nil {
			op = model.Application{}
		}
		options = &op
	}
	appsData := data.LoadFavoriteBookmarks()
	b, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		b = &strings.Builder{}
	}
	b.Reset()
	defer builderPool.Put(b)

	n := len(appsData.Items)
	parseApps := make([]model.Bookmark, 0, n)
	for _, app := range appsData.Items {
		app.URL = fn.ParseDynamicUrl(app.URL)
		parseApps = append(parseApps, app)
	}

	var apps []model.Bookmark
	if filter != "" {
		apps = make([]model.Bookmark, 0, n)
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
			templateURL = "/redir/url?go=" + data.Base64EncodeUrl(app.URL)
		}
		templateIcon := mdi.GetIconByName(app.Icon)
		if strings.HasPrefix(app.Icon, "http://") || strings.HasPrefix(app.Icon, "https://") {
			templateIcon = `<img src="` + app.Icon + `"/>`
		} else if app.Icon != "" {
			templateIcon = mdi.GetIconByName(app.Icon)
		} else if options.IconMode == "FILLING" {
			templateIcon = fn.GetYandexFavicon(app.URL, mdi.GetIconByName(app.Icon))
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
