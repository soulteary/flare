package FlareFn

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// 获取主题存放目录
func GetThemeDir() string {
	return path.Join(GetWorkDir(), "themes")
}

// 自定义主题名称转换
func CustomThemeNameTransform(themeName string) string {
	if themeName == "" {
		return ""
	}
	// TODO 确认规则，使用正则来通杀
	themeName = strings.ReplaceAll(themeName, " ", "")
	themeName = strings.ReplaceAll(themeName, "-", "")
	themeName = strings.ReplaceAll(themeName, "_", "")
	themeName = strings.ToLower(themeName)
	return themeName
}

// 判断自定义主题是否存在
func IsCustomThemeExist(themeName string) bool {
	themeDir := GetThemeDir()
	_, err := os.Stat(path.Join(themeDir, CustomThemeNameTransform(themeName)))
	return err == nil
}

// TODO 完善结构（作者等），移动到 model 中
type FlareCustomTheme struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Dir         string `json:"dir"`
}

// 获取所有自定义主题信息
func GetAllCustomThemes() []FlareCustomTheme {
	var themes []FlareCustomTheme
	themeDir := GetThemeDir()

	themeDirFiles, err := os.ReadDir(themeDir)
	if err != nil {
		return themes
	}

	for _, themeDirFile := range themeDirFiles {
		if !themeDirFile.IsDir() {
			continue
		}
		themes = append(themes, FlareCustomTheme{
			Name: themeDirFile.Name(),
			Dir:  path.Join(themeDir, themeDirFile.Name()),
		})
	}

	// TODO 读取主题信息
	for i, theme := range themes {
		fmt.Println(theme)
		// TODO 从文件中读取主题信息
		themes[i].Description = "TODO"
		// TODO 判断主题完整性
	}

	return themes
}
