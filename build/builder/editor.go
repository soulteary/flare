package builder

func TaskForEditorAssets() {
	_PrepareDirectory("pkg/editor/editor-assets")
	_CopyDirectory("embed/assets/vendor/editor-assets", "pkg/editor/editor-assets")
}
