package cmd

import (
	"bytes"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

const startupPublic = "categories: []\nlinks:\n  - name: public-sentinel\n    link: https://example.com\n"
const startupPrivate = "categories: []\nlinks:\n  - name: private-sentinel\n    link: https://secret.example.com\n    private: true\n"

func startupFlags(inputs map[string]string) *model.Flags {
	return &model.Flags{Visibility: "DEFAULT", DisableLoginMode: true, User: "flare", RuntimeInputs: inputs}
}
func writeStartupFiles(t *testing.T, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte(content), 0600))
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: []\n"), 0600))
	require.NoError(t, os.WriteFile("config.yml", []byte("title: existing deployment\n"), 0600))
}
func closePreparation(t *testing.T, p SecurityPreparation) {
	t.Helper()
	if p.Store != nil {
		require.NoError(t, p.Store.Close())
	}
}
func TestSecurityStartupDefaultsPrepareTrustedState(t *testing.T) {
	for _, content := range []string{"empty", "samples", "existing"} {
		for _, explicit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/explicit=%v", content, explicit), func(t *testing.T) {
				t.Chdir(t.TempDir())
				if content == "existing" {
					writeStartupFiles(t, startupPublic)
				} else if content == "samples" {
					_, err := data.LoadFavoriteBookmarks()
					require.NoError(t, err)
					_, err = data.LoadNormalBookmarks()
					require.NoError(t, err)
				}
				inputs := map[string]string{}
				if explicit {
					inputs["FLARE_DISABLE_LOGIN"] = "true"
				}
				var instance string
				for i := 0; i < 2; i++ {
					f := startupFlags(inputs)
					p := PrepareSecurity(f)
					require.Equal(t, "ready", p.Status, p.Message)
					require.True(t, f.DisableLoginMode)
					require.Equal(t, "DEFAULT", f.Visibility)
					require.True(t, p.Store.Info().Initialized)
					require.Equal(t, security.ModeTrusted, p.Store.Info().Mode)
					if i == 0 {
						instance = p.Store.Info().InstanceID
					} else {
						require.Equal(t, instance, p.Store.Info().InstanceID)
					}
					closePreparation(t, p)
				}
			})
		}
	}
}

func TestSecurityStartupExplicitLoginRequiresCredentials(t *testing.T) {
	t.Chdir(t.TempDir())
	f := startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "false"})
	p := PrepareSecurity(f)
	defer closePreparation(t, p)
	require.Equal(t, "new", p.Status)
	require.False(t, f.DisableLoginMode)
	require.Equal(t, "DEFAULT", f.Visibility)
	require.False(t, p.Store.Info().Initialized)
}

func TestSecurityStartupPreservesExistingPrivateDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	p := PrepareSecurity(startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "A strong initial administrator phrase 42!"}))
	require.Equal(t, "ready", p.Status, p.Message)
	closePreparation(t, p)
	meta, _, err := readInstallation(".flare-security")
	require.NoError(t, err)
	meta.DefaultPrivate = true // Metadata written by the previous startup policy.
	require.NoError(t, writePrivateJSON(".flare-security/installation.json", meta))
	f := startupFlags(nil)
	p = PrepareSecurity(f)
	defer closePreparation(t, p)
	require.Equal(t, "ready", p.Status, p.Message)
	require.False(t, f.DisableLoginMode)
	require.Equal(t, "PRIVATE", f.Visibility)
}

