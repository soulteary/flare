package define

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// The themes to choose from are those of the list that are in use, each
// once, in the order of the list and without the selected theme.
func TestThemeChoices(t *testing.T) {
	selected, _ := ThemeSnapshot()
	others := make([]string, 0, 2)
	for _, theme := range ThemePalettes {
		if theme.Name != selected && len(others) < 2 {
			others = append(others, theme.Name)
		}
	}
	require.Len(t, others, 2)
	require.Empty(t, ThemeChoices(model.Application{}))
	require.Empty(t, ThemeChoices(model.Application{ThemeChoices: []string{selected, "no-such-theme", ""}}))
	listed := []string{others[1], selected, others[0], "no-such-theme", others[1], others[0]}
	require.Equal(t, []string{others[1], others[0]}, ThemeChoices(model.Application{ThemeChoices: listed}))
}
