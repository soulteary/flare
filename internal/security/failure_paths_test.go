package security

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreRejectsUnsafeStateAndJournal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"directory symlink", func(t *testing.T, dir string) { require.NoError(t, os.Symlink(t.TempDir(), dir)) }},
		{"regular file", func(t *testing.T, dir string) { require.NoError(t, os.WriteFile(dir, []byte("x"), 0600)) }},
		{"lock symlink", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(dir, 0700))
			require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "lock"), filepath.Join(dir, "instance.lock")))
		}},
		{"journal directory", func(t *testing.T, dir string) {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "commit.json"), 0700))
		}},
		{"bad journal checksum", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(dir, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "commit.json"), []byte(`{"Checksum":"wrong","State":{}}`), 0600))
		}},
		{"invalid journal state", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(dir, 0700))
			raw := json.RawMessage(`{"Version":2,"InstanceID":"id"}`)
			b, err := json.Marshal(envelope{digest(string(raw)), raw})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "commit.json"), b, 0600))
		}},
		{"mismatched journal marker", func(t *testing.T, dir string) {
			require.NoError(t, os.Mkdir(dir, 0700))
			raw := json.RawMessage(`{"Version":1,"InstanceID":"id"}`)
			b, err := json.Marshal(envelope{digest(string(raw)), raw})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "commit.json"), b, 0600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "complete"), []byte("other"), 0600))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			tc.setup(t, dir)
			store, err := Open(dir)
			if store != nil {
				defer store.Close()
			}
			require.Error(t, err)
		})
	}
	for _, mutate := range []func(*state){
		func(s *state) { s.Version = 2 }, func(s *state) { s.Sessions = nil }, func(s *state) { s.Mode = "unknown" }, func(s *state) { s.Password.Hash = nil },
	} {
		t.Run("invalid persisted state", func(t *testing.T) {
			s, dir := fixture(t)
			next := s.clone()
			mutate(&next)
			raw, err := json.Marshal(next)
			require.NoError(t, err)
			b, err := json.Marshal(envelope{digest(string(raw)), raw})
			require.NoError(t, err)
			require.NoError(t, s.Close())
			require.NoError(t, os.WriteFile(filepath.Join(dir, "state.json"), b, 0600))
			got, err := Open(dir)
			if got != nil {
				defer got.Close()
			}
			require.ErrorIs(t, err, ErrFaulted)
		})
	}
}

func TestStoreCredentialFailuresDoNotRevokeValidSession(t *testing.T) {
	s, _ := fixture(t)
	token, err := s.CreateSession()
	require.NoError(t, err)
	require.Error(t, s.Initialize("other", "password", "local"))
	require.Error(t, s.SetCredentials("", "password", "local"))
	require.ErrorIs(t, s.ChangePassword("wrong", "a unique replacement phrase 42"), ErrInvalidCredentials)
	require.ErrorIs(t, s.ChangePassword("old", "short"), ErrPasswordPolicy)
	require.ErrorIs(t, s.ResetPassword("short", false), ErrPasswordPolicy)
	_, err = s.ChangePasswordAndCreateSession(token, "old", "short")
	require.ErrorIs(t, err, ErrPasswordPolicy)
	_, err = s.ChangePasswordAndCreateSession("invalid", "old", "a unique replacement phrase 42")
	require.ErrorIs(t, err, ErrInvalidSession)
	_, err = s.ChangePasswordAndCreateSession(token, "wrong", "a unique replacement phrase 42")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = s.TakeLocalControl(token, "old")
	require.Error(t, err)
	_, err = s.Reauthenticate("invalid", "old")
	require.ErrorIs(t, err, ErrInvalidSession)
	_, err = s.Reauthenticate(token, "wrong")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = s.Session(token, false)
	require.NoError(t, err)
	require.NoError(t, s.Verify(" admin ", "old"))
	require.True(t, (Session{VerifiedAt: time.Now()}).Recent())
	require.False(t, (Session{VerifiedAt: time.Now().Add(-2 * ReauthTimeout)}).Recent())
	require.NoError(t, s.SetCredentials(" admin ", "old", "deployment"))
	_, err = s.ChangePasswordAndCreateSession(token, "old", "a unique replacement phrase 42")
	require.ErrorIs(t, err, ErrDeploymentManaged)
}

func TestStoreClosedAndUninitializedFailClosed(t *testing.T) {
	s, err := Open(t.TempDir())
	require.NoError(t, err)
	require.Error(t, s.SetCredentials("admin", "password", "local"))
	require.Error(t, s.Initialize("admin", "", ModeTrusted))
	require.Error(t, s.Initialize("", "password", "local"))
	require.ErrorIs(t, s.Verify("admin", "wrong"), ErrInvalidCredentials)
	_, err = s.Authenticate("admin", "wrong")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	require.NoError(t, s.Initialize("admin", "password", "local"))
	require.NoError(t, s.Close())
	require.ErrorIs(t, s.Verify("admin", "password"), ErrFaulted)
	_, err = s.Authenticate("admin", "password")
	require.ErrorIs(t, err, ErrFaulted)
	_, err = s.CreateSession()
	require.ErrorIs(t, err, ErrFaulted)
	require.ErrorIs(t, s.RevokeAll(), ErrFaulted)
	require.ErrorIs(t, s.CheckWritable(), ErrFaulted)
	called := false
	require.ErrorIs(t, s.WithAuthorizedWrite("", true, func() error { called = true; return nil }), ErrFaulted)
	require.False(t, called)
}

