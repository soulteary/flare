package security

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionProofRepeatedGuardsDoNotObserveWallClockOrWrite(t *testing.T) {
	s, _ := fixture(t)
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	wall := s.data.HighWater.Add(time.Second)
	observations, writes := 0, 0
	s.now = func() time.Time { observations++; return wall }
	s.write = func(name string, b []byte) error { writes++; return s.atomicWrite(name, b) }
	_, proof, e := s.SessionSnapshot(token, false)
	if e != nil {
		t.Fatal(e)
	}
	firstWrites, firstObservations := writes, observations
	for i := 0; i < 50; i++ {
		if _, e = s.GuardSession(proof); e != nil {
			t.Fatal(e)
		}
	}
	if writes != firstWrites || observations != firstObservations {
		t.Fatalf("guards performed wall/disk work: writes%d observations%d", writes-firstWrites, observations-firstObservations)
	}
	// Every new request obtains a fresh durable snapshot, even with the same token.
	wall = wall.Add(time.Second)
	_, next, e := s.SessionSnapshot(token, false)
	if e != nil {
		t.Fatal(e)
	}
	if proof == next || writes <= firstWrites || observations <= firstObservations {
		t.Fatal("new request reused prior authorization point")
	}
	if !s.data.HighWater.Equal(wall) {
		t.Fatal("new request clock was not persisted")
	}
}
func TestSessionProofRevocationFaultAndStoreBinding(t *testing.T) {
	for _, action := range []string{"current", "all", "fault", "closed", "other-store"} {
		t.Run(action, func(t *testing.T) {
			s, _ := fixture(t)
			token, e := s.CreateSession()
			if e != nil {
				t.Fatal(e)
			}
			_, proof, e := s.SessionSnapshot(token, false)
			if e != nil {
				t.Fatal(e)
			}
			want := ErrInvalidSession
			switch action {
			case "current":
				e = s.Revoke(token)
			case "all":
				e = s.RevokeAll()
			case "fault":
				s.faulted = true
				want = ErrFaulted
			case "closed":
				e = s.Close()
				want = ErrFaulted
			case "other-store":
				other, _ := fixture(t)
				_, e = other.GuardSession(proof)
				if !errors.Is(e, want) {
					t.Fatal(e)
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.GuardSession(proof); !errors.Is(e, want) {
				t.Fatalf("stale proof accepted: %v", e)
			}
		})
	}
}
func TestSessionProofLatestDurableClockExpiresAndPersistsRemoval(t *testing.T) {
	s, dir := fixture(t)
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	_, proof, e := s.SessionSnapshot(token, false)
	if e != nil {
		t.Fatal(e)
	}
	next := s.clone()
	next.HighWater = next.Sessions[digest(token)].ExpiresAt
	if e = s.commit(next); e != nil {
		t.Fatal(e)
	}
	if _, e = s.GuardSession(proof); !errors.Is(e, ErrInvalidSession) {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, ok := s.data.Sessions[digest(token)]; ok {
		t.Fatal("expired session was not durably removed")
	}
}
func TestSessionProofLongRequestRefreshAndFinalClockRollback(t *testing.T) {
	s, _ := fixture(t)
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	wall := s.data.HighWater.Add(time.Second)
	observations := 0
	s.now = func() time.Time { observations++; return wall }
	_, proof, e := s.SessionSnapshot(token, false)
	if e != nil {
		t.Fatal(e)
	}
	initial := observations
	proof.started = time.Now().Add(-2 * sessionProofLifetime)
	wall = wall.Add(time.Second)
	if _, e = s.GuardSession(proof); e != nil {
		t.Fatal(e)
	}
	if observations <= initial || !s.data.HighWater.Equal(wall) {
		t.Fatal("long request did not refresh durable clock")
	}
	// No new wall observation is made inside a short proof. The final durable
	// success check must notice the rollback and revoke before rendering output.
	wall = wall.Add(-61 * time.Second)
	if _, e = s.GuardSession(proof); e != nil {
		t.Fatal(e)
	}
	if _, _, e = s.SessionSnapshot(token, true); !errors.Is(e, ErrInvalidSession) {
		t.Fatalf("final rollback check accepted: %v", e)
	}
	if len(s.data.Sessions) != 0 {
		t.Fatal("rollback did not durably revoke sessions")
	}
}
func TestCommitReusesValidatedDurableInstanceMarker(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	markerWrites := 0
	s.write = func(name string, b []byte) error {
		if name == "complete" {
			markerWrites++
		}
		return s.atomicWrite(name, b)
	}
	if e = s.Initialize("admin", "old", "local"); e != nil {
		t.Fatal(e)
	}
	if markerWrites != 1 {
		t.Fatal("initial marker was not written")
	}
	if _, e = s.CreateSession(); e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeAll(); e != nil {
		t.Fatal(e)
	}
	if markerWrites != 1 {
		t.Fatal("immutable marker was rewritten")
	}
}
func TestCommitDamagedInstanceMarkerFaultsBeforeJournal(t *testing.T) {
	for _, damage := range []string{"missing", "changed"} {
		t.Run(damage, func(t *testing.T) {
			s, dir := fixture(t)
			before, e := os.ReadFile(filepath.Join(dir, "state.json"))
			if e != nil {
				t.Fatal(e)
			}
			if damage == "missing" {
				e = os.Remove(filepath.Join(dir, "complete"))
			} else {
				e = os.WriteFile(filepath.Join(dir, "complete"), []byte("wrong-instance"), 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			if e = s.RevokeAll(); !errors.Is(e, ErrFaulted) {
				t.Fatal(e)
			}
			after, e := os.ReadFile(filepath.Join(dir, "state.json"))
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("marker fault changed state")
			}
			if _, e = os.Stat(filepath.Join(dir, "commit.json")); !os.IsNotExist(e) {
				t.Fatal("marker fault published a new commit intent")
			}
		})
	}
}

func TestSessionProofFinalPersistenceFailureFaultsAllGuards(t *testing.T) {
	s, _ := fixture(t)
	token, e := s.CreateSession()
	if e != nil {
		t.Fatal(e)
	}
	wall := s.data.HighWater.Add(time.Second)
	s.now = func() time.Time { return wall }
	_, proof, e := s.SessionSnapshot(token, false)
	if e != nil {
		t.Fatal(e)
	}
	s.write = func(string, []byte) error { return os.ErrPermission }
	// This short guard has no new clock observation; final successful-response
	// validation must still do durable I/O and reject the failed persistence.
	if _, e = s.GuardSession(proof); e != nil {
		t.Fatal(e)
	}
	wall = wall.Add(time.Second)
	if _, _, e = s.SessionSnapshot(token, true); !errors.Is(e, ErrFaulted) {
		t.Fatal(e)
	}
	if _, e = s.GuardSession(proof); !errors.Is(e, ErrFaulted) {
		t.Fatal("proof survived known persistence fault")
	}
}
