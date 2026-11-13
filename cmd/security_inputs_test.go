package cmd

import (
	"errors"
	"os"
	"testing"

	flags "github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

const booleanDiagnostic = "a boolean (true, false, 1, 0, yes, no, on or off)"

// parseWith runs Parse in an empty working directory with only the given
// environment, .env content and arguments.
func parseWith(t *testing.T, environment map[string]string, dotenv string, args ...string) model.Flags {
	t.Helper()
	t.Chdir(t.TempDir())
	cleared := append([]string{"FLARE_EDITOR", "FLARE_COOKIE_SECURE", "FLARE_GUIDE", "FLARE_OFFLINE", "FLARE_MINI_REQUEST", "FLARE_PORT"}, securityInputKeys...)
	for _, key := range cleared {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	for key, value := range environment {
		t.Setenv(key, value)
	}
	if dotenv != "" {
		require.NoError(t, os.WriteFile(".env", []byte(dotenv), 0600))
	}
	originalArgs, originalFlags := os.Args, define.AppFlags
	t.Cleanup(func() { os.Args, define.AppFlags = originalArgs, originalFlags })
	os.Args = append([]string{"flare"}, args...)
	return Parse()
}

// parseFrom supplies the same settings through one configuration source.
func parseFrom(t *testing.T, source string, settings ...[3]string) model.Flags {
	t.Helper()
	environment, dotenv, args := map[string]string{}, "", []string{}
	for _, setting := range settings {
		key, flag, value := setting[0], setting[1], setting[2]
		switch source {
		case "environment":
			environment[key] = value
		case ".env":
			dotenv += key + "=" + value + "\n"
		default:
			args = append(args, "--"+flag+"="+value)
		}
	}
	return parseWith(t, environment, dotenv, args...)
}

var inputSources = []string{"environment", ".env", "CLI"}

func TestNormalizeInput(t *testing.T) {
	for raw, want := range map[string]string{
		"true":        "true",
		"  true\t":    "true",
		`"true"`:      "true",
		`'true'`:      "true",
		` " true " `:  "true",
		`"true'`:      `"true'`,
		`"`:           `"`,
		`""`:          "",
		"":            "",
		`"'true'"`:    `'true'`,
		`pass"word`:   `pass"word`,
		"two  words ": "two  words",
	} {
		require.Equal(t, want, normalizeInput(raw), "input %q", raw)
	}
}

func TestParseBoolInput(t *testing.T) {
	for _, raw := range []string{"1", "t", "T", "true", "TRUE", "True", "y", "Y", "yes", "YES", "on", "On", ` "true" `, "'ON'"} {
		value, ok := parseBoolInput(raw)
		require.True(t, ok, "input %q", raw)
		require.True(t, value, "input %q", raw)
	}
	for _, raw := range []string{"0", "f", "F", "false", "FALSE", "False", "n", "N", "no", "No", "off", "OFF", ` 'false' `, `"0"`} {
		value, ok := parseBoolInput(raw)
		require.True(t, ok, "input %q", raw)
		require.False(t, value, "input %q", raw)
	}
	for _, raw := range []string{"", " ", `""`, "2", "maybe", "tru e", "enabled", `"'true'"`, "yes!"} {
		_, ok := parseBoolInput(raw)
		require.False(t, ok, "input %q", raw)
	}
}

func TestValidateSecurityInputsNormalizesAccessInputs(t *testing.T) {
	for _, values := range []map[string]string{
		{"FLARE_DISABLE_LOGIN": `"true"`},
		{"FLARE_DISABLE_LOGIN": " No "},
		{"FLARE_DISABLE_LOGIN": "OFF"},
		{"FLARE_VISIBILITY": `"private"`},
		{"FLARE_VISIBILITY": " 'Default' "},
		{"FLARE_CREDENTIAL_MODE": `"Local"`},
		{"FLARE_CREDENTIAL_MODE": " DEPLOYMENT "},
		{"FLARE_CREDENTIAL_MODE": ""},
		{"FLARE_CREDENTIAL_MODE": `""`},
	} {
		require.NoError(t, validateSecurityInputs(values), "%v", values)
	}

	for _, tc := range []struct {
		key, raw, want string
	}{
		{"FLARE_DISABLE_LOGIN", "SENTINEL-maybe", "FLARE_DISABLE_LOGIN must be " + booleanDiagnostic},
		{"FLARE_DISABLE_LOGIN", "", "FLARE_DISABLE_LOGIN must be " + booleanDiagnostic},
		{"FLARE_DISABLE_LOGIN", `""`, "FLARE_DISABLE_LOGIN must be " + booleanDiagnostic},
		{"FLARE_VISIBILITY", "SENTINEL-public", "FLARE_VISIBILITY must be DEFAULT or PRIVATE"},
		{"FLARE_VISIBILITY", "", "FLARE_VISIBILITY must be DEFAULT or PRIVATE"},
		{"FLARE_CREDENTIAL_MODE", "SENTINEL-external", "FLARE_CREDENTIAL_MODE must be local or deployment"},
	} {
		err := validateSecurityInputs(map[string]string{tc.key: tc.raw})
		require.EqualError(t, err, tc.want, "%s=%q", tc.key, tc.raw)
		require.NotContains(t, err.Error(), "SENTINEL")
	}
}

func TestApplyExplicitInterpretsNormalizedInputs(t *testing.T) {
	f := startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": `"off"`, "FLARE_VISIBILITY": " 'private' ", "FLARE_CREDENTIAL_MODE": ` "Deployment" `})
	f.StartupDiagnostic = "stale diagnostic"
	require.NoError(t, applyExplicit(f))
	require.False(t, f.DisableLoginMode)
	require.Equal(t, "PRIVATE", f.Visibility)
	require.Equal(t, "deployment", f.RuntimeInputs["FLARE_CREDENTIAL_MODE"])
	require.Empty(t, f.StartupDiagnostic)

	f = startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "YES"})
	f.DisableLoginMode = false
	require.NoError(t, applyExplicit(f))
	require.True(t, f.DisableLoginMode)
}

func TestApplyExplicitRecordsPublicDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  *model.Flags
		source map[string]string
		want   string
	}{
		{
			name:  "configuration error",
			flags: &model.Flags{ConfigError: "unknown command-line flag --example"},
			want:  "unknown command-line flag --example",
		},
		{
			name:   "invalid input with a known source",
			flags:  startupFlags(map[string]string{"FLARE_VISIBILITY": "SENTINEL-public"}),
			source: map[string]string{"FLARE_VISIBILITY": ".env"},
			want:   "FLARE_VISIBILITY (from .env) must be DEFAULT or PRIVATE",
		},
		{
			name:  "invalid input without a recorded source",
			flags: startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "SENTINEL-maybe"}),
			want:  "FLARE_DISABLE_LOGIN must be " + booleanDiagnostic,
		},
		{
			name:  "invalid resolved visibility",
			flags: &model.Flags{Visibility: "SENTINEL-public"},
			want:  "FLARE_VISIBILITY must be DEFAULT or PRIVATE",
		},
		{
			name:  "conflicting access inputs",
			flags: startupFlags(map[string]string{"FLARE_DISABLE_LOGIN": "on", "FLARE_VISIBILITY": `"private"`}),
			want:  "FLARE_DISABLE_LOGIN conflicts with PRIVATE visibility; enable login",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.flags.InputSources = tc.source
			require.EqualError(t, applyExplicit(tc.flags), tc.want)
			require.Equal(t, tc.want, tc.flags.StartupDiagnostic)
			require.NotContains(t, tc.flags.StartupDiagnostic, "SENTINEL")
		})
	}
}

func TestDisableLoginAcceptsSharedSpellingsFromEverySource(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`"false"`, false}, {"'true'", true}, {"FALSE", false}, {"no", false}, {"Yes", true}, {"off", false}, {"ON", true}, {"0", false}, {"1", true},
	} {
		for _, source := range inputSources {
			t.Run(source+" "+tc.raw, func(t *testing.T) {
				resolved := parseFrom(t, source, [3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, tc.raw})
				require.Empty(t, resolved.ConfigError)
				require.Equal(t, tc.want, resolved.DisableLoginMode, "value resolved while parsing")
				require.Equal(t, source, resolved.InputSources["FLARE_DISABLE_LOGIN"])

				resolved.DisableLoginMode = !tc.want
				require.NoError(t, applyExplicit(&resolved))
				require.Equal(t, tc.want, resolved.DisableLoginMode, "value applied by the startup check")
				require.Empty(t, resolved.StartupDiagnostic)
			})
		}
	}
}

func TestVisibilityAcceptsQuotesAndCaseFromEverySource(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`"private"`, "PRIVATE"}, {"'PRIVATE'", "PRIVATE"}, {"Private", "PRIVATE"}, {`"default"`, "DEFAULT"}, {"Default", "DEFAULT"},
	} {
		for _, source := range inputSources {
			t.Run(source+" "+tc.raw, func(t *testing.T) {
				resolved := parseFrom(t, source,
					[3]string{"FLARE_VISIBILITY", _KEY_VISIBILITY, tc.raw},
					[3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, "false"},
				)
				require.Empty(t, resolved.ConfigError)
				require.Equal(t, tc.want, resolved.Visibility, "value resolved while parsing")

				require.NoError(t, applyExplicit(&resolved))
				require.Equal(t, tc.want, resolved.Visibility)
				require.False(t, resolved.DisableLoginMode)
			})
		}
	}
}

