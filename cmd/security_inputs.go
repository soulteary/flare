package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

var securityInputKeys = []string{"FLARE_DISABLE_LOGIN", "FLARE_VISIBILITY", "FLARE_USER", "FLARE_PASS", "FLARE_PASS_FILE", "FLARE_STATE_DIR", "FLARE_CREDENTIAL_MODE", "FLARE_EXTERNAL_URL", "FLARE_ALLOWED_HOSTS", "FLARE_TRUSTED_PROXIES"}

// Capture before caarlos/env's `unset` handling consumes account variables.
// Values are never included in diagnostics, JSON reports, or logs.
func captureSecurityInputs() (map[string]string, map[string]string, error) {
	values, sources := map[string]string{}, map[string]string{}
	for _, key := range securityInputKeys {
		if v, ok := os.LookupEnv(key); ok {
			values[key], sources[key] = v, "environment"
		}
	}
	if _, err := os.Stat(".env"); err == nil {
		file, loadErr := ini.Load(".env")
		if loadErr != nil {
			return values, sources, fmt.Errorf("cannot parse working-directory .env")
		}
		for _, key := range securityInputKeys {
			if file.Section("").HasKey(key) {
				v := file.Section("").Key(key).String()
				// Preserve the established empty .env fallback for account strings.
				if (key == "FLARE_USER" || key == "FLARE_PASS") && strings.TrimSpace(v) == "" {
					continue
				}
				values[key], sources[key] = v, ".env"
			}
		}
	} else if !os.IsNotExist(err) {
		return values, sources, fmt.Errorf("cannot read working-directory .env")
	}
	return values, sources, nil
}

func validateSecurityInputs(values map[string]string) error {
	if raw, exists := values["FLARE_DISABLE_LOGIN"]; exists {
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("FLARE_DISABLE_LOGIN must be a boolean")
		}
	}
	if raw, exists := values["FLARE_VISIBILITY"]; exists {
		v := strings.ToUpper(strings.TrimSpace(raw))
		if v != "DEFAULT" && v != "PRIVATE" {
			return fmt.Errorf("FLARE_VISIBILITY must be DEFAULT or PRIVATE")
		}
	}
	if raw := values["FLARE_CREDENTIAL_MODE"]; raw != "" && raw != "local" && raw != "deployment" {
		return fmt.Errorf("FLARE_CREDENTIAL_MODE must be local or deployment")
	}
	return nil
}
