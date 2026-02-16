package home

import (
	"html/template"
	"strings"

	FlareData "github.com/soulteary/flare/config/data"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareFn "github.com/soulteary/flare/internal/fn"
	FlareMDI "github.com/soulteary/flare/internal/resources/mdi"
)

func GenerateBookmarkTemplate(filter string, options *FlareModel.Application) template.HTML {
	if options == nil {
		op := FlareData.GetAllSettingsOptions()
		options = &op
	}
	bookmarksData := FlareData.LoadNormalBookmarks()
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	defer builderPool.Put(b)

	n := len(bookmarksData.Items)
	parseBookmarks := make([]FlareModel.Bookmark, 0, n)
	for _, bookmark := range bookmarksData.Items {
		bookmark.URL = FlareFn.ParseDynamicUrl(bookmark.URL)
		parseBookmarks = append(parseBookmarks, bookmark)
	}

	bookmarks := parseBookmarks
	if filter != "" {
		bookmarks = make([]FlareModel.Bookmark, 0, n)
	}

	if filter != "" {
		filterLower := strings.ToLower(filter)
		for _, bookmark := range parseBookmarks {
			if strings.Contains(strings.ToLower(bookmark.Name), filterLower) || strings.Contains(strings.ToLower(bookmark.URL), filterLower) {
				bookmarks = append(bookmarks, bookmark)
			}
		}
	}

	if len(bookmarksData.Categories) > 0 {
		defaultCategory := bookmarksData.Categories[0]
		for _, category := range bookmarksData.Categories {
			categoryCopy := category
			renderBookmarksWithCategories(b, &bookmarks, &categoryCopy, &defaultCategory, options.OpenBookmarkNewTab, options.EnableEncryptedLink, options.IconMode)
		}
	} else {
		renderBookmarksWithoutCategories(b, &bookmarks, options.OpenBookmarkNewTab, options.EnableEncryptedLink, options.IconMode)
	}

	return template.HTML(b.String())
}

func renderBookmarksWithoutCategories(b *strings.Builder, bookmarks *[]FlareModel.Bookmark, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string) {
	b.WriteString(`<div class="bookmark-group-container pull-left"><ul class="bookmark-list">`)
	for _, bookmark := range *bookmarks {
		templateURL := bookmark.URL
		if strings.HasPrefix(bookmark.URL, "chrome-extension://") || EnableEncryptedLink {
			templateURL = "/redir/url?go=" + FlareData.Base64EncodeUrl(bookmark.URL)
		}
		templateIcon := FlareMDI.GetIconByName(bookmark.Icon)
		if strings.HasPrefix(bookmark.Icon, "http://") || strings.HasPrefix(bookmark.Icon, "https://") {
			templateIcon = `<img src="` + bookmark.Icon + `"/>`
		} else if bookmark.Icon != "" {
			templateIcon = FlareMDI.GetIconByName(bookmark.Icon)
		} else if IconMode == "FILLING" {
			templateIcon = FlareFn.GetYandexFavicon(bookmark.URL, FlareMDI.GetIconByName(bookmark.Icon))
		}
		if OpenBookmarkNewTab {
			b.WriteString(`<li><a target="_blank" rel="noopener" href="`)
			b.WriteString(templateURL)
			b.WriteString(`" class="bookmark">`)
			b.WriteString(templateIcon)
			b.WriteString(`<span>`)
			b.WriteString(bookmark.Name)
			b.WriteString(`</span></a></li>`)
		} else {
			b.WriteString(`<li><a rel="noopener" href="`)
			b.WriteString(templateURL)
			b.WriteString(`" class="bookmark">`)
			b.WriteString(templateIcon)
			b.WriteString(`<span>`)
			b.WriteString(bookmark.Name)
			b.WriteString(`</span></a></li>`)
		}
	}
	b.WriteString(`</ul></div>`)
}

func renderBookmarksWithCategories(b *strings.Builder, bookmarks *[]FlareModel.Bookmark, category *FlareModel.Category, defaultCategory *FlareModel.Category, OpenBookmarkNewTab bool, EnableEncryptedLink bool, IconMode string) {
	var itemBuf strings.Builder
	for _, bookmark := range *bookmarks {
		templateURL := bookmark.URL
		if strings.HasPrefix(bookmark.URL, "chrome-extension://") || EnableEncryptedLink {
			templateURL = "/redir/url?go=" + FlareData.Base64EncodeUrl(bookmark.URL)
		}
		templateIcon := FlareMDI.GetIconByName(bookmark.Icon)
		if strings.HasPrefix(bookmark.Icon, "http://") || strings.HasPrefix(bookmark.Icon, "https://") {
			templateIcon = `<img src="` + bookmark.Icon + `"/>`
		} else if bookmark.Icon != "" {
			templateIcon = FlareMDI.GetIconByName(bookmark.Icon)
		} else if IconMode == "FILLING" {
			templateIcon = FlareFn.GetYandexFavicon(bookmark.URL, FlareMDI.GetIconByName(bookmark.Icon))
		}
		matched := false
		if bookmark.Category != "" {
			matched = bookmark.Category == category.ID
		} else {
			matched = category.ID == defaultCategory.ID
		}
		if !matched {
			continue
		}
		if OpenBookmarkNewTab {
			itemBuf.WriteString(`<li><a target="_blank" rel="noopener" href="`)
		} else {
			itemBuf.WriteString(`<li><a rel="noopener" href="`)
		}
		itemBuf.WriteString(templateURL)
		itemBuf.WriteString(`" class="bookmark">`)
		itemBuf.WriteString(templateIcon)
		itemBuf.WriteString(`<span>`)
		itemBuf.WriteString(bookmark.Name)
		itemBuf.WriteString(`</span></a></li>`)
	}
	if itemBuf.Len() == 0 {
		return
	}
	b.WriteString(`<div class="bookmark-group-container pull-left"><h3 class="bookmark-group-title">`)
	b.WriteString(category.Name)
	b.WriteString(`</h3><ul class="bookmark-list">`)
	b.WriteString(itemBuf.String())
	b.WriteString(`</ul></div>`)
}
