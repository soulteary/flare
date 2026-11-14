package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Safe mode for themes is read from every source of the configuration, with
// the spellings and the precedence of the other switches.
func TestThemeSafeModeFromEverySource(t *testing.T) {
	on := map[string]string{"FLARE_THEME_SAFE_MODE": "true"}
	for name, tc := range map[string]struct {
		environment map[string]string
		dotenv      string
		args        []string
		want        bool
	}{
		"default":                           {nil, "", nil, false},
		"environment":                       {on, "", nil, true},
		"environment, another spelling":     {map[string]string{"FLARE_THEME_SAFE_MODE": " On "}, "", nil, true},
		"environment, off":                  {map[string]string{"FLARE_THEME_SAFE_MODE": "0"}, "", nil, false},
		"environment, not a boolean":        {map[string]string{"FLARE_THEME_SAFE_MODE": "maybe"}, "", nil, false},
		"env file":                          {nil, "FLARE_THEME_SAFE_MODE=yes\n", nil, true},
		"env file over the environment":     {on, "FLARE_THEME_SAFE_MODE=false\n", nil, false},
		"command line":                      {nil, "", []string{"--theme_safe_mode"}, true},
		"command line with a value":         {nil, "", []string{"--theme_safe_mode=on"}, true},
		"command line over the environment": {on, "", []string{"--theme_safe_mode=false"}, false},
		"command line over the env file":    {nil, "FLARE_THEME_SAFE_MODE=false\n", []string{"--theme_safe_mode"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("FLARE_THEME_SAFE_MODE", "")
			got := parseWith(t, tc.environment, tc.dotenv, tc.args...)
			require.Equal(t, tc.want, got.ThemeSafeMode)
		})
	}
}
