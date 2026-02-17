package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppConfig(t *testing.T) {

	filePath := getConfigPath("config")
	os.Remove(filePath)

	data, err := loadAppConfigFromYaml("config")
	if err != nil {
		t.Fatalf("Load App Config: %v", err)
	}
	if data.Title != "flare" {
		t.Fatal("Load App Config Failed")
	}
	ok := saveAppConfigToYamlFile("config", data)
	if !ok {
		t.Fatal("Save App Config Failed")
	}

	os.Remove(filePath)
}

// TestLoadAppConfigFromYaml_DefaultValues 验证无配置文件时加载得到默认值
func TestLoadAppConfigFromYaml_DefaultValues(t *testing.T) {
	filePath := getConfigPath("config_default_test")
	os.Remove(filePath)
	defer os.Remove(filePath)

	// 使用独立文件名避免影响其他测试；getConfigPath 基于当前目录
	// 先切到临时目录再加载名为 config 的配置，使 getConfigPath("config") 指向临时文件
	origWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origWd) }()

	// 无文件时 load 会创建默认配置
	data, err := loadAppConfigFromYaml("config")
	require.NoError(t, err)
	assert.Equal(t, "flare", data.Title, "默认 Title")
	assert.Equal(t, "blackboard", data.Theme, "默认 Theme")
	assert.Equal(t, "zh", data.Locale, "默认 Locale")
	assert.True(t, data.ShowWeather, "默认 ShowWeather")
}

// TestLoadAppConfigFromYaml_InvalidYAML 验证配置文件内容非法时返回解析错误
func TestLoadAppConfigFromYaml_InvalidYAML(t *testing.T) {
	origWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origWd) }()

	configPath := filepath.Join(tmpDir, "config.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("Title: [invalid\n  broken"), 0644))

	_, err = loadAppConfigFromYaml("config")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析", "应返回解析相关错误")
}
