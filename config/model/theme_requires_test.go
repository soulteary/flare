package model

import (
	"errors"
	"strings"
	"testing"
)

func TestThemeFileNamesTheReleaseItNeeds(t *testing.T) {
	for _, release := range []string{"0.7.0", "0.0.0", "0.6.15", "10.20.30", "1.0.0", "999999.0.0"} {
		file, unused, err := ReadThemeFile([]byte(validThemeFile + "requires: " + release + "\n"))
		if err != nil || file.Requires != release || !unused.Empty() {
			t.Errorf("%s: %+v %+v %v", release, file.Requires, unused, err)
		}
		// The bundled themes are read in full, and requires is a known key.
		if _, err = ParseThemeFile([]byte(validThemeFile + "requires: " + release + "\n")); err != nil {
			t.Errorf("%s, read in full: %v", release, err)
		}
	}
	// Without the key a theme names no release.
	if file, _, err := ReadThemeFile([]byte(validThemeFile)); err != nil || file.Requires != "" {
		t.Fatalf("no requires: %q %v", file.Requires, err)
	}

	// A release is three numbers and nothing else.
	for written, subject := range map[string]string{
		`""`:                                    "",
		`">=0.7.0"`:                             ">=0.7.0",
		"v0.7.0":                                "v0.7.0",
		"0.7":                                   "0.7",
		"1":                                     "1",
		"0.7.0.1":                               "0.7.0.1",
		"0.7.x":                                 "0.7.x",
		"0.07.0":                                "0.07.0",
		"00.7.0":                                "00.7.0",
		"0.7.0-rc.1":                            "0.7.0-rc.1",
		"0.7.0+build":                           "0.7.0+build",
		`" 0.7.0"`:                              " 0.7.0",
		`"0.7.0 "`:                              "0.7.0 ",
		"0..0":                                  "0..0",
		"0.7.-1":                                "0.7.-1",
		"a.b.c":                                 "a.b.c",
		"1.2.3456789":                           "1.2.3456789",
		"latest":                                "latest",
		"true":                                  "true",
		"~":                                     "",
		`"٠.٧.٠"`:                               "٠.٧.٠",
		`"0.7.0\n"`:                             "0.7.0\n",
		`"` + strings.Repeat("9", 40) + `.0.0"`: strings.Repeat("9", 32) + "…",
	} {
		_, _, err := ReadThemeFile([]byte(validThemeFile + "requires: " + written + "\n"))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeRequiresValue) || rejection.Subject != subject {
			t.Errorf("requires: %s: %v", written, err)
		}
	}
	// A value of another shape is not read at all.
	for _, written := range []string{"[0, 7, 0]", "{release: 0.7.0}"} {
		_, _, err := ReadThemeFile([]byte(validThemeFile + "requires: " + written + "\n"))
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeSyntax) || !strings.HasPrefix(rejection.Subject, "requires: ") {
			t.Errorf("requires: %s: %v", written, err)
		}
	}
	// A theme that is put together in code is held to the same form.
	file, _, _ := ReadThemeFile([]byte(validThemeFile))
	file.Requires = ">=0.7.0"
	if err := file.Validate(); !errors.Is(err, ErrThemeRequiresValue) {
		t.Fatalf("validate: %v", err)
	}
}

