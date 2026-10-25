package home

import (
	"fmt"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

func BenchmarkCategorizedNavigation(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		for _, groups := range []int{10, 100} {
			b.Run(fmt.Sprintf("entries-%d/categories-%d", size, groups), func(b *testing.B) {
				b.Chdir(b.TempDir())
				fixture := model.Bookmarks{}
				for c := 0; c < groups; c++ {
					fixture.Categories = append(fixture.Categories, model.Category{ID: fmt.Sprintf("c%d", c), Name: fmt.Sprintf("Category %d", c)})
				}
				for i := 0; i < size; i++ {
					fixture.Items = append(fixture.Items, model.Bookmark{Name: fmt.Sprintf("Entry %d", i), URL: fmt.Sprintf("https://example.com/%d?a=1&b=2", i), Category: fmt.Sprintf("c%d", i%groups)})
				}
				if !data.SaveNormalBookmarks(fixture) {
					b.Fatal("save fixture")
				}
				options := &model.Application{Locale: "en", ShowBookmarks: true}
				if len(GenerateBookmarkTemplate("", options)) == 0 {
					b.Fatal("empty output")
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if len(GenerateBookmarkTemplate("", options)) == 0 {
						b.Fatal("empty output")
					}
				}
			})
		}
	}
}
