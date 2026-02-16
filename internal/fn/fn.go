package fn

import (
	"os"
	"path"
)

// GetWorkDir returns the current working directory, or empty string on error.
// Deprecated: use GetWorkDirE to handle errors.
func GetWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// GetWorkDirE returns the current working directory or an error.
func GetWorkDirE() (string, error) {
	return os.Getwd()
}

// GetWorkDirFile returns the path joining current working directory and filename.
func GetWorkDirFile(filename string) string {
	dir := GetWorkDir()
	return path.Join(dir, filename)
}
