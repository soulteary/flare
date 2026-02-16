package data

import (
	"fmt"
	"os"
	"path/filepath"
)

func checkExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getConfigPath(config string) string {
	rootDir, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", config+".yml")
	}
	return filepath.Join(rootDir, config+".yml")
}

func saveFile(filePath string, data []byte) bool {
	err := os.WriteFile(filePath, data, os.ModePerm)
	return err == nil
}

// readFile reads the file and returns (nil, error) on failure. Callers should handle errors.
func readFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 %s 失败: %w", filePath, err)
	}
	return data, nil
}
