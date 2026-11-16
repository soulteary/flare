package themerepo

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
)

const (
	colors    = "contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n"
	described = colors + "name: Harbor\nversion: 1.2.0\nauthor: Someone\nlicense: MIT\n"
	homePage  = `<div>{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
		`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`
)

// emptyDirectory runs a test in a directory of its own, as Check asks.
func emptyDirectory(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	define.ReloadUserThemes()
	t.Cleanup(define.ReloadUserThemes)
}

func pack(t *testing.T, files ...string) []byte {
	t.Helper()
	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	for i := 0; i < len(files); i += 2 {
		part, err := writer.Create(files[i])
		require.NoError(t, err)
		_, err = part.Write([]byte(files[i+1]))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return packed.Bytes()
}

func stored(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(define.UserThemeDirectory)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// A theme that meets the rules has no finding and is listed with what its
// file says and what it brings. Nothing of it stays in the directory.
func TestCheckAcceptsAThemeOfTheRepository(t *testing.T) {
	emptyDirectory(t)
	raw := []byte(described + "requires: 0.7.0\nextends: tron\n")
	digest := sha256.Sum256(raw)
	entry, findings := Check("harbor.yml", raw)
	require.Empty(t, findings)
	require.Equal(t, Entry{
		Name: "harbor", File: "harbor.yml", Size: len(raw), SHA256: hex.EncodeToString(digest[:]),
		Title: "Harbor", Version: "1.2.0", Author: "Someone", License: "MIT", Requires: "0.7.0", Extends: "tron", Templates: []string{},
	}, entry)
	require.Empty(t, stored(t))

	archive := pack(t, "harbor/theme.yml", described, "harbor/theme.css", ".app-item { color: red }\n", "harbor/templates/home.html", homePage, "harbor/LICENSE.txt", "MIT")
	entry, findings = Check("harbor.zip", archive)
	require.Empty(t, findings)
	require.Equal(t, "harbor", entry.Name)
	require.True(t, entry.Stylesheet)
	require.Equal(t, []string{"home.html"}, entry.Templates)
	require.Equal(t, len(archive), entry.Size)
	require.Empty(t, stored(t))

	// The entry is written with every key, so the index has one form.
	written, err := json.Marshal(Entry{Templates: []string{}})
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"","file":"","size":0,"sha256":"","title":"","version":"","author":"","license":"","stylesheet":false,"assets":0,"templates":[]}`, string(written))
}

// Each rule, with a theme that breaks it and nothing else.
func TestCheckFindings(t *testing.T) {
	emptyDirectory(t)
	for name, tc := range map[string]struct {
		file   string
		raw    []byte
		rule   string
		detail string
	}{
		"another kind of file":        {"harbor.tar.gz", []byte(described), RuleFile, "harbor.tar.gz"},
		"a file without a kind":       {"harbor", []byte(described), RuleFile, "harbor"},
		"a name no theme can have":    {"Harbor.yml", []byte(described), RuleFile, "Harbor.yml"},
		"the name of a bundled theme": {"tron.yml", []byte(described), RuleFile, "tron.yml"},
		"no theme file":               {"harbor.yml", []byte("contract: 2\n"), RuleAccepted, ""},
		"no archive":                  {"harbor.zip", []byte(described), RuleAccepted, ""},
		"a stylesheet that loads":     {"harbor.zip", pack(t, "theme.yml", described, "theme.css", `@import "a.css";`), RuleAccepted, ""},
		"a template that is refused":  {"harbor.zip", pack(t, "theme.yml", described, "templates/home.html", "<script></script>"), RuleAccepted, "harbor/templates/home.html template_element script"},
		"a part that is not used":     {"harbor.yml", []byte(described + "future: 1\n"), RuleUnused, "future"},
		"a token that is not used":    {"harbor.yml", []byte(described + "modes:\n  light:\n    glow: 1px\n"), RuleUnused, "modes.light.glow"},
		"an entry that is left out":   {"harbor.zip", pack(t, "theme.yml", described, "preview.png", "x"), RuleLeftOut, "preview.png"},
		"no name":                     {"harbor.yml", []byte(colors + "version: 1.0.0\nauthor: Someone\nlicense: MIT\n"), RuleDescribed, "name"},
		"no version":                  {"harbor.yml", []byte(colors + "name: Harbor\nauthor: Someone\nlicense: MIT\n"), RuleDescribed, "version"},
		"no author":                   {"harbor.yml", []byte(colors + "name: Harbor\nversion: 1.0.0\nlicense: MIT\n"), RuleDescribed, "author"},
		"no license":                  {"harbor.yml", []byte(colors + "name: Harbor\nversion: 1.0.0\nauthor: Someone\n"), RuleLicense, ""},
		"a license with conditions":   {"harbor.yml", []byte(colors + "name: Harbor\nversion: 1.0.0\nauthor: Someone\nlicense: GPL-3.0-only\n"), RuleLicense, "GPL-3.0-only"},
		"a license in other words":    {"harbor.yml", []byte(colors + "name: Harbor\nversion: 1.0.0\nauthor: Someone\nlicense: mit\n"), RuleLicense, "mit"},
	} {
		entry, findings := Check(tc.file, tc.raw)
		require.Len(t, findings, 1, name)
		require.Equal(t, tc.rule, findings[0].Rule, name)
		if tc.detail != "" || tc.rule == RuleLicense {
			require.Equal(t, tc.detail, findings[0].Detail, name)
		}
		require.NotEmpty(t, findings[0].Detail+tc.rule, name)
		require.Equal(t, tc.file, entry.File, name)
		require.Len(t, entry.SHA256, 64, name)
		require.Empty(t, stored(t), name)
	}

	// Several at once are all named, in a fixed order.
	_, findings := Check("harbor.yml", []byte(colors+"future: 1\nlicense: Proprietary\n"))
	require.Equal(t, []Finding{
		{Rule: RuleDescribed, Detail: "author"}, {Rule: RuleDescribed, Detail: "name"}, {Rule: RuleDescribed, Detail: "version"},
		{Rule: RuleLicense, Detail: "Proprietary"}, {Rule: RuleUnused, Detail: "future"},
	}, findings)
}

