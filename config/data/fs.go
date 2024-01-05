package FlareData

import (
	"log"
	"os"
	"path/filepath"
)

func checkExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return true
}

func getConfigPath(config string) string {
	rootDir, _ := os.Getwd()
	return filepath.Join(rootDir, config+".yml")
}

func saveFile(filePath string, data []byte) bool {
	err := os.WriteFile(filePath, data, os.ModePerm)
	return err == nil
}

func readFile(filePath string, crashOnError bool) []byte {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		if crashOnError {
			log.Fatalf("程序不能读取配置文件" + filePath + "，请检查文件权限是否正常")
		}
		return []byte("")
	}
	return data
}
