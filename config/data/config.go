package data

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"

	yaml3 "go.yaml.in/yaml/v3"
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

	out = append(out, []byte(fmt.Sprintf("\n# Application version that last saved this configuration\nAppVersion: %q\n", applicationVersion()))...)
	f, createErr := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if createErr != nil {
		return result, createErr
	}
	_, writeErr := f.Write(out)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return result, fmt.Errorf("initial configuration could not be written")
	}

	parseErr := yaml.Unmarshal(out, &result)
	if parseErr != nil {
		return result, fmt.Errorf("初始化程序配置失败: %w", parseErr)
	}

	result.ConfigRevision = backupDigest(out)
	return result, nil
}

func saveAppConfigToYamlFile(name string, result model.Application) bool {
	path := getConfigPath(name)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// Creation is exclusive; never overwrite a file that appeared concurrently.
		result.AppVersion = applicationVersion()
		out, e := yaml.Marshal(result)
		if e != nil {
			return false
		}
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return false
		}
		_, e = f.Write(out)
		if e == nil {
			e = f.Sync()
		}
		closed := f.Close()
		invalidateFileCache(name)
		return e == nil && closed == nil
	}
	if err != nil {
		return false
	}
	doc, original, err := readConfigDocument(raw)
	if err != nil {
		return false
	}
	// Runtime-only state and version-only differences do not create writes.
	compare := result
	compare.AppVersion = original.AppVersion
	compare.ConfigVersionState = ""
	compare.ConfigRevision = ""
	original.ConfigVersionState = ""
	if result.ConfigRevision != "" && result.ConfigRevision != backupDigest(raw) {
		return false
	}
	if reflect.DeepEqual(original, compare) {
		return true
	}
	writer, err := versionForSave(original)
	if err != nil {
		log.Println(err)
		return false
	}
	if err = rejectSharedConfigNodes(doc); err != nil {
		log.Println(err)
		return false
	}
	result.AppVersion = writer
	var patch yaml3.Node
	if err = patch.Encode(result); err != nil {
		return false
	}
	if err = rejectAmbiguousEngineReplacement(doc.Content[0], &patch); err != nil {
		return false
	}
	mergeSearchNodes(doc.Content[0], &patch)
	out, err := yaml3.Marshal(doc)
	if err != nil {
		return false
	}
	if _, _, err = readConfigDocument(out); err != nil {
		return false
	}
	state := ConfigurationVersionState(original.AppVersion)
	if err = commitConfigDocument(path, raw, out, state == "upgrade" || state == "legacy"); err != nil {
		log.Println(err)
		return false
	}
	invalidateFileCache(name)
	return true
}

func loadAppConfigFromYaml(name string) (model.Application, error) {
	var result model.Application
	filePath := getConfigPath(name)

	if _, statErr := os.Lstat(filePath); os.IsNotExist(statErr) {
		fmt.Println("找不到配置文件" + name + "，创建默认配置。")
		var createErr error
		result, createErr = initAppConfig(filePath)
		if createErr != nil {
			return result, fmt.Errorf("尝试创建应用配置文件 %s 失败: %w", name, createErr)
		}
		return result, nil
	}
	// Configuration/version checks must observe external edits, especially downgrades.
	configFile, err := readFile(filePath)
	if err != nil {
		return result, fmt.Errorf("读取配置文件 %s: %w", name, err)
	}
	_, result, parseErr := readConfigDocument(configFile)
	if parseErr != nil {
		return result, fmt.Errorf("解析配置文件 %s 错误，请检查配置文件内容: %w", name, parseErr)
	}
	result.ConfigRevision = backupDigest(configFile)
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
	if err = os.Rename(name, path); err != nil {
		return false
	}
	// Sync the rename as well as the contents; backup and manifest ordering relies
	// on directory entries surviving a process/machine interruption.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return false
	}
	syncErr := dir.Sync()
	closeErr = dir.Close()
	return syncErr == nil && closeErr == nil
}
