package builder

import "log"

func TaskForEditorAssets(src string, dest string) {
	_PrepareDirectory(dest)
	if err := _CopyDirectoryWithoutSymlink(src, dest); err != nil {
		log.Fatal(err)
	}
}
