package security

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrInspectionIndeterminate means a writer or pending transaction prevents a
// read-only check from establishing a settled state. It is not proof of damage.
var ErrInspectionIndeterminate = errors.New("security state is changing or has a pending transaction; stop the process and retry")

type inspectionFile struct {
	raw []byte
	err error
}

func inspectionSnapshot(dir string) map[string]inspectionFile {
	files := map[string]inspectionFile{}
	for _, name := range []string{"state.json", "complete", "commit.json"} {
		raw, err := readStateFile(filepath.Join(dir, name))
		files[name] = inspectionFile{raw, err}
	}
	return files
}
func sameInspection(a, b map[string]inspectionFile) bool {
	for name, x := range a {
		y := b[name]
		if !bytes.Equal(x.raw, y.raw) || (x.err == nil) != (y.err == nil) || os.IsNotExist(x.err) != os.IsNotExist(y.err) {
			return false
		}
	}
	return true
}
func inspectionState(raw []byte) (state, error) {
	var env envelope
	var value state
	if json.Unmarshal(raw, &env) != nil || digest(string(env.State)) != env.Checksum {
		return value, ErrFaulted
	}
	if json.Unmarshal(env.State, &value) != nil || value.Version != 1 || value.InstanceID == "" || value.Sessions == nil {
		return value, ErrFaulted
	}
	if value.Mode != ModeTrusted && (!validHash(value.Password) || (value.Mode != "local" && value.Mode != "deployment")) {
		return value, ErrFaulted
	}
	return value, nil
}

// Inspect reads settled on-disk metadata without acquiring locks, creating a
// directory, replaying a journal, advancing clocks, or exposing credentials.
// Two matching snapshots reduce racing-writer ambiguity; this is a preflight,
// not a transaction or a substitute for Open's locked startup validation.
func Inspect(dir string) (Info, error) {
	if fi, e := os.Lstat(dir); e == nil && (!fi.IsDir() || fi.Mode()&os.ModeSymlink != 0) {
		return Info{}, ErrFaulted
	} else if e != nil && !os.IsNotExist(e) {
		return Info{}, ErrFaulted
	}
	first, second := inspectionSnapshot(dir), inspectionSnapshot(dir)
	if !sameInspection(first, second) {
		return Info{}, ErrInspectionIndeterminate
	}
	marker, raw, journal := second["complete"], second["state.json"], second["commit.json"]
	for _, file := range second {
		if file.err != nil && !os.IsNotExist(file.err) {
			return Info{}, ErrFaulted
		}
	}
	if journal.err == nil {
		pending, e := inspectionState(journal.raw)
		if e != nil {
			return Info{}, e
		}
		if marker.err == nil && string(marker.raw) != pending.InstanceID {
			return Info{}, ErrFaulted
		}
		return Info{}, ErrInspectionIndeterminate
	}
	if os.IsNotExist(marker.err) && os.IsNotExist(raw.err) {
		return Info{}, nil
	}
	if marker.err != nil || raw.err != nil {
		return Info{}, ErrFaulted
	}
	value, e := inspectionState(raw.raw)
	if e != nil {
		return Info{}, e
	}
	if string(marker.raw) != value.InstanceID {
		return Info{}, ErrFaulted
	}
	return Info{Initialized: true, Username: value.Username, Mode: value.Mode, InstanceID: value.InstanceID, Generation: value.Generation}, nil
}
