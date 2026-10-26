package data

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const searchBackupDir = ".flare-search-backups"

type searchBackupManifest struct {
	Owner          string               `json:"owner"`
	Upgrade        string               `json:"upgrade,omitempty"`
	Latest         string               `json:"latest,omitempty"`
	UpgradeFile    string               `json:"upgrade_file,omitempty"`
	UpgradeVersion string               `json:"upgrade_version,omitempty"`
	Pending        *searchPendingBackup `json:"pending,omitempty"`
}

type searchPendingBackup struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Version string `json:"version,omitempty"`
}

var upgradeBackupName = regexp.MustCompile(`^first-upgrade-to-[A-Za-z0-9._+-]+\.bak$`)

func backupDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func regularBackupFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("managed backup is not a regular file: %s", path)
	}
	return nil
}

// Only files recorded with matching hashes in our private manifest are replaced.
// Never glob or delete historical *.bak files: ownership cannot be established.
func retainSearchBackup(raw []byte, upgrading bool) error {
	return retainSearchBackupWithWriter(raw, upgrading, saveConfigAtomically)
}

func retainSearchBackupWithWriter(raw []byte, upgrading bool, write func(string, []byte) bool) error {
	dir := filepath.Join(filepath.Dir(getConfigPath("config")), searchBackupDir)
	manifestPath := filepath.Join(dir, "manifest.json")
	state := searchBackupManifest{Owner: "flare-search-backups-v1"}
	if err := initializeSearchBackup(dir, manifestPath, &state, write); err != nil {
		return err
	}
	if state.UpgradeFile != "" && state.Upgrade == "" {
		return fmt.Errorf("incomplete upgrade backup metadata")
	}
	if err := recoverSearchBackup(dir, &state); err != nil {
		return err
	}
	if err := validateSearchBackups(dir, state); err != nil {
		return err
	}
	name := "latest.bak"
	if upgrading && state.Upgrade == "" {
		safeVersion := regexp.MustCompile(`[^A-Za-z0-9._+-]`).ReplaceAllString(applicationVersion(), "_")
		name = "first-upgrade-to-" + safeVersion + ".bak"
		// Do not overwrite a user file that happens to use the new name.
		if _, e := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(e) {
			return fmt.Errorf("unowned upgrade backup: %s", name)
		}
	}
	state.Pending = &searchPendingBackup{Name: name, Hash: backupDigest(raw), Version: applicationVersion()}
	intent, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if !write(manifestPath, intent) {
		return fmt.Errorf("cannot record pending backup")
	}
	if !write(filepath.Join(dir, name), raw) {
		return fmt.Errorf("cannot save backup")
	}
	if name != "latest.bak" {
		state.UpgradeFile = name
		state.UpgradeVersion = applicationVersion()
		state.Upgrade = backupDigest(raw)
	} else {
		state.Latest = backupDigest(raw)
	}
	state.Pending = nil
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if !write(manifestPath, data) {
		return fmt.Errorf("cannot update backup manifest")
	}
	return nil
}

func initializeSearchBackup(dir, manifestPath string, state *searchBackupManifest, write func(string, []byte) bool) error {
	err := os.Mkdir(dir, 0700)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
		info, e := os.Lstat(dir)
		if e != nil {
			return e
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("unsafe backup directory")
		}
		if e = regularBackupFile(manifestPath); e != nil {
			return fmt.Errorf("unrecognized backup directory: %w", e)
		}
		data, e := readFile(manifestPath)
		if e != nil {
			return e
		}
		if err := decodeSearchBackupManifest(data, state); err != nil {
			return err
		}
	} else {
		// Claim the newly created directory before adding any backup.
		data, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			return marshalErr
		}
		if !write(manifestPath, data) {
			// Only remove our newly created directory if still empty.
			if cleanupErr := os.Remove(dir); cleanupErr != nil && !os.IsNotExist(cleanupErr) {
				return fmt.Errorf("cannot initialize backup manifest (cleanup: %w)", cleanupErr)
			}
			return fmt.Errorf("cannot initialize backup manifest")
		}
	}

	return nil
}

func recoverSearchBackup(dir string, state *searchBackupManifest) error {
	if state.Pending != nil {
		pending := state.Pending
		if pending.Hash == "" || (pending.Name != "latest.bak" && !upgradeBackupName.MatchString(pending.Name)) || (pending.Name != "latest.bak" && state.Upgrade != "") {
			return fmt.Errorf("invalid pending backup")
		}
		oldHash := state.Latest
		if pending.Name != "latest.bak" {
			oldHash = ""
		}
		if err := recoverPendingBackup(dir, state, pending, oldHash); err != nil {
			return err
		}
		state.Pending = nil
		// Recovery updates the manifest only after validating all owned files below.
	}

	return nil
}

func validateSearchBackups(dir string, state searchBackupManifest) error {
	upgradeName := "first-upgrade.bak"
	if state.UpgradeFile != "" {
		if !upgradeBackupName.MatchString(state.UpgradeFile) {
			return fmt.Errorf("invalid managed upgrade filename")
		}
		upgradeName = state.UpgradeFile
	}
	for _, entry := range []struct{ name, hash string }{{upgradeName, state.Upgrade}, {"latest.bak", state.Latest}} {
		path := filepath.Join(dir, entry.name)
		if entry.hash == "" {
			if _, e := os.Lstat(path); !os.IsNotExist(e) {
				return fmt.Errorf("unowned backup file: %s", entry.name)
			}
			continue
		}
		if e := regularBackupFile(path); e != nil {
			return e
		}
		data, e := readFile(path)
		if e != nil {
			return e
		}
		if backupDigest(data) != entry.hash {
			return fmt.Errorf("backup was externally modified: %s", entry.name)
		}
	}

	return nil
}

func decodeSearchBackupManifest(data []byte, state *searchBackupManifest) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(state); e != nil {
		return e
	}
	var extra any
	if e := decoder.Decode(&extra); e != io.EOF {
		return fmt.Errorf("invalid backup manifest trailer")
	}
	if state.Owner != "flare-search-backups-v1" {
		return fmt.Errorf("unrecognized backup owner")
	}
	return nil
}

func recoverPendingBackup(dir string, state *searchBackupManifest, pending *searchPendingBackup, oldHash string) error {
	p := filepath.Join(dir, pending.Name)
	info, e := os.Lstat(p)
	if e == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("unsafe pending backup")
		}
		contents, readErr := readFile(p)
		if readErr != nil {
			return readErr
		}
		digest := backupDigest(contents)
		if digest == pending.Hash {
			if pending.Name == "latest.bak" {
				state.Latest = digest
			} else {
				state.Upgrade = digest
				state.UpgradeFile = pending.Name
				state.UpgradeVersion = pending.Version
			}
		} else if digest != oldHash {
			return fmt.Errorf("pending backup was externally modified")
		}
	} else if !os.IsNotExist(e) || oldHash != "" {
		return fmt.Errorf("pending backup missing or unreadable")
	}
	return nil
}
