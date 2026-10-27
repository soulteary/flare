package security

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The only fault checkpoints live in the test binary, never in production.
func TestRecoveryKilledWriter(t *testing.T) {
	if dir := os.Getenv("FLARE_TEST_CRASH_DIR"); dir != "" {
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		point := os.Getenv("FLARE_TEST_CRASH_POINT")
		s.write = func(name string, b []byte) error {
			if writeErr := s.atomicWrite(name, b); writeErr != nil {
				return writeErr
			}
			if name == point {
				fmt.Println("durable-checkpoint")
				for {
					time.Sleep(time.Hour)
				}
			}
			return nil
		}
		if os.Getenv("FLARE_TEST_CRASH_OPERATION") == "initialize" {
			err = s.Initialize("admin", "old", "local")
		} else {
			err = s.RevokeAll()
		}
		t.Fatalf("checkpoint not reached: %v", err)
	}
	for _, operation := range []string{"initialize", "revoke"} {
		points := []string{"commit.json", "state.json"}
		if operation == "initialize" {
			points = append(points, "complete")
		}
		for _, point := range points {
			t.Run(operation+"/"+point, func(t *testing.T) {
				dir := t.TempDir()
				var token string
				if operation == "revoke" {
					s, e := Open(dir)
					if e != nil {
						t.Fatal(e)
					}
					if e = s.Initialize("admin", "old", "local"); e != nil {
						t.Fatal(e)
					}
					token, e = s.CreateSession()
					if e != nil {
						t.Fatal(e)
					}
					s.Close()
				}
				child := exec.Command(os.Args[0], "-test.run=^TestRecoveryKilledWriter$")
				child.Env = append(os.Environ(), "FLARE_TEST_CRASH_DIR="+dir, "FLARE_TEST_CRASH_POINT="+point, "FLARE_TEST_CRASH_OPERATION="+operation)
				pipe, e := child.StdoutPipe()
				if e != nil {
					t.Fatal(e)
				}
				child.Stderr = os.Stderr
				if e = child.Start(); e != nil {
					t.Fatal(e)
				}
				done := make(chan bool, 1)
				go func() {
					sc := bufio.NewScanner(pipe)
					for sc.Scan() {
						if strings.Contains(sc.Text(), "durable-checkpoint") {
							done <- true
							return
						}
					}
					done <- false
				}()
				select {
				case ok := <-done:
					if !ok {
						child.Wait()
						t.Fatal("writer exited before checkpoint")
					}
				case <-time.After(20 * time.Second):
					child.Process.Kill()
					child.Wait()
					t.Fatal("checkpoint timed out")
				}
				if e = child.Process.Signal(syscall.SIGKILL); e != nil {
					t.Fatal(e)
				}
				if e = child.Wait(); e == nil {
					t.Fatal("expected SIGKILL exit")
				}
				s, e := Open(dir)
				if e != nil {
					t.Fatalf("recovery: %v", e)
				}
				defer s.Close()
				if !s.Info().Initialized {
					t.Fatal("durable journal was not replayed")
				}
				if e = s.Verify("admin", "old"); e != nil {
					t.Fatal(e)
				}
				if operation == "revoke" {
					if _, e = s.Session(token, false); !errors.Is(e, ErrInvalidSession) {
						t.Fatalf("revoked session revived: %v", e)
					}
				}
				if _, e = os.Stat(dir + "/commit.json"); !os.IsNotExist(e) {
					t.Fatalf("journal not retired: %v", e)
				}
			})
		}
	}
}

func TestRecoveryDiskFullFaultsWithoutAcknowledgingRevocation(t *testing.T) {
	s, dir := fixture(t)
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	s.write = func(string, []byte) error { return syscall.ENOSPC }
	if e = s.RevokeAll(); !errors.Is(e, ErrFaulted) {
		t.Fatal(e)
	}
	if _, e = s.Session(token, false); !errors.Is(e, ErrFaulted) {
		t.Fatalf("failed persistence continued serving: %v", e)
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Session(token, false); e != nil {
		t.Fatalf("unacknowledged transaction changed durable state: %v", e)
	}
}
