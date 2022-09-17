package builder

func TaskForGuideAssets() {
	_PrepareDirectory("internal/guide/guide-assets")
	_CopyDirectory("embed/assets/vendor/guide-assets", "internal/guide/guide-assets")
}
