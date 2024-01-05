package FlareCMD_test

import (
	"os"
	"path"
	"testing"

	FlareCMD "github.com/soulteary/flare/cmd"
	FlareDefine "github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/assert"
)

func TestParseEnvFile_NotExist(t *testing.T) {
	os.Setenv("FLARE_DEBUG", "true")
	defer os.Unsetenv("FLARE_DEBUG")

	envParsed := FlareCMD.ParseEnvVars()

	workDir, _ := os.Getwd()
	envPath := path.Join(workDir, ".env")

	// test .env not exist
	os.Remove(envPath)
	flags := FlareCMD.ParseEnvFile(envParsed)
	defaultEnvs := FlareDefine.GetDefaultEnvVars()

	assert.Equal(t, flags.Port, defaultEnvs.Port)
	assert.Equal(t, flags.EnableGuide, defaultEnvs.EnableGuide)
	assert.Equal(t, flags.EnableDeprecatedNotice, defaultEnvs.EnableDeprecatedNotice)
	assert.Equal(t, flags.EnableMinimumRequest, defaultEnvs.EnableMinimumRequest)
	assert.Equal(t, flags.DisableLoginMode, defaultEnvs.DisableLoginMode)
	assert.Equal(t, flags.Visibility, defaultEnvs.Visibility)
	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
	assert.Equal(t, flags.EnableEditor, defaultEnvs.EnableEditor)
}

// func TestParseEnvFile_Normal(t *testing.T) {
// 	os.Setenv("FLARE_DEBUG", "true")
// 	defer os.Unsetenv("FLARE_DEBUG")

// 	envParsed := FlareCMD.ParseEnvVars()

// 	workDir, _ := os.Getwd()
// 	envPath := path.Join(workDir, ".env")

// 	// test normal
// 	os.Remove(envPath)
// 	f, _ := os.Create(envPath)
// 	defer os.Remove(f.Name())
// 	f.Write([]byte("FLARE_PORT=5000\nFLARE_GUIDE=false\nFLARE_OFFLINE=true\nFLARE_VISIBILITY=private"))
// 	flags := FlareCMD.ParseEnvFile(envParsed)
// 	assert.Equal(t, flags.Port, 5000)
// 	assert.Equal(t, flags.EnableGuide, false)
// 	assert.Equal(t, flags.EnableOfflineMode, true)
// 	assert.Equal(t, flags.Visibility, "private")
// }

// func TestParseEnvFile_EmptyConfig(t *testing.T) {
// 	os.Setenv("FLARE_DEBUG", "true")
// 	defer os.Unsetenv("FLARE_DEBUG")

// 	envParsed := FlareCMD.ParseEnvVars()
// 	defaultEnvs := FlareDefine.GetDefaultEnvVars()

// 	workDir, _ := os.Getwd()
// 	envPath := path.Join(workDir, ".env")

// 	// test empty .env
// 	os.Remove(envPath)
// 	f, _ := os.Create(envPath)
// 	defer os.Remove(f.Name())
// 	f.Write([]byte(""))
// 	flags := FlareCMD.ParseEnvFile(envParsed)
// 	assert.Equal(t, flags.Port, defaultEnvs.Port)
// 	assert.Equal(t, flags.EnableGuide, defaultEnvs.EnableGuide)
// 	assert.Equal(t, flags.EnableDeprecatedNotice, defaultEnvs.EnableDeprecatedNotice)
// 	assert.Equal(t, flags.EnableMinimumRequest, defaultEnvs.EnableMinimumRequest)
// 	assert.Equal(t, flags.DisableLoginMode, defaultEnvs.DisableLoginMode)
// 	assert.Equal(t, flags.Visibility, defaultEnvs.Visibility)
// 	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
// 	assert.Equal(t, flags.EnableEditor, defaultEnvs.EnableEditor)
// }

// func TestParseEnvFile(t *testing.T) {
// 	// test unmarshal error
// 	// os.MkdirAll(".env", 0755)
// 	// defer os.Remove(".env")
// 	// flags := FlareCMD.ParseEnvFile(envParsed)
// 	// defaultEnvs := FlareDefine.GetDefaultEnvVars()
// 	// assert.Equal(t, flags.Port, defaultEnvs.Port)
// }
