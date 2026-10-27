package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestSecurityAcceptanceRejectsInvalidInvocationAndConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		prepare func(*testing.T)
		code    int
	}{
		{name: "missing command", code: 2},
		{name: "unknown flag", args: []string{"init", "--unknown-option"}, code: 2},
		{name: "unexpected positional", args: []string{"init", "extra"}, code: 2},
		{name: "unknown command", args: []string{"rotate-magic"}, code: 2},
		{name: "invalid access mode", args: []string{"init"}, prepare: func(t *testing.T) { t.Setenv("FLARE_VISIBILITY", "unrecognized") }, code: 2},
		{name: "invalid env file", args: []string{"migration-check"}, prepare: func(t *testing.T) { require.NoError(t, os.WriteFile(".env", []byte("[broken"), 0600)) }, code: 2},
		{name: "invalid content", args: []string{"migration-check"}, prepare: func(t *testing.T) { require.NoError(t, os.WriteFile("apps.yml", []byte("links: [broken"), 0600)) }, code: 2},
		{name: "maintenance invalid content", args: []string{"init"}, prepare: func(t *testing.T) { require.NoError(t, os.WriteFile("apps.yml", []byte("links: [broken"), 0600)) }, code: 2},
		{name: "private trusted conflict", args: []string{"init"}, prepare: func(t *testing.T) { writeStartupFiles(t, startupPrivate) }, code: 2},
		{name: "state path is file", args: []string{"init", "--state-dir", "state-file"}, prepare: func(t *testing.T) { require.NoError(t, os.WriteFile("state-file", []byte("user-owned"), 0600)) }, code: 4},
		{name: "damaged metadata", args: []string{"reset-password"}, prepare: func(t *testing.T) {
			require.NoError(t, os.Mkdir(".flare-security", 0700))
			require.NoError(t, os.WriteFile(".flare-security/installation.json", []byte("corrupt"), 0600))
		}, code: 4},
		{name: "damaged metadata read-only check", args: []string{"migration-check"}, prepare: func(t *testing.T) {
			require.NoError(t, os.Mkdir(".flare-security", 0700))
			require.NoError(t, os.WriteFile(".flare-security/installation.json", []byte("corrupt"), 0600))
		}, code: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			commandDirectory(t)
			if test.prepare != nil {
				test.prepare(t)
			}
			code, out, errout := runSecurity(t, test.args, "")
			require.Equal(t, test.code, code, out+errout)
			require.NotContains(t, out, "Security state updated")
			if test.name == "state path is file" {
				raw, readErr := os.ReadFile("state-file")
				require.NoError(t, readErr)
				require.Equal(t, "user-owned", string(raw))
			}
			if test.name == "private trusted conflict" {
				raw, readErr := os.ReadFile("bookmarks.yml")
				require.NoError(t, readErr)
				require.Equal(t, startupPrivate, string(raw))
			}
		})
	}
	for _, args := range [][]string{nil, {"serve"}} {
		var out bytes.Buffer
		handled, code := ExecuteSecurityCommand(args, strings.NewReader(""), &out, &out)
		require.False(t, handled)
		require.Zero(t, code)
		require.Empty(t, out.String())
	}
}

