package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

// Exercise durable startup boundaries in a disposable test process. The test
// invokes production phase functions; there is no production crash switch.
func TestSecurityMigrationKilledProcess(t *testing.T) {
	if dir := os.Getenv("FLARE_TEST_STARTUP_CRASH_DIR"); dir != "" {
		require.NoError(t, os.Chdir(dir))
		s, e := security.Open(".flare-security")
		require.NoError(t, e)
		files, _, _, e := inspectContent()
		require.NoError(t, e)
		require.NoError(t, migrationSnapshot(".flare-security", files))
		stage := os.Getenv("FLARE_TEST_STARTUP_CRASH_STAGE")
		if stage != "snapshot" {
			require.NoError(t, writePrivateJSON(filepath.Join(".flare-security", "installation.json"), installation{Version: 1}))
		}
		if stage == "initialized" {
			require.NoError(t, s.Initialize("flare", "short", "deployment"))
		}
		fmt.Println("startup-checkpoint " + s.Info().InstanceID)
		for {
			time.Sleep(time.Hour)
		}
	}
	for _, stage := range []string{"snapshot", "metadata", "initialized"} {
		t.Run(stage, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			writeStartupFiles(t, startupPrivate)
			require.NoError(t, os.WriteFile(".flare-cookie-secret", []byte("existing-old-key"), 0600))
			child := exec.Command(os.Args[0], "-test.run=^TestSecurityMigrationKilledProcess$")
			child.Env = append(os.Environ(), "FLARE_TEST_STARTUP_CRASH_DIR="+dir, "FLARE_TEST_STARTUP_CRASH_STAGE="+stage)
			pipe, e := child.StdoutPipe()
			require.NoError(t, e)
			child.Stderr = os.Stderr
			require.NoError(t, child.Start())
			checkpoint := make(chan string, 1)
			go func() {
				sc := bufio.NewScanner(pipe)
				for sc.Scan() {
					if strings.HasPrefix(sc.Text(), "startup-checkpoint ") {
						checkpoint <- strings.TrimPrefix(sc.Text(), "startup-checkpoint ")
						return
					}
				}
				checkpoint <- "not-reached"
			}()
			var id string
			select {
			case id = <-checkpoint:
			case <-time.After(20 * time.Second):
				child.Process.Kill()
				child.Wait()
				t.Fatal("startup checkpoint timed out")
			}
			require.NotEqual(t, "not-reached", id)
			require.NoError(t, child.Process.Signal(syscall.SIGKILL))
			require.Error(t, child.Wait())
			original, e := os.ReadFile(".flare-security/migration-backups/initial/manifest.json")
			require.NoError(t, e)
			p := PrepareSecurity(startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": "short"}))
			defer closePreparation(t, p)
			require.Equal(t, "ready", p.Status, p.Message)
			if id != "" {
				require.Equal(t, id, p.Store.Info().InstanceID, "restart created another administrator instance")
			}
			require.NoError(t, p.Store.Verify("flare", "short"))
			after, e := os.ReadFile(".flare-security/migration-backups/initial/manifest.json")
			require.NoError(t, e)
			require.Equal(t, original, after)
			yaml, e := os.ReadFile("bookmarks.yml")
			require.NoError(t, e)
			require.Equal(t, startupPrivate, string(yaml))
			oldkey, e := os.ReadFile(".flare-cookie-secret")
			require.NoError(t, e)
			require.Equal(t, "existing-old-key", string(oldkey))
		})
	}
}
