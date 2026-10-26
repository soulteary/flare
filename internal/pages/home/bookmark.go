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
	if options == nil {
		op, err := data.GetAllSettingsOptions()
		if err != nil {
			return navigationDataError("bookmarks", "", err)
		}
		options = &op
	}
	bookmarksData, err := data.LoadNormalBookmarks()
	if err != nil {
		return navigationDataError("bookmarks", options.Locale, err)
	}
	b, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		b = &strings.Builder{}
	}
	b.Reset()
	defer builderPool.Put(b)

	bookmarks := make([]model.Bookmark, 0, len(bookmarksData.Items))
	filterLower := strings.ToLower(filter)
	for _, bookmark := range bookmarksData.Items {
		bookmark.URL = fn.ParseDynamicUrlWith(bookmark.URL, firstRequestInfo(requestInfo))
		if filter != "" && !strings.Contains(strings.ToLower(bookmark.Name), filterLower) && !strings.Contains(strings.ToLower(bookmark.URL), filterLower) {
			continue
		}
		bookmarks = append(bookmarks, bookmark)
	}

	if len(bookmarksData.Categories) > 0 {
		mode := "all"
		if options.Homepage != nil && options.Homepage.CategoryNavigation != "" {
			mode = options.Homepage.CategoryNavigation
		}
		valid := false
		for _, category := range bookmarksData.Categories {
			if category.ID == selected {
				valid = true
			}
		}
		if !valid || mode != "tabs" {
			selected = ""
		}
		if mode == "tabs" {
			renderCategoryNavigation(b, bookmarksData.Categories, selected, filter, path, options.Locale)
		}
		defaultCategory := bookmarksData.Categories[0]
		// Group once before rendering. Default-placement entries follow the first
		// named group; unknown IDs remain absent, matching the existing behavior.
		groups := make(map[string][]model.Bookmark, len(bookmarksData.Categories))
		for _, bookmark := range bookmarks {
			id := bookmark.Category
			if id == "" {
				id = defaultCategory.ID
			}
			groups[id] = append(groups[id], bookmark)
		}
		for _, category := range bookmarksData.Categories {
			if selected != "" && category.ID != selected {
				continue
			}
			categoryCopy := category
			items := groups[category.ID]
			renderBookmarksWithCategoriesForRequest(b, &items, &categoryCopy, &defaultCategory, options.OpenBookmarkNewTab, options.EnableEncryptedLink, options.IconMode, firstRequestInfo(requestInfo), mode == "collapse" && filter == "")
		}
	} else {
		renderBookmarksWithoutCategories(b, &bookmarks, options.OpenBookmarkNewTab, options.EnableEncryptedLink, options.IconMode, firstRequestInfo(requestInfo))
	}

	return template.HTML(b.String())
}

func renderBookmarksWithoutCategories(b *strings.Builder, bookmarks *[]model.Bookmark, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string, requestInfo ...*fn.DynamicURL) {
	b.WriteString(`<div class="bookmark-group-container pull-left"><ul class="bookmark-list">`)
	for _, bookmark := range *bookmarks {
		templateURL := navigationHref(bookmark.URL, EnableEncryptedLink, firstRequestInfo(requestInfo))
		templateIcon := navigationIcon(bookmark.Icon, bookmark.URL, IconMode)
		if OpenBookmarkNewTab {
			b.WriteString(`<li><a target="_blank" rel="noopener" href="`)
			b.WriteString(html.EscapeString(templateURL))
			b.WriteString(`" class="bookmark">`)
			b.WriteString(templateIcon)
			b.WriteString(`<span>`)
			b.WriteString(html.EscapeString(bookmark.Name))
			b.WriteString(`</span></a></li>`)
		} else {
			b.WriteString(`<li><a rel="noopener" href="`)
			b.WriteString(html.EscapeString(templateURL))
			b.WriteString(`" class="bookmark">`)
			b.WriteString(templateIcon)
			b.WriteString(`<span>`)
			b.WriteString(html.EscapeString(bookmark.Name))
			b.WriteString(`</span></a></li>`)
		}
	}
	b.WriteString(`</ul></div>`)
}

func renderBookmarksWithCategories(b *strings.Builder, bookmarks *[]model.Bookmark, category *model.Category, defaultCategory *model.Category, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string, collapse ...bool) {
	renderBookmarksWithCategoriesForRequest(b, bookmarks, category, defaultCategory, OpenBookmarkNewTab, EnableEncryptedLink, IconMode, nil, collapse...)
}

func renderBookmarksWithCategoriesForRequest(b *strings.Builder, bookmarks *[]model.Bookmark, category *model.Category, defaultCategory *model.Category, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string, info *fn.DynamicURL, collapse ...bool) {
	var itemBuf strings.Builder
	for _, bookmark := range *bookmarks {
		matched := false
		if bookmark.Category != "" {
			matched = bookmark.Category == category.ID
		} else {
			matched = category.ID == defaultCategory.ID
		}
		if !matched {
			continue
		}
		templateURL := navigationHref(bookmark.URL, EnableEncryptedLink, info)
		templateIcon := navigationIcon(bookmark.Icon, bookmark.URL, IconMode)
		if OpenBookmarkNewTab {
			itemBuf.WriteString(`<li><a target="_blank" rel="noopener" href="`)
		} else {
			itemBuf.WriteString(`<li><a rel="noopener" href="`)
		}
		itemBuf.WriteString(html.EscapeString(templateURL))
		itemBuf.WriteString(`" class="bookmark">`)
		itemBuf.WriteString(templateIcon)
		itemBuf.WriteString(`<span>`)
		itemBuf.WriteString(html.EscapeString(bookmark.Name))
		itemBuf.WriteString(`</span></a></li>`)
	}
	if itemBuf.Len() == 0 {
		return
	}
	collapsible := len(collapse) > 0 && collapse[0]
	if collapsible {
		b.WriteString(`<details class="bookmark-group-container bookmark-collapsible"><summary class="bookmark-group-title">`)
	} else {
		b.WriteString(`<div class="bookmark-group-container pull-left"><h3 class="bookmark-group-title">`)
	}
	b.WriteString(html.EscapeString(category.Name))
	if collapsible {
		b.WriteString(`</summary><ul class="bookmark-list">`)
	} else {
		b.WriteString(`</h3><ul class="bookmark-list">`)
	}
	b.WriteString(itemBuf.String())
	if collapsible {
		b.WriteString(`</ul></details>`)
	} else {
		b.WriteString(`</ul></div>`)
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
