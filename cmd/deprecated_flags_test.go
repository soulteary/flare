package cmd

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/define"
)

func TestParseDeprecatedBooleanAliasesOverrideEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name                string
		args                []string
		envLogin, envMini   string
		wantLogin, wantMini bool
	}{
		{"bare legacy login", []string{"--nologin"}, "false", "false", true, false},
		{"bare legacy minimum", []string{"--mr"}, "false", "false", false, true},
		{"legacy login false", []string{"--nologin=false"}, "true", "false", false, false},
		{"legacy login zero", []string{"--nologin=0"}, "true", "false", false, false},
		{"legacy login true", []string{"--nologin=true"}, "false", "false", true, false},
		{"legacy minimum request", []string{"--mr=true"}, "false", "false", false, true},
		{"legacy minimum zero", []string{"--mr=0"}, "false", "true", false, false},
		{"canonical then legacy login", []string{"--disable_login=true", "--nologin=false"}, "true", "false", false, false},
		{"legacy then canonical login", []string{"--nologin=false", "--disable_login=true"}, "false", "false", true, false},
		{"short then legacy login", []string{"-l=true", "--nologin=false"}, "true", "false", false, false},
		{"legacy then short login", []string{"--nologin=false", "-l=true"}, "false", "false", true, false},
		{"canonical then legacy minimum", []string{"--mini_request=false", "--mr=true"}, "true", "false", true, true},
		{"legacy then canonical minimum", []string{"--mr=true", "--mini_request=false"}, "true", "true", true, false},
		{"short then legacy minimum", []string{"-m=false", "--mr=true"}, "true", "false", true, true},
		{"legacy then short minimum", []string{"--mr=true", "-m=false"}, "true", "true", true, false},
		{"repeated legacy login", []string{"--nologin=true", "--nologin=false"}, "true", "false", false, false},
		{"repeated legacy minimum", []string{"--mr=false", "--mr=true"}, "true", "false", true, true},
		{"both legacy aliases", []string{"--nologin=false", "--mr=true"}, "true", "false", false, true},
		{"no flags retains env", nil, "false", "true", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FLARE_DISABLE_LOGIN", tc.envLogin)
			t.Setenv("FLARE_MINI_REQUEST", tc.envMini)
			t.Setenv("FLARE_VISIBILITY", "DEFAULT")
			t.Setenv("FLARE_USER", "test-user")
			t.Setenv("FLARE_PASS", "test-password")
			originalArgs, originalFlags := os.Args, define.AppFlags
			defer func() { os.Args = originalArgs; define.AppFlags = originalFlags }()
			os.Args = append([]string{"flare"}, tc.args...)
			got := Parse()
			if got.DisableLoginMode != tc.wantLogin || got.EnableMinimumRequest != tc.wantMini {
				t.Fatalf("args=%v loginDisabled=%v want=%v minimum=%v want=%v", tc.args, got.DisableLoginMode, tc.wantLogin, got.EnableMinimumRequest, tc.wantMini)
			}
		})
	}
}

func TestParseDeprecatedAliasesOverrideApplicationDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	t.Setenv("FLARE_MINI_REQUEST", "true")
	t.Setenv("FLARE_VISIBILITY", "DEFAULT")
	t.Setenv("FLARE_USER", "test-user")
	t.Setenv("FLARE_PASS", "test-password")
	if err := os.WriteFile(".env", []byte("FLARE_DISABLE_LOGIN=true\nFLARE_MINI_REQUEST=false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	beforeArgs, beforeFlags := os.Args, define.AppFlags
	defer func() { os.Args = beforeArgs; define.AppFlags = beforeFlags }()
	os.Args = []string{"flare", "--nologin=false", "--mr=true"}
	got := Parse()
	if got.DisableLoginMode || !got.EnableMinimumRequest {
		t.Fatalf("aliases did not override .env: loginDisabled=%v minimum=%v", got.DisableLoginMode, got.EnableMinimumRequest)
	}
}
