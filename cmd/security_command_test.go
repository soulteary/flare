package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func commandDirectory(t *testing.T) {
	t.Helper()
	old, e := os.Getwd()
	require.NoError(t, e)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { _ = os.Chdir(old) })
	for _, key := range securityInputKeys {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}
func runSecurity(t *testing.T, args []string, input string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	handled, code := ExecuteSecurityCommand(append([]string{"security"}, args...), strings.NewReader(input), &out, &err)
	require.True(t, handled)
	return code, out.String(), err.String()
}
func TestSecurityCommandsInitializeResetAndRevoke(t *testing.T) {
	commandDirectory(t)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links: []\ncustom: unchanged\n"), 0600))
	raw, e := os.ReadFile("bookmarks.yml")
	require.NoError(t, e)
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	t.Setenv("FLARE_VISIBILITY", "PRIVATE")
	code, out, err := runSecurity(t, []string{"init", "--username", "legacy-user"}, "A unique local password 2026!\nA unique local password 2026!\n")
	require.Equal(t, 0, code, err)
	require.NotContains(t, out, "A unique local")
	current, e := os.ReadFile("bookmarks.yml")
	require.NoError(t, e)
	require.Equal(t, raw, current)
	f := &model.Flags{Visibility: "PRIVATE", User: "flare", RuntimeInputs: map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_VISIBILITY": "PRIVATE"}}
	prepared := PrepareSecurity(f)
	require.Equal(t, "ready", prepared.Status, prepared.Message)
	token, e := prepared.Store.Authenticate("legacy-user", "A unique local password 2026!")
	require.NoError(t, e)
	require.NoError(t, prepared.Store.Close())
	code, _, err = runSecurity(t, []string{"revoke-all"}, "")
	require.Equal(t, 0, code, err)
	store, e := security.Open(".flare-security")
	require.NoError(t, e)
	_, e = store.Session(token, false)
	require.ErrorIs(t, e, security.ErrInvalidSession)
	require.NoError(t, store.Close())
	code, _, err = runSecurity(t, []string{"reset-password"}, "Another unique password 2026!\nAnother unique password 2026!\n")
	require.Equal(t, 0, code, err)
	store, e = security.Open(".flare-security")
	require.NoError(t, e)
	defer store.Close()
	require.NoError(t, store.Verify("legacy-user", "Another unique password 2026!"))
	require.ErrorIs(t, store.Verify("legacy-user", "A unique local password 2026!"), security.ErrInvalidCredentials)
}
func TestMigrationCheckIsReadOnlyAndRedacted(t *testing.T) {
	commandDirectory(t)
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	t.Setenv("FLARE_PASS", "short-legacy-secret")
	require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n- name: private-sentinel\n  link: https://private-sentinel.test/\n  private: true\n"), 0600))
	code, out, err := runSecurity(t, []string{"migration-check", "--json"}, "")
	require.Equal(t, 0, code, err)
	require.Contains(t, out, `"private_resources":1`)
	require.NotContains(t, out, "short-legacy-secret")
	require.NotContains(t, out, "private-sentinel")
	_, e := os.Stat(".flare-security")
	require.True(t, os.IsNotExist(e))
}
func TestSecurityCommandRejectsCorruptOrRunningState(t *testing.T) {
	commandDirectory(t)
	code, _, err := runSecurity(t, []string{"init"}, "First unique password 2026!\nFirst unique password 2026!\n")
	require.Equal(t, 0, code, err)
	store, e := security.Open(".flare-security")
	require.NoError(t, e)
	code, _, _ = runSecurity(t, []string{"revoke-all"}, "")
	require.Equal(t, 3, code)
	require.NoError(t, store.Close())
	require.NoError(t, os.Remove(filepath.Join(".flare-security", "state.json")))
	require.NoError(t, os.Remove(filepath.Join(".flare-security", "complete")))
	code, _, err = runSecurity(t, []string{"init"}, "Second unique password 2026!\nSecond unique password 2026!\n")
	require.Equal(t, 4, code, err)
}
func TestCaptureCredentialsBeforeUnsetAndEnvFilePrecedence(t *testing.T) {
	commandDirectory(t)
	t.Setenv("FLARE_PASS", "environment-secret")
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	require.NoError(t, os.WriteFile(".env", []byte("FLARE_PASS=file-secret\nFLARE_VISIBILITY=PRIVATE\n"), 0600))
	inputs, sources, e := captureSecurityInputs()
	require.NoError(t, e)
	require.Equal(t, "file-secret", inputs["FLARE_PASS"])
	require.Equal(t, ".env", sources["FLARE_PASS"])
	_ = ParseEnvVars()
	require.Equal(t, "file-secret", inputs["FLARE_PASS"])
	require.Error(t, validateSecurityInputs(map[string]string{"FLARE_DISABLE_LOGIN": "invalid"}))
	require.Error(t, validateSecurityInputs(map[string]string{"FLARE_VISIBILITY": "0"}))
}

func TestStartupReadsSymlinkedYAMLWithoutReplacingIt(t *testing.T) {
	commandDirectory(t)
	require.NoError(t, os.WriteFile("source.yaml", []byte("links: []\n"), 0400))
	require.NoError(t, os.Symlink("source.yaml", "apps.yml"))
	f := &model.Flags{Visibility: "DEFAULT", DisableLoginMode: true, User: "flare", RuntimeInputs: map[string]string{"FLARE_DISABLE_LOGIN": "true", "FLARE_VISIBILITY": "DEFAULT"}}
	prepared := PrepareSecurity(f)
	if prepared.Store != nil {
		defer prepared.Store.Close()
	}
	require.Equal(t, "ready", prepared.Status, prepared.Message)
	target, e := os.Readlink("apps.yml")
	require.NoError(t, e)
	require.Equal(t, "source.yaml", target)
	raw, e := os.ReadFile("source.yaml")
	require.NoError(t, e)
	require.Equal(t, "links: []\n", string(raw))
}

func TestSecurityMigrationCheckDetectsCorruptStateWithoutWriting(t *testing.T) {
	for _, fault := range []string{"checksum", "marker", "journal", "pending", "missing-metadata"} {
		t.Run(fault, func(t *testing.T) {
			commandDirectory(t)
			writeStartupFiles(t, startupPublic)
			t.Setenv("FLARE_DISABLE_LOGIN", "false")
			t.Setenv("FLARE_PASS", "short")
			p := PrepareSecurity(startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "short"}))
			require.Equal(t, "ready", p.Status)
			closePreparation(t, p)
			raw, e := os.ReadFile(".flare-security/state.json")
			require.NoError(t, e)
			switch fault {
			case "checksum":
				require.NoError(t, os.WriteFile(".flare-security/state.json", []byte("corrupt"), 0600))
			case "marker":
				require.NoError(t, os.WriteFile(".flare-security/complete", []byte("wrong-instance"), 0600))
			case "journal":
				require.NoError(t, os.WriteFile(".flare-security/commit.json", []byte("corrupt"), 0600))
			case "pending":
				require.NoError(t, os.WriteFile(".flare-security/commit.json", raw, 0600))
			case "missing-metadata":
				require.NoError(t, os.Remove(".flare-security/installation.json"))
			}
			before := map[string][]byte{}
			for _, name := range []string{"state.json", "complete", "commit.json", "installation.json"} {
				before[name], _ = os.ReadFile(filepath.Join(".flare-security", name))
			}
			code, out, _ := runSecurity(t, []string{"migration-check", "--json"}, "")
			if fault == "pending" {
				require.Equal(t, 3, code)
				require.Contains(t, out, `"status":"indeterminate"`)
			} else {
				require.Equal(t, 4, code)
				require.Contains(t, out, `"status":"faulted"`)
			}
			require.NotContains(t, out, "existing_installation")
			require.NotContains(t, out, "short")
			require.NotContains(t, out, "Checksum")
			for name, want := range before {
				got, _ := os.ReadFile(filepath.Join(".flare-security", name))
				require.Equal(t, want, got)
			}
		})
	}
}

