package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	flags "github.com/spf13/pflag"
)

// ExecuteSecurityCommand handles maintenance before ordinary flag parsing or any
// listener is started. Passwords are read hidden from a terminal or a file.
func ExecuteSecurityCommand(args []string, input io.Reader, out, errout io.Writer) (bool, int) {
	if len(args) == 0 || args[0] != "security" {
		return false, 0
	}
	if len(args) < 2 {
		fmt.Fprintln(errout, "usage: flare security migration-check|init|reset-password|set-credentials|adopt-legacy-defaults|revoke-all")
		return true, 2
	}
	fs := flags.NewFlagSet("security", flags.ContinueOnError)
	fs.SetOutput(errout)
	var dir, user, passwordFile, mode string
	var takeLocal, jsonOutput bool
	fs.StringVar(&dir, "state-dir", "", "persistent security directory")
	fs.StringVar(&user, "username", "", "administrator username")
	fs.StringVar(&passwordFile, "password-file", "", "read new password from a file")
	fs.StringVar(&mode, "mode", "", "local or deployment credential control")
	fs.BoolVar(&takeLocal, "take-local-control", false, "explicitly stop following deployment password settings")
	fs.BoolVar(&jsonOutput, "json", false, "machine-readable redacted check")
	if err := fs.Parse(args[2:]); err != nil || len(fs.Args()) != 0 {
		return true, 2
	}
	values, sources, err := captureSecurityInputs()
	if err != nil {
		fmt.Fprintln(errout, err)
		return true, 2
	}
	if dir != "" {
		values["FLARE_STATE_DIR"] = dir
	}
	f := model.Flags{User: define.DEFAULT_USER_NAME, DisableLoginMode: define.DEFAULT_DISABLE_LOGIN, Visibility: define.DEFAULT_VISIBILITY, RuntimeInputs: values, InputSources: sources}
	if err = applyExplicit(&f); err != nil {
		fmt.Fprintln(errout, err)
		return true, 2
	}
	dir = stateDirectory(&f)
	if args[1] == "migration-check" {
		files, count, legacy, e := inspectContent()
		if e != nil {
			fmt.Fprintln(errout, e)
			return true, 2
		}
		report := migrationReport{Status: "ready", LoginDisabled: f.DisableLoginMode, Visibility: f.Visibility, PrivateResources: count, Sources: sources, PersistentDirectory: dir, LegacyEvidence: legacy || explicitLegacy(&f, files)}
		if f.DisableLoginMode && count > 0 {
			report.Status = "attention_required"
		}
		if !report.LegacyEvidence {
			report.Status = "new"
		}
		meta, exists, metadataErr := readInstallation(dir)
		if metadataErr != nil {
			fmt.Fprintln(errout, "installation metadata requires recovery")
			return true, 4
		}
		if !f.DisableLoginMode {
			if _, _, e = readCredential(&f); e != nil && (!exists || !meta.NewInstall) {
				report.Status = "attention_required"
			}
		}
		if exists && meta.Committed {
			report.Status = "existing_installation"
			if meta.NewInstall {
				if _, set := values["FLARE_DISABLE_LOGIN"]; !set {
					report.LoginDisabled = false
				}
				if _, set := values["FLARE_VISIBILITY"]; !set {
					report.Visibility = "PRIVATE"
				}
			}
			if report.LoginDisabled && count > 0 {
				report.Status = "attention_required"
			}
		}
		info, inspectErr := security.Inspect(dir)
		if errors.Is(inspectErr, security.ErrInspectionIndeterminate) {
			report.Status = "indeterminate"
		} else if inspectErr != nil || (exists && meta.Committed && !info.Initialized) || (info.Initialized && !exists) {
			report.Status = "faulted"
		}
		if jsonOutput {
			err = json.NewEncoder(out).Encode(report)
		} else {
			_, err = fmt.Fprintf(out, "status=%s login_disabled=%v visibility=%s private_resources=%d legacy_evidence=%v state_directory=%s\n", report.Status, report.LoginDisabled, report.Visibility, report.PrivateResources, report.LegacyEvidence, dir)
		}
		if err != nil || report.Status == "faulted" {
			return true, 4
		}
		if report.Status == "indeterminate" {
			return true, 3
		}
		return true, 0
	}
	switch args[1] {
	case "init", "reset-password", "set-credentials", "adopt-legacy-defaults", "revoke-all":
	default:
		fmt.Fprintln(errout, "unknown security command")
		return true, 2
	}
	files, count, legacy, err := inspectContent()
	if err != nil {
		fmt.Fprintln(errout, err)
		return true, 2
	}
	if f.DisableLoginMode && count > 0 {
		fmt.Fprintln(errout, "private content requires login; correct FLARE_DISABLE_LOGIN before proceeding")
		return true, 2
	}
	meta, hasMeta, err := readInstallation(dir)
	if err != nil {
		fmt.Fprintln(errout, "installation metadata requires recovery")
		return true, 4
	}
	store, err := security.Open(dir)
	if err != nil {
		fmt.Fprintln(errout, "cannot open security state:", err)
		if errors.Is(err, security.ErrLocked) {
			return true, 3
		}
		return true, 4
	}
	defer store.Close()
	info := store.Info()
	if hasMeta && meta.Committed && !info.Initialized {
		fmt.Fprintln(errout, "completed security state is missing; restore complete backup")
		return true, 4
	}
	if info.Initialized && !hasMeta {
		fmt.Fprintln(errout, "installation metadata missing; restore complete security directory")
		return true, 4
	}
	fail := func(e error) (bool, int) {
		fmt.Fprintln(errout, e)
		if errors.Is(e, security.ErrFaulted) {
			return true, 4
		}
		return true, 2
	}
	if args[1] == "revoke-all" {
		if !info.Initialized {
			return fail(errors.New("security state has not been initialized"))
		}
		if err = store.RevokeAll(); err != nil {
			return fail(err)
		}
		fmt.Fprintln(out, "All existing sessions revoked.")
		return true, 0
	}
	if args[1] == "adopt-legacy-defaults" {
		if info.Initialized {
			return fail(errors.New("already initialized"))
		}
		if !f.DisableLoginMode || f.Visibility != "DEFAULT" || count > 0 {
			return fail(errors.New("legacy defaults require public trusted content without private flags"))
		}
		if err = migrationSnapshot(dir, files); err != nil {
			return fail(err)
		}
		meta = installation{Version: 1, LastDisableLogin: true}
		if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
			return fail(err)
		}
		if err = store.Initialize("", "", security.ModeTrusted); err != nil {
			return fail(err)
		}
		meta.Committed = true
		if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
			return fail(err)
		}
		fmt.Fprintln(out, "Legacy trusted access retained; network visitors can modify content.")
		return true, 0
	}
	if args[1] == "init" && info.Initialized {
		return fail(errors.New("administrator already initialized; use reset-password"))
	}
	if args[1] == "set-credentials" && !info.Initialized {
		return fail(errors.New("initialize before changing credential control"))
	}
	if args[1] == "set-credentials" && mode != "local" && mode != "deployment" {
		return fail(errors.New("--mode must be local or deployment"))
	}
	if user == "" {
		user = info.Username
		if user == "" {
			user = f.User
			if v := values["FLARE_USER"]; v != "" {
				user = v
			}
		}
	}
	var password string
	if args[1] == "set-credentials" && mode == "deployment" {
		if passwordFile != "" {
			f.RuntimeInputs["FLARE_PASS_FILE"] = passwordFile
			delete(f.RuntimeInputs, "FLARE_PASS")
		}
		user, password, err = readCredential(&f)
	} else {
		password, err = readNewPassword(passwordFile, input, out)
		if err == nil {
			err = security.ValidatePassword(password)
		}
	}
	if err != nil {
		return fail(err)
	}
	if !info.Initialized {
		if err = migrationSnapshot(dir, files); err != nil {
			return fail(err)
		}
		if !hasMeta {
			isNew := !legacy && !explicitLegacy(&f, files)
			meta = installation{Version: 1, NewInstall: isNew, DefaultPrivate: isNew, LastDisableLogin: f.DisableLoginMode}
		}
		if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
			return fail(err)
		}
		err = store.Initialize(user, password, "local")
	} else if args[1] == "set-credentials" {
		err = store.SetCredentials(user, password, mode)
	} else {
		err = store.ResetPassword(password, takeLocal)
	}
	if err != nil {
		return fail(err)
	}
	meta.Committed = true
	if err = writePrivateJSON(filepath.Join(dir, "installation.json"), meta); err != nil {
		return fail(err)
	}
	fmt.Fprintln(out, "Security state updated; previous sessions revoked. YAML content and access settings were not changed.")
	return true, 0
}
func readNewPassword(path string, input io.Reader, out io.Writer) (string, error) {
	if path != "" {
		fp, err := openPasswordFile(path)
		if err != nil {
			return "", errors.New("cannot read password file")
		}
		defer fp.Close()
		raw, err := io.ReadAll(io.LimitReader(fp, 515))
		if err != nil || len(raw) > 514 {
			return "", errors.New("invalid password file")
		}
		p := string(raw)
		if strings.HasSuffix(p, "\r\n") {
			p = strings.TrimSuffix(p, "\r\n")
		} else {
			p = strings.TrimSuffix(p, "\n")
		}
		return p, nil
	}
	reader := bufio.NewReader(input)
	read := func() (string, error) {
		if fp, ok := input.(*os.File); ok {
			if info, e := fp.Stat(); e == nil && info.Mode()&os.ModeCharDevice != 0 {
				return readTerminalPassword(fp)
			}
		}
		line, e := reader.ReadString('\n')
		if e != nil && !errors.Is(e, io.EOF) {
			return "", e
		}
		return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), nil
	}
	fmt.Fprint(out, "New password: ")
	first, err := read()
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	fmt.Fprint(out, "Confirm password: ")
	second, err := read()
	fmt.Fprintln(out)
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("password confirmation does not match")
	}
	return first, nil
}
