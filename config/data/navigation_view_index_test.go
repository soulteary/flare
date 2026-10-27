package data

import (
	"fmt"
	"github.com/soulteary/flare/config/model"
	"sync"
	"testing"
)

func TestNavigationViewConcurrentIndexes(t *testing.T) {
	snapshot := &parsedBookmarks{value: model.Bookmarks{Categories: []model.Category{{ID: "a"}, {ID: "b"}}, Items: []model.Bookmark{
		{Name: "First", URL: "https://example.com/first"},
		{Name: "Dynamic", URL: "https://{host}/path", Category: "a"},
		{Name: "Other", URL: "https://example.com/other", Category: "b"},
	}}}
	view := NavigationView{snapshot: snapshot}
	var wg sync.WaitGroup
	errors := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Go(func() {
			got := view.Matches("CLIENT", false, "a", func(string) string { return "https://client.test/path" })
			if len(got) != 1 || got[0] != 1 {
				errors <- fmt.Sprintf("dynamic match: %v", got)
				return
			}
			got[0] = 2
			if next := view.Matches("", false, "a", nil); len(next) != 2 || next[0] != 0 || next[1] != 1 {
				errors <- fmt.Sprintf("default category/order: %v", next)
			}
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if got := (NavigationView{}).Matches("", false, "", nil); got != nil {
		t.Fatalf("zero view: %v", got)
	}
	if got := (NavigationView{}).Categories(); got != nil {
		t.Fatalf("zero categories: %v", got)
	}
	// Unfiltered views should not allocate the search-normalization index.
	plain := &parsedBookmarks{value: snapshot.value}
	_ = (NavigationView{snapshot: plain}).Matches("", false, "a", nil)
	if plain.search != nil {
		t.Fatal("unfiltered category initialized search index")
	}
}