// A quoted but valid visibility must not take the invalid-value path, which
// switches the instance to PRIVATE with login enabled.
func TestQuotedDefaultVisibilityKeepsLoginDisabled(t *testing.T) {
	for _, source := range inputSources {
		t.Run(source, func(t *testing.T) {
			resolved := parseFrom(t, source, [3]string{"FLARE_VISIBILITY", _KEY_VISIBILITY, `"DEFAULT"`})
			require.Equal(t, "DEFAULT", resolved.Visibility)
			require.True(t, resolved.DisableLoginMode)
			require.NoError(t, applyExplicit(&resolved))
			require.Equal(t, "DEFAULT", resolved.Visibility)
			require.True(t, resolved.DisableLoginMode)
		})
	}
}

func TestUninterpretableAccessInputsBlockReadiness(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		setting      [3]string
		want         string
	}{
		{"login from environment", "environment", [3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, "SENTINEL-maybe"}, "FLARE_DISABLE_LOGIN (from environment) must be " + booleanDiagnostic},
		{"empty login from environment", "environment", [3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, ""}, "FLARE_DISABLE_LOGIN (from environment) must be " + booleanDiagnostic},
		{"login from .env", ".env", [3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, "SENTINEL-maybe"}, "FLARE_DISABLE_LOGIN (from .env) must be " + booleanDiagnostic},
		{"login from CLI", "CLI", [3]string{"FLARE_DISABLE_LOGIN", _KEY_DISABLE_LOGIN, "SENTINEL-maybe"}, "invalid value for command-line flag --disable_login; booleans accept " + acceptedBooleans},
		{"visibility from environment", "environment", [3]string{"FLARE_VISIBILITY", _KEY_VISIBILITY, "SENTINEL-public"}, "FLARE_VISIBILITY (from environment) must be DEFAULT or PRIVATE"},
		{"visibility from .env", ".env", [3]string{"FLARE_VISIBILITY", _KEY_VISIBILITY, "SENTINEL-public"}, "FLARE_VISIBILITY (from .env) must be DEFAULT or PRIVATE"},
		{"visibility from CLI", "CLI", [3]string{"FLARE_VISIBILITY", _KEY_VISIBILITY, "SENTINEL-public"}, "FLARE_VISIBILITY (from CLI) must be DEFAULT or PRIVATE"},
		{"credential mode from environment", "environment", [3]string{"FLARE_CREDENTIAL_MODE", "", "SENTINEL-external"}, "FLARE_CREDENTIAL_MODE (from environment) must be local or deployment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved := parseFrom(t, tc.source, tc.setting)
			prepared := PrepareSecurity(&resolved)
			defer closePreparation(t, prepared)

			require.Equal(t, "attention_required", prepared.Status)
			require.Equal(t, tc.want, prepared.Message)
			require.Equal(t, tc.want, resolved.StartupDiagnostic)
			require.NotContains(t, resolved.StartupDiagnostic, "SENTINEL")
			require.Nil(t, prepared.Store)
			_, err := os.Stat(".flare-security")
			require.True(t, os.IsNotExist(err), "a rejected input must not create security state")
		})
	}
}

func TestStartupAcceptsQuotedCredentialInputs(t *testing.T) {
	t.Chdir(t.TempDir())
	writeStartupFiles(t, startupPublic)
	inputs := map[string]string{"FLARE_DISABLE_LOGIN": `"false"`, "FLARE_PASS": "old", "FLARE_CREDENTIAL_MODE": `"Local"`}
	prepared := PrepareSecurity(startupFlags(inputs))
	defer closePreparation(t, prepared)
	require.Equal(t, "ready", prepared.Status, prepared.Message)
	require.Equal(t, "local", prepared.Store.Info().Mode)
}

