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

func TestParseCompression(t *testing.T) {
	originalArgs := os.Args
	originalFlags := define.AppFlags
	originalLogger := logger.GetLogger()
	t.Cleanup(func() {
		os.Args = originalArgs
		define.AppFlags = originalFlags
		logger.SetLogger(originalLogger)
	})
	value := func(text string) *string { return &text }

	tests := []struct {
		name       string
		env        *string
		envfile    string
		args       []string
		want       string
		warnSource string
	}{
		{name: "default sends gzip and deflate", want: "gzip"},
		{name: "environment all", env: value("all"), want: "all"},
		{name: "environment off", env: value("off"), want: "off"},
		{name: "normalize environment", env: value("  AlL \t"), want: "all"},
		{name: "invalid environment", env: value("brotli"), want: "gzip", warnSource: "FLARE_COMPRESSION"},
		{name: "empty environment", env: value(""), want: "gzip", warnSource: "FLARE_COMPRESSION"},
		{name: "absent envfile setting preserves environment", env: value("off"), envfile: "FLARE_PORT=5123\n", want: "off"},
		{name: "envfile overrides environment", env: value("off"), envfile: "FLARE_COMPRESSION= AlL \n", want: "all"},
		{name: "invalid envfile uses the default", env: value("all"), envfile: "FLARE_COMPRESSION=zstd\n", want: "gzip", warnSource: ".env FLARE_COMPRESSION"},
		{name: "CLI overrides envfile and environment", env: value("all"), envfile: "FLARE_COMPRESSION=gzip\n", args: []string{"--compression", " OfF "}, want: "off"},
		{name: "CLI all", envfile: "FLARE_COMPRESSION=off\n", args: []string{"--compression=all"}, want: "all"},
		{name: "invalid CLI uses the default", env: value("all"), args: []string{"--compression=br"}, want: "gzip", warnSource: "--compression"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("FLARE_PORT", "5123")
			t.Setenv("FLARE_COMPRESSION", "")
			if tt.env == nil {
				require.NoError(t, os.Unsetenv("FLARE_COMPRESSION"))
			} else {
				t.Setenv("FLARE_COMPRESSION", *tt.env)
			}
			if tt.envfile != "" {
				require.NoError(t, os.WriteFile(".env", []byte(tt.envfile), 0600))
			}
			os.Args = append([]string{"flare"}, tt.args...)
			var output bytes.Buffer
			logger.SetLogger(slog.New(slog.NewJSONHandler(&output, nil)))

			resolved := cmd.Parse()

			assert.Equal(t, tt.want, resolved.Compression)
			assert.Equal(t, tt.want, define.AppFlags.Compression)
			assert.Equal(t, 5123, resolved.Port, "the compression setting must not change other settings")
			if tt.warnSource == "" {
				assert.NotContains(t, output.String(), "响应压缩模式无效")
			} else {
				assert.Contains(t, output.String(), `"level":"WARN"`)
				assert.Contains(t, output.String(), tt.warnSource+" 的响应压缩模式无效")
				assert.Contains(t, output.String(), "使用 gzip")
			}
		})
	}
}
