package builder

func TaskForGuideAssets() {
	_PrepareDirectory("pkg/guide/guide-assets")
	_CopyDirectory("embed/assets/vendor/guide-assets", "pkg/guide/guide-assets")
}
