package FlareCMD_test

import (
	"os"
	"testing"

	env "github.com/caarlos0/env/v6"
	FlareCMD "github.com/soulteary/flare/cmd"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
)

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

	assert.Equal(t, flags.Port, 5000)
	assert.Equal(t, flags.EnableGuide, false)
	assert.Equal(t, flags.EnableOfflineMode, true)
	assert.Equal(t, flags.Visibility, "private")
	assert.Equal(t, flags.User, "test")

	// test error parse
	os.Setenv("FLARE_OFFLINE", ")))))))@#$%^&*()")
	defer os.Unsetenv("FLARE_OFFLINE")
	flags = FlareCMD.ParseEnvVars()
	defaultEnvs := FlareDefine.DefaultEnvVars
	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
}

func TestInitAccountFromEnvVars_normal(t *testing.T) {
	defaultEnvs := FlareDefine.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target FlareModel.Flags

	// 3. update username and password
	FlareCMD.InitAccountFromEnvVars(
		"custom",
		defaultEnvs.Pass,
		&target.User,
		&target.Pass,
		FlareDefine.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, "custom")
	assert.Equal(t, target.UserIsGenerated, false)
	assert.Equal(t, target.PassIsGenerated, true)
	assert.Equal(t, len(target.Pass), 8)
}

func TestInitAccountFromEnvVars_EmptyUser(t *testing.T) {
	defaultEnvs := FlareDefine.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target FlareModel.Flags

	// 4. test empty username and password
	FlareCMD.InitAccountFromEnvVars(
		"",
		defaultEnvs.Pass,
		&target.User,
		&target.Pass,
		FlareDefine.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, FlareDefine.DEFAULT_USER_NAME)
	assert.Equal(t, target.UserIsGenerated, true)
	assert.Equal(t, target.PassIsGenerated, true)
	assert.Equal(t, len(target.Pass), 8)
}

func TestInitAccountFromEnvVars_EmptyPass(t *testing.T) {
	defaultEnvs := FlareDefine.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")

	var target FlareModel.Flags

	// 4. test empty password
	FlareCMD.InitAccountFromEnvVars(
		"custom",
		"",
		&target.User,
		&target.Pass,
		FlareDefine.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, "custom")
	assert.Equal(t, len(target.Pass), 8)
	assert.Equal(t, target.PassIsGenerated, true)
}

func TestInitAccountFromEnvVars_Pass(t *testing.T) {
	defaultEnvs := FlareDefine.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target FlareModel.Flags

	// 4. test empty password
	FlareCMD.InitAccountFromEnvVars(
		"custom",
		"custom",
		&target.User,
		&target.Pass,
		FlareDefine.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, "custom")
	assert.Equal(t, target.Pass, "custom")
	assert.Equal(t, target.PassIsGenerated, false)
	assert.Equal(t, target.UserIsGenerated, false)
}
