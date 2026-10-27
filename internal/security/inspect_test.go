package security

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectReadOnlyValidation(t *testing.T) {
	for _, fault := range []string{"none", "checksum", "marker", "journal", "pending", "missing-state", "missing-marker"} {
		t.Run(fault, func(t *testing.T) {
			s, dir := fixture(t)
			s.Close()
			raw, e := os.ReadFile(filepath.Join(dir, "state.json"))
			if e != nil {
				t.Fatal(e)
			}
			switch fault {
			case "checksum":
				var env envelope
				if err := json.Unmarshal(raw, &env); err != nil {
					t.Fatal(err)
				}
				env.Checksum = "invalid-checksum"
				damaged, err := json.Marshal(env)
				if err != nil {
					t.Fatal(err)
				}
				e = os.WriteFile(filepath.Join(dir, "state.json"), damaged, 0600)
			case "marker":
				e = os.WriteFile(filepath.Join(dir, "complete"), []byte("wrong-instance"), 0600)
			case "journal":
				e = os.WriteFile(filepath.Join(dir, "commit.json"), []byte("corrupt"), 0600)
			case "pending":
				e = os.WriteFile(filepath.Join(dir, "commit.json"), raw, 0600)
			case "missing-state":
				e = os.Remove(filepath.Join(dir, "state.json"))
			case "missing-marker":
				e = os.Remove(filepath.Join(dir, "complete"))
			}
			if e != nil {
				t.Fatal(e)
			}
			before := inspectionSnapshot(dir)
			info, e := Inspect(dir)
			if fault == "none" {
				if e != nil || !info.Initialized {
					t.Fatalf("%+v %v", info, e)
				}
			} else if fault == "pending" {
				if !errors.Is(e, ErrInspectionIndeterminate) {
					t.Fatal(e)
				}
			} else if !errors.Is(e, ErrFaulted) {
				t.Fatal(e)
			}
			if !sameInspection(before, inspectionSnapshot(dir)) {
				t.Fatal("read-only inspection mutated state")
			}
		})
	}
	dir := filepath.Join(t.TempDir(), "absent")
	if info, e := Inspect(dir); e != nil || info.Initialized {
		t.Fatalf("new: %+v %v", info, e)
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("inspection created a state directory")
	}
}
