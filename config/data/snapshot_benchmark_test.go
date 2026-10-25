package data

import (
	"fmt"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// These benchmarks cover server-side data loading only: no HTTP client or browser.
func BenchmarkNavigationSnapshotScale(b *testing.B) {
	for _, count := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			prepareBookmarkHotPath(b)
			fixture := model.Bookmarks{Categories: []model.Category{{ID: "first", Name: "First"}}}
			for i := 0; i < count; i++ {
				fixture.Items = append(fixture.Items, model.Bookmark{Name: fmt.Sprintf("Entry %d", i), URL: fmt.Sprintf("https://service.example/%d", i), Desc: "Local application description", Category: "first"})
			}
			if !SaveNormalBookmarks(fixture) {
				b.Fatal("save fixture")
			}
			if _, err := LoadNormalBookmarks(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value, err := LoadNormalBookmarks()
				if err != nil || len(value.Items) != count {
					b.Fatal("load snapshot", err)
				}
			}
		})
	}
}

func BenchmarkNavigationTargetIndexScale(b *testing.B) {
	for _, count := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			prepareBookmarkHotPath(b)
			fixture := model.Bookmarks{}
			for i := 0; i < count; i++ {
				fixture.Items = append(fixture.Items, model.Bookmark{Name: fmt.Sprintf("Entry %d", i), URL: fmt.Sprintf("https://service.example/%d", i)})
			}
			if !SaveFavoriteBookmarks(model.Bookmarks{}) || !SaveNormalBookmarks(fixture) {
				b.Fatal("save")
			}
			target := fixture.Items[count-1].URL
			if !MatchNavigationTarget(target, nil) {
				b.Fatal("warm lookup")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if !MatchNavigationTarget(target, nil) {
					b.Fatal("target missing")
				}
			}
		})
	}
}

func BenchmarkNavigationSearchIndexScale(b *testing.B) {
	for _, count := range []int{1000, 5000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			prepareBookmarkHotPath(b)
			fixture := model.Bookmarks{}
			for i := 0; i < count; i++ {
				fixture.Items = append(fixture.Items, model.Bookmark{Name: fmt.Sprintf("Entry %d", i), URL: fmt.Sprintf("https://service.example/%d", i)})
			}
			if !SaveNormalBookmarks(fixture) {
				b.Fatal("save")
			}
			query := fmt.Sprintf("Entry %d", count-1)
			if _, err := SearchNavigationMatches(query, false, true, 8, nil, nil); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				matches, err := SearchNavigationMatches(query, false, true, 8, nil, nil)
				if err != nil || len(matches) != 1 {
					b.Fatal("match missing", err)
				}
			}
		})
	}
}
