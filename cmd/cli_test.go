package cmd_test

import (
	"flag"
	"os"
	"testing"

	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseCookiePrecedence covers the complete cookie configuration pipeline.
func TestParseCookiePrecedence(t *testing.T) {
	t.Chdir(t.TempDir())
	originalArgs := os.Args
	originalFlags := define.AppFlags
	t.Cleanup(func() {
		os.Args = originalArgs
		define.AppFlags = originalFlags
	})

	tests := []struct {
		name       string
		envs       map[string]string
		envfile    string
		args       []string
		wantName   string
		wantSecret string
	}{
		{
			name:       "defaults",
			wantName:   define.DEFAULT_COOKIE_NAME,
			wantSecret: define.DEFAULT_COOKIE_SECRET,
		},
		{
			name: "environment",
			envs: map[string]string{
				"FLARE_COOKIE_NAME":   "environment-session",
				"FLARE_COOKIE_SECRET": "environment-session-secret-32-bytes",
			},
			wantName:   "environment-session",
			wantSecret: "environment-session-secret-32-bytes",
		},
		{
			name: "envfile overrides environment",
			envs: map[string]string{
				"FLARE_COOKIE_NAME":   "environment-session",
				"FLARE_COOKIE_SECRET": "environment-session-secret-32-bytes",
			},
			envfile:    "FLARE_COOKIE_NAME=file-session\nFLARE_COOKIE_SECRET=file-session-secret-at-least-32-bytes\n",
			wantName:   "file-session",
			wantSecret: "file-session-secret-at-least-32-bytes",
		},
		{
			name: "CLI overrides envfile and environment",
			envs: map[string]string{
				"FLARE_COOKIE_NAME":   "environment-session",
				"FLARE_COOKIE_SECRET": "environment-session-secret-32-bytes",
			},
			envfile:    "FLARE_COOKIE_NAME=file-session\nFLARE_COOKIE_SECRET=file-session-secret-at-least-32-bytes\n",
			args:       []string{"--cookie_name", "cli-session", "--cookie_secret", "cli-session-secret-at-least-32-bytes"},
			wantName:   "cli-session",
			wantSecret: "cli-session-secret-at-least-32-bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"FLARE_COOKIE_NAME", "FLARE_COOKIE_SECRET"} {
				t.Setenv(key, "")
				require.NoError(t, os.Unsetenv(key))
			}
			for key, value := range tt.envs {
				t.Setenv(key, value)
			}
			if tt.envfile != "" {
				require.NoError(t, os.WriteFile(".env", []byte(tt.envfile), 0600))
				t.Cleanup(func() { require.NoError(t, os.Remove(".env")) })
			}
			os.Args = append([]string{"flare"}, tt.args...)

			flags := cmd.Parse()
			assert.Equal(t, tt.wantName, flags.CookieName)
			assert.Equal(t, tt.wantSecret, flags.CookieSecret)
		})
	}
}

func TestGetCliFlags(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	tests := []struct {
		name       string
		args       []string
		wantPort   int
		wantEnable bool
	}{
		{
			name:       "empty args",
			args:       []string{""},
			wantPort:   define.DEFAULT_PORT,
			wantEnable: false,
		},
		{
			name:       "set port and enable guide",
			args:       []string{"app", "--port", "9090", "--enable-guide"},
			wantPort:   9090,
			wantEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = append([]string{"app"}, tt.args...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			gotFlags, _ := cmd.GetCliFlags()
			assert.Equal(t, tt.wantPort, gotFlags.Port)
		})
	}
}

func TestGetFlagsMaps(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name string
		args []string
		want map[string]bool
	}{
		{
			name: "test single dash flags",
			args: []string{"cmd", "-foo", "-bar=value", "-baz"},
			want: map[string]bool{"foo": true, "bar": true, "baz": true},
		},
		{
			name: "test double dash flags",
			args: []string{"cmd", "--alpha", "--beta=ok", "--gamma"},
			want: map[string]bool{"alpha": true, "beta": true, "gamma": true},
		},
		{
			name: "test mixed dash flags",
			args: []string{"cmd", "--apple", "-banana=yellow", "--cherry", "-date"},
			want: map[string]bool{"apple": true, "banana": true, "cherry": true, "date": true},
		},
		{
			name: "test no flags",
			args: []string{"cmd"},
			want: map[string]bool{},
		},
		{
			name: "test empty args",
			args: []string{},
			want: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			got := cmd.GetFlagsMaps()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckFlagsExists(t *testing.T) {
	tests := []struct {
		name   string
		dict   map[string]bool
		keys   []string
		expect bool
	}{
		{
			name:   "all false",
			dict:   map[string]bool{"a": false, "b": false, "c": false},
			keys:   []string{"a", "b"},
			expect: false,
		},
		{
			name:   "one true",
			dict:   map[string]bool{"a": true, "b": false, "c": false},
			keys:   []string{"a", "b"},
			expect: true,
		},
		{
			name:   "none existent",
			dict:   map[string]bool{"a": true, "b": true},
			keys:   []string{"c", "d"},
			expect: false,
		},
		{
			name:   "empty keys",
			dict:   map[string]bool{"a": true, "b": true},
			keys:   []string{},
			expect: false,
		},
		{
			name:   "empty dict",
			dict:   map[string]bool{},
			keys:   []string{"a", "b"},
			expect: false,
		},
		{
			name:   "nil dict",
			dict:   nil,
			keys:   []string{"a", "b"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmd.CheckFlagsExists(tt.dict, tt.keys)
			assert.Equal(t, result, tt.expect)
		})
	}
}
