package data

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/soulteary/flare/internal/logger"
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

// unusableBackups marks an existing backup directory whose contents cannot be
// trusted or continued. Such a directory is set aside under another name and a
// new one is started; nothing in it is modified or deleted.
type unusableBackups struct{ err error }

func (e *unusableBackups) Error() string { return e.err.Error() }
func (e *unusableBackups) Unwrap() error { return e.err }

func unusable(err error) error { return &unusableBackups{err} }

// errBackupLocationOccupied means the backup path is a symbolic link or not a
// directory. It is left alone and nothing is written through it.
var errBackupLocationOccupied = errors.New("backup path is a symbolic link or not a directory")

// Tests replace these to observe warnings, to simulate filesystems that ignore
// or refuse permission changes and renames, and to fix the set-aside timestamp.
var (
	backupWarn = func(path, reason, action string) {
		logger.GetLogger().Warn("settings backup", "path", path, "reason", reason, "action", action)
	}
	backupChmod  = os.Chmod
	backupRename = os.Rename
	backupNow    = time.Now
)

var backupWarned sync.Map

func searchBackupPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), searchBackupDir)
}

func absoluteBackupPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// warnBackupOnce reports each distinct condition once per process, so a
// persistent one (a filesystem without permission bits, a read-only backup
// directory) is not repeated on every save.
func warnBackupOnce(path, reason, action string) {
	path = absoluteBackupPath(path)
	if _, seen := backupWarned.LoadOrStore(path+"\x00"+reason, struct{}{}); seen {
		return
	}
	backupWarn(path, reason, action)
}

// keepBackupPrivate restores owner-only access on a managed directory or file.
// Access by group or others is repaired instead of refused: copying or
// restoring the data directory and filesystems without permission bits all
// produce it, and refusing blocked every settings save. Callers pass only
// paths already verified not to be symbolic links.
func keepBackupPrivate(path string, info os.FileInfo, want os.FileMode) {
	if info.Mode().Perm() == want {
		return
	}
	chmodErr := backupChmod(path, want)
	if current, err := os.Lstat(path); err == nil {
		info = current
	}
	if info.Mode().Perm()&0077 == 0 {
		return
	}
	reason := fmt.Sprintf("mode %04o still allows group or other access", info.Mode().Perm())
	if chmodErr != nil {
		reason += ": " + chmodErr.Error()
	}
	warnBackupOnce(path, reason, "kept using it; restrict access to the data directory if this filesystem does not support permissions")
}

func regularBackupFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed backup is not a regular file: %s", path)
	}
	keepBackupPrivate(path, info, 0600)
	return nil
}

// Only files recorded with matching hashes in our private manifest are replaced.
// Never glob or delete historical *.bak files: ownership cannot be established.
func retainSearchBackup(raw []byte, upgrading bool) error {
	return retainSearchBackupWithWriter(raw, upgrading, saveConfigAtomically)
}

func retainSearchBackupWithWriter(raw []byte, upgrading bool, write func(string, []byte) bool) error {
	dir := searchBackupPath(getConfigPath("config"))
	err := writeSearchBackup(dir, raw, upgrading, write)
	var rejected *unusableBackups
	if !errors.As(err, &rejected) {
		return err
	}
	// Keep the rejected directory intact for the operator and start again, so
	// one damaged or foreign file does not disable backups for good.
	moved, moveErr := setAsideSearchBackups(dir)
	if moveErr != nil {
		return fmt.Errorf("%w; the directory could not be set aside: %w", err, moveErr)
	}
	backupWarn(absoluteBackupPath(dir), rejected.Error(), "moved it unchanged to "+absoluteBackupPath(moved)+" and started a new backup directory")
	return writeSearchBackup(dir, raw, upgrading, write)
}

// setAsideSearchBackups renames the directory to an unused sibling name. It
// never replaces an existing path and never removes anything.
func setAsideSearchBackups(dir string) (string, error) {
	base := dir + ".quarantined-" + backupNow().UTC().Format("20060102T150405Z")
	target := base
	for attempt := 0; attempt < 5; attempt++ {
		_, err := os.Lstat(target)
		if os.IsNotExist(err) {
			if err = backupRename(dir, target); err != nil {
				return "", err
			}
			return target, nil
		}
		if err != nil {
			return "", err
		}
		target = base + "-" + GenerateRandomString(6)
	}
	return "", fmt.Errorf("no unused name for the set-aside backup directory")
}

func writeSearchBackup(dir string, raw []byte, upgrading bool, write func(string, []byte) bool) error {
	manifestPath := filepath.Join(dir, "manifest.json")
	state := searchBackupManifest{Owner: "flare-search-backups-v1"}
	if err := initializeSearchBackup(dir, manifestPath, &state, write); err != nil {
		return err
	}
	if state.UpgradeFile != "" && state.Upgrade == "" {
		return unusable(fmt.Errorf("incomplete upgrade backup metadata"))
	}
	if err := recoverSearchBackup(dir, &state); err != nil {
		return unusable(err)
	}
	if err := validateSearchBackups(dir, state); err != nil {
		return unusable(err)
	}
	name := "latest.bak"
	if upgrading && state.Upgrade == "" {
		safeVersion := regexp.MustCompile(`[^A-Za-z0-9._+-]`).ReplaceAllString(applicationVersion(), "_")
		name = "first-upgrade-to-" + safeVersion + ".bak"
		// Do not overwrite a user file that happens to use the new name.
		if _, e := os.Lstat(filepath.Join(dir, name)); !os.IsNotExist(e) {
			return unusable(fmt.Errorf("unowned upgrade backup: %s", name))
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
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errBackupLocationOccupied
		}
		keepBackupPrivate(dir, info, 0700)
		if e = regularBackupFile(manifestPath); e != nil {
			return unusable(fmt.Errorf("unrecognized backup directory: %w", e))
		}
		data, e := readFile(manifestPath)
		if e != nil {
			return unusable(e)
		}
		if err := decodeSearchBackupManifest(data, state); err != nil {
			return unusable(err)
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
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe pending backup")
		}
		keepBackupPrivate(p, info, 0600)
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
