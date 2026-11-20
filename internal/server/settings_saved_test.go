package server

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/internal/i18n"
)

// A page of the settings looks after a save as it did before it: only the
// notice tells that the values were taken.
func TestAStoredSettingsFormSaysSoHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Locale: zh\nTheme: blackboard\nShowTitle: true\n")
	notice := func(locale string) *regexp.Regexp {
		return regexp.MustCompile(`<p class="?settings-notice"? role="?status"?>` + regexp.QuoteMeta(i18n.T(locale, "settings_saved")) + `</p>`)
	}
	require.NotEqual(t, i18n.T("zh", "settings_saved"), i18n.T("en", "settings_saved"))
	forms := splitValues
	require.Len(t, forms, 5)
	for path, values := range forms {
		if path == "/settings/site" {
			continue
		}
		// The page as it is opened says nothing.
		require.NotRegexp(t, `class="?settings-notice`, getPage(t, handler, path), path)
		saved := postSettingsForm(t, handler, path, values)
		require.Equal(t, http.StatusOK, saved.Code, path)
		require.Len(t, notice("zh").FindAllString(saved.Body.String(), -1), 1, path)
		// Opened again, it says nothing: the notice belongs to the save.
		require.NotRegexp(t, `class="?settings-notice`, getPage(t, handler, path), path)
	}
	// The form that sets the language answers in the language it set.
	saved := postSettingsForm(t, handler, "/settings/site", forms["/settings/site"])
	require.Equal(t, http.StatusOK, saved.Code)
	require.Len(t, notice("en").FindAllString(saved.Body.String(), -1), 1)
	require.NotRegexp(t, notice("zh"), saved.Body.String())

	// A form that is refused is not said to be stored.
	refused := postSettingsForm(t, handler, "/settings/site", url.Values{"locale": {"xx"}})
	require.Equal(t, http.StatusBadRequest, refused.Code)
	require.NotRegexp(t, `class="?settings-notice`, refused.Body.String())
}
