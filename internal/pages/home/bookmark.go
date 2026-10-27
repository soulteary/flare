package home

import (
	"html"
	"html/template"
	"net/url"
	"strings"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/i18n"
)

func GenerateBookmarkTemplate(filter string, options *model.Application) template.HTML {
	return GenerateBookmarkNavigationTemplate(filter, "", "/", options)
}

// GenerateBookmarkNavigationTemplate keeps category navigation server-rendered and search-scoped.
func GenerateBookmarkNavigationTemplate(filter, selected, path string, options *model.Application, requestInfo ...*fn.DynamicURL) template.HTML {
	markup, _ := generateBookmarkNavigation(filter, selected, path, options, requestInfo...)
	return markup
}

func generateBookmarkNavigation(filter, selected, path string, options *model.Application, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	return generateBookmarkNavigationFor(filter, selected, path, options, false, requestInfo...)
}

func generateBookmarkNavigationFor(filter, selected, path string, options *model.Application, administrator bool, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	return generateBookmarkNavigationForRead(filter, selected, path, options, administrator, false, requestInfo...)
}
func generateBookmarkNavigationForRead(filter, selected, path string, options *model.Application, administrator, existingOnly bool, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	if options == nil {
		readSettings := data.GetAllSettingsOptions
		if existingOnly {
			readSettings = data.ReadExistingSettingsOptions
		}
		op, err := readSettings()
		if err != nil {
			return "", navigationDataError("bookmarks", "", err)
		}
		options = &op
	}
	_, view, err := data.LoadNavigationViews(false, true, administrator, existingOnly)
	if err != nil {
		return "", navigationDataError("bookmarks", options.Locale, err)
	}
	return navigationFragment(view, options, false, filter, selected, path, firstRequestInfo(requestInfo)), ""
}
func generateBookmarkSnapshot(filter, selected, path string, options *model.Application, bookmarksData model.Bookmarks, requestInfo ...*fn.DynamicURL) (template.HTML, string) {
	info := firstRequestInfo(requestInfo)
	matches := make([]int, 0, len(bookmarksData.Items))
	filterLower := strings.ToLower(filter)
	for i, bookmark := range bookmarksData.Items {
		if filter != "" && !strings.Contains(strings.ToLower(bookmark.Name), filterLower) && !strings.Contains(strings.ToLower(expandNavigationURL(bookmark.URL, info)), filterLower) {
			continue
		}
		matches = append(matches, i)
	}
	return renderBookmarkSelection(filter, selected, path, options, bookmarksData.Categories, func(i int) model.Bookmark { return bookmarksData.Items[i] }, matches, info)
}

func renderBookmarkSelection(filter, selected, path string, options *model.Application, categories []model.Category, item func(int) model.Bookmark, matches []int, info *fn.DynamicURL) (template.HTML, string) {
	privateCategories := make(map[string]bool, len(categories))
	for _, c := range categories {
		privateCategories[c.ID] = c.Private
	}
	b, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		b = &strings.Builder{}
	}
	b.Reset()
	defer releaseNavigationBuilder(b)
	writeItem := func(index int) {
		bookmark := item(index)
		bookmark.URL = expandNavigationURL(bookmark.URL, info)
		bookmark.Private = bookmark.Private || privateCategories[bookmark.Category]
		writeBookmarkItem(b, bookmark, options.OpenBookmarkNewTab, options.EnableEncryptedLink, options.IconMode, info)
	}
	if len(categories) == 0 {
		growNavigationBuilder(b, bookmarkSelectionEstimate(item, matches))
		b.WriteString(`<div class="bookmark-group-container pull-left"><ul class="bookmark-list">`)
		for _, index := range matches {
			writeItem(index)
		}
		b.WriteString(`</ul></div>`)
	} else {
		renderBookmarkIndexGroupsRead(b, categories, item, matches, selected, filter, path, options, writeItem)
	}
	// #nosec G203 -- Names/categories are escaped and URL/icon policies are enforced at item emission.
	return template.HTML(b.String()), ""
}

func writeBookmarkItem(b *strings.Builder, bookmark model.Bookmark, newTab, encoded bool, iconMode string, info *fn.DynamicURL) {
	if newTab {
		b.WriteString(`<li><a target="_blank" rel="noopener" href="`)
	} else {
		b.WriteString(`<li><a rel="noopener" href="`)
	}
	b.WriteString(html.EscapeString(navigationHref(bookmark.URL, encoded, info)))
	b.WriteString(`" class="bookmark">`)
	b.WriteString(navigationIcon(bookmark.Icon, bookmark.URL, iconMode, bookmark.Private))
	b.WriteString(`<span>`)
	b.WriteString(html.EscapeString(bookmark.Name))
	b.WriteString(`</span></a></li>`)
}

func writeBookmarkGroupStart(b *strings.Builder, category model.Category, collapse bool) {
	if collapse {
		b.WriteString(`<details class="bookmark-group-container bookmark-collapsible"><summary class="bookmark-group-title">`)
	} else {
		b.WriteString(`<div class="bookmark-group-container pull-left"><h3 class="bookmark-group-title">`)
	}
	b.WriteString(html.EscapeString(category.Name))
	if collapse {
		b.WriteString(`</summary><ul class="bookmark-list">`)
	} else {
		b.WriteString(`</h3><ul class="bookmark-list">`)
	}
}
func writeBookmarkGroupEnd(b *strings.Builder, collapse bool) {
	if collapse {
		b.WriteString(`</ul></details>`)
	} else {
		b.WriteString(`</ul></div>`)
	}
}

