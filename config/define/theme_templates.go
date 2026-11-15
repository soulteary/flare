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

// themeTemplateFiles names the templates a theme can bring, in the order
// they are looked at: those of the pages, then those of the items.
func themeTemplateFiles() []string {
	files := make([]string, 0, len(themetemplate.PageKinds)+len(themetemplate.ItemKinds))
	for _, kind := range themetemplate.PageKinds {
		files = append(files, kind.File())
	}
	for _, kind := range themetemplate.ItemKinds {
		files = append(files, kind.File())
	}
	return files
}

// themeItemKind returns the kind of item a template is for, by the name of
// its file, and false for the template of a page.
func themeItemKind(file string) (themetemplate.ItemKind, bool) {
	for _, kind := range themetemplate.ItemKinds {
		if kind.File() == file {
			return kind, true
		}
	}
	return "", false
}

// readThemeTemplates reads the templates a theme that is a directory brings:
// templates/home.html, applications.html and bookmarks.html for the pages,
// and app.html, bookmark.html and group.html for the items. Other entries of
// that directory are left alone. A template that cannot be read is reported
// and the others are still read.
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
	for _, name := range themeTemplateFiles() {
		file := filepath.Join(theme, model.ThemeTemplateDirectory, name)
		if _, err = root.Lstat(file); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		raw, reason := readThemeDirectoryFile(root, file, MaximumThemeTemplateBytes)
		if reason == "" && !utf8.Valid(raw) {
			reason = ThemeProblemTemplateEncoding
		}
		if reason != "" {
			problems = append(problems, ThemeProblem{File: shown + "/" + name, Reason: reason})
			continue
		}
		digest := sha256.Sum256(raw)
		templates = append(templates, model.ThemeTemplate{Name: name, Source: string(raw), Hash: hex.EncodeToString(digest[:16])})
	}
	return templates, problems
}

// checkThemeTemplates compiles each template with the options of the theme
// at their defaults, and returns those that compile with the remarks about
// the others: the template of a page once, for a page with every module
// turned on, and the template of an item for every combination of what an
// entry has. Which way a template goes for other values is found when a page
// is rendered for them.
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
		var err error
		if item, isItem := themeItemKind(template.Name); isItem {
			for _, values := range themetemplate.ItemValueSets(item) {
				values.Lang, values.Options = "en", options
				if _, err = themetemplate.CompileItem(template.Source, item, values, funcs); err != nil {
					break
				}
			}
		} else {
			kind := themetemplate.PageKind(template.Name[:len(template.Name)-len(".html")])
			values := themetemplate.PageValues{Lang: "en", Kind: kind, Modules: modules, Options: options}
			_, err = themetemplate.CompilePage(template.Source, kind, values, funcs)
		}
		if err != nil {
			problems = append(problems, themeTemplateProblem(theme.Name, template.Name, err))
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
