package home

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNavigationPrivateSentinelsAbsentBeforeRenderingAndSuggestions(t *testing.T) {
	t.Chdir(t.TempDir())
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Hidden app sentinel", URL: "https://app.secret.example", Private: true}}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{Categories: []model.Category{{ID: "public", Name: "Public"}, {ID: "private", Name: "Hidden category sentinel", Private: true}}, Items: []model.Bookmark{{Name: "Visible sentinel", URL: "https://public.example", Category: "public"}, {Name: "Hidden child sentinel", URL: "https://child.secret.example", Category: "private", Private: false}, {Name: "Hidden explicit sentinel", URL: "https://explicit.secret.example", Private: true}}}))
	options := &model.Application{ShowApps: true, ShowBookmarks: true, IconMode: "FILLING", Homepage: &model.HomepageOptions{CategoryNavigation: "tabs"}}
	anonymous := string(GenerateApplicationsTemplate("sentinel", options)) + string(GenerateBookmarkTemplate("sentinel", options))
	require.Contains(t, anonymous, "Visible sentinel")
	require.NotContains(t, anonymous, "Hidden")
	require.NotContains(t, anonymous, "secret.example")
	admin, errText := generateBookmarkNavigationFor("sentinel", "", "/", options, true)
	require.Empty(t, errText)
	require.Contains(t, string(admin), "Hidden child sentinel")
	require.NotContains(t, string(admin), "favicon/child.secret.example")
	suggestions, err := collectSearchSuggestions(*options, "sentinel", nil)
	require.NoError(t, err)
	require.Len(t, suggestions, 1)
	require.Equal(t, "Visible sentinel", suggestions[0].Name)
}
