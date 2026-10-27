package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	"github.com/soulteary/flare/internal/yamlutil"
	"golang.org/x/sys/unix"
)

type SecurityPreparation struct {
	Store           *security.Store
	Status, Message string
}
type installation struct {
	Version          int  `json:"version"`
	Committed        bool `json:"committed"`
	NewInstall       bool `json:"new_install"`
	DefaultPrivate   bool `json:"default_private"`
	LastDisableLogin bool `json:"last_disable_login"`
}
type migrationReport struct {
	Status              string            `json:"status"`
	LoginDisabled       bool              `json:"login_disabled"`
	Visibility          string            `json:"visibility"`
	PrivateResources    int               `json:"private_resources"`
	Sources             map[string]string `json:"sources,omitempty"`
	PersistentDirectory string            `json:"persistent_directory"`
	LegacyEvidence      bool              `json:"legacy_evidence"`
}

func stateDirectory(f *model.Flags) string {
	if p := f.RuntimeInputs["FLARE_STATE_DIR"]; p != "" {
		return p
	}
	return ".flare-security"
}
func sum(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

// Read actual content bytes without initializing missing YAML or warming navigation caches.
func inspectContent() (map[string][]byte, int, bool, error) {
	files := map[string][]byte{}
	privateCount := 0
	legacy := false
	for _, name := range []string{"apps.yml", "bookmarks.yml", "config.yml"} {
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, 0, false, fmt.Errorf("cannot inspect %s", name)
		}
		// Projected ConfigMap/GitOps YAML may be a symlink; reading is safe,
		// while editor writers independently enforce read-only symlink handling.
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(name)
			if err != nil {
				return nil, 0, false, fmt.Errorf("cannot resolve %s", name)
			}
		}
		if !info.Mode().IsRegular() {
			return nil, 0, false, fmt.Errorf("%s must be a regular file", name)
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			return nil, 0, false, fmt.Errorf("cannot read %s", name)
		}
		files[name] = raw
		if sum(raw) != releaseSampleDigests[name] {
			legacy = true
		}
		if name == "config.yml" {
			continue
		}
		var doc model.Bookmarks
		if err := yamlutil.DecodeBookmarks(raw, &doc, name == "apps.yml"); err != nil {
			return nil, 0, false, fmt.Errorf("invalid %s: %w", name, err)
		}
		for _, c := range doc.Categories {
			if c.Private {
				privateCount++
			}
		}
		for _, b := range doc.Items {
			if b.Private {
				privateCount++
			}
		}
	}
	if _, err := os.Stat(".flare-cookie-secret"); err == nil {
		legacy = true
	} else if !os.IsNotExist(err) {
		return nil, 0, false, errors.New("cannot inspect legacy cookie key")
	}
	return files, privateCount, legacy, nil
}
func applyExplicit(f *model.Flags) error {
	f.ExternalURL = f.RuntimeInputs["FLARE_EXTERNAL_URL"]
	f.AllowedHosts = f.RuntimeInputs["FLARE_ALLOWED_HOSTS"]
	f.TrustedProxies = f.RuntimeInputs["FLARE_TRUSTED_PROXIES"]
	if f.ConfigError != "" {
		return errors.New(f.ConfigError)
	}
	if err := validateSecurityInputs(f.RuntimeInputs); err != nil {
		return err
	}
	if raw, ok := f.RuntimeInputs["FLARE_DISABLE_LOGIN"]; ok {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return errors.New("FLARE_DISABLE_LOGIN must be a boolean")
		}
		f.DisableLoginMode = parsed
	}
	if raw, ok := f.RuntimeInputs["FLARE_VISIBILITY"]; ok {
		f.Visibility = strings.ToUpper(strings.TrimSpace(raw))
	}
	if f.Visibility != "DEFAULT" && f.Visibility != "PRIVATE" {
		return errors.New("FLARE_VISIBILITY must be DEFAULT or PRIVATE")
	}
	if f.DisableLoginMode && f.Visibility == "PRIVATE" {
		return errors.New("FLARE_DISABLE_LOGIN conflicts with PRIVATE visibility; enable login")
	}
	return nil
}
func explicitLegacy(f *model.Flags, files map[string][]byte) bool {
	if len(files) == 0 {
		return false
	}
	for _, k := range []string{"FLARE_DISABLE_LOGIN", "FLARE_VISIBILITY"} {
		if _, ok := f.RuntimeInputs[k]; ok {
			return true
		}
	}
	return false
}

// Secret files may be projected symlinks, but their final target must be a
// regular file. Nonblocking open prevents a mistaken FIFO from hanging startup
// or a maintenance command indefinitely.
func openPasswordFile(path string) (*os.File, error) {
	fp, err := os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, err := fp.Stat()
	if err != nil || !info.Mode().IsRegular() {
		fp.Close()
		return nil, errors.New("password source must be a regular file")
	}
	return fp, nil
}