// A theme of the repository is installed by itself: it is checked without
// the themes that were checked before it, so it cannot be based on one.
func TestCheckIsAboutOneTheme(t *testing.T) {
	emptyDirectory(t)
	_, findings := Check("harbor.yml", []byte(described))
	require.Empty(t, findings)
	_, findings = Check("night.yml", []byte("contract: 1\nextends: harbor\nname: Night\nversion: 1.0.0\nauthor: Someone\nlicense: MIT\n"))
	require.Len(t, findings, 1)
	require.Equal(t, RuleAccepted, findings[0].Rule)
	// The same file is checked twice with the same result.
	first, _ := Check("harbor.yml", []byte(described))
	second, findings := Check("harbor.yml", []byte(described))
	require.Empty(t, findings)
	require.Equal(t, first, second)
}

func TestNewIndex(t *testing.T) {
	index, err := NewIndex([]Entry{{Name: "night"}, {Name: "harbor"}, {Name: "zelda"}})
	require.NoError(t, err)
	require.Equal(t, IndexFormat, index.Format)
	require.Equal(t, []Entry{{Name: "harbor"}, {Name: "night"}, {Name: "zelda"}}, index.Themes)
	_, err = NewIndex([]Entry{{Name: "harbor", File: "harbor.yml"}, {Name: "night"}, {Name: "harbor", File: "harbor.zip"}})
	require.ErrorIs(t, err, ErrListedTwice)
	require.ErrorContains(t, err, "harbor")
	index, err = NewIndex(nil)
	require.NoError(t, err)
	written, err := json.Marshal(index)
	require.NoError(t, err)
	require.JSONEq(t, `{"format":1,"themes":[]}`, string(written))
}

// The example theme of the guide for authors of templates meets the rules
// of the repository, packed as its archive would be.
func TestExampleOfTheTemplateGuideMeetsTheRules(t *testing.T) {
	guide, err := os.ReadFile(filepath.Join("..", "..", "docs", "theme-template-guide.md"))
	require.NoError(t, err)
	emptyDirectory(t)
	var files []string
	for _, found := range regexp.MustCompile("(?s)\\*\\*`themes/harbor/([a-z./]+)`\\*\\*\n\n```[a-z]+\n(.*?)\n```\n").FindAllStringSubmatch(string(guide), -1) {
		files = append(files, "harbor/"+found[1], found[2]+"\n")
	}
	require.Len(t, files, 12)
	entry, findings := Check("harbor.zip", pack(t, files...))
	require.Empty(t, findings)
	require.Equal(t, "Harbor", entry.Title)
	require.Equal(t, "MIT", entry.License)
	require.True(t, entry.Stylesheet)
	require.Equal(t, []string{"home.html", "app.html", "bookmark.html", "group.html"}, entry.Templates)
}
