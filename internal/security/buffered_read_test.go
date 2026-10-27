package security

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBufferedReadMergesDurableReadTransactions(t *testing.T) {
	for _, mode := range []string{"ordinary", "buffered"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := fixture(t)
			token, authErr := s.CreateSession()
			if authErr != nil {
				t.Fatal(authErr)
			}
			wall := s.data.HighWater.Add(time.Second)
			observations, transactions, atomicFiles := 0, 0, 0
			s.now = func() time.Time { observations++; return wall }
			s.write = func(name string, b []byte) error {
				atomicFiles++
				if name == "commit.json" {
					transactions++
				}
				return s.atomicWrite(name, b)
			}
			var proof *SessionProof
			var beginErr error
			if mode == "buffered" {
				_, proof, beginErr = s.BeginBufferedRead(token)
			} else {
				_, proof, beginErr = s.SessionSnapshot(token, false)
			}
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			for i := 0; i < 20; i++ {
				if _, guardErr := s.GuardSession(proof); guardErr != nil {
					t.Fatal(guardErr)
				}
			}
			wall = wall.Add(time.Second)
			if _, finishErr := s.Session(token, true); finishErr != nil {
				t.Fatal(finishErr)
			}
			expected := 2
			if mode == "buffered" {
				expected = 1
			}
			if transactions != expected || atomicFiles != 2*expected || observations != expected {
				t.Fatalf("transactions=%d atomic-files=%d wall-observations=%d; want %d/%d/%d", transactions, atomicFiles, observations, expected, 2*expected, expected)
			}
			t.Logf("%s: %d durable transactions, %d atomic-file writes, %d calendar observations", mode, transactions, atomicFiles, observations)
		})
	}
}

func TestBufferedReadFailureDoesNotExtendIdleAndRestartKeepsClock(t *testing.T) {
	s, dir := fixture(t)
	token, createErr := s.CreateSession()
	if createErr != nil {
		t.Fatal(createErr)
	}
	original := s.data.Sessions[digest(token)].LastActive
	wall := s.data.HighWater.Add(5 * time.Second)
	observations := 0
	s.now = func() time.Time { observations++; return wall }
	_, proof, beginErr := s.BeginBufferedRead(token)
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	proof.started = time.Now().Add(-10 * time.Minute)
	if _, guardErr := s.GuardSession(proof); guardErr != nil {
		t.Fatal(guardErr)
	}
	if observations != 0 {
		t.Fatal("tentative guard observed a calendar time")
	}
	// Rendering failed: the caller still finishes durably, with touch=false.
	if _, finishErr := s.Session(token, false); finishErr != nil {
		t.Fatal(finishErr)
	}
	if !s.data.Sessions[digest(token)].LastActive.Equal(original) || !s.data.HighWater.Equal(wall) {
		t.Fatal("failed read touched idle or lost final high-water mark")
	}
	s.Close()
	reopened, openErr := Open(dir)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer reopened.Close()
	if !reopened.data.Sessions[digest(token)].LastActive.Equal(original) || !reopened.data.HighWater.Equal(wall) {
		t.Fatal("restart changed durable failed-read result")
	}
}

func TestBufferedReadFinalDecisionRejectsRevocationExpiryRollbackAndFault(t *testing.T) {
	for _, failure := range []string{"revoked", "idle-expired", "absolute-expired", "clock-rollback", "missing-marker", "disk-failure"} {
		t.Run(failure, func(t *testing.T) {
			s, dir := fixture(t)
			token, createErr := s.CreateSession()
			if createErr != nil {
				t.Fatal(createErr)
			}
			wall := s.data.HighWater.Add(time.Second)
			s.now = func() time.Time { return wall }
			_, proof, beginErr := s.BeginBufferedRead(token)
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			switch failure {
			case "revoked":
				if revokeErr := s.RevokeAll(); revokeErr != nil {
					t.Fatal(revokeErr)
				}
			case "idle-expired":
				wall = s.data.Sessions[digest(token)].LastActive.Add(IdleTimeout)
			case "absolute-expired":
				wall = s.data.Sessions[digest(token)].ExpiresAt
			case "clock-rollback":
				wall = s.data.HighWater.Add(-61 * time.Second)
			case "missing-marker":
				if removeErr := os.Remove(filepath.Join(dir, "complete")); removeErr != nil {
					t.Fatal(removeErr)
				}
			case "disk-failure":
				s.write = func(string, []byte) error { return syscall.ENOSPC }
			}
			// The proof is tentative; its valid return cannot authorize body release.
			_, finishErr := s.Session(token, true)
			expected := ErrInvalidSession
			if failure == "missing-marker" || failure == "disk-failure" {
				expected = ErrFaulted
			}
			if !errors.Is(finishErr, expected) {
				t.Fatalf("final %s decision: %v", failure, finishErr)
			}
			if _, guardErr := s.GuardSession(proof); !errors.Is(guardErr, expected) {
				t.Fatalf("proof survived final %s: %v", failure, guardErr)
			}
			if failure == "idle-expired" || failure == "absolute-expired" || failure == "clock-rollback" {
				if _, exists := s.data.Sessions[digest(token)]; exists {
					t.Fatal("final expiry or rollback did not durably remove the session")
				}
			}
		})
	}
}

