package FlareFn

import (
	"os"
	"path"
)

func GetWorkDir() string {
	workDir, _ := os.Getwd()
	return workDir
}

func GetWorkDirFile(filename string) string {
	return path.Join(GetWorkDir(), ".env")
}
