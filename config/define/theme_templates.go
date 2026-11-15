package define

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/themetemplate"
)

// MaximumThemeTemplateBytes is the largest template of a theme read.
const MaximumThemeTemplateBytes = 64 << 10

// Reasons a template of a theme is not accepted. The theme stays in use
// without it: the page the template is for keeps the markup of the
// application. A template that does not compile has the reason of the
// compiler behind ThemeProblemTemplate, such as template_element.
const (
	ThemeProblemTemplate         = "template_"
	ThemeProblemTemplateEncoding = "template_encoding"
)

// readThemeTemplates reads the page templates a theme that is a directory
// brings: templates/home.html, templates/applications.html and
// templates/bookmarks.html. Other entries of that directory are left alone.
// A template that cannot be read is reported and the others are still read.
func readThemeTemplates(root *os.Root, theme string) ([]model.ThemeTemplate, []ThemeProblem) {
	shown := theme + "/" + model.ThemeTemplateDirectory
	info, err := root.Lstat(filepath.Join(theme, model.ThemeTemplateDirectory))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, []ThemeProblem{{File: shown, Reason: ThemeProblemRead}}
	case info.Mode()&fs.ModeSymlink != 0:
		return nil, []ThemeProblem{{File: shown, Reason: ThemeProblemLink}}
	case !info.IsDir():
		// A file of that name is not a directory of templates.
		return nil, nil
	}
	var templates []model.ThemeTemplate
	var problems []ThemeProblem
	for _, kind := range themetemplate.PageKinds {
		file := filepath.Join(theme, model.ThemeTemplateDirectory, kind.File())
		if _, err = root.Lstat(file); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		raw, reason := readThemeDirectoryFile(root, file, MaximumThemeTemplateBytes)
		if reason == "" && !utf8.Valid(raw) {
			reason = ThemeProblemTemplateEncoding
		}
		if reason != "" {
			problems = append(problems, ThemeProblem{File: shown + "/" + kind.File(), Reason: reason})
			continue
		}
		digest := sha256.Sum256(raw)
		templates = append(templates, model.ThemeTemplate{Name: kind.File(), Source: string(raw), Hash: hex.EncodeToString(digest[:16])})
	}
	return templates, problems
}

// checkThemeTemplates compiles each template once, for a page with every
// module turned on and the options of the theme at their defaults, and
// returns those that compile with the remarks about the others. Which way a
// template goes for other values is found when a page is rendered for them.
func checkThemeTemplates(theme model.Theme, templates []model.ThemeTemplate) ([]model.ThemeTemplate, []ThemeProblem) {
	if len(templates) == 0 {
		return nil, nil
	}
	options := make(map[string]any, len(theme.Options))
	for _, option := range theme.Options {
		options[option.ID] = option.Default
	}
	modules := make(map[string]bool, len(themetemplate.PageSlotNames))
	for _, name := range themetemplate.PageSlotNames {
		modules[name[len(themetemplate.ReservedPrefix):]] = true
	}
	funcs := ThemeTemplateFuncs(theme, func(key string) string { return key })
	var accepted []model.ThemeTemplate
	var problems []ThemeProblem
	for _, template := range templates {
		kind := themetemplate.PageKind(template.Name[:len(template.Name)-len(".html")])
		values := themetemplate.PageValues{Lang: "en", Kind: kind, Modules: modules, Options: options}
		if _, err := themetemplate.CompilePage(template.Source, kind, values, funcs); err != nil {
			problem := themeTemplateProblem(theme.Name, template.Name, err)
			problems = append(problems, problem)
			continue
		}
		accepted = append(accepted, template)
	}
	return accepted, problems
}

// ThemeTemplateFuncs are the functions a page template of the theme is
// compiled with: the texts of the interface as t gives them, and the
// addresses of the assets of the theme. An asset is named as the stylesheet
// of the theme names it, without its directory.
func ThemeTemplateFuncs(theme model.Theme, t func(key string) string) themetemplate.PageFuncs {
	funcs := themetemplate.PageFuncs{T: t}
	if theme.Hash != "" && len(theme.Assets) > 0 {
		funcs.AssetPrefix = themeStylesheetBase + theme.Name + "/" + theme.Hash + "/" + model.ThemeAssetDirectory + "/"
	}
	funcs.Asset = func(name string) (string, error) {
		for _, asset := range theme.Assets {
			if asset.Name == name {
				return funcs.AssetPrefix + name, nil
			}
		}
		return "", errors.New("no asset " + name)
	}
	return funcs
}