func TestStoreSessionCapacityAndExpiredCleanup(t *testing.T) {
	s, _ := fixture(t)
	now := s.data.HighWater
	s.now = func() time.Time { return now }
	for i := 0; i < 4096; i++ {
		s.data.Sessions[fmt.Sprint(i)] = Session{CreatedAt: now, LastActive: now, ExpiresAt: now.Add(time.Hour), Generation: s.data.Generation}
	}
	_, err := s.CreateSession()
	require.ErrorIs(t, err, ErrBusy)
	expired := s.data.Sessions["0"]
	expired.LastActive = now.Add(-IdleTimeout)
	s.data.Sessions["0"] = expired
	token, err := s.CreateSession()
	require.NoError(t, err)
	require.NotContains(t, s.data.Sessions, "0")
	_, err = s.Session(token, false)
	require.NoError(t, err)
}

func TestStoreWriteProbeAndExpiredSessionPersistenceFailures(t *testing.T) {
	t.Run("missing state directory", func(t *testing.T) {
		s, dir := fixture(t)
		require.NoError(t, os.Rename(dir, dir+"-moved"))
		t.Cleanup(func() { require.NoError(t, os.Rename(dir+"-moved", dir)) })
		require.ErrorIs(t, s.CheckWritable(), ErrFaulted)
		require.ErrorIs(t, s.Verify(" admin ", "old"), ErrFaulted)
	})
	t.Run("expired cleanup cannot persist", func(t *testing.T) {
		s, _ := fixture(t)
		token, err := s.CreateSession()
		require.NoError(t, err)
		now := s.data.HighWater.Add(IdleTimeout)
		s.now = func() time.Time { return now }
		s.write = func(string, []byte) error { return errors.New("disk failure") }
		_, err = s.Session(token, false)
		require.ErrorIs(t, err, ErrFaulted)
	})
	t.Run("oversized state", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "state.json")
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("x", (8<<20)+1)), 0600))
		_, err := readStateFile(path)
		require.ErrorIs(t, err, ErrFaulted)
	})
}

func TestJournalReplayFailsClosedWhenDestinationsAreBlocked(t *testing.T) {
	for _, name := range []string{"state.json", "complete"} {
		t.Run(name, func(t *testing.T) {
			s, dir := fixture(t)
			raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
			require.NoError(t, err)
			require.NoError(t, s.Close())
			require.NoError(t, os.WriteFile(filepath.Join(dir, "commit.json"), raw, 0600))
			require.NoError(t, os.Remove(filepath.Join(dir, name)))
			require.NoError(t, os.Mkdir(filepath.Join(dir, name), 0700))
			opened, err := Open(dir)
			if opened != nil {
				defer opened.Close()
			}
			require.ErrorIs(t, err, ErrFaulted)
			journal, err := os.ReadFile(filepath.Join(dir, "commit.json"))
			require.NoError(t, err)
			require.Equal(t, raw, journal, "failed replay must preserve commit intent")
		})
	}
}

func TestAuthenticationCapacityRejectsWithoutChangingCredentials(t *testing.T) {
	s, _ := fixture(t)
	for i := 0; i < cap(hashWaiters); i++ {
		hashWaiters <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(hashWaiters); i++ {
			<-hashWaiters
		}
	}()
	require.ErrorIs(t, s.Verify(" admin ", "old"), ErrBusy)
	_, err := s.Authenticate(" admin ", "old")
	require.ErrorIs(t, err, ErrBusy)
	require.ErrorIs(t, s.SetCredentials(" admin ", "new", "local"), ErrBusy)
	require.Equal(t, "local", s.Info().Mode)
	require.Empty(t, s.data.Sessions)
	fresh, err := Open(t.TempDir())
	require.NoError(t, err)
	defer fresh.Close()
	require.ErrorIs(t, fresh.Initialize("admin", "password", "local"), ErrBusy)
	require.False(t, fresh.Info().Initialized)
	for i := 0; i < cap(authenticationSlots); i++ {
		authenticationSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(authenticationSlots); i++ {
			<-authenticationSlots
		}
	}()
	_, err = s.Authenticate(" admin ", "old")
	require.ErrorIs(t, err, ErrBusy)
}

func TestStateChecksumAndLockDestinationFailures(t *testing.T) {
	t.Run("state checksum", func(t *testing.T) {
		s, dir := fixture(t)
		require.NoError(t, s.Close())
		require.NoError(t, os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"Checksum":"wrong","State":{}}`), 0600))
		opened, err := Open(dir)
		if opened != nil {
			defer opened.Close()
		}
		require.ErrorIs(t, err, ErrFaulted)
	})
	t.Run("parent file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(path, []byte("blocked"), 0600))
		_, err := Open(filepath.Join(path, "state"))
		require.Error(t, err)
	})
	t.Run("missing directory during write", func(t *testing.T) {
		s, dir := fixture(t)
		require.NoError(t, os.Rename(dir, dir+"-saved"))
		t.Cleanup(func() { require.NoError(t, os.Rename(dir+"-saved", dir)) })
		require.Error(t, s.atomicWrite("state.json", []byte("new")))
	})
}
