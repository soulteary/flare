package home

import (
	"fmt"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"testing"
)

// Pins authorized views before timing: measures the render bypass, not file I/O or auth.
func BenchmarkNavigationReadIndex(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		for _, scenario := range []string{"apps-miss", "apps-hit", "bookmarks-miss", "bookmarks-hit", "category", "category-sparse"} {
			b.Run(fmt.Sprintf("%d/%s", size, scenario), func(b *testing.B) {
				b.Chdir(b.TempDir())
				fragmentFixture(b, size)
				if scenario == "category-sparse" {
					value := model.Bookmarks{}
					for c := 0; c < 20; c++ {
						value.Categories = append(value.Categories, model.Category{ID: fmt.Sprintf("c%d", c), Name: fmt.Sprintf("Category %d", c)})
					}
					for i := 0; i < size; i++ {
						value.Items = append(value.Items, model.Bookmark{Name: fmt.Sprintf("public-%d", i), URL: fmt.Sprintf("https://example.com/%d", i), Category: fmt.Sprintf("c%d", i%20)})
					}
					if !data.SaveNormalBookmarks(value) {
						b.Fatal("save categories")
					}
				}
				apps, bookmarks, err := data.LoadNavigationViews(true, true, false, true)
				if err != nil {
					b.Fatal(err)
				}
				options := &model.Application{Locale: "en", Homepage: &model.HomepageOptions{CategoryNavigation: "tabs"}}
				view := bookmarks
				isApp := false
				query := "not-present"
				category := ""
				switch scenario {
				case "apps-miss":
					view = apps
					isApp = true
				case "apps-hit":
					view = apps
					isApp = true
					query = "public-99"
				case "bookmarks-hit":
					query = "public-99"
				case "category-sparse":
					query = ""
					category = "c0"
				case "category":
					query = ""
					category = "public"
				}
				expected := navigationFragment(view, options, isApp, query, category, "/", nil)
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					if got := navigationFragment(view, options, isApp, query, category, "/", nil); got != expected {
						b.Fatal("output changed")
					}
				}
			})
		}
	}
}
