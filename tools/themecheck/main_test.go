package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/themerepo"
)

const described = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\nversion: 1.0.0\nauthor: Someone\nlicense: MIT\n"

func TestThemecheck(t *testing.T) {
	origin, err := os.Getwd()
	require.NoError(t, err)
	// The tool leaves the working directory; the test comes back to it.
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(origin))
		define.ReloadUserThemes()
	})
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		return path
	}
	night := write("night.yml", described+"name: Night\n")
	harbor := write("harbor.yml", described+"name: Harbor\n")
	unnamed := write("unnamed.yml", described)
	call := func(args ...string) (int, string, string) {
		var out, report bytes.Buffer
		status := run(args, &out, &report)
		require.NoError(t, os.Chdir(origin))
		return status, out.String(), report.String()
	}

	status, out, report := call(night, harbor)
	require.Equal(t, 0, status)
	require.Empty(t, out)
	require.Equal(t, "ok    night.yml\nok    harbor.yml\n", report)

	// The index lists the themes in the order of their names.
	status, out, _ = call("-index", night, harbor)
	require.Equal(t, 0, status)
	var index themerepo.Index
	require.NoError(t, json.Unmarshal([]byte(out), &index))
	require.Equal(t, themerepo.IndexFormat, index.Format)
	require.Len(t, index.Themes, 2)
	require.Equal(t, []string{"harbor", "night"}, []string{index.Themes[0].Name, index.Themes[1].Name})
	require.Equal(t, "Night", index.Themes[1].Title)
	require.True(t, strings.HasSuffix(out, "}\n"))
	// It is the same in whatever order the files are given.
	again, reordered, _ := call("-index", harbor, night)
	require.Equal(t, 0, again)
	require.Equal(t, out, reordered)

	// A theme that does not meet the rules is named with the rule, and no
	// index is written.
	status, out, report = call("-index", harbor, unnamed)
	require.Equal(t, 1, status)
	require.Empty(t, out)
	require.Equal(t, "ok    harbor.yml\nFAIL  unnamed.yml: described: name\n", report)
	status, _, report = call(harbor, harbor)
	require.Equal(t, 1, status)
	require.Contains(t, report, "FAIL  a theme is listed more than once\nharbor\n")

	// What the tool cannot work with.
	status, _, report = call()
	require.Equal(t, 2, status)
	require.Contains(t, report, "usage: themecheck")
	status, _, _ = call(filepath.Join(dir, "absent.yml"))
	require.Equal(t, 2, status)
	status, _, _ = call("-unknown", harbor)
	require.Equal(t, 2, status)
	// Nothing is left in the directory of the files.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	// Nor in the directory the tool was called in: a theme is stored and
	// read in a directory of its own.
	require.NoDirExists(t, filepath.Join(origin, define.UserThemeDirectory))
}
