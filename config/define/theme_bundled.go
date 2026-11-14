package define

import (
	"embed"
	"fmt"

	"github.com/soulteary/flare/config/model"
)

// Bundled themes are theme files. They are read with the parser and the
// checks that apply to every theme file.
//
//go:embed themes/*.yml
var bundledThemeFiles embed.FS

// bundledThemeOrder is the order of the theme page. The first theme is the
// default and the fallback for a theme name that cannot be used.
var bundledThemeOrder = []string{
	"blackboard", "gazette", "espresso", "cab", "cloud", "lime", "white", "tron",
	"blues", "passion", "chalk", "paper", "neon", "pumpkin", "onedark",
}

// loadBundledThemes reads the embedded theme files in their display order.
func loadBundledThemes() ([]model.Theme, error) {
	themes := make([]model.Theme, 0, len(bundledThemeOrder))
	for _, name := range bundledThemeOrder {
		raw, err := bundledThemeFiles.ReadFile("themes/" + name + ".yml")
		if err != nil {
			return nil, fmt.Errorf("bundled theme %s: %w", name, err)
		}
		file, err := model.ParseThemeFile(raw)
		if err != nil {
			return nil, fmt.Errorf("bundled theme %s: %w", name, err)
		}
		themes = append(themes, file.Theme(name))
	}
	return themes, nil
}

// getDefaultThemePalettes returns the bundled themes. The files are compiled
// into the binary and covered by tests, so a failure is a build defect.
func getDefaultThemePalettes() []model.Theme {
	themes, err := loadBundledThemes()
	if err != nil {
		panic(err)
	}
	return themes
}
