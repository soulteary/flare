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

func GenerateApplicationsTemplate(filter string, options *model.Application, requestInfo ...*fn.DynamicURL) template.HTML {
	markup, _ := generateApplications(filter, options, requestInfo...)
	return markup
}

func generateApplications(filter string, options *model.Application, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	if options == nil {
		op, err := data.GetAllSettingsOptions()
		if err != nil {
			return "", navigationDataError("applications", "", err)
		}
		options = &op
	}
	appsData, err := data.LoadFavoriteBookmarks()
	if err != nil {
		return "", navigationDataError("applications", options.Locale, err)
	}
	b, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		b = &strings.Builder{}
	}
	b.Reset()
	defer builderPool.Put(b)

	apps := make([]model.Bookmark, 0, len(appsData.Items))
	filterLower := strings.ToLower(filter)
	for _, app := range appsData.Items {
		app.URL = fn.ParseDynamicUrlWith(app.URL, firstRequestInfo(requestInfo))
		if filter != "" && !strings.Contains(strings.ToLower(app.Name), filterLower) && !strings.Contains(strings.ToLower(app.URL), filterLower) && !strings.Contains(strings.ToLower(app.Desc), filterLower) {
			continue
		}
		apps = append(apps, app)
	}

	for _, app := range apps {
		desc := app.Desc
		if desc == "" {
			desc = app.URL
		}
		templateURL := navigationHref(app.URL, options.EnableEncryptedLink, firstRequestInfo(requestInfo))
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
	// #nosec G203 -- Names, descriptions, and attributes are HTML-escaped; navigationHref validates schemes and navigationIcon emits validated images or bundled SVG.
	return template.HTML(b.String()), ""
}

func firstRequestInfo(info []*fn.DynamicURL) *fn.DynamicURL {
	if len(info) > 0 {
		return info[0]
	}
	return nil
}
