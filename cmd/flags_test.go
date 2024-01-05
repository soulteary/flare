package FlareCMD_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	env "github.com/caarlos0/env/v6"
	FlareCMD "github.com/soulteary/flare/cmd"
	FlareModel "github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/version"
	flags "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func getDefaultEnvVars() FlareModel.Envs {
	return FlareModel.Envs{
		Port:                   FlareCMD.DEFAULT_PORT,
		EnableGuide:            FlareCMD.DEFAULT_ENABLE_GUIDE,
		EnableDeprecatedNotice: FlareCMD.DEFAULT_ENABLE_DEPRECATED_NOTICE,
		EnableMinimumRequest:   FlareCMD.DEFAULT_ENABLE_MINI_REQUEST,
		DisableLoginMode:       FlareCMD.DEFAULT_DISABLE_LOGIN,
		EnableOfflineMode:      FlareCMD.DEFAULT_ENABLE_OFFLINE,
		EnableEditor:           FlareCMD.DEFAULT_ENABLE_EDITOR,
		Visibility:             FlareCMD.DEFAULT_VISIBILITY,
	}
}

func TestParseEnvVars(t *testing.T) {
	os.Setenv("FLARE_PORT", "5000")
	defer os.Unsetenv("FLARE_PORT")

	os.Setenv("FLARE_GUIDE", "false")
	defer os.Unsetenv("FLARE_GUIDE")

	os.Setenv("FLARE_OFFLINE", "true")
	defer os.Unsetenv("FLARE_OFFLINE")

	os.Setenv("FLARE_USER", "test")
	defer os.Unsetenv("FLARE_USER")

	os.Setenv("FLARE_VISIBILITY", "private")
	defer os.Unsetenv("FLARE_VISIBILITY")

	flags := FlareCMD.ParseEnvVars()

	if flags.Port != 5000 {
		t.Fatal("Check `FLARE_PORT` Faild")
	}

	if flags.EnableGuide != false {
		t.Fatal("Check `FLARE_GUIDE` Faild")
	}

	if flags.EnableOfflineMode != true {
		t.Fatal("Check `FLARE_OFFLINE` Faild")
	}

	if flags.Visibility != "private" {
		t.Fatal("Check `FLARE_VISIBILITY` Faild")
	}

	if flags.User != "test" {
		t.Fatal("Check `FLARE_USER` Faild")
	}

	// test error parse
	os.Setenv("FLARE_OFFLINE", ")))))))@#$%^&*()")
	defer os.Unsetenv("FLARE_OFFLINE")
	flags = FlareCMD.ParseEnvVars()
	defaultEnvs := getDefaultEnvVars()
	if flags.EnableOfflineMode != defaultEnvs.EnableOfflineMode {
		t.Fatal("Check `FLARE_OFFLINE` Faild")
	}
}

func TestInitAccountFromEnvVars(t *testing.T) {
	defaults := getDefaultEnvVars()

	if err := env.Parse(&defaults); err != nil {
		t.Fatal("TestInitAccountFromEnvVars Faild")
		return
	}

	var target FlareModel.Flags

	// 3. update username and password
	FlareCMD.InitAccountFromEnvVars(
		"custom",
		defaults.Pass,
		&target.User,
		&target.Pass,
		FlareCMD.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)

	if target.User != "custom" && target.UserIsGenerated != true && target.PassIsGenerated != true && len(target.Pass) != 8 {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}

	// 4. test empty username
	FlareCMD.InitAccountFromEnvVars(
		"",
		defaults.Pass,
		&target.User,
		&target.Pass,
		FlareCMD.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)

	if target.User != "flare" && target.UserIsGenerated != false && target.PassIsGenerated != true && len(target.Pass) != 8 {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}

	// 4. test empty password
	FlareCMD.InitAccountFromEnvVars(
		"",
		"custom",
		&target.User,
		&target.Pass,
		FlareCMD.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)

	if target.Pass != "custom" && target.PassIsGenerated != true {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}
}

