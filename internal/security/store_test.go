package security

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	if e = s.Initialize(" admin ", "old", "local"); e != nil {
		t.Fatal(e)
	}
	return s, dir
}
func TestPersistenceAndLock(t *testing.T) {
	s, dir := fixture(t)
	if _, e := Open(dir); !errors.Is(e, ErrLocked) {
		t.Fatalf("lock: %v", e)
	}
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	id := s.Info().InstanceID
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.Info().InstanceID != id {
		t.Fatal("identity changed")
	}
	if e = s.Verify(" admin ", "old"); e != nil {
		t.Fatal(e)
	}
	if e = s.Verify("admin", "old"); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal(e)
	}
	if _, e = s.Session(token, false); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "state.json"))
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), `"old"`) {
		t.Fatal("plaintext persisted")
	}
	for _, name := range []string{"state.json", "complete", "instance.lock"} {
		info, e := os.Stat(filepath.Join(dir, name))
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("file mode %s", name)
		}
	}
}
func TestSessionLifetimesAndRotation(t *testing.T) {
	s, _ := fixture(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	v, _ := s.Session(token, false)
	expiry := v.ExpiresAt
	now = now.Add(11 * time.Minute)
	rotated, e := s.Reauthenticate(token, "old")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Session(token, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal("old token valid")
	}
	v, e = s.Session(rotated, true)
	if e != nil || !v.ExpiresAt.Equal(expiry) || !v.VerifiedAt.Equal(now) {
		t.Fatal("rotation changed deadline")
	}
	now = now.Add(IdleTimeout)
	if _, e = s.Session(rotated, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal("idle expiry failed")
	}
	now = now.Add(-10 * time.Second)
	if _, e = s.Session(rotated, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal("expired session revived")
	}
}
func TestRevocationAndCredentialModes(t *testing.T) {
	s, _ := fixture(t)
	a, _ := s.CreateSession()
	b, _ := s.CreateSession()
	if e := s.Revoke(a); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Session(b, false); e != nil {
		t.Fatal(e)
	}
	if e := s.SetCredentials(" admin ", "old", "local"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Session(b, false); e != nil {
		t.Fatal("same credentials revoked sessions")
	}
	if e := s.ChangePassword("old", "A long unusual password 987!"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Session(b, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal("password did not revoke")
	}
	if e := s.SetCredentials("admin", "short", "deployment"); e != nil {
		t.Fatal(e)
	}
	if e := s.ChangePassword("short", "A long unusual password 987!"); !errors.Is(e, ErrDeploymentManaged) {
		t.Fatal(e)
	}
	if e := s.ResetPassword("A long unusual password 987!", false); !errors.Is(e, ErrDeploymentManaged) {
		t.Fatal(e)
	}
	if e := s.ResetPassword("A long unusual password 987!", true); e != nil {
		t.Fatal(e)
	}
	if s.Info().Mode != "local" {
		t.Fatal("not local")
	}
}
func TestDamageFailsClosed(t *testing.T) {
	for _, name := range []string{"state.json", "complete"} {
		t.Run(name, func(t *testing.T) {
			s, dir := fixture(t)
			s.Close()
			os.Remove(filepath.Join(dir, name))
			if _, e := Open(dir); !errors.Is(e, ErrFaulted) {
				t.Fatal(e)
			}
		})
	}
	t.Run("corrupt", func(t *testing.T) {
		s, dir := fixture(t)
		s.Close()
		os.WriteFile(filepath.Join(dir, "state.json"), []byte("{}"), 0600)
		if _, e := Open(dir); !errors.Is(e, ErrFaulted) {
			t.Fatal(e)
		}
	})
}
func TestCommitRecoveryAndFailure(t *testing.T) {
	for _, point := range []string{"commit.json", "state.json"} {
		t.Run(point, func(t *testing.T) {
			s, dir := fixture(t)
			token, _ := s.CreateSession()
			s.write = func(name string, b []byte) error {
				if name == point {
					return errors.New("injected")
				}
				return s.atomicWrite(name, b)
			}
			e := s.ChangePassword("old", "A long unusual password 987!")
			if !errors.Is(e, ErrFaulted) {
				t.Fatal(e)
			}
			if _, e = s.Session(token, false); !errors.Is(e, ErrFaulted) {
				t.Fatal("continued after write failure")
			}
			s.Close()
			s, e = Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer s.Close()
			if point == "commit.json" {
				if e = s.Verify(" admin ", "old"); e != nil {
					t.Fatal(e)
				}
			} else {
				if e = s.Verify(" admin ", "A long unusual password 987!"); e != nil {
					t.Fatal(e)
				}
				if _, e = s.Session(token, false); !errors.Is(e, ErrInvalidSession) {
					t.Fatal("recovered old session")
				}
			}
		})
	}
}
func TestClockRollbackRevokesAcrossRestart(t *testing.T) {
	s, dir := fixture(t)
	token, _ := s.CreateSession()
	high := s.data.HighWater
	s.now = func() time.Time { return high.Add(-61 * time.Second) }
	if _, e := s.Session(token, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal(e)
	}
	s.Close()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Session(token, false); !errors.Is(e, ErrInvalidSession) {
		t.Fatal("revived across restart")
	}
}
func TestPasswordPolicy(t *testing.T) {
	for _, p := range []string{"short", "123456789012345", strings.Repeat("字", 129), string([]byte{0xff})} {
		if !errors.Is(ValidatePassword(p), ErrPasswordPolicy) {
			t.Fatal("accepted invalid")
		}
	}
	if e := ValidatePassword("  多语言口令应当保留空格并通过验证  "); e != nil {
		t.Fatal(e)
	}
}

func TestTrustedStateAndUpgradeToAdministrator(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Initialize("", "", ModeTrusted); e != nil {
		t.Fatal(e)
	}
	if !s.Info().Initialized || s.Info().Mode != ModeTrusted {
		t.Fatal("trusted state not initialized")
	}
	if _, e = s.CreateSession(); !errors.Is(e, ErrInvalidCredentials) {
		t.Fatal("trusted state acquired identity")
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.SetCredentials("flare", "legacy", "deployment"); e != nil {
		t.Fatal(e)
	}
	if e = s.Verify("flare", "legacy"); e != nil {
		t.Fatal(e)
	}
}
func TestAbsoluteExpiryDespiteActivity(t *testing.T) {
	s, _ := fixture(t)
	now := s.data.HighWater
	s.now = func() time.Time { return now }
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 36; i++ {
		now = now.Add(20 * time.Minute)
		_, e = s.Session(token, true)
		if i < 35 && e != nil {
			t.Fatal(e)
		}
	}
	if !errors.Is(e, ErrInvalidSession) {
		t.Fatal("activity extended absolute expiry")
	}
}
func TestHashQueueBound(t *testing.T) {
	s, _ := fixture(t)
	for i := 0; i < cap(hashWaiters); i++ {
		hashWaiters <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(hashWaiters); i++ {
			<-hashWaiters
		}
	}()
	if e := s.Verify(" admin ", "old"); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
}
func TestAtomicPasswordAndReplacementSession(t *testing.T) {
	s, _ := fixture(t)
	a, _ := s.Authenticate(" admin ", "old")
	b, _ := s.Authenticate(" admin ", "old")
	next, e := s.ChangePasswordAndCreateSession(a, "old", "A long unusual password 987!")
	if e != nil {
		t.Fatal(e)
	}
	for _, old := range []string{a, b} {
		if _, e = s.Session(old, false); !errors.Is(e, ErrInvalidSession) {
			t.Fatal("old token survived")
		}
	}
	if _, e = s.Session(next, false); e != nil {
		t.Fatal(e)
	}
	if e = s.Verify(" admin ", "A long unusual password 987!"); e != nil {
		t.Fatal(e)
	}
}
func TestStateSymlinksRejected(t *testing.T) {
	s, dir := fixture(t)
	s.Close()
	path := filepath.Join(dir, "state.json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	other := filepath.Join(t.TempDir(), "state")
	os.WriteFile(other, raw, 0600)
	os.Remove(path)
	os.Symlink(other, path)
	if _, e = Open(dir); !errors.Is(e, ErrFaulted) {
		t.Fatal(e)
	}
}
func TestVersionedCommonPasswordDenylistAndPatterns(t *testing.T) {
	for _, password := range []string{"PASSWORDPASSWORD", "administrator12345", "12345678901234567890", "aaaaaaaaaaaaaaaaaaaa", "abcdabcdabcdabcd", "口令口令口令口令口令口令口令口令"} {
		requirePolicyError := ValidatePassword(password)
		if !errors.Is(requirePolicyError, ErrPasswordPolicy) {
			t.Fatalf("accepted known weak pattern %q", password)
		}
	}
	if e := ValidatePassword("uncommon sunlit forest 5728"); e != nil {
		t.Fatal(e)
	}
}

func TestAuthorizedWriteOrdersRevocation(t *testing.T) {
	s, _ := fixture(t)
	token, e := s.Authenticate(" admin ", "old")
	if e != nil {
		t.Fatal(e)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.WithAuthorizedWrite(token, false, func() error { close(entered); <-release; return nil })
	}()
	<-entered
	revoked := make(chan error, 1)
	go func() { revoked <- s.RevokeAll() }()
	select {
	case revokeErr := <-revoked:
		t.Fatalf("revocation passed in-flight authorized commit: %v", revokeErr)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if writeErr := <-done; writeErr != nil {
		t.Fatal(writeErr)
	}
	if revokeErr := <-revoked; revokeErr != nil {
		t.Fatal(revokeErr)
	}
	ran := false
	if e = s.WithAuthorizedWrite(token, false, func() error { ran = true; return nil }); !errors.Is(e, ErrInvalidSession) || ran {
		t.Fatal("revoked write was committed")
	}
}
