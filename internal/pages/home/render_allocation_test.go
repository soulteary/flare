package home

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/stretchr/testify/require"
)

func allocationRenderFixture() model.Bookmarks {
	return model.Bookmarks{
		Categories: []model.Category{{ID: "first", Name: "First & <"}, {ID: "second", Name: "Second", Private: true}},
		Items: []model.Bookmark{
			{Name: "Alpha & <", URL: "{origin}/alpha?a=1&b=2", Category: "first", Desc: "A & < description"},
			{Name: "Beta", URL: "javascript:alert(1)", Category: "second"},
			{Name: "Implicit", URL: "{origin}/implicit"},
			{Name: "Orphan", URL: "{origin}/orphan", Category: "missing"},
		},
	}
}
func renderGolden(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "navigation-render", name+".html"))
	require.NoError(t, err)
	return string(raw)
}
func TestNavigationRenderingPreservesExactHTML(t *testing.T) {
	for _, test := range []struct {
		name, mode, selected, filter string
		noCategories, empty          bool
	}{
		{name: "all", mode: "all"},
		{name: "collapse", mode: "collapse"},
		{name: "tabs-second", mode: "tabs", selected: "second"},
		{name: "filtered-collapse", mode: "collapse", filter: "alpha"},
		{name: "no-matches", mode: "all", filter: "not-present"},
		{name: "no-categories", noCategories: true},
		{name: "empty-no-categories", noCategories: true, empty: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := allocationRenderFixture()
			if test.noCategories {
				fixture.Categories = nil
			}
			if test.empty {
				fixture.Items = nil
			}
			options := model.Application{Locale: "en", Homepage: &model.HomepageOptions{CategoryNavigation: test.mode}}
			info := fn.DynamicURL{Origin: "https://nav.example"}
			output, message := generateBookmarkSnapshot(test.filter, test.selected, "/bookmarks", &options, fixture, &info)
			require.Empty(t, message)
			require.Equal(t, renderGolden(t, test.name), string(output))
		})
	}
	fixture := allocationRenderFixture()
	output, message := generateApplicationsSnapshot("description", &model.Application{Locale: "en"}, fixture, &fn.DynamicURL{Origin: "https://nav.example"})
	require.Empty(t, message)
	require.Equal(t, renderGolden(t, "application"), string(output))
}

func TestNavigationRenderingDoesNotMutateOrRetainInput(t *testing.T) {
	fixture := allocationRenderFixture()
	expected := allocationRenderFixture()
	options := model.Application{Locale: "en", IconMode: "FILLING"}
	info := fn.DynamicURL{Origin: "https://one.example"}
	first, _ := generateBookmarkSnapshot("", "", "/", &options, fixture, &info)
	before := string(first)
	for i := 0; i < 10; i++ {
		info.Origin = "https://two.example"
		next, _ := generateBookmarkSnapshot("beta", "", "/", &options, fixture, &info)
		require.NotContains(t, string(next), "https://one.example")
		// An inherited-private entry must never invoke the favicon fallback, even
		// though the underlying stored private:false remains unchanged.
		require.NotContains(t, string(next), "favicon.yandex.net")
		_, _ = generateApplicationsSnapshot("alpha", &options, fixture, &info)
	}
	require.Equal(t, expected, fixture)
	require.Equal(t, before, string(first), "reusing pooled builder objects must not alter a prior response string")
	require.Contains(t, before, "https://one.example")
}
