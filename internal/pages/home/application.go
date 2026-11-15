package home

import (
	"html"
	"html/template"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/themetemplate"
)

var builderPool = sync.Pool{
	New: func() any { return &strings.Builder{} },
}

func GenerateApplicationsTemplate(filter string, options *model.Application, requestInfo ...*fn.DynamicURL) template.HTML {
	markup, _ := generateApplications(filter, options, requestInfo...)
	return markup
}

func generateApplications(filter string, options *model.Application, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	return generateApplicationsFor(filter, options, false, requestInfo...)
}

func generateApplicationsFor(filter string, options *model.Application, administrator bool, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	return generateApplicationsForRead(nil, filter, options, administrator, false, requestInfo...)
}
func generateApplicationsForRead(items *define.ThemeItems, filter string, options *model.Application, administrator, existingOnly bool, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	if options == nil {
		readSettings := data.GetAllSettingsOptions
		if existingOnly {
			readSettings = data.ReadExistingSettingsOptions
		}
		op, err := readSettings()
		if err != nil {
			return "", navigationDataError("applications", "", err)
		}
		options = &op
	}
	view, _, err := data.LoadNavigationViews(true, false, administrator, existingOnly)
	if err != nil {
		return "", navigationDataError("applications", options.Locale, err)
	}
	markup, _ := navigationFragmentShown(items, view, options, true, filter, "", "/applications", firstRequestInfo(requestInfo))
	return markup, ""
}
func generateApplicationsSnapshot(filter string, options *model.Application, appsData model.Bookmarks, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	info := firstRequestInfo(requestInfo)
	matches := make([]int, 0, len(appsData.Items))
	filterLower := strings.ToLower(filter)
	for index, app := range appsData.Items {
		link := expandNavigationURL(app.URL, info)
		if filter != "" && !strings.Contains(strings.ToLower(app.Name), filterLower) && !strings.Contains(strings.ToLower(link), filterLower) && !strings.Contains(strings.ToLower(app.Desc), filterLower) {
			continue
		}
		matches = append(matches, index)
	}
	return renderApplicationSelection(options, func(i int) model.Bookmark { return appsData.Items[i] }, matches, info)
}

func renderApplicationSelection(options *model.Application, item func(int) model.Bookmark, matches []int, info *fn.DynamicURL) (template.HTML, string) {
	return renderApplicationSelectionWith(nil, options, item, matches, info)
}

// renderApplicationSelectionWith renders the applications that match, each
// with the item template of the theme where it has one that compiles for
// what the application has, and with the markup of the application otherwise.
func renderApplicationSelectionWith(items *define.ThemeItems, options *model.Application, item func(int) model.Bookmark, matches []int, info *fn.DynamicURL) (template.HTML, string) {
	var entry entryContent
	b, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		b = &strings.Builder{}
	}
	b.Reset()
	defer releaseNavigationBuilder(b)
	estimate := 0
	for _, index := range matches {
		app := item(index)
		estimate = boundedNavigationEstimate(estimate, 320+len(app.Name)*2+len(expandNavigationURL(app.URL, info))+len(app.Desc)+len(app.Icon))
	}
	growNavigationBuilder(b, estimate)
	for _, index := range matches {
		app := item(index)
		app.URL = expandNavigationURL(app.URL, info)

		desc := app.Desc
		if desc == "" {
			desc = app.URL
		}
		templateURL := navigationHref(app.URL, options.EnableEncryptedLink, info)
		templateIcon := navigationIcon(app.Icon, app.URL, options.IconMode, app.Private)
		if items != nil {
			// An application whose description the site hides is one
			// without a description for the template.
			values := themetemplate.ItemValues{Icon: templateIcon != "", Description: app.Desc != "" && !items.HideDescriptions(), NewTab: options.OpenAppNewTab}
			if fragment := items.Fragment(themetemplate.ItemApp, values); fragment != nil {
				entry = entryContent{name: app.Name, host: entryHost(app.URL), initial: entryInitial(app.Name), href: templateURL, icon: templateIcon, newTab: options.OpenAppNewTab}
				if values.Description {
					entry.description = app.Desc
				}
				fragment.RenderString(b, &entry)
				continue
			}
		}
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

// Reset drops the rendered buffer without changing the already-returned string;
// pooling builder objects must not pin multi-megabyte navigation responses.
func releaseNavigationBuilder(b *strings.Builder) { b.Reset(); builderPool.Put(b) }

const maxNavigationPreallocation = 8 << 20

func boundedNavigationEstimate(current, extra int) int {
	if current >= maxNavigationPreallocation || extra >= maxNavigationPreallocation-current {
		return maxNavigationPreallocation
	}
	return current + extra
}
func growNavigationBuilder(b *strings.Builder, estimate int) {
	if estimate > maxNavigationPreallocation {
		estimate = maxNavigationPreallocation
	}
	if estimate > 0 {
		b.Grow(estimate)
	}
}
func expandNavigationURL(raw string, info *fn.DynamicURL) string {
	if info == nil || !strings.ContainsRune(raw, '{') {
		return raw
	}
	return fn.ParseDynamicUrlWith(raw, info)
}