func TestBufferedReadUsesLiveDurableStateWithoutEarlyCommit(t *testing.T) {
	s, _ := fixture(t)
	token, createErr := s.CreateSession()
	if createErr != nil {
		t.Fatal(createErr)
	}
	calls := 0
	s.now = func() time.Time { calls++; return s.data.HighWater.Add(time.Second) }
	_, invalidProof, invalidErr := s.BeginBufferedRead("not-a-session")
	if !errors.Is(invalidErr, ErrInvalidSession) || invalidProof != nil || calls != 0 {
		t.Fatal("invalid identity must retain the ordinary fallback without observing time")
	}
	_, proof, beginErr := s.BeginBufferedRead(token)
	if beginErr != nil {
		t.Fatal(beginErr)
	}
	next := s.clone()
	next.HighWater = next.Sessions[digest(token)].ExpiresAt
	if commitErr := s.commit(next); commitErr != nil {
		t.Fatal(commitErr)
	}
	writes := 0
	s.write = func(name string, b []byte) error { writes++; return s.atomicWrite(name, b) }
	if _, guardErr := s.GuardSession(proof); !errors.Is(guardErr, ErrInvalidSession) {
		t.Fatal(guardErr)
	}
	if writes != 0 || calls != 0 {
		t.Fatal("tentative durable-expiry guard performed an early transaction or calendar observation")
	}
	if _, finishErr := s.Session(token, false); !errors.Is(finishErr, ErrInvalidSession) {
		t.Fatal(finishErr)
	}
	if writes != 2 || calls != 1 {
		t.Fatal("final decision did not persist expiry exactly once")
	}
}

// The test-only process is killed after a tentative render point. No new clock
// was observed and no response was authorized, so the durable state must remain
// byte-identical. Production contains no crash environment switch.
func TestBufferedReadKilledTentativeProcess(t *testing.T) {
	if dir := os.Getenv("FLARE_TEST_BUFFERED_DIR"); dir != "" {
		s, openErr := Open(dir)
		if openErr != nil {
			t.Fatal(openErr)
		}
		s.now = func() time.Time { t.Fatal("tentative render observed a calendar time"); return time.Time{} }
		s.write = func(string, []byte) error { t.Fatal("tentative render wrote durable state"); return os.ErrPermission }
		_, proof, beginErr := s.BeginBufferedRead(os.Getenv("FLARE_TEST_BUFFERED_TOKEN"))
		if beginErr != nil {
			t.Fatal(beginErr)
		}
		if _, guardErr := s.GuardSession(proof); guardErr != nil {
			t.Fatal(guardErr)
		}
		fmt.Println("tentative-buffer-ready")
		for {
			time.Sleep(time.Hour)
		}
	}
	s, dir := fixture(t)
	token, createErr := s.CreateSession()
	if createErr != nil {
		t.Fatal(createErr)
	}
	s.Close()
	original, readErr := os.ReadFile(filepath.Join(dir, "state.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestBufferedReadKilledTentativeProcess$")
	child.Env = append(os.Environ(), "FLARE_TEST_BUFFERED_DIR="+dir, "FLARE_TEST_BUFFERED_TOKEN="+token)
	pipe, pipeErr := child.StdoutPipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	child.Stderr = os.Stderr
	if startErr := child.Start(); startErr != nil {
		t.Fatal(startErr)
	}
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "tentative-buffer-ready") {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			child.Wait()
			t.Fatal("child did not reach tentative render")
		}
	case <-time.After(20 * time.Second):
		child.Process.Kill()
		child.Wait()
		t.Fatal("tentative render timeout")
	}
	if killErr := child.Process.Signal(syscall.SIGKILL); killErr != nil {
		t.Fatal(killErr)
	}
	if waitErr := child.Wait(); waitErr == nil {
		t.Fatal("child was not killed")
	}
	after, afterErr := os.ReadFile(filepath.Join(dir, "state.json"))
	if afterErr != nil {
		t.Fatal(afterErr)
	}
	if !bytes.Equal(original, after) {
		t.Fatal("unreleased tentative render changed durable clock or session state")
	}
	reopened, openErr := Open(dir)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer reopened.Close()
	if _, sessionErr := reopened.Session(token, false); sessionErr != nil {
		t.Fatal("tentative crash lost the last durable valid session:", sessionErr)
	}
}
