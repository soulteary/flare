package data

import (
	"strings"

	"github.com/soulteary/flare/config/model"
)

// MatchNavigationTarget checks static targets in constant expected time, then
// asks the caller to validate request-scoped expansions of dynamic templates.
// The callback runs without cache or persistence locks and cannot mutate indexes.
// Malformed collections are skipped, matching the redirect route's prior behavior.
func MatchNavigationTarget(target string, matchDynamic func(string) bool) bool {
	return MatchNavigationTargetFor(target, true, matchDynamic)
}

// MatchNavigationTargetFor authorizes current YAML membership before redirecting.
func MatchNavigationTargetFor(target string, administrator bool, matchDynamic func(string) bool) bool {
	snapshots, err := navigationSnapshotSet([]string{"apps", "bookmarks"})
	if err != nil {
		return false
	}
	for _, snapshot := range snapshots {
		if !administrator {
			snapshot = snapshot.publicSnapshot()
		}
		snapshot.prepareTargets()
		if _, ok := snapshot.targets[target]; ok {
			return true
		}
		if matchDynamic != nil {
			for _, template := range snapshot.dynamic {
				if matchDynamic(template) {
					return true
				}
			}
		}

	}
	return false
}

// NavigationMatch contains a value copy, never a mutable alias into the cache.
type NavigationMatch struct {
	Bookmark model.Bookmark
	URL      string
	IsApp    bool
}

// SearchNavigationMatches visits only enabled collections and uses pre-normalized
// fields and category membership. URL expansion and URL policy stay request-local.
func SearchNavigationMatches(query string, showApps, showBookmarks bool, limit int, resolve func(string) string, accept func(string) bool) ([]NavigationMatch, error) {
	return SearchNavigationMatchesFor(query, showApps, showBookmarks, limit, resolve, accept, true)
}

// SearchNavigationMatchesFor applies visibility before matching and limiting.
func SearchNavigationMatchesFor(query string, showApps, showBookmarks bool, limit int, resolve func(string) string, accept func(string) bool, administrator bool) ([]NavigationMatch, error) {
	if limit < 1 || limit > 64 {
		limit = 8
	}
	query = strings.ToLower(query)
	collections := make([]searchCollection, 0, 2)
	names := []string{}
	if showApps {
		names = append(names, "apps")
	}
	if showBookmarks {
		names = append(names, "bookmarks")
	}
	snapshots, err := navigationSnapshotSet(names)
	if err != nil {
		return nil, err
	}
	for i, snapshot := range snapshots {
		isApp := names[i] == "apps"
		if !administrator {
			snapshot = snapshot.publicSnapshot()
		}
		snapshot.prepareSearch(isApp)
		collections = append(collections, searchCollection{snapshot: snapshot, isApp: isApp})
	}

	return collectNavigationMatches(collections, query, limit, resolve, accept), nil
}

type searchCollection struct {
	snapshot *parsedBookmarks
	isApp    bool
}

func collectNavigationMatches(collections []searchCollection, query string, limit int, resolve func(string) string, accept func(string) bool) []NavigationMatch {
	matches := make([]NavigationMatch, 0, limit)
	for _, collection := range collections {
		snapshot := collection.snapshot
		for _, entry := range snapshot.search {
			if !entry.visible {
				continue
			}
			item := snapshot.value.Items[entry.index]
			link := item.URL
			normalizedLink := entry.link
			if entry.dynamic && resolve != nil {
				link = resolve(link)
				normalizedLink = strings.ToLower(link)
			}
			matched := strings.Contains(entry.name, query) || strings.Contains(normalizedLink, query) || (collection.isApp && strings.Contains(entry.description, query))
			if !matched || (accept != nil && !accept(link)) {
				continue
			}
			matches = append(matches, NavigationMatch{Bookmark: item, URL: link, IsApp: collection.isApp})
			if len(matches) == limit {
				return matches
			}
		}
	}
	return matches
}
