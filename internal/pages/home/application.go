package home

import (
	"html"
	"html/template"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
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
	appsData, err := data.LoadFavoriteBookmarks()
	if err != nil {
		return template.HTML("")
	}
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
		templateURL := navigationHref(app.URL, options.EnableEncryptedLink)
		templateIcon := navigationIcon(app.Icon, app.URL, options.IconMode)
		if options.OpenAppNewTab {
			b.WriteString(`<div class="app-container" data-id="`)
			b.WriteString(html.EscapeString(app.Icon))
			b.WriteString(`"><a target="_blank" rel="noopener" href="`)
			b.WriteString(html.EscapeString(templateURL))
			b.WriteString(`" class="app-item" title="`)
			b.WriteString(html.EscapeString(app.Name))
			b.WriteString(`"><div class="app-icon">`)
			b.WriteString(templateIcon)
			b.WriteString(`</div><div class="app-text"><p class="app-title">`)
			b.WriteString(html.EscapeString(app.Name))
			b.WriteString(`</p><p class="app-desc">`)
			b.WriteString(html.EscapeString(desc))
			b.WriteString(`</p></div></a></div>`)
		} else {
			b.WriteString(`<div class="app-container" data-id="`)
			b.WriteString(html.EscapeString(app.Icon))
			b.WriteString(`"><a rel="noopener" href="`)
			b.WriteString(html.EscapeString(templateURL))
			b.WriteString(`" class="app-item" title="`)
			b.WriteString(html.EscapeString(app.Name))
			b.WriteString(`"><div class="app-icon">`)
			b.WriteString(templateIcon)
			b.WriteString(`</div><div class="app-text"><p class="app-title">`)
			b.WriteString(html.EscapeString(app.Name))
			b.WriteString(`</p><p class="app-desc">`)
			b.WriteString(html.EscapeString(desc))
			b.WriteString(`</p></div></a></div>`)
		}
	}
	return template.HTML(b.String())
}