func TestThemeFileIsNotUsedByAnEarlierRelease(t *testing.T) {
	for _, tc := range []struct {
		requires, version string
		used              bool
	}{
		{"0.7.0", "0.7.0", true},
		{"0.7.0", "0.7.1", true},
		{"0.7.0", "1.0.0", true},
		{"0.7.1", "0.7.0", false},
		{"0.8.0", "0.7.9", false},
		{"1.0.0", "0.99.99", false},
		// Numbers are compared as numbers.
		{"0.9.0", "0.10.0", true},
		{"0.10.0", "0.9.9", false},
		{"0.0.10", "0.0.9", false},
		// The version of an application can carry a v and a suffix, and
		// counts as the release its numbers name.
		{"0.7.0", "v0.7.0", true},
		{"0.7.1", "v0.7.0", false},
		{"0.7.0", "0.7.0-rc.1", true},
		{"0.7.1", "0.7.0-rc.1", false},
		{"0.7.0", "0.7.0+build.5", true},
		{"0.7.0", "v0.7.0-rc.1+build", true},
		// A version that is not that of a release meets every requirement.
		{"99.0.0", "dev", true},
		{"99.0.0", "", true},
		{"99.0.0", "unknown", true},
		{"99.0.0", "1.0", true},
		{"99.0.0", "0.7.0.1", true},
		{"99.0.0", "v", true},
		{"99.0.0", "0.6.15", false},
	} {
		file, _, err := ReadThemeFileFor([]byte(validThemeFile+"requires: "+tc.requires+"\n"), tc.version)
		if tc.used {
			if err != nil || file.Requires != tc.requires {
				t.Errorf("requires %s on %q: %v", tc.requires, tc.version, err)
			}
			continue
		}
		var rejection *ThemeFileError
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeRequires) || rejection.Subject != tc.requires {
			t.Errorf("requires %s on %q: %v", tc.requires, tc.version, err)
		}
	}
	// A file without requires is used by every release.
	if _, _, err := ReadThemeFileFor([]byte(validThemeFile), "0.0.1"); err != nil {
		t.Fatal(err)
	}
}

// What a file says about itself comes first: a file for a later release can
// hold what this release cannot read or does not accept, and the reason given
// for it is the release it needs.
func TestThemeFileForALaterReleaseIsReportedAsSuch(t *testing.T) {
	later := "requires: 0.9.0\n"
	for name, tc := range map[string]struct {
		file   string
		reason error
	}{
		"a token value of a later release":    {validThemeFile + "  radius: 4em\n" + later, ErrThemeRequires},
		"a shape of a later release":          {"contract: 1\ntokens: red\n" + later, ErrThemeRequires},
		"colors missing":                      {"contract: 1\n" + later, ErrThemeRequires},
		"a name of another shape":             {validThemeFile + "name: [a, b]\n" + later, ErrThemeRequires},
		"another contract":                    {strings.Replace(validThemeFile, "contract: 1", "contract: 2", 1) + later, ErrThemeContract},
		"no contract":                         {strings.TrimPrefix(validThemeFile, "contract: 1\n") + later, ErrThemeContract},
		"another contract with another shape": {"contract: 2\ntokens: red\n", ErrThemeContract},
		"a contract that is no number":        {"contract: one\ntokens: red\n" + later, ErrThemeSyntax},
	} {
		_, unused, err := ReadThemeFileFor([]byte(tc.file), "0.8.0")
		if !errors.Is(err, tc.reason) {
			t.Errorf("%s: %v", name, err)
		}
		if !unused.Empty() {
			t.Errorf("%s: parts are listed as unused: %+v", name, unused)
		}
	}
	// The parts a file for a later release leaves unused are not listed:
	// the file is not used at all.
	_, unused, err := ReadThemeFileFor([]byte(validThemeFile+"sounds:\n  - id: chime\n"+later), "0.8.0")
	if !errors.Is(err, ErrThemeRequires) || !unused.Empty() {
		t.Fatalf("%v %+v", err, unused)
	}
	// On a release that meets the requirement the file is read as any other.
	_, unused, err = ReadThemeFileFor([]byte(validThemeFile+"sounds:\n  - id: chime\n"+later), "0.9.0")
	if err != nil || len(unused.Keys) != 1 || unused.Keys[0] != "sounds" {
		t.Fatalf("%v %+v", err, unused)
	}
	if _, _, err = ReadThemeFileFor([]byte(validThemeFile+"  radius: 4em\n"+later), "0.9.0"); !errors.Is(err, ErrThemeTokenValue) {
		t.Fatalf("a value that is not accepted on a release that meets the requirement: %v", err)
	}
}