func readCredential(f *model.Flags) (string, string, error) {
	user := f.User
	if raw, ok := f.RuntimeInputs["FLARE_USER"]; ok {
		user = raw
	}
	if user == "" {
		user = "flare"
	}
	if len(user) > 256 {
		return "", "", errors.New("FLARE_USER is too long")
	}
	pass, hasPass := f.RuntimeInputs["FLARE_PASS"]
	file := f.RuntimeInputs["FLARE_PASS_FILE"]
	if hasPass && pass != "" && file != "" {
		return "", "", errors.New("FLARE_PASS conflicts with FLARE_PASS_FILE")
	}
	if file != "" {
		fp, err := openPasswordFile(file)
		if err != nil {
			return "", "", errors.New("cannot read FLARE_PASS_FILE")
		}
		defer fp.Close()
		raw, err := io.ReadAll(io.LimitReader(fp, 515))
		if err != nil || len(raw) > 514 {
			return "", "", errors.New("invalid FLARE_PASS_FILE length")
		}
		pass = string(raw)
		if strings.HasSuffix(pass, "\r\n") {
			pass = strings.TrimSuffix(pass, "\r\n")
		} else {
			pass = strings.TrimSuffix(pass, "\n")
		}
		if !utf8.ValidString(pass) || len(pass) > 512 {
			return "", "", errors.New("FLARE_PASS_FILE must contain valid UTF-8 within length limit")
		}
	}
	if pass == "" {
		return "", "", errors.New("no persistent password; set FLARE_PASS/FLARE_PASS_FILE or run flare security init")
	}
	if len(pass) > 4096 {
		return "", "", errors.New("deployment password exceeds authentication input limit")
	}
	return user, pass, nil
}
func readInstallation(dir string) (installation, bool, error) {
	var m installation
	fp, err := os.OpenFile(filepath.Join(dir, "installation.json"), os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return m, false, nil
	}
	if err != nil {
		return m, false, err
	}
	defer fp.Close()
	info, err := fp.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return m, false, errors.New("installation metadata is damaged")
	}
	raw, err := io.ReadAll(io.LimitReader(fp, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return m, false, errors.New("installation metadata is damaged")
	}
	if json.Unmarshal(raw, &m) != nil || m.Version != 1 {
		return m, false, errors.New("installation metadata is damaged")
	}
	return m, true, nil
}
func writePrivateJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".security-write-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func migrationSnapshot(dir string, files map[string][]byte) error {
	path := filepath.Join(dir, "migration-backups", "initial")
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	manifest := map[string]string{}
	if key, err := os.ReadFile(".flare-cookie-secret"); err == nil {
		files = cloneFiles(files)
		files[".flare-cookie-secret"] = key
	} else if !os.IsNotExist(err) {
		return err
	}
	// Preserve the first snapshot even when an interrupted migration is retried
	// with deliberately corrected source files. A new input set gets its own copy.
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var fingerprint strings.Builder
	changed := false
	// A completed backup is immutable, including its file set. Removing or
	// adding a source on a retry must not rewrite the original manifest.
	if raw, e := os.ReadFile(filepath.Join(path, "manifest.json")); e == nil {
		var previous map[string]string
		if json.Unmarshal(raw, &previous) != nil {
			return errors.New("migration snapshot manifest is damaged; preserve backup and recover")
		}
		if len(previous) != len(files) {
			changed = true
		}
		for name, raw := range files {
			if previous[name] != sum(raw) {
				changed = true
			}
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	for _, name := range names {
		fingerprint.WriteString(name + ":" + sum(files[name]) + "\n")
		if previous, e := os.ReadFile(filepath.Join(path, name)); e == nil && sum(previous) != sum(files[name]) {
			changed = true
		}
	}
	if changed {
		path = filepath.Join(dir, "migration-backups", "retry-"+sum([]byte(fingerprint.String()))[:16])
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
	}
	for name, raw := range files {
		manifest[name] = sum(raw)
		dest := filepath.Join(path, name)
		if prior, err := os.ReadFile(dest); err == nil {
			if sum(prior) != sum(raw) {
				return errors.New("migration snapshot differs; preserve original backup and resolve changed source")
			}
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		fp, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, err = fp.Write(raw)
		if err == nil {
			err = fp.Sync()
		}
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	if err := writePrivateJSON(filepath.Join(path, "manifest.json"), manifest); err != nil {
		return err
	}
	// Persist newly-created backup directory entries before the installation
	// transaction can be committed. Syncing files and the leaf alone does not
	// make a newly-created parent directory durable across a power failure.
	for _, parent := range []string{filepath.Join(dir, "migration-backups"), dir} {
		fp, err := os.Open(parent)
		if err != nil {
			return err
		}
		err = fp.Sync()
		closeErr := fp.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	for name, raw := range files {
		now, err := os.ReadFile(name)
		if err != nil || sum(now) != sum(raw) {
			return errors.New("source files changed during migration")
		}
	}
	return nil
}
func cloneFiles(in map[string][]byte) map[string][]byte {
	out := map[string][]byte{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

// PrepareSecurity is the production startup gate. A non-ready result must only
// serve the generic setup/error page; it never enables anonymous fallback.
func PrepareSecurity(f *model.Flags) SecurityPreparation {
	fail := func(status, msg string, s *security.Store) SecurityPreparation {
		f.StartupStatus, f.StartupMessage = status, msg
		return SecurityPreparation{s, status, msg}
	}
	if err := applyExplicit(f); err != nil {
		return fail("attention_required", err.Error(), nil)
	}
	files, count, legacy, err := inspectContent()
	if err != nil {
		return fail("attention_required", err.Error(), nil)
	}
	dir := stateDirectory(f)
	meta, hasMeta, err := readInstallation(dir)
	if err != nil {
		return fail("faulted", "security installation metadata requires recovery", nil)
	}
	store, err := security.Open(dir)
	if err != nil {
		return fail("faulted", "cannot open persistent security state; check permissions, lock and backup", nil)
	}
	info := store.Info()
	if hasMeta && meta.Committed && !info.Initialized {
		return fail("faulted", "completed security state is missing; restore the complete directory", store)
	}
	if info.Initialized && !hasMeta {
		return fail("faulted", "security installation metadata is missing; restore the complete security directory", store)
	}
	legacy = legacy || explicitLegacy(f, files)
	isNew := !legacy
	if hasMeta {
		isNew = meta.NewInstall
	}
	if isNew {
		if _, ok := f.RuntimeInputs["FLARE_DISABLE_LOGIN"]; !ok {
			f.DisableLoginMode = false
		}
		if _, ok := f.RuntimeInputs["FLARE_VISIBILITY"]; !ok {
			f.Visibility = "PRIVATE"
		}
	}
	if f.DisableLoginMode && f.Visibility == "PRIVATE" {
		return fail("attention_required", "PRIVATE visibility requires login; correct the explicit access configuration", store)
	}
	if f.DisableLoginMode && count > 0 {
		return fail("attention_required", "private content conflicts with disabled login; enable login or review private flags offline", store)
	}
	if !info.Initialized {
		if f.DisableLoginMode {
			if isNew {
				if _, ok := f.RuntimeInputs["FLARE_DISABLE_LOGIN"]; !ok {
					return fail("new", "Initialize Flare locally before browsing.", store)
				}
			}
		} else {
			if _, _, err = readCredential(f); err != nil {
				if isNew {
					return fail("new", "Initialize Flare locally or supply an initial password.", store)
				}
				return fail("attention_required", err.Error(), store)
			}
		}
		if err = migrationSnapshot(dir, files); err != nil {
			return fail("faulted", "cannot create migration snapshot; original YAML was not changed", store)
		}
		meta = installation{Version: 1, NewInstall: isNew, DefaultPrivate: isNew, LastDisableLogin: f.DisableLoginMode}
		if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
			return fail("faulted", "cannot persist installation metadata", store)
		}
		if f.DisableLoginMode {
			err = store.Initialize("", "", security.ModeTrusted)
		} else {
			user, pass, credentialErr := readCredential(f)
			if credentialErr != nil {
				return fail("attention_required", credentialErr.Error(), store)
			}
			mode := "deployment"
			if isNew {
				mode = "local"
			}
			if v := f.RuntimeInputs["FLARE_CREDENTIAL_MODE"]; v != "" {
				mode = v
			}
			if isNew {
				err = security.ValidatePassword(pass)
			}
			if err == nil {
				err = store.Initialize(user, pass, mode)
			}
		}
		if err != nil {
			return fail("attention_required", "cannot initialize credentials; use a compliant password or local recovery", store)
		}
	} else {
		if v := f.RuntimeInputs["FLARE_CREDENTIAL_MODE"]; v != "" && v != info.Mode {
			return fail("attention_required", "credential mode conflicts with stored mode; use security set-credentials", store)
		}
		if !f.DisableLoginMode && (info.Mode == security.ModeTrusted || info.Mode == "deployment") {
			user, pass, e := readCredential(f)
			if e != nil {
				return fail("attention_required", e.Error(), store)
			}
			mode := "deployment"
			if info.Mode == security.ModeTrusted && isNew {
				mode = "local"
			}
			if e = store.SetCredentials(user, pass, mode); e != nil {
				return fail("faulted", "cannot apply deployment credentials", store)
			}
		}
		if meta.LastDisableLogin != f.DisableLoginMode {
			if err = store.RevokeAll(); err != nil {
				return fail("faulted", "cannot revoke sessions for access-mode change", store)
			}
			meta.LastDisableLogin = f.DisableLoginMode
			if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
				return fail("faulted", "cannot persist access-mode transition", store)
			}
		}
	}
	if !meta.Committed {
		meta.Committed = true
		if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
			return fail("faulted", "cannot commit installation metadata", store)
		}
	}
	f.StartupStatus = "ready"
	f.StartupMessage = ""
	return SecurityPreparation{store, "ready", ""}
}
