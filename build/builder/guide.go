package builder

func TaskForGuideAssets(src string, dest string) {
	_PrepareDirectory(dest)
	_CopyDirectoryWithoutSymlink(src, dest)
}
