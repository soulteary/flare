package data

import (
	"slices"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/model"
	"gopkg.in/yaml.v2"
)

type parsedApplication struct {
	raw   []byte
	value model.Application
	err   error
}
type parsedBookmarks struct {
	raw        []byte
	value      model.Bookmarks
	err        error
	targetOnce sync.Once
	targets    map[string]struct{}
	dynamic    []string
	searchOnce sync.Once
	search     []searchEntry
}

type searchEntry struct {
	index                   int
	name, link, description string
	dynamic                 bool
	visible                 bool
}

var cachedApplication *parsedApplication
var cachedFavorite *parsedBookmarks
var cachedNormal *parsedBookmarks

// Cached bytes are immutable. Identity detects invalidation without hashing or
// scanning every YAML byte on every request. Empty byte slices are equivalent.
func sameCachedBytes(a, b []byte) bool { return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0]) }

func cloneApplication(value model.Application) model.Application {
	if value.Clock != nil {
		copy := *value.Clock
		value.Clock = &copy
	}
	if value.Homepage != nil {
		copy := *value.Homepage
		value.Homepage = &copy
	}
	if value.Search != nil {
		copy := *value.Search
		copy.Engines = slices.Clone(copy.Engines)
		value.Search = &copy
	}
	return value
}
func cloneBookmarks(value model.Bookmarks) model.Bookmarks {
	value.Categories = slices.Clone(value.Categories)
	value.Items = slices.Clone(value.Items)
	return value
}

func parsedApp(raw []byte) (model.Application, error) {
	fileCacheMu.RLock()
	snapshot := cachedApplication
	fileCacheMu.RUnlock()
	if snapshot != nil && sameCachedBytes(snapshot.raw, raw) {
		return cloneApplication(snapshot.value), snapshot.err
	}
	next := &parsedApplication{raw: raw}
	next.err = yaml.Unmarshal(raw, &next.value)
	fileCacheMu.Lock()
	if sameCachedBytes(cachedConfig, raw) && cachedConfig != nil {
		cachedApplication = next
	}
	fileCacheMu.Unlock()
	return cloneApplication(next.value), next.err
}

func hasDynamicURL(value string) bool {
	for _, token := range []string{"{host}", "{hostname}", "{href}", "{origin}", "{pathname}", "{port}", "{protocol}"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func parsedBookmarkData(name string, raw []byte) *parsedBookmarks {
	fileCacheMu.RLock()
	snapshot := cachedNormal
	if name == "apps" {
		snapshot = cachedFavorite
	}
	fileCacheMu.RUnlock()
	if snapshot != nil && sameCachedBytes(snapshot.raw, raw) {
		return snapshot
	}
	next := &parsedBookmarks{raw: raw}
	next.err = yaml.Unmarshal(raw, &next.value)

	fileCacheMu.Lock()
	if name == "apps" {
		if cachedApps != nil && sameCachedBytes(cachedApps, raw) {
			cachedFavorite = next
		}
	} else if cachedBookmarks != nil && sameCachedBytes(cachedBookmarks, raw) {
		cachedNormal = next
	}
	fileCacheMu.Unlock()
	return next
}

func (snapshot *parsedBookmarks) prepareTargets() {
	snapshot.targetOnce.Do(func() {
		snapshot.targets = make(map[string]struct{}, len(snapshot.value.Items))
		for _, item := range snapshot.value.Items {
			if hasDynamicURL(item.URL) {
				snapshot.dynamic = append(snapshot.dynamic, item.URL)
			} else {
				snapshot.targets[item.URL] = struct{}{}
			}
		}
	})
}

func (snapshot *parsedBookmarks) prepareSearch(isApp bool) {
	snapshot.searchOnce.Do(func() {
		categoryIDs := make(map[string]struct{}, len(snapshot.value.Categories))
		for _, category := range snapshot.value.Categories {
			categoryIDs[category.ID] = struct{}{}
		}
		snapshot.search = make([]searchEntry, 0, len(snapshot.value.Items))
		for index, item := range snapshot.value.Items {
			_, known := categoryIDs[item.Category]
			visible := isApp || item.Category == "" || len(categoryIDs) == 0 || known
			snapshot.search = append(snapshot.search, searchEntry{index: index, name: strings.ToLower(item.Name), link: strings.ToLower(item.URL), description: strings.ToLower(item.Desc), dynamic: hasDynamicURL(item.URL), visible: visible})
		}
	})
}