func TestSecurityAcceptanceMigrationReportsAreReadOnlyAndUseful(t *testing.T) {
	for _, test := range []struct {
		name           string
		private, login bool
		want           string
	}{
		{name: "empty installation", want: "new"},
		{name: "legacy needs credentials", login: true, want: "attention_required"},
		{name: "trusted private conflict", private: true, want: "attention_required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			commandDirectory(t)
			if test.login {
				writeStartupFiles(t, startupPublic)
				t.Setenv("FLARE_DISABLE_LOGIN", "false")
			}
			if test.private {
				writeStartupFiles(t, startupPrivate)
			}
			code, out, errout := runSecurity(t, []string{"migration-check", "--json"}, "")
			require.Zero(t, code, errout)
			var report migrationReport
			require.NoError(t, json.Unmarshal([]byte(out), &report))
			require.Equal(t, test.want, report.Status)
			_, statErr := os.Stat(".flare-security")
			require.True(t, os.IsNotExist(statErr), "report must not initialize the instance")
		})
	}
	t.Run("new local installation reports effective safe defaults", func(t *testing.T) {
		commandDirectory(t)
		password := "An uncommon administrator phrase 2026!"
		code, _, errout := runSecurity(t, []string{"init", "--state-dir", "private-state"}, password+"\n"+password+"\n")
		require.Zero(t, code, errout)
		code, out, errout := runSecurity(t, []string{"migration-check", "--state-dir", "private-state", "--json"}, "")
		require.Zero(t, code, errout)
		var report migrationReport
		require.NoError(t, json.Unmarshal([]byte(out), &report))
		require.Equal(t, "existing_installation", report.Status)
		require.False(t, report.LoginDisabled)
		require.Equal(t, "PRIVATE", report.Visibility)
		require.Equal(t, "private-state", report.PersistentDirectory)
		code, out, errout = runSecurity(t, []string{"migration-check", "--state-dir", "private-state"}, "")
		require.Zero(t, code, errout)
		require.Contains(t, out, "visibility=PRIVATE")
		require.NotContains(t, out, password)
	})
	t.Run("failed report output is a storage error", func(t *testing.T) {
		commandDirectory(t)
		require.NoError(t, os.WriteFile("report", []byte("preserve"), 0600))
		out, openErr := os.Open("report")
		require.NoError(t, openErr)
		defer out.Close()
		var errout bytes.Buffer
		handled, code := ExecuteSecurityCommand([]string{"security", "migration-check", "--json"}, strings.NewReader(""), out, &errout)
		require.True(t, handled)
		require.Equal(t, 4, code)
		raw, readErr := os.ReadFile("report")
		require.NoError(t, readErr)
		require.Equal(t, "preserve", string(raw))
	})
}