func TestParseEnvFile(t *testing.T) {
	envParsed := FlareCMD.ParseEnvVars()
	// test .env not exist
	os.Remove(".env")
	flags := FlareCMD.ParseEnvFile(envParsed)
	defaultEnvs := getDefaultEnvVars()
	assert.Equal(t, flags.Port, defaultEnvs.Port)
	assert.Equal(t, flags.EnableGuide, defaultEnvs.EnableGuide)
	assert.Equal(t, flags.EnableDeprecatedNotice, defaultEnvs.EnableDeprecatedNotice)
	assert.Equal(t, flags.EnableMinimumRequest, defaultEnvs.EnableMinimumRequest)
	assert.Equal(t, flags.DisableLoginMode, defaultEnvs.DisableLoginMode)
	assert.Equal(t, flags.Visibility, defaultEnvs.Visibility)
	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
	assert.Equal(t, flags.EnableEditor, defaultEnvs.EnableEditor)

	// test unmarshal error
	// os.MkdirAll(".env", 0755)
	// defer os.Remove(".env")
	// flags := FlareCMD.ParseEnvFile(envParsed)
	// defaultEnvs := getDefaultEnvVars()
	// assert.Equal(t, flags.Port, defaultEnvs.Port)

	// test normal
	os.Remove(".env")
	f, _ := os.Create(".env")
	defer os.Remove(f.Name())
	f.Write([]byte("FLARE_PORT=5000\nFLARE_GUIDE=false\nFLARE_OFFLINE=true\nFLARE_VISIBILITY=private"))
	flags = FlareCMD.ParseEnvFile(envParsed)
	assert.Equal(t, flags.Port, 5000)
	assert.Equal(t, flags.EnableGuide, false)
	assert.Equal(t, flags.EnableOfflineMode, true)
	assert.Equal(t, flags.Visibility, "private")

	// test empty .env
	os.Remove(".env")
	f2, _ := os.Create(".env")
	defer os.Remove(f2.Name())
	f.Write([]byte(""))
	flags = FlareCMD.ParseEnvFile(envParsed)
	assert.Equal(t, flags.Port, defaultEnvs.Port)
	assert.Equal(t, flags.EnableGuide, defaultEnvs.EnableGuide)
	assert.Equal(t, flags.EnableDeprecatedNotice, defaultEnvs.EnableDeprecatedNotice)
	assert.Equal(t, flags.EnableMinimumRequest, defaultEnvs.EnableMinimumRequest)
	assert.Equal(t, flags.DisableLoginMode, defaultEnvs.DisableLoginMode)
	assert.Equal(t, flags.Visibility, defaultEnvs.Visibility)
	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
	assert.Equal(t, flags.EnableEditor, defaultEnvs.EnableEditor)
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	f()
	w.Close()
	os.Stdout = old
	return <-outC
}

func TestExcuteCLI_ShowHelp(t *testing.T) {
	cliFlags := &FlareModel.Flags{ShowHelp: true}
	options := &flags.FlagSet{}

	output := captureOutput(func() {
		_ = FlareCMD.ExcuteCLI(cliFlags, options)
	})

	assert.Contains(t, output, "支持命令：", "应该打印出支持命令")
	assert.True(t, FlareCMD.ExcuteCLI(cliFlags, options), "在 ShowHelp 为 true 时，应该返回 true")
}

func TestExcuteCLI_ShowVersion(t *testing.T) {
	cliFlags := &FlareModel.Flags{ShowVersion: true}
	options := &flags.FlagSet{}

	output := captureOutput(func() {
		_ = FlareCMD.ExcuteCLI(cliFlags, options)
	})

	assert.Contains(t, output, version.Version, "应该打印出版本信息")
	assert.True(t, FlareCMD.ExcuteCLI(cliFlags, options), "在 ShowVersion 为 true 时，应该返回 true")
}

func TestExcuteCLI_NoFlags(t *testing.T) {
	cliFlags := &FlareModel.Flags{}
	options := &flags.FlagSet{}

	assert.False(t, FlareCMD.ExcuteCLI(cliFlags, options), "当没有任何标志被设置时，应该返回 false")
}

func TestGetVersionEcho(t *testing.T) {
	ver := ""
	// output := captureOutput(func() {
	ver = FlareCMD.GetVersion(true)
	// })
	assert.Contains(t, ver, version.Version, "应该打印出版本信息")
	// assert.Contains(t, output, "Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.", "应该打印详细信息")
}

func TestGetVersionMute(t *testing.T) {
	ver := ""
	output := captureOutput(func() {
		ver = FlareCMD.GetVersion(false)
	})
	assert.Contains(t, ver, version.Version, "应该打印出版本信息")
	assert.NotContains(t, output, "Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.", "不应该打印详细信息")
}
