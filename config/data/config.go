package data

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/soulteary/flare/config/model"
)

func initAppConfig(filePath string) (result model.Application, err error) {

	out := []byte(`
# 应用标题
Title: "flare"
# 应用页脚
Footer: 由 <a href="https://github.com/soulteary/docker-flare">Flare</a> ❤️ 强力驱动
# 在新窗口中打开应用
OpenAppNewTab: true
# 在新窗口中打开书签
OpenBookmarkNewTab: true
# 展示顶部标题组件
ShowTitle: true
# 默认的首页问候语
Greetings: 你好
# 展示搜索组件
ShowSearchComponent: true
# 禁用搜索框自动获取焦点
DisabledSearchAutoFocus: false
# 展示日期组件
ShowDateTime: true
# 展示应用组件
ShowApps: true
# 展示书签组件
ShowBookmarks: true
# 隐藏界面中的设置按钮
HideSettingButton: false
# 隐藏界面中的帮助按钮
HideHelpButton: false
# 加密展示链接
EnableEncryptedLink: false
# 链接图标展示模式
IconMode: "DEFAULT"
# 应用主体
Theme: "blackboard"
# 是否启用天气组件
ShowWeather: true
# 天气组件使用的位置，仅在程序自动识别出错时，需要修改
Location: "北京市"
# 保持界面中链接大小写和配置中一致
KeepLetterCase: false
# 界面语言 (zh / en)
Locale: "zh"
`)

	ok := saveFile(filePath, out)
	if !ok {
		log.Println("初始化默认程序配置文件出错。")
		return result, fmt.Errorf("初始化默认配置文件失败: %s", filePath)
	}

	parseErr := yaml.Unmarshal(out, &result)
	if parseErr != nil {
		return result, fmt.Errorf("初始化程序配置失败: %w", parseErr)
	}

	return result, nil
}

func saveAppConfigToYamlFile(name string, result model.Application) bool {
	out, err := yaml.Marshal(result)
	if err != nil {
		log.Println("转换程序配置的数据格式失败")
		return false
	}

	filePath := getConfigPath(name)
	ok := saveConfigAtomically(filePath, out)
	if !ok {
		log.Println("保存程序配置文件失败")
		return false
	}
	invalidateFileCache(name)
	return true
}

func loadAppConfigFromYaml(name string) (model.Application, error) {
	var result model.Application
	filePath := getConfigPath(name)

	if !checkExists(filePath) {
		fmt.Println("找不到配置文件" + name + "，创建默认配置。")
		invalidateFileCache(name)
		var createErr error
		result, createErr = initAppConfig(filePath)
		if createErr != nil {
			return result, fmt.Errorf("尝试创建应用配置文件 %s 失败: %w", name, createErr)
		}
		return result, nil
	}
	configFile, err := readFileCached(name, func() ([]byte, error) { return readFile(filePath) })
	if err != nil {
		return result, fmt.Errorf("读取配置文件 %s: %w", name, err)
	}
	result, parseErr := parsedApp(configFile)
	if parseErr != nil {
		return result, fmt.Errorf("解析配置文件 %s 错误，请检查配置文件内容: %w", name, parseErr)
	}
	return result, nil
}

// saveConfigAtomically prevents an interrupted settings save from truncating config.yml.
func saveConfigAtomically(path string, contents []byte) bool {
	f, err := os.CreateTemp(filepath.Dir(path), ".flare-config-*")
	if err != nil {
		return false
	}
	name := f.Name()
	defer func() {
		if removeErr := os.Remove(name); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Printf("clean up temporary config: %v", removeErr)
		}
	}()
	mode := os.FileMode(0600)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(contents)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return false
	}
	return os.Rename(name, path) == nil
}