func TestSecurityAcceptanceMaintenancePreconditionsAndCredentialSwitch(t *testing.T) {
	t.Run("uninitialized maintenance", func(t *testing.T) {
		commandDirectory(t)
		for _, args := range [][]string{{"revoke-all"}, {"set-credentials", "--mode", "local"}} {
			code, _, errout := runSecurity(t, args, "")
			require.Equal(t, 2, code, errout)
		}
	})
	t.Run("explicit trusted adoption", func(t *testing.T) {
		commandDirectory(t)
		writeStartupFiles(t, startupPublic)
		code, out, errout := runSecurity(t, []string{"adopt-legacy-defaults"}, "")
		require.Zero(t, code, errout)
		require.Contains(t, out, "network visitors can modify content")
		state, openErr := security.Open(".flare-security")
		require.NoError(t, openErr)
		require.Equal(t, security.ModeTrusted, state.Info().Mode)
		require.Empty(t, state.Info().Username)
		require.NoError(t, state.Close())
		code, _, errout = runSecurity(t, []string{"adopt-legacy-defaults"}, "")
		require.Equal(t, 2, code, errout)
	})
	t.Run("adoption requires trusted mode", func(t *testing.T) {
		commandDirectory(t)
		t.Setenv("FLARE_DISABLE_LOGIN", "false")
		code, _, errout := runSecurity(t, []string{"adopt-legacy-defaults"}, "")
		require.Equal(t, 2, code)
		require.Contains(t, errout, "legacy defaults require")
	})
	t.Run("local deployment local switch revokes sessions", func(t *testing.T) {
		commandDirectory(t)
		writeStartupFiles(t, startupPublic)
		t.Setenv("FLARE_DISABLE_LOGIN", "false")
		password := "An uncommon administrator phrase 2026!"
		code, _, errout := runSecurity(t, []string{"init", "--username", "local-admin"}, password+"\n"+password+"\n")
		require.Zero(t, code, errout)
		for _, args := range [][]string{{"init"}, {"set-credentials", "--mode", "external"}} {
			code, _, errout = runSecurity(t, args, "")
			require.Equal(t, 2, code, errout)
		}
		state, openErr := security.Open(".flare-security")
		require.NoError(t, openErr)
		token, authErr := state.Authenticate("local-admin", password)
		require.NoError(t, authErr)
		require.NoError(t, state.Close())
		require.NoError(t, os.WriteFile("secret", []byte("deployment short\r\n"), 0400))
		t.Setenv("FLARE_USER", "deployment-admin")
		code, _, errout = runSecurity(t, []string{"set-credentials", "--mode", "deployment", "--password-file", "secret"}, "")
		require.Zero(t, code, errout)
		state, openErr = security.Open(".flare-security")
		require.NoError(t, openErr)
		require.Equal(t, "deployment", state.Info().Mode)
		require.NoError(t, state.Verify("deployment-admin", "deployment short"))
		_, sessionErr := state.Session(token, false)
		require.ErrorIs(t, sessionErr, security.ErrInvalidSession)
		require.NoError(t, state.Close())
		require.NoError(t, os.WriteFile("local-secret", []byte(password+"\n"), 0600))
		code, _, errout = runSecurity(t, []string{"set-credentials", "--mode", "local", "--password-file", "local-secret"}, "")
		require.Zero(t, code, errout)
		state, openErr = security.Open(".flare-security")
		require.NoError(t, openErr)
		defer state.Close()
		require.Equal(t, "local", state.Info().Mode)
		require.NoError(t, state.Verify("deployment-admin", password))
		raw, readErr := os.ReadFile("bookmarks.yml")
		require.NoError(t, readErr)
		require.Equal(t, startupPublic, string(raw))
	})
}

func TestSecurityAcceptancePasswordInputErrorsDoNotCreateAccount(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		prepare func(*testing.T)
		input   io.Reader
	}{
		{name: "missing file", args: []string{"--password-file", "missing"}},
		{name: "directory source", args: []string{"--password-file", "directory"}, prepare: func(t *testing.T) { require.NoError(t, os.Mkdir("directory", 0700)) }},
		{name: "oversize source", args: []string{"--password-file", "long-secret"}, prepare: func(t *testing.T) {
			require.NoError(t, os.WriteFile("long-secret", []byte(strings.Repeat("x", 515)), 0600))
		}},
		{name: "weak source", args: []string{"--password-file", "short-secret"}, prepare: func(t *testing.T) { require.NoError(t, os.WriteFile("short-secret", []byte("short\n"), 0600)) }},
		{name: "confirmation mismatch", input: strings.NewReader("An uncommon administrator phrase 2026!\nDifferent administrator phrase 2026!\n")},
		{name: "reader failure", input: iotest.ErrReader(io.ErrUnexpectedEOF)},
		{name: "confirmation reader failure", input: io.MultiReader(strings.NewReader("An uncommon administrator phrase 2026!\n"), iotest.ErrReader(io.ErrUnexpectedEOF))},
	} {
		t.Run(test.name, func(t *testing.T) {
			commandDirectory(t)
			if test.prepare != nil {
				test.prepare(t)
			}
			input := test.input
			if input == nil {
				input = strings.NewReader("")
			}
			var out, errout bytes.Buffer
			handled, code := ExecuteSecurityCommand(append([]string{"security", "init"}, test.args...), input, &out, &errout)
			require.True(t, handled)
			require.Equal(t, 2, code, errout.String())
			require.NotContains(t, out.String(), "An uncommon administrator phrase")
			_, statErr := os.Stat(filepath.Join(".flare-security", "complete"))
			require.True(t, os.IsNotExist(statErr), "failed input must not publish an administrator")
		})
	}
}
