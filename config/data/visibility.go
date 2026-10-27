package data

import "github.com/soulteary/flare/config/model"

// FilterNavigation removes private categories and items before any rendering,
// counting or matching. A child's false never overrides a private parent.
func FilterNavigation(value model.Bookmarks, administrator bool) model.Bookmarks {
	if administrator {
		return cloneBookmarks(value)
	}
	result := model.Bookmarks{}
	private := map[string]bool{}
	for _, c := range value.Categories {
		private[c.ID] = c.Private
		if !c.Private {
			result.Categories = append(result.Categories, c)
		}
	}
	for _, b := range value.Items {
		if !b.Private && !private[b.Category] {
			result.Items = append(result.Items, b)
		}
	}
	return result
}
func HasPrivateNavigation() (bool, error) {
	for _, name := range []string{"apps", "bookmarks"} {
		s, err := navigationSnapshot(name)
		if err != nil {
			return false, err
		}
		for _, c := range s.value.Categories {
			if c.Private {
				return true, nil
			}
		}
		for _, b := range s.value.Items {
			if b.Private {
				return true, nil
			}
		}
	}
	return false, nil
}

// LoadNavigationPair selects both content collections under the same reader
// lock. Renderers retain value copies after releasing the lock, so an editor
// commit cannot mix the old apps with new bookmarks in one response.
func LoadNavigationPair(showApps, showBookmarks bool) (model.Bookmarks, model.Bookmarks, error) {
	return LoadVisibleNavigationPair(showApps, showBookmarks, true)
}

// LoadVisibleNavigationPair selects an immutable pair under the same reader
// lock, then returns one caller-owned copy of each authorized collection. Public
// filtering is cached on that immutable snapshot; neither a renderer mutation
// nor a concurrent editor publish can alter the cache or mix old/new collections.
func LoadVisibleNavigationPair(showApps, showBookmarks, administrator bool) (model.Bookmarks, model.Bookmarks, error) {
	return loadVisibleNavigationPair(showApps, showBookmarks, administrator, false)
}

// ReadExistingVisibleNavigationPair never initializes missing YAML. Selection,
// parsing and caller-owned authorized copies otherwise match the legacy reader.
func ReadExistingVisibleNavigationPair(showApps, showBookmarks, administrator bool) (model.Bookmarks, model.Bookmarks, error) {
	return loadVisibleNavigationPair(showApps, showBookmarks, administrator, true)
}
func loadVisibleNavigationPair(showApps, showBookmarks, administrator, existingOnly bool) (model.Bookmarks, model.Bookmarks, error) {
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
		return model.Bookmarks{}, model.Bookmarks{}, err
	}
	var apps, bookmarks model.Bookmarks
	for i, name := range names {
		if name == "apps" {
			apps = cloneVisibleNavigation(snapshots[i], administrator)
		} else {
			bookmarks = cloneVisibleNavigation(snapshots[i], administrator)
		}
	}
	return apps, bookmarks, nil
}
func cloneVisibleNavigation(snapshot *parsedBookmarks, administrator bool) model.Bookmarks {
	if !administrator {
		snapshot = snapshot.publicSnapshot()
	}
	return cloneBookmarks(snapshot.value)
}

// Single-collection variants retain the existing initialization/error behavior
// while avoiding the full clone followed by another authorization clone.
func LoadVisibleFavoriteBookmarks(administrator bool) (model.Bookmarks, error) {
	snapshot, err := navigationSnapshot("apps")
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneVisibleNavigation(snapshot, administrator), nil
}
func LoadVisibleNormalBookmarks(administrator bool) (model.Bookmarks, error) {
	snapshot, err := navigationSnapshot("bookmarks")
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneVisibleNavigation(snapshot, administrator), nil
}

// ReadableBookmarkCategory checks only immutable category metadata. It does
// not clone or scan bookmark items merely to validate a category route.
func ReadableBookmarkCategory(id string, administrator bool) (bool, error) {
	snapshot, err := navigationSnapshot("bookmarks")
	if err != nil {
		return false, err
	}
	for _, category := range snapshot.value.Categories {
		if category.ID == id && (administrator || !category.Private) {
			return true, nil
		}
	}
	return false, nil
}

func existingNavigationSnapshotSet(names []string) ([]*parsedBookmarks, error) {
	bookmarkMu.RLock()
	defer bookmarkMu.RUnlock()
	snapshots := make([]*parsedBookmarks, 0, len(names))
	for _, name := range names {
		// Cached content must not hide a removed or unreadable backing file.
		// Opening read-only never invokes the legacy initialization path.
		if err := checkReadableConfig(getConfigPath(name)); err != nil {
			return nil, err
		}
		snapshot, err := readExistingBookmarkSnapshot(name)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}
func readExistingVisibleNavigation(name string, administrator bool) (model.Bookmarks, error) {
	snapshots, err := existingNavigationSnapshotSet([]string{name})
	if err != nil {
		return model.Bookmarks{}, err
	}
	return cloneVisibleNavigation(snapshots[0], administrator), nil
}
func ReadExistingVisibleFavoriteBookmarks(administrator bool) (model.Bookmarks, error) {
	return readExistingVisibleNavigation("apps", administrator)
}
func ReadExistingVisibleNormalBookmarks(administrator bool) (model.Bookmarks, error) {
	return readExistingVisibleNavigation("bookmarks", administrator)
}
func ReadableExistingBookmarkCategory(id string, administrator bool) (bool, error) {
	snapshots, err := existingNavigationSnapshotSet([]string{"bookmarks"})
	if err != nil {
		return false, err
	}
	for _, category := range snapshots[0].value.Categories {
		if category.ID == id && (administrator || !category.Private) {
			return true, nil
		}
	}
	return false, nil
}

func navigationSnapshotSet(names []string) ([]*parsedBookmarks, error) {
	// Initialization, if needed, writes outside the shared read lock.
	for _, name := range names {
		if !checkExists(name + ".yml") {
			if _, err := navigationSnapshot(name); err != nil {
				return nil, err
			}
		}
	}
	bookmarkMu.RLock()
	defer bookmarkMu.RUnlock()
	snapshots := make([]*parsedBookmarks, 0, len(names))
	for _, name := range names {
		snapshot, err := readExistingBookmarkSnapshot(name)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}
