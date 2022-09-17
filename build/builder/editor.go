package builder

func TaskForEditorAssets(src string, dest string) {
	_PrepareDirectory(dest)
	_CopyDirectory(src, dest)
}
