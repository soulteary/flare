package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestCredentialSourceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    model.Flags
		want string
	}{
		{"default user", model.Flags{RuntimeInputs: map[string]string{"FLARE_PASS": "password"}}, ""},
		{"oversized user", model.Flags{User: strings.Repeat("u", 257)}, "FLARE_USER is too long"},
		{"oversized deployment password", model.Flags{RuntimeInputs: map[string]string{"FLARE_PASS": strings.Repeat("p", 4097)}}, "deployment password exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u, p, err := readCredential(&tc.f)
			if tc.want != "" {
				require.ErrorContains(t, err, tc.want)
			} else {
				require.NoError(t, err)
				require.Equal(t, "flare", u)
				require.Equal(t, "password", p)
			}
		})
	}
	for _, raw := range []string{strings.Repeat("x", 515), strings.Repeat("x", 513), "invalid\xff"} {
		t.Run("invalid password file", func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "password")
			require.NoError(t, os.WriteFile(file, []byte(raw), 0600))
			_, _, err := readCredential(&model.Flags{RuntimeInputs: map[string]string{"FLARE_PASS_FILE": file}})
			require.Error(t, err)
		})
	}
}

func TestStartupInspectionRejectsBrokenSources(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
	}{
		{"broken content link", func(t *testing.T) { require.NoError(t, os.Symlink("missing", "apps.yml")) }},
		{"directory content", func(t *testing.T) { require.NoError(t, os.Mkdir("bookmarks.yml", 0700)) }},
		{"unreadable content", func(t *testing.T) {
			require.NoError(t, os.WriteFile("apps.yml", []byte("links: []\n"), 0000))
			t.Cleanup(func() { _ = os.Chmod("apps.yml", 0600) })
		}},
		{"legacy cookie link loop", func(t *testing.T) { require.NoError(t, os.Symlink(".flare-cookie-secret", ".flare-cookie-secret")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			tc.setup(t)
			_, _, _, err := inspectContent()
			if tc.name == "unreadable content" && os.Geteuid() == 0 {
				t.Skip("root bypasses file permissions")
			}
			require.Error(t, err)
		})
	}
	t.Run("private category", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.WriteFile("bookmarks.yml", []byte("categories:\n- id: private\n  title: Private\n  private: true\nlinks: []\n"), 0600))
		files, count, legacy, err := inspectContent()
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.True(t, legacy)
		require.False(t, explicitLegacy(startupFlags(nil), files))
		require.True(t, explicitLegacy(startupFlags(map[string]string{"FLARE_VISIBILITY": "DEFAULT"}), files))
	})
}

func TestExplicitSecurityConfigurationErrors(t *testing.T) {
	require.EqualError(t, applyExplicit(&model.Flags{ConfigError: "invalid config"}), "invalid config")
	require.ErrorContains(t, applyExplicit(&model.Flags{Visibility: "INVALID"}), "FLARE_VISIBILITY")
	require.ErrorContains(t, applyExplicit(&model.Flags{Visibility: "DEFAULT", RuntimeInputs: map[string]string{"FLARE_DISABLE_LOGIN": ""}}), "FLARE_DISABLE_LOGIN")
	t.Run("installation directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(dir, "installation.json"), 0700))
		_, _, err := readInstallation(dir)
		require.Error(t, err)
	})
}

func TestNewInstallationCredentialPolicyAndModeTransitions(t *testing.T) {
	for _, password := range []string{"short", "a unique initial administrator phrase 42"} {
		t.Run(password, func(t *testing.T) {
			t.Chdir(t.TempDir())
			inputs := map[string]string{"FLARE_PASS": password}
			p := PrepareSecurity(startupFlags(inputs))
			defer closePreparation(t, p)
			if password == "short" {
				require.Equal(t, "attention_required", p.Status)
				require.False(t, p.Store.Info().Initialized)
			} else {
				require.Equal(t, "ready", p.Status, p.Message)
				require.Equal(t, "local", p.Store.Info().Mode)
			}
		})
	}
	t.Run("trusted to local", func(t *testing.T) {
		t.Chdir(t.TempDir())
		inputs := map[string]string{"FLARE_DISABLE_LOGIN": "true", "FLARE_VISIBILITY": "DEFAULT"}
		p := PrepareSecurity(startupFlags(inputs))
		require.Equal(t, "ready", p.Status, p.Message)
		closePreparation(t, p)
		inputs = map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "a unique initial administrator phrase 42"}
		p = PrepareSecurity(startupFlags(inputs))
		require.Equal(t, "ready", p.Status, p.Message)
		require.Equal(t, "local", p.Store.Info().Mode)
		token, err := p.Store.CreateSession()
		require.NoError(t, err)
		closePreparation(t, p)
		inputs = map[string]string{"FLARE_DISABLE_LOGIN": "true", "FLARE_VISIBILITY": "DEFAULT"}
		p = PrepareSecurity(startupFlags(inputs))
		require.Equal(t, "ready", p.Status, p.Message)
		_, err = p.Store.Session(token, false)
		require.ErrorIs(t, err, security.ErrInvalidSession)
		closePreparation(t, p)
		inputs = map[string]string{"FLARE_CREDENTIAL_MODE": "deployment"}
		p = PrepareSecurity(startupFlags(inputs))
		defer closePreparation(t, p)
		require.Equal(t, "attention_required", p.Status)
	})
	t.Run("invalid content", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.Mkdir("apps.yml", 0700))
		p := PrepareSecurity(startupFlags(nil))
		defer closePreparation(t, p)
		require.Equal(t, "attention_required", p.Status)
		require.Nil(t, p.Store)
	})
	t.Run("invalid metadata", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.Mkdir(".flare-security", 0700))
		require.NoError(t, os.WriteFile(".flare-security/installation.json", []byte("{"), 0600))
		p := PrepareSecurity(startupFlags(nil))
		defer closePreparation(t, p)
		require.Equal(t, "faulted", p.Status)
	})
}
