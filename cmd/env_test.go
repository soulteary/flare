package FlareCMD_test

import (
	"os"
	"testing"

	env "github.com/caarlos0/env/v6"
	FlareCMD "github.com/soulteary/flare/cmd"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
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
	defaultEnvs := FlareDefine.GetDefaultEnvVars()
	if flags.EnableOfflineMode != defaultEnvs.EnableOfflineMode {
		t.Fatal("Check `FLARE_OFFLINE` Faild")
	}
}

func TestInitAccountFromEnvVars(t *testing.T) {
	defaultEnvs := FlareDefine.GetDefaultEnvVars()

	if err := env.Parse(&defaultEnvs); err != nil {
		t.Fatal("TestInitAccountFromEnvVars Faild")
		return
	}

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

	if target.User != "custom" && target.UserIsGenerated != true && target.PassIsGenerated != true && len(target.Pass) != 8 {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}

	// 4. test empty username
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

	if target.User != "flare" && target.UserIsGenerated != false && target.PassIsGenerated != true && len(target.Pass) != 8 {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}

	// 4. test empty password
	FlareCMD.InitAccountFromEnvVars(
		"",
		"custom",
		&target.User,
		&target.Pass,
		FlareDefine.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)

	if target.Pass != "custom" && target.PassIsGenerated != true {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}
}
