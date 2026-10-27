package data

import (
	"strings"
	"sync/atomic"

	"github.com/soulteary/flare/config/model"
)

var navigationViewSequence atomic.Uint64

// NavigationView pins one immutable, already-authorized content snapshot. Its
// methods never expose mutable aliases; render caches can check identity before
// paying for a caller-owned collection copy.
type NavigationView struct{ snapshot *parsedBookmarks }

func (v NavigationView) Version() uint64 {
	if v.snapshot == nil {
		return 0
	}
	v.snapshot.renderOnce.Do(func() {
		v.snapshot.renderVersion = navigationViewSequence.Add(1)
		v.snapshot.renderIndependent = true
		for _, item := range v.snapshot.value.Items {
			if hasDynamicURL(item.URL) || strings.HasPrefix(strings.ToLower(item.URL), "chrome-extension:") {
				v.snapshot.renderIndependent = false
				break
			}
		}
	})
	return v.snapshot.renderVersion
}

func (v NavigationView) RequestIndependent() bool {
	return v.Version() != 0 && v.snapshot.renderIndependent
}

func (v NavigationView) Copy() model.Bookmarks {
	if v.snapshot == nil {
		return model.Bookmarks{}
	}
	return cloneBookmarks(v.snapshot.value)
}

// LoadNavigationViews preserves paired snapshot selection and backing-file
// checks even when the caller subsequently finds a rendered HTML cache hit.
func LoadNavigationViews(showApps, showBookmarks, administrator, existingOnly bool) (NavigationView, NavigationView, error) {
	names := []string{}
	if showApps {
		names = append(names, "apps")
	}
	if showBookmarks {
		names = append(names, "bookmarks")
	}
	read := navigationSnapshotSet
	if existingOnly {
		read = existingNavigationSnapshotSet
	}
	snapshots, err := read(names)
	if err != nil {
		return NavigationView{}, NavigationView{}, err
	}
	var apps, bookmarks NavigationView
	for i, name := range names {
		snapshot := snapshots[i]
		if !administrator {
			snapshot = snapshot.publicSnapshot()
		}
		if name == "apps" {
			apps = NavigationView{snapshot}
		} else {
			bookmarks = NavigationView{snapshot}
		}
	}
	return apps, bookmarks, nil
}

// Item returns a value copy from this already-authorized immutable view.
// The index must come from Matches on the same pinned view.
func (v NavigationView) Item(index int) model.Bookmark { return v.snapshot.value.Items[index] }

// Categories returns caller-owned metadata; mutable slices never escape the view.
func (v NavigationView) Categories() []model.Category {
	if v.snapshot == nil {
		return nil
	}
	return append([]model.Category(nil), v.snapshot.value.Categories...)
}

// Matches returns request-owned indexes, reusing normalized snapshot fields.
// Dynamic URL resolution remains request-local and runs outside data locks.
// A valid selected category narrows bookmarks; an unknown category keeps the
// legacy all-categories fallback. Callers pass selected only for tab layout.
func (v NavigationView) Matches(query string, apps bool, selected string, resolve func(string) string) []int {
	if v.snapshot == nil {
		return nil
	}
	query = strings.ToLower(query)
	if query != "" {
		v.snapshot.prepareSearch(apps)
	}
	if selected != "" && !apps {
		v.snapshot.categoryOnce.Do(func() {
			v.snapshot.categoryIndexes = make(map[string][]int)
			for _, c := range v.snapshot.value.Categories {
				v.snapshot.categoryIndexes[c.ID] = nil
			}
			for i, value := range v.snapshot.value.Items {
				id := value.Category
				if id == "" && len(v.snapshot.value.Categories) > 0 {
					id = v.snapshot.value.Categories[0].ID
				}
				if _, ok := v.snapshot.categoryIndexes[id]; ok {
					v.snapshot.categoryIndexes[id] = append(v.snapshot.categoryIndexes[id], i)
				}
			}
		})
		if indexes, ok := v.snapshot.categoryIndexes[selected]; ok {
			matches := make([]int, 0, len(indexes))
			for _, index := range indexes {
				if query == "" || navigationEntryMatches(v.snapshot, v.snapshot.search[index], query, apps, resolve) {
					matches = append(matches, index)
				}
			}
			return matches
		}
	}
	matches := make([]int, 0, len(v.snapshot.value.Items))
	for index := range v.snapshot.value.Items {
		if query == "" || navigationEntryMatches(v.snapshot, v.snapshot.search[index], query, apps, resolve) {
			matches = append(matches, index)
		}
	}
	return matches
}

func navigationEntryMatches(snapshot *parsedBookmarks, entry searchEntry, query string, apps bool, resolve func(string) string) bool {
	if query == "" {
		return true
	}
	link := entry.link
	if entry.dynamic && resolve != nil {
		link = strings.ToLower(resolve(snapshot.value.Items[entry.index].URL))
	}
	return strings.Contains(entry.name, query) || strings.Contains(link, query) || (apps && strings.Contains(entry.description, query))
}
