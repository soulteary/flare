package main

import (
	builder "github.com/soulteary/flare/build/builder"
)

func main() {
	builder.TaskForMdi(
		"embed/assets/vendor/mdi-cheat-sheets", "internal/mdi/mdi-cheat-sheets",
		"embed/assets/vendor/mdi/mdi.js", "internal/mdi/icons.go",
	)
	builder.TaskForSimpleIcons("internal/simpleicon")
	builder.TaskForGuideAssets("embed/assets/vendor/guide-assets", "internal/guide/guide-assets")
	builder.TaskForEditorAssets("embed/assets/vendor/editor-assets", "internal/editor/editor-assets")
	builder.TaskForStyles("state/style.go")
	builder.TaskForFavicon("embed/assets/favicon.ico", "internal/assets/favicon.ico")
	builder.TaskForTemplates("embed/templates", "internal/templates/html")
}
