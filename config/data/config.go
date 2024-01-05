package FlareData

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v2"

	FlareModel "github.com/soulteary/flare/config/model"
)

func initAppConfig(filePath string) (result FlareModel.Application, err error) {

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
`)

	ok := saveFile(filePath, out)
	if !ok {
		log.Println("初始化默认程序配置文件出错。")
		return result, err
	}

	parseErr := yaml.Unmarshal(out, &result)
	if parseErr != nil {
		log.Fatalf("初始化程序配置失败。")
	}

	return result, nil
}

func saveAppConfigToYamlFile(name string, result FlareModel.Application) bool {
	out, err := yaml.Marshal(result)
	if err != nil {
		log.Println("转换程序配置的数据格式失败")
		return false
	}

	filePath := getConfigPath(name)
	ok := saveFile(filePath, out)
	if !ok {
		log.Println("保存程序配置文件失败")
		return false
	}

	return true
}

func loadAppConfigFromYaml(name string) (result FlareModel.Application) {
	filePath := getConfigPath(name)

	if checkExists(filePath) {
		configFile := readFile(filePath, true)
		parseErr := yaml.Unmarshal(configFile, &result)
		if parseErr != nil {
			log.Fatalf("解析配置文件" + name + "错误，请检查配置文件内容。")
		}
	} else {
		fmt.Println("找不到配置文件" + name + "，创建默认配置。")
		var createErr error
		result, createErr = initAppConfig(filePath)
		if createErr != nil {
			log.Fatalf("尝试创建应用配置文件" + name + "失败")
		}
	}

	return result
}
