package define

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// The layout options the selected theme does not support are known while
// its templates are in use. An option of the theme that gives a layout
// option its value counts as the theme specifying it.
func TestThemeLayoutUnsupported(t *testing.T) {
	userThemeDirectory(t, "zelda")
	writeThemeDirectory(t, "zelda", userThemeColors+"supports: [DesktopColumns, TitleSize, MobileBookmarkLayout]\nlayout:\n  CategoryNavigation: tabs\n"+
		"options:\n  - id: frames\n    type: boolean\n    layout:\n      BookmarkLayout: sections\n")
	writeThemeTemplate(t, "zelda", "home.html", `<div>`+everyPagePart+`</div>`)
	writeThemeDirectory(t, "other", userThemeColors+"supports: []\n")
	writeThemeTemplate(t, "other", "home.html", `<div>`+everyPagePart+`</div>`)
	ReloadUserThemes()
	UpdatePagePalettes()
	require.Equal(t, []string{model.LayoutMobileBookmarkLayout, model.LayoutTitleSize, model.LayoutDesktopColumns}, userTheme(t, "zelda").Supports)

	settings := allowed()
	require.Equal(t, []string{model.LayoutHideAppDescriptions, model.LayoutBookmarkLayout}, ThemeLayoutUnsupported(settings))
	settings.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	require.Equal(t, []string{model.LayoutHideAppDescriptions}, ThemeLayoutUnsupported(settings))

	// It is about the selected theme, whatever is confirmed for another.
	settings.ThemeSettings["other"] = model.ThemeSettings{Templates: true}
	require.Equal(t, []string{model.LayoutHideAppDescriptions}, ThemeLayoutUnsupported(settings))

	// Without its templates in use, every option works.
	require.Empty(t, ThemeLayoutUnsupported(model.Application{ThemeTemplates: true}))
	off := allowed()
	off.ThemeTemplates = false
	require.Empty(t, ThemeLayoutUnsupported(off))
	previous := AppFlags.DisableCSP
	AppFlags.DisableCSP = true
	t.Cleanup(func() { AppFlags.DisableCSP = previous })
	require.Empty(t, ThemeLayoutUnsupported(allowed()))
}
