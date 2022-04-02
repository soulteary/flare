package main

import (
	builder "github.com/soulteary/flare/build/builder"
)

func main() {
	builder.TaskForMdi()
	builder.TaskForSimpleIcons()
	builder.TaskForGuideAssets()
	builder.TaskForEditorAssets()
	builder.TaskForStyles()
	builder.TaskForFavicon()
	builder.TaskForTemplates()
}