func TestSecurityResetMissingDeploymentSourceCompletesLegacyMigration(t *testing.T) {
	commandDirectory(t)
	writeStartupFiles(t, startupPrivate)
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	t.Setenv("FLARE_VISIBILITY", "DEFAULT")
	t.Setenv("FLARE_USER", " old admin ")
	t.Setenv("FLARE_PASS_FILE", "missing-source")
	inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_VISIBILITY": "DEFAULT", "FLARE_USER": " old admin ", "FLARE_PASS_FILE": "missing-source"}
	p := PrepareSecurity(startupFlags(inputs))
	require.Equal(t, "attention_required", p.Status)
	closePreparation(t, p)
	password := "A new unique recovery password 42!"
	code, _, errout := runSecurity(t, []string{"reset-password"}, password+"\n"+password+"\n")
	require.Equal(t, 0, code, errout)
	p = PrepareSecurity(startupFlags(inputs))
	defer closePreparation(t, p)
	require.Equal(t, "ready", p.Status, p.Message)
	require.Equal(t, "local", p.Store.Info().Mode)
	require.NoError(t, p.Store.Verify(" old admin ", password))
	raw, e := os.ReadFile("bookmarks.yml")
	require.NoError(t, e)
	require.Equal(t, startupPrivate, string(raw))
	meta, exists, e := readInstallation(".flare-security")
	require.NoError(t, e)
	require.True(t, exists)
	require.True(t, meta.Committed)
	require.False(t, meta.NewInstall)
}
