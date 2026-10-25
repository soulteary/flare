package home

import (
	"html/template"

	"github.com/soulteary/flare/internal/logger"
)

// navigationDataError keeps saved-data failures visible without exposing local
// filenames or parser details to page visitors. Operators get the original error.
func navigationDataError(module, locale string, err error) template.HTML {
	logger.GetLogger().Error("Cannot render navigation module", "module", module, "error", err.Error())
	// Every response is a fixed HTML literal; no diagnostic or field value enters
	// trusted markup. Keep this boundary explicit for static security analysis.
	const appsEN template.HTML = `<p class="navigation-data-error" role="alert">Unable to load saved applications. Check the configuration and server logs.</p>`
	const bookmarksEN template.HTML = `<p class="navigation-data-error" role="alert">Unable to load saved bookmarks. Check the configuration and server logs.</p>`
	const appsZH template.HTML = `<p class="navigation-data-error" role="alert">无法读取应用数据，请检查配置文件和服务日志。</p>`
	const bookmarksZH template.HTML = `<p class="navigation-data-error" role="alert">无法读取书签数据，请检查配置文件和服务日志。</p>`
	if locale == "zh" || locale == "" {
		if module == "applications" {
			return appsZH
		}
		return bookmarksZH
	}
	if module == "applications" {
		return appsEN
	}
	return bookmarksEN
}
