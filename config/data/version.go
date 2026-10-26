package data

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/soulteary/flare/config/model"
	version "github.com/soulteary/version-kit/v4"
)

// ConfigurationVersionState describes the stored writer relative to this binary.
// Unknown/development versions are never ordered lexicographically.
func ConfigurationVersionState(stored string) string {
	current := applicationVersion()
	if stored == "" {
		return "legacy"
	}
	if stored == current {
		return "same"
	}
	a, b := parseAppVersion(stored), parseAppVersion(current)
	if a == nil || b == nil {
		return "unknown"
	}
	cmp := 0
	for i := 1; i <= 3; i++ {
		if cmp = compareNumeric(a[i], b[i]); cmp != 0 {
			break
		}
	}
	if cmp == 0 {
		cmp = comparePrerelease(a[4], b[4])
	}
	if cmp < 0 {
		return "upgrade"
	}
	if cmp > 0 {
		return "downgrade"
	}
	return "same"
}

func applicationVersion() string {
	v := strings.TrimSpace(version.Version)
	if v == "" {
		return "dev"
	}
	return v
}

var appVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

func parseAppVersion(v string) []string {
	parts := appVersionPattern.FindStringSubmatch(v)
	if parts == nil {
		return nil
	}
	for i := 4; i <= 5; i++ {
		if parts[i] != "" {
			for _, p := range strings.Split(parts[i], ".") {
				if p == "" || (i == 4 && numeric(p) && len(p) > 1 && p[0] == '0') {
					return nil
				}
			}
		}
	}
	return parts
}
func numeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func compareNumeric(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	aa, bb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aa) && i < len(bb); i++ {
		x, y := aa[i], bb[i]
		cmp := 0
		if numeric(x) && numeric(y) {
			cmp = compareNumeric(x, y)
		} else if numeric(x) {
			cmp = -1
		} else if numeric(y) {
			cmp = 1
		} else {
			cmp = strings.Compare(x, y)
		}
		if cmp != 0 {
			return cmp
		}
	}
	if len(aa) < len(bb) {
		return -1
	}
	return 1
}
func versionForSave(original model.Application) (string, error) {
	state := ConfigurationVersionState(original.AppVersion)
	if state == "downgrade" {
		return "", fmt.Errorf("configuration was saved by newer Flare %s; current version %s cannot overwrite it", original.AppVersion, applicationVersion())
	}
	if state == "unknown" {
		return original.AppVersion, nil
	}
	return applicationVersion(), nil
}
