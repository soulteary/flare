package cmd

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

func TestMalformedServicePortPreservesIndependentSettings(t *testing.T) {
	t.Setenv("FLARE_PORT", "tcp://10.0.0.1:5005")
	t.Setenv("FLARE_DISABLE_LOGIN", "false")
	t.Setenv("FLARE_VISIBILITY", "PRIVATE")
	t.Setenv("FLARE_USER", "configured-user")
	t.Setenv("FLARE_PASS", "configured-password")
	t.Setenv("FLARE_COOKIE_SECURE", "true")
	t.Setenv("FLARE_OFFLINE", "true")
	got := ParseEnvVars()
	if got.Port != define.DEFAULT_PORT || got.DisableLoginMode || got.Visibility != "PRIVATE" || got.User != "configured-user" || got.Pass != "configured-password" || !got.CookieSecure || !got.EnableOfflineMode {
		t.Fatalf("independent options lost: port=%d loginDisabled=%v visibility=%q cookieSecure=%v offline=%v", got.Port, got.DisableLoginMode, got.Visibility, got.CookieSecure, got.EnableOfflineMode)
	}
}

func TestVisibilityInvalidFailsClosedAfterAllOverrides(t *testing.T) {
	for _, tc := range []struct {
		name     string
		base     string
		args     []string
		want     string
		disabled bool
	}{
		{"invalid environment", "0", nil, "PRIVATE", false},
		{"empty environment", "", nil, "PRIVATE", false},
		{"invalid cli overrides valid private", "PRIVATE", []string{"--visibility=public", "--disable_login=true"}, "PRIVATE", false},
		{"valid cli overrides invalid base", "0", []string{"--visibility=DEFAULT"}, "DEFAULT", true},
		{"case and whitespace", " private ", nil, "PRIVATE", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := os.Args
			os.Args = append([]string{"flare"}, tc.args...)
			defer func() { os.Args = before }()
			got := parseCLI(model.Flags{Port: 5005, Visibility: tc.base, DisableLoginMode: true})
			if got.Visibility != tc.want || got.DisableLoginMode != tc.disabled {
				t.Fatalf("visibility=%q disabled=%v", got.Visibility, got.DisableLoginMode)
			}
		})
	}
}
