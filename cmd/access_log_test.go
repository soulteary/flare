package cmd_test

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAccessLog(t *testing.T) {
	originalArgs := os.Args
	originalFlags := define.AppFlags
	originalLogger := logger.GetLogger()
	t.Cleanup(func() {
		os.Args = originalArgs
		define.AppFlags = originalFlags
		logger.SetLogger(originalLogger)
	})

	tests := []struct {
		name       string
		env        *string
		envfile    string
		args       []string
		want       string
		warnSource string
	}{
		{name: "default only records failures", want: "errors"},
		{name: "environment errors", env: accessLogValue("errors"), want: "errors"},
		{name: "environment all", env: accessLogValue("all"), want: "all"},
		{name: "environment off", env: accessLogValue("off"), want: "off"},
		{name: "normalize environment", env: accessLogValue("  AlL \t"), want: "all"},
		{name: "invalid environment", env: accessLogValue("invalid"), want: "errors", warnSource: "FLARE_ACCESS_LOG"},
		{name: "empty environment", env: accessLogValue(""), want: "errors", warnSource: "FLARE_ACCESS_LOG"},
		{
			name:    "absent envfile setting preserves environment",
			env:     accessLogValue("off"),
			envfile: "FLARE_PORT=5123\n",
			want:    "off",
		},
		{
			name:    "envfile overrides environment",
			env:     accessLogValue("off"),
			envfile: "FLARE_ACCESS_LOG= AlL \n",
			want:    "all",
		},
		{
			name:       "invalid envfile uses safe default",
			env:        accessLogValue("all"),
			envfile:    "FLARE_ACCESS_LOG=invalid\n",
			want:       "errors",
			warnSource: ".env FLARE_ACCESS_LOG",
		},
		{
			name:       "empty envfile uses safe default",
			env:        accessLogValue("all"),
			envfile:    "FLARE_ACCESS_LOG=\n",
			want:       "errors",
			warnSource: ".env FLARE_ACCESS_LOG",
		},
		{
			name:    "CLI overrides envfile and environment",
			env:     accessLogValue("all"),
			envfile: "FLARE_ACCESS_LOG=errors\n",
			args:    []string{"--access_log", " OfF "},
			want:    "off",
		},
		{
			name: "CLI errors mode",
			env:  accessLogValue("off"),
			args: []string{"--access_log=errors"},
			want: "errors",
		},
		{
			name:    "CLI all mode",
			envfile: "FLARE_ACCESS_LOG=off\n",
			args:    []string{"--access_log=all"},
			want:    "all",
		},
		{
			name:       "invalid CLI uses safe default",
			env:        accessLogValue("all"),
			envfile:    "FLARE_ACCESS_LOG=off\n",
			args:       []string{"--access_log=invalid"},
			want:       "errors",
			warnSource: "--access_log",
		},
		{
			name:       "empty CLI uses safe default",
			env:        accessLogValue("all"),
			args:       []string{"--access_log="},
			want:       "errors",
			warnSource: "--access_log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FLARE_PORT", "5123")
			t.Setenv("FLARE_ACCESS_LOG", "")
			if tt.env == nil {
				require.NoError(t, os.Unsetenv("FLARE_ACCESS_LOG"))
			} else {
				t.Setenv("FLARE_ACCESS_LOG", *tt.env)
			}
			if tt.envfile != "" {
				require.NoError(t, os.WriteFile(".env", []byte(tt.envfile), 0600))
			}
			os.Args = append([]string{"flare"}, tt.args...)
			var output bytes.Buffer
			logger.SetLogger(slog.New(slog.NewJSONHandler(&output, nil)))

			resolved := cmd.Parse()

			assert.Equal(t, tt.want, resolved.AccessLog)
			assert.Equal(t, tt.want, define.AppFlags.AccessLog)
			assert.Equal(t, 5123, resolved.Port, "access log configuration must not change other settings")
			if tt.warnSource == "" {
				assert.NotContains(t, output.String(), "访问日志模式无效")
			} else {
				assert.Contains(t, output.String(), `"level":"WARN"`)
				assert.Contains(t, output.String(), tt.warnSource+" 的访问日志模式无效")
				assert.Contains(t, output.String(), "使用 errors")
			}
		})
	}
}

func accessLogValue(value string) *string { return &value }
