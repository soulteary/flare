package define

import (
	"slices"

	"github.com/soulteary/flare/config/model"
)

// SelectableThemes names the themes that can be selected, in the order of
// the theme page: the bundled themes, the custom colors once a palette is
// stored for them, and the themes of the themes directory.
func SelectableThemes() []string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	names := make([]string, 0, len(ThemePalettes)+1+len(userThemeState.order))
	for _, theme := range ThemePalettes {
		names = append(names, theme.Name)
	}
	if themeCustomSet {
		names = append(names, model.ThemeCustomName)
	}
	return append(names, userThemeState.order...)
}

// ThemeChoices returns the themes a visitor can have the pages shown with
// instead of the selected one: those the administrator allows that are
// themes in use, each once, in the order they are stored in. The selected
// theme is what the pages show anyway and is not among them. A theme of the
// themes directory that went away, was rejected, or is not read in safe mode
// is no choice while that lasts, and is one again afterwards.
func ThemeChoices(options model.Application) []string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	var choices []string
	for _, name := range options.ThemeChoices {
		if name == ThemeCurrent || slices.Contains(choices, name) {
			continue
		}
		if _, known := themeDefinition(name); known {
			choices = append(choices, name)
		}
	}
	return choices
}
