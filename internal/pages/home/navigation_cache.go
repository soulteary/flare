package home

import (
	"html/template"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
)

const navigationCacheBytes = 16 << 20
const navigationCacheEntryBytes = 1 << 20
const navigationCacheEntries = 64

// Keys retain numeric identities, not snapshots (which may contain large private
// collections). Public views have independent identities from administrator views.
type navigationFragmentKey struct {
	themeGeneration                                              uint64
	version                                                      uint64
	revision, locale, iconMode, categoryMode, path, theme, color string
	apps, appNewTab, bookmarkNewTab, inline                      bool
}
type navigationBuild struct {
	done     chan struct{}
	value    template.HTML
	complete bool
}
type navigationHTMLCache struct {
	mu       sync.Mutex
	entries  map[navigationFragmentKey]template.HTML
	order    []navigationFragmentKey
	bytes    int
	building map[navigationFragmentKey]*navigationBuild
}

var navigationFragments = newNavigationHTMLCache()

func newNavigationHTMLCache() *navigationHTMLCache {
	return &navigationHTMLCache{entries: make(map[navigationFragmentKey]template.HTML), building: make(map[navigationFragmentKey]*navigationBuild)}
}

func (cache *navigationHTMLCache) get(key navigationFragmentKey, build func() template.HTML) template.HTML {
	cache.mu.Lock()
	if value, ok := cache.entries[key]; ok {
		cache.mu.Unlock()
		return value
	}
	if pending := cache.building[key]; pending != nil {
		cache.mu.Unlock()
		<-pending.done
		if pending.complete {
			return pending.value
		}
		return build() // A panicking builder must not strand another request.
	}
	if len(cache.building) >= navigationCacheEntries {
		cache.mu.Unlock()
		return build()
	}
	pending := &navigationBuild{done: make(chan struct{})}
	cache.building[key] = pending
	cache.mu.Unlock()
	defer func() {
		cache.mu.Lock()
		delete(cache.building, key)
		close(pending.done)
		cache.mu.Unlock()
	}()
	value := build() // No configuration, authentication or cache lock during rendering.
	cache.mu.Lock()
	_, _, generation := define.ThemeRenderSnapshot()
	if generation == key.themeGeneration && len(value) <= navigationCacheEntryBytes {
		for len(cache.order) > 0 && (len(cache.entries) >= navigationCacheEntries || cache.bytes+len(value) > navigationCacheBytes) {
			old := cache.order[0]
			cache.order = cache.order[1:]
			cache.bytes -= len(cache.entries[old])
			delete(cache.entries, old)
		}
		cache.entries[key] = value
		cache.order = append(cache.order, key)
		cache.bytes += len(value)
	}
	pending.value, pending.complete = value, true
	cache.mu.Unlock()
	return value
}

func navigationFragment(view data.NavigationView, options *model.Application, apps bool, filter, selected, path string, info *fn.DynamicURL) template.HTML {
	build := func() template.HTML {
		if apps {
			value, _ := generateApplicationsSnapshot(filter, options, view.Copy(), info)
			return value
		}
		value, _ := generateBookmarkSnapshot(filter, selected, path, options, view.Copy(), info)
		return value
	}
	// Arbitrary queries/categories/URL contexts do not create cache entries.
	// Encoded and extension links include request-local `from` metadata even
	// when the destination itself has no dynamic placeholder.
	if filter != "" || selected != "" || options.EnableEncryptedLink || !view.RequestIndependent() ||
		(path != "/" && path != "/applications" && path != "/bookmarks") {
		return build()
	}
	theme, color, generation := define.ThemeRenderSnapshot()
	key := navigationFragmentKey{version: view.Version(), themeGeneration: generation, revision: options.ConfigRevision, locale: options.Locale,
		iconMode: options.IconMode, path: path, theme: theme, color: color, apps: apps,
		appNewTab: options.OpenAppNewTab, bookmarkNewTab: options.OpenBookmarkNewTab, inline: define.AppFlags.EnableMinimumRequest}
	if options.Homepage != nil {
		key.categoryMode = options.Homepage.CategoryNavigation
	}
	return navigationFragments.get(key, build)
}
