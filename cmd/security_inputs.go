package cmd

import (
	"fmt"
	"os"
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

const acceptedBooleans = "true, false, 1, 0, yes, no, on or off"

// inputError reports a runtime input that cannot be interpreted. Its text names
// the input and the accepted values, and never repeats the supplied value.
type inputError struct {
	key      string
	accepted string
	source   string
}

func (e *inputError) Error() string {
	if e.source != "" {
		return fmt.Sprintf("%s (from %s) must be %s", e.key, e.source, e.accepted)
	}
	return fmt.Sprintf("%s must be %s", e.key, e.accepted)
}

// normalizeInput removes surrounding whitespace and one pair of matching quotes.
// Deployment tools can pass the quotes through literally, as the Compose list
// form `- FLARE_DISABLE_LOGIN="true"` does.
func normalizeInput(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

// parseBoolInput accepts the spellings of strconv.ParseBool and of the .env
// parser in any letter case, so every configuration source reads a boolean
// the same way.
func parseBoolInput(raw string) (value, ok bool) {
	switch strings.ToLower(normalizeInput(raw)) {
	case "1", "t", "true", "y", "yes", "on":
		return true, true
	case "0", "f", "false", "n", "no", "off":
		return false, true
	}
	return false, false
}

func normalizeVisibility(raw string) string { return strings.ToUpper(normalizeInput(raw)) }

func normalizeCredentialMode(raw string) string { return strings.ToLower(normalizeInput(raw)) }

func validateSecurityInputs(values map[string]string) error {
	if raw, exists := values["FLARE_DISABLE_LOGIN"]; exists {
		if _, ok := parseBoolInput(raw); !ok {
			return &inputError{key: "FLARE_DISABLE_LOGIN", accepted: "a boolean (" + acceptedBooleans + ")"}
		}
	}
	if raw, exists := values["FLARE_VISIBILITY"]; exists {
		if v := normalizeVisibility(raw); v != "DEFAULT" && v != "PRIVATE" {
			return &inputError{key: "FLARE_VISIBILITY", accepted: "DEFAULT or PRIVATE"}
		}
	}
	if mode := normalizeCredentialMode(values["FLARE_CREDENTIAL_MODE"]); mode != "" && mode != "local" && mode != "deployment" {
		return &inputError{key: "FLARE_CREDENTIAL_MODE", accepted: "local or deployment"}
	}
	return nil
}
