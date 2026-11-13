package server

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestNotReadyPageExplainsEachStatus(t *testing.T) {
	const initialization = "security init --username admin"
	for _, tc := range []struct {
		name       string
		status     string
		diagnostic string
		contains   []string
		excludes   []string
	}{
		{
			name:     "missing administrator shows initialization",
			status:   "new",
			contains: []string{initialization, "docker compose stop flare", "initialize the administrator", "初始化管理员"},
		},
		{
			name:       "rejected input shows only its diagnostic",
			status:     "attention_required",
			diagnostic: "FLARE_VISIBILITY (from .env) must be DEFAULT or PRIVATE",
			contains:   []string{"<pre>FLARE_VISIBILITY (from .env) must be DEFAULT or PRIVATE</pre>", "Configuration to correct", "需要修正的配置", "docs/configuration.md"},
			excludes:   []string{initialization, "docs/backup-restore.md"},
		},
		{
			name:     "other attention states point to the log",
			status:   "attention_required",
			contains: []string{"Configuration or data needs attention", "配置或数据需要处理", "docs/accounts.md", "docs/backup-restore.md"},
			excludes: []string{initialization, "<pre>"},
		},
		{
			name:     "storage faults describe recovery",
			status:   "faulted",
			contains: []string{"Security state needs attention", "安全状态需要处理", "complete backup"},
			excludes: []string{initialization, "<pre>"},
		},
		{
			name:     "unknown statuses use the generic explanation",
			status:   "migrating",
			contains: []string{"Configuration or data needs attention"},
			excludes: []string{initialization},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := notReadyPage(tc.status, tc.diagnostic)
			require.Contains(t, page, "Flare 尚未就绪 / Flare is not ready")
			require.Contains(t, page, "Existing content is not being served.")
			require.Contains(t, page, "<code>"+tc.status+"</code>")
			for _, want := range tc.contains {
				require.Contains(t, page, want)
			}
			for _, unwanted := range tc.excludes {
				require.NotContains(t, page, unwanted)
			}
		})
	}
}

func TestNotReadyPageEscapesDynamicText(t *testing.T) {
	page := notReadyPage(`<script>alert(1)</script>`, `unknown command-line flag --<b>x</b>&y`)
	require.NotContains(t, page, "<script>")
	require.NotContains(t, page, "<b>x</b>")
	require.Contains(t, page, "<code>&lt;script&gt;alert(1)&lt;/script&gt;</code>")
	require.Contains(t, page, "<pre>unknown command-line flag --&lt;b&gt;x&lt;/b&gt;&amp;y</pre>")
}

// The page shows the fixed-text input diagnostic but never StartupMessage, which
// can quote the content of navigation files.
func TestNotReadyResponseShowsDiagnosticWithoutStartupMessage(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")

	const diagnostic = "FLARE_DISABLE_LOGIN (from environment) must be a boolean (true, false, 1, 0, yes, no, on or off)"
	app, err := NewFiberApp(&model.Flags{
		StartupStatus:     "attention_required",
		StartupMessage:    "INTERNAL-MESSAGE-SENTINEL",
		StartupDiagnostic: diagnostic,
	})
	require.NoError(t, err)
	for _, path := range []string{"/", "/ping", "/login", "/editor"} {
		response, err := app.Test(httptest.NewRequest("GET", "http://flare.test"+path, nil))
		require.NoError(t, err)
		raw, err := io.ReadAll(response.Body)
		require.NoError(t, response.Body.Close())
		require.NoError(t, err)
		require.Equal(t, 503, response.StatusCode, path)
		require.Equal(t, "private, no-store", response.Header.Get("Cache-Control"))
		require.Contains(t, response.Header.Get("Content-Type"), "text/html")
		require.Contains(t, string(raw), diagnostic)
		require.NotContains(t, string(raw), "INTERNAL-MESSAGE-SENTINEL")
		require.NotContains(t, string(raw), "security init")
	}
}
