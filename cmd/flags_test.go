package cmd

import (
	"fmt"
	"os"
	"testing"

	env "github.com/caarlos0/env/v6"
	FlareModel "github.com/soulteary/flare/model"
)

func TestParseEnvVars(t *testing.T) {
	os.Setenv("FLARE_PORT", "5000")
	os.Setenv("FLARE_GUIDE", "false")
	os.Setenv("FLARE_OFFLINE", "true")
	os.Setenv("FLARE_USER", "test")
	os.Setenv("FLARE_VISIBILITY", "private")

	flags := parseEnvVars()

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
}

func TestInitAccountFromEnvVars(t *testing.T) {

	defaults := FlareModel.Envs{
		Port:                   _DEFAULT_PORT,
		EnableGuide:            _DEFAULT_ENABLE_GUIDE,
		EnableDeprecatedNotice: _DEFAULT_ENABLE_DEPRECATED_NOTICE,
		EnableMinimumRequest:   _DEFAULT_ENABLE_MINI_REQUEST,
		DisableLoginMode:       _DEFAULT_DISABLE_LOGIN,
		EnableOfflineMode:      _DEFAULT_ENABLE_OFFLINE,
		EnableEditor:           _DEFAULT_ENABLE_EDITOR,
		Visibility:             _DEFAULT_VISIBILITY,
	}

	if err := env.Parse(&defaults); err != nil {
		t.Fatal("TestInitAccountFromEnvVars Faild")
		return
	}

	var target FlareModel.Flags

	// 3. update username and password
	initAccountFromEnvVars(
		defaults.User,
		defaults.Pass,
		&target.User,
		&target.Pass,
		_DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)

	fmt.Println(target)
	if target.User != "flare" && target.UserIsGenerated != true && target.PassIsGenerated != true && len(target.Pass) != 8 {
		t.Fatal("TestInitAccountFromEnvVars Faild")
	}
}
