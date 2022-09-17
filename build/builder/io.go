package builder

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// https://stackoverflow.com/questions/51779243/copy-a-folder-in-go

func _CopyDirectoryWithoutSymlink(scrDir, dest string) error {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := _CreateIfNotExists(destPath, 0755); err != nil {
				return err
			}
			if err := _CopyDirectoryWithoutSymlink(sourcePath, destPath); err != nil {
				return err
			}
		case os.ModeSymlink:
			return fmt.Errorf("no need to copy symlink'%s'", sourcePath)
		default:
			if err := _Copy(sourcePath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func _Copy(srcFile, dstFile string) error {
	out, err := os.Create(filepath.Clean(dstFile))
	if err != nil {
		return err
	}

	defer func() {
		if err := out.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	in, err := os.Open(filepath.Clean(srcFile))
	if err != nil {
		return err
	}

	defer func() {
		if err := in.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func _Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func _CreateIfNotExists(dir string, perm os.FileMode) error {
	if _Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

// https://stackoverflow.com/questions/55300117/how-do-i-find-all-files-that-have-a-certain-extension-in-go-regardless-of-depth
func _WalkMatch(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func _PrepareDirectory(dir string) {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		checkErr(err)
	}
}
