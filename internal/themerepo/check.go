// Package themerepo holds what the theme repository asks of a theme beyond
// what the application asks, and the index the repository lists its themes
// in. The application reads neither the repository nor its index: this is
// for the tool that checks a theme before it is listed.
package themerepo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

// IndexFormat is the version of the format of the index.
const IndexFormat = 1

// Licenses are the licenses a theme of the repository can have, by their
// SPDX identifiers: licenses that let a theme be used, changed and passed
// on without further conditions than naming its author.
var Licenses = []string{"0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "CC-BY-4.0", "CC0-1.0", "ISC", "MIT", "Unlicense"}

// Rules a theme of the repository has to meet. Each finding names one.
const (
	// RuleFile: the file is named <name>.yml or <name>.zip with a name a
	// theme of the themes directory can have.
	RuleFile = "file-name"
	// RuleAccepted: the application takes the theme in and has it in use,
	// with every template accepted.
	RuleAccepted = "accepted"
	// RuleUnused: the theme file has no part the application does not use.
	RuleUnused = "unused-part"
	// RuleLeftOut: an archive holds nothing but files of the theme.
	RuleLeftOut = "left-out"
	// RuleDescribed: the theme file gives a name, a version and an author.
	RuleDescribed = "described"
	// RuleLicense: the theme file names one of Licenses.
	RuleLicense = "license"
	// RuleAlone: the theme stands on its own or is based on a bundled
	// theme, so that it is installed by itself.
	RuleAlone = "stands-alone"
)

// Finding is one way a theme does not meet the rules of the repository.
type Finding struct {
	Rule   string `json:"rule"`
	Detail string `json:"detail"`
}

// Entry is a theme as the index lists it.
type Entry struct {
	// Name is the name the theme is installed under, and File the name of
	// its file in the repository: <name>.yml or <name>.zip.
	Name string `json:"name"`
	File string `json:"file"`
	// Size is the size of the file in bytes, and SHA256 its SHA-256 in
	// hexadecimal digits, which the application shows before it installs
	// a theme from an address.
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
	// Title, Version, Author and License are what the theme file says.
	Title   string `json:"title"`
	Version string `json:"version"`
	Author  string `json:"author"`
	License string `json:"license"`
	// Requires is the first release the theme works with, and Extends the
	// bundled theme it is based on; both can be empty.
	Requires string `json:"requires,omitempty"`
	Extends  string `json:"extends,omitempty"`
	// Stylesheet and Assets say what an archive brings, and Templates
	// names its templates. A theme with templates is read by a person
	// before it is listed.
	Stylesheet bool     `json:"stylesheet"`
	Assets     int      `json:"assets"`
	Templates  []string `json:"templates"`
}

// Index is the list of the themes of the repository, in the order of their
// names.
type Index struct {
	Format int     `json:"format"`
	Themes []Entry `json:"themes"`
}

// Check reads a theme as the application does and returns how the index
// lists it, with every way it does not meet the rules of the repository.
// file is the name of the file, <name>.yml or <name>.zip, and raw its
// content.
//
// The theme is stored in the themes directory of the working directory, as
// an import stores it, read there and removed again. The working directory
// is therefore one that is used for nothing else, with no themes directory
// or an empty one, so that the theme is checked by itself.
func Check(file string, raw []byte) (entry Entry, findings []Finding) {
	digest := sha256.Sum256(raw)
	entry = Entry{File: file, Size: len(raw), SHA256: hex.EncodeToString(digest[:]), Templates: []string{}}
	name, archive := strings.CutSuffix(file, ".zip")
	if !archive {
		var single bool
		if name, single = strings.CutSuffix(file, ".yml"); !single {
			return entry, []Finding{{Rule: RuleFile, Detail: file}}
		}
	}
	if !define.IsUserThemeName(name) {
		return entry, []Finding{{Rule: RuleFile, Detail: file}}
	}
	entry.Name = name

	manifest := raw
	if archive {
		leftOut, err := define.StoreUserThemeArchive(name, bytes.NewReader(raw), int64(len(raw)), false)
		if err != nil {
			return entry, []Finding{{Rule: RuleAccepted, Detail: err.Error()}}
		}
		for _, left := range leftOut {
			findings = append(findings, Finding{Rule: RuleLeftOut, Detail: left})
		}
		stored, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, name, model.ThemeManifestName))
		if err != nil {
			return entry, []Finding{{Rule: RuleAccepted, Detail: err.Error()}}
		}
		manifest = stored
	} else if err := define.StoreUserTheme(name, raw, false); err != nil {
		return entry, []Finding{{Rule: RuleAccepted, Detail: err.Error()}}
	}
	defer func() {
		// The theme leaves the directory as it was found. One that stays
		// would be read with the next theme that is checked.
		for _, stored := range []string{name, name + ".yml"} {
			if err := os.RemoveAll(filepath.Join(define.UserThemeDirectory, stored)); err != nil {
				findings = append(findings, Finding{Rule: RuleAccepted, Detail: "the theme could not be removed again: " + err.Error()})
			}
		}
		define.ReloadUserThemes()
	}()

	themes, problems, notices := define.UserThemes()
	at := slices.IndexFunc(themes, func(theme model.Theme) bool { return theme.Name == name })
	if at < 0 {
		findings = append(findings, Finding{Rule: RuleAccepted, Detail: "the theme is not in use"})
	}
	for _, problem := range problems {
		findings = append(findings, Finding{Rule: RuleAccepted, Detail: strings.TrimSpace(problem.File + " " + problem.Reason + " " + problem.Subject)})
	}
	for _, notice := range notices {
		for _, part := range append(slices.Clone(notice.Unused.Keys), notice.Unused.Tokens...) {
			findings = append(findings, Finding{Rule: RuleUnused, Detail: part})
		}
	}
	if at < 0 {
		return entry, findings
	}
	theme := themes[at]
	entry.Title, entry.Version, entry.Author, entry.License = theme.Title.For("en"), theme.Version, theme.Author, theme.License
	entry.Extends, entry.Stylesheet, entry.Assets = theme.Extends, theme.Stylesheet != "", len(theme.Assets)
	for _, template := range theme.Templates {
		entry.Templates = append(entry.Templates, template.Name)
	}
	if written, _, err := model.ReadThemeFile(manifest); err == nil {
		entry.Requires = written.Requires
	}
	for field, value := range map[string]string{"name": entry.Title, "version": entry.Version, "author": entry.Author} {
		if value == "" {
			findings = append(findings, Finding{Rule: RuleDescribed, Detail: field})
		}
	}
	if !slices.Contains(Licenses, entry.License) {
		findings = append(findings, Finding{Rule: RuleLicense, Detail: entry.License})
	}
	if entry.Extends != "" && define.IsUserThemeName(entry.Extends) {
		findings = append(findings, Finding{Rule: RuleAlone, Detail: entry.Extends})
	}
	slices.SortFunc(findings, func(a, b Finding) int { return strings.Compare(a.Rule+"\x00"+a.Detail, b.Rule+"\x00"+b.Detail) })
	return entry, findings
}

// ErrListedTwice reports a name that more than one file of the repository
// has, as a.yml and a.zip do.
var ErrListedTwice = errors.New("a theme is listed more than once")

// NewIndex lists entries in the order of their names. A name can be listed
// once: an error names the first that is there again.
func NewIndex(entries []Entry) (Index, error) {
	index := Index{Format: IndexFormat, Themes: slices.Clone(entries)}
	slices.SortStableFunc(index.Themes, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	for i := 1; i < len(index.Themes); i++ {
		if index.Themes[i].Name == index.Themes[i-1].Name {
			return Index{}, errors.Join(ErrListedTwice, errors.New(index.Themes[i].Name))
		}
	}
	if index.Themes == nil {
		index.Themes = []Entry{}
	}
	return index, nil
}