func TestSecurityStartupLegacyCredentialsAndYAMLRemainExact(t *testing.T) {
	for _, visibility := range []string{"DEFAULT", "PRIVATE"} {
		t.Run(visibility, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeStartupFiles(t, startupPrivate)
			inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_VISIBILITY": visibility, "FLARE_USER": " legacy admin ", "FLARE_PASS": "short"}
			f := startupFlags(inputs)
			p := PrepareSecurity(f)
			require.Equal(t, "ready", p.Status, p.Message)
			require.Equal(t, "deployment", p.Store.Info().Mode)
			require.NoError(t, p.Store.Verify(" legacy admin ", "short"))
			token, e := p.Store.Authenticate(" legacy admin ", "short")
			require.NoError(t, e)
			closePreparation(t, p)
			for _, name := range []string{"apps.yml", "bookmarks.yml", "config.yml"} {
				raw, readErr := os.ReadFile(name)
				require.NoError(t, readErr)
				backup, backupErr := os.ReadFile(filepath.Join(".flare-security", "migration-backups", "initial", name))
				require.NoError(t, backupErr)
				require.Equal(t, raw, backup)
			}
			raw, e := os.ReadFile("bookmarks.yml")
			require.NoError(t, e)
			require.Equal(t, startupPrivate, string(raw))
			p = PrepareSecurity(startupFlags(inputs))
			defer closePreparation(t, p)
			require.Equal(t, "ready", p.Status, p.Message)
			_, e = p.Store.Session(token, false)
			require.NoError(t, e, "ordinary restart must preserve sessions")
		})
	}
}
func TestSecurityStartupTrustedAndPrivateConflict(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(map[bool]string{false: "public", true: "private"}[private], func(t *testing.T) {
			t.Chdir(t.TempDir())
			content := startupPublic
			if private {
				content = startupPrivate
			}
			writeStartupFiles(t, content)
			f := startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "true"})
			p := PrepareSecurity(f)
			defer closePreparation(t, p)
			if private {
				require.Equal(t, "attention_required", p.Status)
				require.False(t, p.Store.Info().Initialized)
			} else {
				require.Equal(t, "ready", p.Status, p.Message)
				require.Equal(t, security.ModeTrusted, p.Store.Info().Mode)
			}
			raw, e := os.ReadFile("bookmarks.yml")
			require.NoError(t, e)
			require.Equal(t, content, string(raw))
		})
	}
}
func TestSecurityStartupInvalidExplicitInputs(t *testing.T) {
	for _, inputs := range []map[string]string{{"FLARE_DISABLE_LOGIN": "bogus"}, {"FLARE_VISIBILITY": "invalid"}, {"FLARE_DISABLE_LOGIN": "true", "FLARE_VISIBILITY": "PRIVATE"}, {"FLARE_CREDENTIAL_MODE": "external"}} {
		t.Run("invalid", func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeStartupFiles(t, startupPublic)
			p := PrepareSecurity(startupFlags(inputs))
			defer closePreparation(t, p)
			require.NotEqual(t, "ready", p.Status)
			_, e := os.Stat(filepath.Join(".flare-security", "complete"))
			require.True(t, os.IsNotExist(e))
		})
	}
}
func TestSecurityStartupDeploymentRotationAndLocalOverride(t *testing.T) {
	for _, mode := range []string{"deployment", "local"} {
		t.Run(mode, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeStartupFiles(t, startupPublic)
			inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "old", "FLARE_CREDENTIAL_MODE": mode}
			p := PrepareSecurity(startupFlags(inputs))
			require.Equal(t, "ready", p.Status, p.Message)
			token, e := p.Store.Authenticate("flare", "old")
			require.NoError(t, e)
			closePreparation(t, p)
			inputs["FLARE_PASS"] = "new"
			p = PrepareSecurity(startupFlags(inputs))
			defer closePreparation(t, p)
			require.Equal(t, "ready", p.Status, p.Message)
			if mode == "deployment" {
				require.NoError(t, p.Store.Verify("flare", "new"))
				_, e = p.Store.Session(token, false)
				require.ErrorIs(t, e, security.ErrInvalidSession)
			} else {
				require.NoError(t, p.Store.Verify("flare", "old"))
				_, e = p.Store.Session(token, false)
				require.NoError(t, e)
			}
		})
	}
}
func TestSecurityStartupStateDamageFailsClosed(t *testing.T) {
	for _, name := range []string{"state.json", "complete", "installation.json"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeStartupFiles(t, startupPublic)
			inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "old"}
			p := PrepareSecurity(startupFlags(inputs))
			require.Equal(t, "ready", p.Status, p.Message)
			closePreparation(t, p)
			require.NoError(t, os.Remove(filepath.Join(".flare-security", name)))
			p = PrepareSecurity(startupFlags(inputs))
			defer closePreparation(t, p)
			require.Equal(t, "faulted", p.Status)
		})
	}
}
func TestSecuritySecretFileExactWhitespace(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, test := range []struct{ raw, want string }{{" password \r\n", " password "}, {"password\n\n", "password\n"}, {"password\r", "password\r"}} {
		require.NoError(t, os.WriteFile("secret", []byte(test.raw), 0600))
		_, p, e := readCredential(startupFlags(map[string]string{"FLARE_PASS_FILE": "secret"}))
		require.NoError(t, e)
		require.Equal(t, test.want, p)
	}
	_, _, e := readCredential(startupFlags(map[string]string{"FLARE_PASS": "plain", "FLARE_PASS_FILE": "secret"}))
	require.Error(t, e)
	require.NoError(t, os.WriteFile("secret", nil, 0600))
	_, _, e = readCredential(startupFlags(map[string]string{"FLARE_PASS_FILE": "secret"}))
	require.Error(t, e)
}
func TestSecuritySnapshotRetryPreservesOriginalBackup(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStartupFiles(t, startupPublic)
	files, _, _, e := inspectContent()
	require.NoError(t, e)
	require.NoError(t, os.Mkdir("state", 0700))
	require.NoError(t, migrationSnapshot("state", files))
	require.NoError(t, migrationSnapshot("state", files))
	files["bookmarks.yml"] = []byte(startupPrivate)
	require.Error(t, migrationSnapshot("state", files))
	raw, e := os.ReadFile("state/migration-backups/initial/bookmarks.yml")
	require.NoError(t, e)
	require.Equal(t, startupPublic, string(raw))
}
func TestSecurityStartupLocalIgnoresRetiredSecret(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStartupFiles(t, startupPublic)
	inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "old", "FLARE_CREDENTIAL_MODE": "local"}
	p := PrepareSecurity(startupFlags(inputs))
	require.Equal(t, "ready", p.Status, p.Message)
	closePreparation(t, p)
	inputs["FLARE_PASS_FILE"] = "missing-retired-file"
	p = PrepareSecurity(startupFlags(inputs))
	defer closePreparation(t, p)
	require.Equal(t, "ready", p.Status, p.Message)
	require.NoError(t, p.Store.Verify("flare", "old"))
}

