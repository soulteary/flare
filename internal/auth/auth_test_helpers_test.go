package auth

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCookieSecret = "test-cookie-secret-with-at-least-32-bytes"

func saveAppFlags() model.Flags {
	return define.AppFlags
}

func restoreAppFlags(f model.Flags) {
	define.AppFlags = f
}

func assertAutomaticCookieSecretLog(t *testing.T, output *bytes.Buffer, configured, effective, reason, action string, persisted bool) {
	t.Helper()
	var record struct {
		Message   string `json:"msg"`
		Reason    string `json:"reason"`
		Action    string `json:"action"`
		Persisted bool   `json:"persisted"`
	}
	require.NoError(t, json.NewDecoder(strings.NewReader(output.String())).Decode(&record))
	assert.NotEmpty(t, record.Message, "启动日志应说明自动初始化行为")
	assert.Equal(t, reason, record.Reason)
	assert.Equal(t, action, record.Action)
	assert.Equal(t, persisted, record.Persisted)
	assert.NotContains(t, output.String(), effective, "日志不应泄漏有效密钥")
	if configured == define.DEFAULT_COOKIE_SECRET {
		// The file name also contains "secret"; reject the actual JSON value instead.
		encoded, err := json.Marshal(configured)
		require.NoError(t, err)
		assert.NotContains(t, output.String(), string(encoded), "日志不应输出配置中的密钥值")
	} else if configured != "" {
		assert.NotContains(t, output.String(), configured, "日志不应泄漏配置中的密钥值")
	}
}
