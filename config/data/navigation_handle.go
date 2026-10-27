package data

import (
	"crypto/rand"
	"encoding/base64"
)

// OpaqueNavigationHandle refers to a current immutable snapshot, never to a YAML
// index. Reloading content expires old handles instead of pointing at another item.
func OpaqueNavigationHandle(target string, matchDynamic func(string) bool) string {
	fileCacheMu.RLock()
	snapshots := []*parsedBookmarks{cachedFavorite, cachedNormal}
	fileCacheMu.RUnlock()
	for _, snapshot := range snapshots {
		if snapshot == nil || snapshot.err != nil {
			continue
		}
		snapshot.prepareHandles()
		if id := snapshot.handles[target]; id != "" {
			return id
		}
		if matchDynamic != nil {
			for _, raw := range snapshot.dynamic {
				if matchDynamic(raw) {
					return snapshot.handles[raw]
				}
			}
		}
	}
	return ""
}

// MatchOpaqueNavigationTarget resolves only handles whose resource remains
// readable by the caller. Dynamic URL expansion remains request-local.
func MatchOpaqueNavigationTarget(id string, administrator bool) (string, bool) {
	if len(id) != 32 {
		return "", false
	}
	for _, name := range []string{"apps", "bookmarks"} {
		snapshot, err := navigationSnapshot(name)
		if err != nil {
			return "", false
		}
		snapshot.prepareHandles()
		target, ok := snapshot.handleTargets[id]
		if !ok {
			continue
		}
		if administrator {
			return target, true
		}
		public := snapshot.publicSnapshot()
		public.prepareTargets()
		if _, ok := public.targets[target]; ok {
			return target, true
		}
		for _, raw := range public.dynamic {
			if raw == target {
				return target, true
			}
		}

	}
	return "", false
}
func (snapshot *parsedBookmarks) prepareHandles() {
	snapshot.handleOnce.Do(func() {
		snapshot.prepareTargets()
		snapshot.handles = map[string]string{}
		snapshot.handleTargets = map[string]string{}
		for _, item := range snapshot.value.Items {
			if snapshot.handles[item.URL] != "" {
				continue
			}
			raw := make([]byte, 24)
			if _, err := rand.Read(raw); err != nil {
				continue
			}
			id := base64.RawURLEncoding.EncodeToString(raw)
			snapshot.handles[item.URL] = id
			snapshot.handleTargets[id] = item.URL
		}
	})
}