func TestCommandLineErrorsNameTheFlagWithoutItsValue(t *testing.T) {
	const generic = "invalid command-line configuration"
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown flag", []string{"--no-such-flag=SENTINEL-value"}, "unknown command-line flag --no-such-flag"},
		{"unknown shorthand", []string{"-Z"}, "unknown command-line flag -Z"},
		{"unknown shorthand with attached text", []string{"-ZSENTINEL"}, "unknown command-line flag -Z"},
		{"missing value", []string{"--port"}, "command-line flag --port needs a value"},
		{"missing shorthand value", []string{"-p"}, "command-line flag --port needs a value"},
		{"invalid number", []string{"--port=SENTINEL"}, "invalid value for command-line flag --port"},
		{"invalid boolean", []string{"--cookie_secure=SENTINEL"}, "invalid value for command-line flag --cookie_secure; booleans accept " + acceptedBooleans},
		{"invalid standard boolean", []string{"--help=SENTINEL"}, "invalid value for command-line flag --help"},
		{"unprintable flag name", []string{"--SENTINEL$name=1"}, generic},
		{"malformed flag", []string{"--=SENTINEL"}, generic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalArgs := os.Args
			t.Cleanup(func() { os.Args = originalArgs })
			os.Args = append([]string{"flare"}, tc.args...)
			parsed, _ := GetCliFlags()
			require.Equal(t, tc.want, parsed.ConfigError)
			require.NotContains(t, parsed.ConfigError, "SENTINEL")
		})
	}
	require.Equal(t, generic, describeFlagError(errors.New("SENTINEL")))
}

func TestBooleanFlagsMatchStandardFlagsExceptAcceptedSpellings(t *testing.T) {
	var standardEditor, standardSecure bool
	standard := flags.NewFlagSet("standard", flags.ContinueOnError)
	standard.BoolVarP(&standardEditor, "enable_editor", "e", true, "enable the editor")
	standard.BoolVar(&standardSecure, "cookie_secure", false, "require secure cookies")

	var editor, secure, offline bool
	options := flags.NewFlagSet("tolerant", flags.ContinueOnError)
	boolVarP(options, &editor, "enable_editor", "e", true, "enable the editor")
	boolVarP(options, &secure, "cookie_secure", "", false, "require secure cookies")
	require.Equal(t, standard.FlagUsages(), options.FlagUsages(), "help output must not change")
	require.True(t, editor, "registration applies the default")
	require.False(t, secure)

	boolVarP(options, &offline, "offline", "o", false, "offline mode")
	require.NoError(t, options.Parse([]string{"--cookie_secure", "--enable_editor=Off", "-o"}))
	require.True(t, secure, "a bare flag means true")
	require.False(t, editor)
	require.True(t, offline)
	value, err := options.GetBool("enable_editor")
	require.NoError(t, err)
	require.False(t, value)

	require.NoError(t, options.Parse([]string{"-eo=no"}))
	require.True(t, editor, "grouped shorthand without a value means true")
	require.False(t, offline)

	var invalid *flags.InvalidValueError
	require.ErrorAs(t, options.Parse([]string{"--cookie_secure=maybe"}), &invalid)
	require.True(t, secure, "a rejected value leaves the flag unchanged")
}

func TestEnvironmentBooleansAcceptSharedSpellings(t *testing.T) {
	for key, value := range map[string]string{
		"FLARE_EDITOR":        `"false"`,
		"FLARE_COOKIE_SECURE": "ON",
		"FLARE_OFFLINE":       " yes ",
		"FLARE_MINI_REQUEST":  "'1'",
		"FLARE_DISABLE_LOGIN": "No",
		"FLARE_GUIDE":         "maybe",
	} {
		t.Setenv(key, value)
	}
	got := ParseEnvVars()
	require.False(t, got.EnableEditor)
	require.True(t, got.CookieSecure)
	require.True(t, got.EnableOfflineMode)
	require.True(t, got.EnableMinimumRequest)
	require.False(t, got.DisableLoginMode)
	require.Equal(t, define.DEFAULT_ENABLE_GUIDE, got.EnableGuide, "an uninterpretable switch keeps its default")
}

func TestEnvFileBooleansAcceptSharedSpellings(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile(".env", []byte("FLARE_EDITOR=Off\nFLARE_COOKIE_SECURE=TRUE\nFLARE_MINI_REQUEST=\"yes\"\nFLARE_DISABLE_LOGIN=no\nFLARE_GUIDE=maybe\nFLARE_OFFLINE=\n"), 0600))
	base := model.Flags{Port: 5005, EnableEditor: true, EnableGuide: true, EnableOfflineMode: true, DisableLoginMode: true, Visibility: "DEFAULT"}
	got := ParseEnvFile(base)
	require.False(t, got.EnableEditor)
	require.True(t, got.CookieSecure)
	require.True(t, got.EnableMinimumRequest)
	require.False(t, got.DisableLoginMode)
	require.True(t, got.EnableGuide, "an uninterpretable entry keeps the earlier value")
	require.True(t, got.EnableOfflineMode, "an empty entry keeps the earlier value")
}

func TestParseCLINormalizesQuotedVisibility(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })
	os.Args = []string{"flare"}
	got := parseCLI(model.Flags{Port: 5005, Visibility: `"default"`, DisableLoginMode: true})
	require.Equal(t, "DEFAULT", got.Visibility)
	require.True(t, got.DisableLoginMode)
}