func renderBookmarkIndexGroupsRead(b *strings.Builder, categories []model.Category, item func(int) model.Bookmark, matches []int, selected, filter, path string, options *model.Application, writeItem func(int)) {
	mode := "all"
	if options.Homepage != nil && options.Homepage.CategoryNavigation != "" {
		mode = options.Homepage.CategoryNavigation
	}
	valid := false
	for _, category := range categories {
		if category.ID == selected {
			valid = true
		}
	}
	if !valid || mode != "tabs" {
		selected = ""
	}
	defaultID := categories[0].ID
	// A selected tab needs neither counts nor buckets for other categories.
	if selected != "" {
		var chosen model.Category
		for _, category := range categories {
			if category.ID == selected {
				chosen = category
				break
			}
		}
		belongs := func(index int) bool {
			id := item(index).Category
			if id == "" {
				id = defaultID
			}
			return id == selected
		}
		estimate := 0
		count := 0
		for _, index := range matches {
			if belongs(index) {
				value := item(index)
				estimate = boundedNavigationEstimate(estimate, 128+len(value.Name)+len(value.URL)+len(value.Icon))
				count++
			}
		}
		growNavigationBuilder(b, boundedNavigationEstimate(estimate, 128+len(chosen.Name)))
		renderCategoryNavigation(b, categories, selected, filter, path, options.Locale)
		if count > 0 {
			writeBookmarkGroupStart(b, chosen, false)
			for _, index := range matches {
				if belongs(index) {
					writeItem(index)
				}
			}
			writeBookmarkGroupEnd(b, false)
		}
		return
	}
	counts := make(map[string]int, len(categories))
	for _, index := range matches {
		id := item(index).Category
		if id == "" {
			id = defaultID
		}
		counts[id]++
	}
	groups := make(map[string][]int, len(categories))
	for _, category := range categories {
		if selected != "" && category.ID != selected {
			continue
		}
		if _, exists := groups[category.ID]; !exists && counts[category.ID] > 0 {
			groups[category.ID] = make([]int, 0, counts[category.ID])
		}
	}
	for _, index := range matches {
		id := item(index).Category
		if id == "" {
			id = defaultID
		}
		if group, exists := groups[id]; exists {
			groups[id] = append(group, index)
		}
	}
	estimate := 0
	for _, category := range categories {
		if (selected == "" || category.ID == selected) && len(groups[category.ID]) > 0 {
			estimate = boundedNavigationEstimate(estimate, bookmarkSelectionEstimate(item, groups[category.ID])+128+len(category.Name))
		}
		if mode == "tabs" {
			estimate = boundedNavigationEstimate(estimate, 128+len(category.Name)+len(category.ID)+len(filter))
		}
	}
	growNavigationBuilder(b, estimate)
	if mode == "tabs" {
		renderCategoryNavigation(b, categories, selected, filter, path, options.Locale)
	}
	collapse := mode == "collapse" && filter == ""
	for _, category := range categories {
		if selected != "" && category.ID != selected {
			continue
		}
		indexes := groups[category.ID]
		if len(indexes) == 0 {
			continue
		}
		writeBookmarkGroupStart(b, category, collapse)
		for _, index := range indexes {
			writeItem(index)
		}
		writeBookmarkGroupEnd(b, collapse)
	}
}

func renderBookmarksWithCategories(b *strings.Builder, bookmarks *[]model.Bookmark, category *model.Category, defaultCategory *model.Category, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string, collapse ...bool) {
	renderBookmarksWithCategoriesForRequest(b, bookmarks, category, defaultCategory, OpenBookmarkNewTab, EnableEncryptedLink, IconMode, nil, collapse...)
}

func renderBookmarksWithCategoriesForRequest(b *strings.Builder, bookmarks *[]model.Bookmark, category *model.Category, defaultCategory *model.Category, newTab, encoded bool, iconMode string, info *fn.DynamicURL, collapse ...bool) {
	opened := false
	collapsible := len(collapse) > 0 && collapse[0]
	for _, bookmark := range *bookmarks {
		id := bookmark.Category
		if id == "" {
			id = defaultCategory.ID
		}
		if id != category.ID {
			continue
		}
		if !opened {
			writeBookmarkGroupStart(b, *category, collapsible)
			opened = true
		}
		writeBookmarkItem(b, bookmark, newTab, encoded, iconMode, info)
	}
	if opened {
		writeBookmarkGroupEnd(b, collapsible)
	}
}

// renderCategoryNavigation preserves search terms and category IDs in escaped links.
func renderCategoryNavigation(b *strings.Builder, categories []model.Category, selected, filter, path, locale string) {
	b.WriteString(`<nav class="bookmark-category-nav" aria-label="`)
	all, label := i18n.T(locale, "navigation.all"), i18n.T(locale, "navigation.categories")
	b.WriteString(html.EscapeString(label))
	b.WriteString(`">`)
	items := append([]model.Category{{Name: all}}, categories...)
	for _, category := range items {
		query := url.Values{}
		if category.ID != "" {
			query.Set("category", category.ID)
		}
		if filter != "" {
			query.Set("search", filter)
		}
		href := path
		if len(query) > 0 {
			href += "?" + query.Encode()
		}
		b.WriteString(`<a href="`)
		b.WriteString(html.EscapeString(href))
		b.WriteString(`"`)
		if category.ID == selected {
			b.WriteString(` aria-current="page"`)
		}
		b.WriteString(`>`)
		b.WriteString(html.EscapeString(category.Name))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</nav>`)
}

func bookmarkSelectionEstimate(item func(int) model.Bookmark, indexes []int) int {
	estimate := 0
	for _, index := range indexes {
		value := item(index)
		estimate = boundedNavigationEstimate(estimate, 128+len(value.Name)+len(value.URL)+len(value.Icon))
	}
	return estimate
}
