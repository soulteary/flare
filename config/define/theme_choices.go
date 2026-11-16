package define

import (
	"slices"

	"github.com/soulteary/flare/config/model"
)

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
