package builder

func TaskForEditorAssets() {
	_PrepareDirectory("internal/editor/editor-assets")
	_CopyDirectory("embed/assets/vendor/editor-assets", "internal/editor/editor-assets")
}
