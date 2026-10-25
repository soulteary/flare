package home

import (
	"html"
	"html/template"

	"github.com/soulteary/flare/internal/logger"
)

// navigationDataError keeps saved-data failures visible without exposing local
// filenames or parser details to page visitors. Operators get the original error.
func navigationDataError(module, locale string, err error) template.HTML {
	logger.GetLogger().Error("Cannot render navigation module", "module", module, "error", err.Error())
	message := "Unable to load saved bookmarks. Check the configuration and server logs."
	if module == "applications" {
		message = "Unable to load saved applications. Check the configuration and server logs."
	}
	if locale == "zh" || locale == "" {
		message = "无法读取书签数据，请检查配置文件和服务日志。"
		if module == "applications" {
			message = "无法读取应用数据，请检查配置文件和服务日志。"
		}
	}
	return template.HTML(`<p class="navigation-data-error" role="alert">` + html.EscapeString(message) + `</p>`)
}
