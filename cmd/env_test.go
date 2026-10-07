package cmd_test

import (
	"os"
	"testing"

	env "github.com/caarlos0/env/v6"
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
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

	flags := cmd.ParseEnvVars()

	assert.Equal(t, flags.Port, 5000)
	assert.Equal(t, flags.EnableGuide, false)
	assert.Equal(t, flags.EnableOfflineMode, true)
	assert.Equal(t, flags.Visibility, "private")
	assert.Equal(t, flags.User, "test")

	// test error parse
	os.Setenv("FLARE_OFFLINE", ")))))))@#$%^&*()")
	defer os.Unsetenv("FLARE_OFFLINE")
	flags = cmd.ParseEnvVars()
	defaultEnvs := define.DefaultEnvVars
	assert.Equal(t, flags.EnableOfflineMode, defaultEnvs.EnableOfflineMode)
}

// TestParseEnvVarsCookies verifies that session configuration reaches the flags.
func TestParseEnvVarsCookies(t *testing.T) {
	tests := []struct {
		name       string
		envs       map[string]string
		wantName   string
		wantSecret string
	}{
		{
			name:       "defaults",
			wantName:   define.DEFAULT_COOKIE_NAME,
			wantSecret: define.DEFAULT_COOKIE_SECRET,
		},
		{
			name: "environment overrides",
			envs: map[string]string{
				"FLARE_COOKIE_NAME":   "custom-session",
				"FLARE_COOKIE_SECRET": "test-session-secret-32-bytes-long",
			},
			wantName:   "custom-session",
			wantSecret: "test-session-secret-32-bytes-long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"FLARE_COOKIE_NAME", "FLARE_COOKIE_SECRET"} {
				t.Setenv(key, "")
				assert.NoError(t, os.Unsetenv(key))
			}
			for key, value := range tt.envs {
				t.Setenv(key, value)
			}

			flags := cmd.ParseEnvVars()
			assert.Equal(t, tt.wantName, flags.CookieName)
			assert.Equal(t, tt.wantSecret, flags.CookieSecret)
		})
	}
}

func TestInitAccountFromEnvVars_normal(t *testing.T) {
	defaultEnvs := define.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target model.Flags

	// 3. update username and password
	cmd.InitAccountFromEnvVars(
		"custom",
		defaultEnvs.Pass,
		&target.User,
		&target.Pass,
		define.DEFAULT_USER_NAME,
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
	defaultEnvs := define.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target model.Flags

	// 4. test empty username and password
	cmd.InitAccountFromEnvVars(
		"",
		defaultEnvs.Pass,
		&target.User,
		&target.Pass,
		define.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, define.DEFAULT_USER_NAME)
	assert.Equal(t, target.UserIsGenerated, true)
	assert.Equal(t, target.PassIsGenerated, true)
	assert.Equal(t, len(target.Pass), 8)
}

func TestInitAccountFromEnvVars_EmptyPass(t *testing.T) {
	defaultEnvs := define.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")

	var target model.Flags

	// 4. test empty password
	cmd.InitAccountFromEnvVars(
		"custom",
		"",
		&target.User,
		&target.Pass,
		define.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, "custom")
	assert.Equal(t, len(target.Pass), 8)
	assert.Equal(t, target.PassIsGenerated, true)
}

func TestInitAccountFromEnvVars_Pass(t *testing.T) {
	defaultEnvs := define.DefaultEnvVars

	err := env.Parse(&defaultEnvs)
	assert.Nil(t, err, "TestInitAccountFromEnvVars Faild")
	var target model.Flags

	// 4. test empty password
	cmd.InitAccountFromEnvVars(
		"custom",
		"custom",
		&target.User,
		&target.Pass,
		define.DEFAULT_USER_NAME,
		&target.UserIsGenerated,
		&target.PassIsGenerated,
		&target.DisableLoginMode,
	)
	assert.Equal(t, target.User, "custom")
	assert.Equal(t, target.Pass, "custom")
	assert.Equal(t, target.PassIsGenerated, false)
	assert.Equal(t, target.UserIsGenerated, false)
}
