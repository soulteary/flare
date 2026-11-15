package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

// A search says that it has no results when no list of the page shows an
// entry. It goes by the number of entries that are shown: an entry that
// matches and is not shown, because its list is turned off or its category
// is not the selected one, is no result.
func TestSearchWithoutResultsHTTP(t *testing.T) {
	const base = "Locale: en\nTheme: blackboard\nShowSearchComponent: true\n"
	handler := customThemeRouter(t, base+"ShowApps: true\nShowBookmarks: true\n")
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{
		{Name: "Alpha application", URL: "https://alpha.example/", Desc: "first of the applications"},
	}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{
		Categories: []model.Category{{ID: "one", Name: "One"}, {ID: "two", Name: "Two"}},
		Items: []model.Bookmark{
			{Name: "Beta bookmark", URL: "https://beta.example/", Category: "one"},
			{Name: "Gamma bookmark", URL: "https://gamma.example/", Category: "two"},
		},
	}))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
	// The text of the notice depends on whether the site has search
	// engines; the element is the same.
	notice := `<p role=status class=search-empty>`
	configure := func(settings string) {
		t.Helper()
		require.NoError(t, os.WriteFile("config.yml", []byte(base+settings), 0600))
	}
	search := func(query, category string) (bool, string) {
		t.Helper()
		values := url.Values{"search": {query}}
		if category != "" {
			values.Set("category", category)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/?"+values.Encode(), nil))
		require.Equal(t, http.StatusOK, rec.Code, query)
		page := rec.Body.String()
		return strings.Contains(page, notice), page
	}

	for _, tc := range []struct {
		name, settings, query, category string
		without                         bool
		shows                           []string
	}{
		{"nothing matches", "ShowApps: true\nShowBookmarks: true\n", "omega", "", true, nil},
		{"an application matches by its name", "ShowApps: true\nShowBookmarks: true\n", "alpha", "", false, []string{"Alpha application"}},
		{"an application matches by its description", "ShowApps: true\nShowBookmarks: true\n", "first of", "", false, []string{"Alpha application"}},
		{"a bookmark matches", "ShowApps: true\nShowBookmarks: true\n", "gamma", "", false, []string{"Gamma bookmark"}},
		{"both lists match", "ShowApps: true\nShowBookmarks: true\n", "example", "", false, []string{"Alpha application", "Beta bookmark", "Gamma bookmark"}},
		{"only a list that is turned off matches", "ShowApps: false\nShowBookmarks: true\n", "alpha", "", true, nil},
		{"only the other list that is turned off matches", "ShowApps: true\nShowBookmarks: false\n", "gamma", "", true, nil},
		{"both lists are turned off", "ShowApps: false\nShowBookmarks: false\n", "example", "", true, nil},
		{"a bookmark of the selected category matches", "ShowApps: true\nShowBookmarks: true\nHomepage:\n    CategoryNavigation: tabs\n", "bookmark", "two", false, []string{"Gamma bookmark"}},
		{"only a bookmark of another category matches", "ShowApps: true\nShowBookmarks: true\nHomepage:\n    CategoryNavigation: tabs\n", "beta", "two", true, nil},
		{"collapsed categories are searched", "ShowApps: true\nShowBookmarks: true\nHomepage:\n    CategoryNavigation: collapse\n", "beta", "", false, []string{"Beta bookmark"}},
	} {
		configure(tc.settings)
		without, page := search(tc.query, tc.category)
		require.Equal(t, tc.without, without, tc.name)
		for _, name := range []string{"Alpha application", "Beta bookmark", "Gamma bookmark"} {
			shown := false
			for _, expected := range tc.shows {
				shown = shown || expected == name
			}
			require.Equal(t, shown, strings.Contains(page, name), "%s: %s", tc.name, name)
		}
	}

	// A page without a search says nothing of the kind.
	configure("ShowApps: false\nShowBookmarks: false\n")
	require.NotContains(t, getPage(t, handler, "/"), notice)
}