func TestSecurityStartupLosingEntireCommittedStateDoesNotReinitialize(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStartupFiles(t, startupPublic)
	inputs := map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "old"}
	p := PrepareSecurity(startupFlags(inputs))
	require.Equal(t, "ready", p.Status, p.Message)
	closePreparation(t, p)
	for _, name := range []string{"state.json", "complete"} {
		require.NoError(t, os.Remove(filepath.Join(".flare-security", name)))
	}
	p = PrepareSecurity(startupFlags(inputs))
	defer closePreparation(t, p)
	require.Equal(t, "faulted", p.Status, "committed metadata must detect lost state")
}

func TestSecuritySnapshotRetryPreservesOriginalManifestFileSet(t *testing.T) {
	for _, change := range []string{"remove", "add"} {
		t.Run(change, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeStartupFiles(t, startupPublic)
			if change == "remove" {
				require.NoError(t, os.WriteFile(".flare-cookie-secret", []byte("original-cookie-key"), 0600))
			}
			files, _, _, e := inspectContent()
			require.NoError(t, e)
			require.NoError(t, os.Mkdir("state", 0700))
			require.NoError(t, migrationSnapshot("state", files))
			original, e := os.ReadFile("state/migration-backups/initial/manifest.json")
			require.NoError(t, e)
			if change == "remove" {
				require.NoError(t, os.Remove(".flare-cookie-secret"))
			} else {
				require.NoError(t, os.WriteFile(".flare-cookie-secret", []byte("new-cookie-key"), 0600))
			}
			require.NoError(t, migrationSnapshot("state", files))
			after, e := os.ReadFile("state/migration-backups/initial/manifest.json")
			require.NoError(t, e)
			require.Equal(t, string(original), string(after), "original backup file set must remain immutable")
			retries, e := filepath.Glob("state/migration-backups/retry-*/manifest.json")
			require.NoError(t, e)
			require.Len(t, retries, 1)
		})
	}
}

func TestSecurityMetadataRejectsSpecialSymlinkAndOversizedFiles(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "installation.json")
			switch kind {
			case "fifo":
				require.NoError(t, unix.Mkfifo(path, 0600))
			case "symlink":
				target := filepath.Join(dir, "outside")
				require.NoError(t, os.WriteFile(target, []byte(`{"version":1,"committed":true}`), 0600))
				require.NoError(t, os.Symlink(target, path))
			case "oversized":
				require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte(" "), (64<<10)+1), 0600))
			}
			_, _, e := readInstallation(dir)
			require.Error(t, e)
		})
	}
}

func TestSecurityPasswordFilesRejectFIFOAllowProjectedSymlink(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, unix.Mkfifo("secret-fifo", 0600))
	_, _, e := readCredential(startupFlags(map[string]string{"FLARE_PASS_FILE": "secret-fifo"}))
	require.Error(t, e)
	_, e = readNewPassword("secret-fifo", strings.NewReader(""), io.Discard)
	require.Error(t, e)
	require.NoError(t, os.WriteFile("secret-target", []byte("projected password\n"), 0600))
	require.NoError(t, os.Symlink("secret-target", "secret-link"))
	_, pass, e := readCredential(startupFlags(map[string]string{"FLARE_PASS_FILE": "secret-link"}))
	require.NoError(t, e)
	require.Equal(t, "projected password", pass)
}

func TestTrustedStartupStillRejectsPrivateContentAndDamagedState(t *testing.T) {
	for _, fixture := range []struct{ name, file, content string }{
		{"bookmark", "bookmarks.yml", startupPrivate},
		{"category", "bookmarks.yml", "categories:\n  - id: secret\n    title: Secret\n    private: true\nlinks: []\n"},
		{"application", "apps.yml", "links:\n  - name: secret\n    link: https://secret.example.com\n    private: true\n"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile(fixture.file, []byte(fixture.content), 0600))
			p := PrepareSecurity(startupFlags(nil))
			defer closePreparation(t, p)
			require.Equal(t, "attention_required", p.Status)
			require.False(t, p.Store.Info().Initialized)
		})
	}
	for _, name := range []string{"state.json", "complete", "installation.json"} {
		t.Run("missing/"+name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			p := PrepareSecurity(startupFlags(nil))
			require.Equal(t, "ready", p.Status, p.Message)
			closePreparation(t, p)
			require.NoError(t, os.Remove(filepath.Join(".flare-security", name)))
			p = PrepareSecurity(startupFlags(nil))
			defer closePreparation(t, p)
			require.Equal(t, "faulted", p.Status)
		})
	}
}
