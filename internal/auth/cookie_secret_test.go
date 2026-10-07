package auth

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// requireGeneratedCookieSecret verifies the persisted format retains 32 random bytes.
func requireGeneratedCookieSecret(t *testing.T, secret string) {
	t.Helper()
	require.Len(t, secret, minimumCookieSecretLength*2)
	decoded, err := hex.DecodeString(secret)
	require.NoError(t, err)
	require.Len(t, decoded, minimumCookieSecretLength)
}

// TestGenerateCookieSecret verifies encoding and failures without replacing global randomness.
func TestGenerateCookieSecret(t *testing.T) {
	raw := bytes.Repeat([]byte{0xa5}, minimumCookieSecretLength)
	secret, err := generateCookieSecret(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(raw), secret)

	secret, err = generateCookieSecret(strings.NewReader("insufficient randomness"))
	require.Error(t, err)
	assert.Empty(t, secret)
}

// TestInitializeCookieSecret_ExplicitConfiguration preserves sufficient custom keys verbatim.
func TestInitializeCookieSecret_ExplicitConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, configured := range []string{
		strings.Repeat("x", minimumCookieSecretLength),
		" \t" + strings.Repeat("x", minimumCookieSecretLength) + "\n",
		strings.Repeat("密", 11), // The minimum measures bytes, not characters.
	} {
		secret, err := initializeCookieSecret(configured)
		require.NoError(t, err)
		assert.Equal(t, configured, secret)
		_, err = os.Stat(cookieSecretFileName)
		require.True(t, os.IsNotExist(err), "explicit keys should not create persistence files")
	}

	stored := strings.Repeat("ab", minimumCookieSecretLength)
	require.NoError(t, os.WriteFile(cookieSecretFileName, []byte(stored), 0600))
	secret, err := initializeCookieSecret(testCookieSecret)
	require.NoError(t, err)
	assert.Equal(t, testCookieSecret, secret)
	content, err := os.ReadFile(cookieSecretFileName)
	require.NoError(t, err)
	assert.Equal(t, stored, string(content), "explicit configuration takes precedence without rewriting the file")
}

// TestInitializeCookieSecret_UnsafeConfiguration creates a key once and reuses it after restart.
func TestInitializeCookieSecret_UnsafeConfiguration(t *testing.T) {
	for _, configured := range []string{
		"", " \t\n", define.DEFAULT_COOKIE_SECRET,
		" \t" + define.DEFAULT_COOKIE_SECRET + "\n",
		strings.Repeat("x", minimumCookieSecretLength-1),
		" \t" + strings.Repeat("x", minimumCookieSecretLength-1) + "\n",
	} {
		t.Run(configured, func(t *testing.T) {
			t.Chdir(t.TempDir())
			secret, err := initializeCookieSecret(configured)
			require.NoError(t, err)
			requireGeneratedCookieSecret(t, secret)
			assert.NotEqual(t, strings.TrimSpace(configured), secret)

			content, err := os.ReadFile(cookieSecretFileName)
			require.NoError(t, err)
			assert.Equal(t, secret, string(content))
			info, err := os.Stat(cookieSecretFileName)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

			restarted, err := initializeCookieSecret(configured)
			require.NoError(t, err)
			assert.Equal(t, secret, restarted, "restarting should retain login sessions")
			artifacts, err := filepath.Glob(cookieSecretFileName + "-*")
			require.NoError(t, err)
			assert.Empty(t, artifacts, "temporary key files should be removed")
		})
	}
}

// TestInitializeCookieSecret_PersistenceFailure keeps login available without trusting a directory.
func TestInitializeCookieSecret_PersistenceFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir(cookieSecretFileName, 0700))
	first, err := initializeCookieSecret(define.DEFAULT_COOKIE_SECRET)
	require.NoError(t, err)
	requireGeneratedCookieSecret(t, first)
	second, err := initializeCookieSecret(define.DEFAULT_COOKIE_SECRET)
	require.NoError(t, err)
	requireGeneratedCookieSecret(t, second)
	assert.NotEqual(t, first, second, "without persistence each process must have a fresh key")
	info, err := os.Stat(cookieSecretFileName)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "fallback must not replace an existing directory")
}

// TestInitializeCookieSecret_CorruptPersistence preserves invalid files and uses a fresh safe key.
func TestInitializeCookieSecret_CorruptPersistence(t *testing.T) {
	for _, stored := range []string{
		define.DEFAULT_COOKIE_SECRET,
		strings.Repeat("z", minimumCookieSecretLength*2),
		strings.Repeat("ab", minimumCookieSecretLength) + "\n",
		strings.Repeat("a", minimumCookieSecretLength*2-1),
	} {
		t.Run(stored, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile(cookieSecretFileName, []byte(stored), 0600))
			secret, err := initializeCookieSecret("too-short")
			require.NoError(t, err)
			requireGeneratedCookieSecret(t, secret)
			assert.NotEqual(t, stored, secret)
			content, err := os.ReadFile(cookieSecretFileName)
			require.NoError(t, err)
			assert.Equal(t, stored, string(content), "damaged persistence should remain untouched")
		})
	}
}

// TestLoadOrCreateCookieSecret_SaveFailure does not report an unwritten key as persistent.
func TestLoadOrCreateCookieSecret_SaveFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-directory", cookieSecretFileName)
	secret, created, err := loadOrCreateCookieSecret(path)
	require.Error(t, err)
	assert.Empty(t, secret)
	assert.False(t, created)
}

// TestLoadOrCreateCookieSecret_RejectsSymlink avoids reading or changing a file outside the data directory.
func TestLoadOrCreateCookieSecret_RejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "other-key")
	stored := strings.Repeat("ab", minimumCookieSecretLength)
	require.NoError(t, os.WriteFile(target, []byte(stored), 0644))
	path := filepath.Join(dir, cookieSecretFileName)
	require.NoError(t, os.Symlink(target, path))
	secret, created, err := loadOrCreateCookieSecret(path)
	require.Error(t, err)
	assert.Empty(t, secret)
	assert.False(t, created)
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm(), "symlink targets must remain untouched")
}

// TestLoadOrCreateCookieSecret_RestrictsExistingPermissions protects a previously stored key.
func TestLoadOrCreateCookieSecret_RestrictsExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), cookieSecretFileName)
	stored := strings.Repeat("ab", minimumCookieSecretLength)
	require.NoError(t, os.WriteFile(path, []byte(stored), 0644))
	secret, created, err := loadOrCreateCookieSecret(path)
	require.NoError(t, err)
	assert.Equal(t, stored, secret)
	assert.False(t, created)
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

// TestLoadOrCreateCookieSecret_ConcurrentInitialization verifies every process adopts the winning key.
func TestLoadOrCreateCookieSecret_ConcurrentInitialization(t *testing.T) {
	const workers = 32
	path := filepath.Join(t.TempDir(), cookieSecretFileName)
	type result struct {
		secret  string
		created bool
		err     error
	}
	results := make(chan result, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			secret, created, err := loadOrCreateCookieSecret(path)
			results <- result{secret: secret, created: created, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	requireGeneratedCookieSecret(t, string(content))
	createdCount := 0
	for result := range results {
		require.NoError(t, result.err)
		assert.Equal(t, string(content), result.secret)
		if result.created {
			createdCount++
		}
	}
	assert.Equal(t, 1, createdCount, "only the process publishing the key should report creation")
	artifacts, err := filepath.Glob(path + "-*")
	require.NoError(t, err)
	assert.Empty(t, artifacts)
}
