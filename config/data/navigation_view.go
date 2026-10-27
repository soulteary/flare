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
