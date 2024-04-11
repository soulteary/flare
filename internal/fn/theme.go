package FlareFn

import (
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v2"
)

// TODO 添加错误日志输出

// 获取主题存放目录
func GetThemeDir() string {
	return path.Join(GetWorkDir(), "themes")
}

// 获取主题预览图
func GetThemePreview(themeName string) string {
	baseDir := GetThemeDir()
	themeDir := path.Join(baseDir, CustomThemeNameTransform(themeName))

	if !IsCustomThemeExist(themeName) {
		return ""
	}

	files, err := os.ReadDir(themeDir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if file.Name() == "preview.png" || file.Name() == "preview.jpg" || file.Name() == "preview.webp" {
			return path.Join(themeDir, file.Name())
		}
	}
	return ""
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
	Name        string                    `yaml:"Name"`
	Description string                    `yaml:"Description"`
	Version     string                    `yaml:"Version"`
	Author      []FlareCustomThemeAuthor  `yaml:"Author"`
	Original    FlareCustomThemeExtraInfo `yaml:"Original,omitempty"`

	Preview    string `yaml:"-"`
	PreviewURL string `yaml:"-"`
	Dir        string `yaml:"-"`
}

type FlareCustomThemeAuthor struct {
	Name string `yaml:"Name"`
	URL  string `yaml:"URL"`
}

type FlareCustomThemeExtraInfo struct {
	Author   string `yaml:"Author"`
	URL      string `yaml:"URL"`
	Download string `yaml:"Download"`
}

// 获取所有自定义主题信息
func GetAllCustomThemes() (result []FlareCustomTheme) {
	themeDir := GetThemeDir()

	themeDirFiles, err := os.ReadDir(themeDir)
	if err != nil {
		return result
	}

	var themes []FlareCustomTheme
	for _, themeDirFile := range themeDirFiles {
		if !themeDirFile.IsDir() {
			continue
		}
		themes = append(themes, FlareCustomTheme{
			Name: themeDirFile.Name(),
			Dir:  path.Join(themeDir, themeDirFile.Name()),
		})
	}

	for _, theme := range themes {
		yamlFile, err := os.ReadFile(path.Join(theme.Dir, "theme.yaml"))
		if err != nil {
			continue
		}
		var themeInfo FlareCustomTheme
		err = yaml.Unmarshal(yamlFile, &themeInfo)
		if err != nil {
			continue
		}
		themeInfo.Dir = theme.Dir
		themeInfo.Preview = GetThemePreview(theme.Name)
		themeInfo.PreviewURL = strings.ReplaceAll(themeInfo.Preview, GetWorkDir(), "")

		result = append(result, themeInfo)
	}
	return result
}
