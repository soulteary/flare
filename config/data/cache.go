package data

import (
	"bytes"
	"os"
	"sync"
	"time"
)

var (
	fileCacheMu     sync.RWMutex
	cachedConfig    []byte
	cachedApps      []byte
	cachedBookmarks []byte
	// The file state each navigation collection was last read from.
	cachedAppsStamp      fileStamp
	cachedBookmarksStamp fileStamp
)

// fileStamp is the modification time and size a cached collection was read at.
type fileStamp struct {
	modTime time.Time
	size    int64
	known   bool
}

// Some filesystems record modification times in steps of up to two seconds, so
// a second edit within the same step can keep both the time and the size. A
// file modified this recently is reread and compared instead of trusted.
const navigationStampSettle = 2 * time.Second

func (stamp fileStamp) matches(info os.FileInfo, now time.Time) bool {
	return stamp.known && stamp.size == info.Size() && stamp.modTime.Equal(info.ModTime()) && now.Sub(info.ModTime()) >= navigationStampSettle
}

// readFileCached serves the navigation collections. config.yml is read through
// readFreshAppConfig, which observes every external change.
func readFileCached(name string, readDisk func() ([]byte, error)) ([]byte, error) {
	if name == "apps" || name == "bookmarks" {
		return readNavigationFileCached(name, readDisk)
	}
	return readDisk()
}

// navigationCacheSlots selects one collection's cache. Callers hold fileCacheMu.
func navigationCacheSlots(name string) (*[]byte, *fileStamp, **parsedBookmarks) {
	if name == "apps" {
		return &cachedApps, &cachedAppsStamp, &cachedFavorite
	}
	return &cachedBookmarks, &cachedBookmarksStamp, &cachedNormal
}

// readNavigationFileCached serves apps.yml and bookmarks.yml from memory while
// the file on disk still has the modification time and size it was read at, so
// an external edit is observed by the next read without a restart. The check
// costs one stat per read; the relative path avoids resolving the working
// directory each time. An edit that keeps both values, with a modification
// time older than the settle window, is not detected.
func readNavigationFileCached(name string, readDisk func() ([]byte, error)) ([]byte, error) {
	info, statErr := os.Stat(name + ".yml")
	fileCacheMu.RLock()
	cached, stamp, _ := navigationCacheSlots(name)
	raw, current := *cached, *stamp
	fileCacheMu.RUnlock()
	if raw != nil && statErr == nil && current.matches(info, time.Now()) {
		return raw, nil
	}
	// A failed read is returned as an error: the cached bytes are not a
	// fallback for a changed or missing file, and nothing is created here.
	next, err := readDisk()
	if err != nil {
		return nil, err
	}
	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	cached, stamp, parsed := navigationCacheSlots(name)
	if *cached != nil && bytes.Equal(*cached, next) {
		// Unchanged content keeps the cached slice: parsed snapshots and their
		// indexes are matched by its identity and remain valid.
		next = *cached
	} else {
		*cached = next
		*parsed = nil
	}
	*stamp = fileStamp{}
	if statErr == nil {
		*stamp = fileStamp{modTime: info.ModTime(), size: info.Size(), known: true}
	}
	return next, nil
}

func invalidateFileCache(name string) {
	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	switch name {
	case "config":
		cachedConfig = nil
		cachedApplication = nil
	case "apps":
		cachedApps = nil
		cachedAppsStamp = fileStamp{}
		cachedFavorite = nil
	case "bookmarks":
		cachedBookmarks = nil
		cachedBookmarksStamp = fileStamp{}
		cachedNormal = nil
	}
}

// Read current bytes for downgrade/revision checks while reusing an unchanged
// snapshot's identity so strict parsing and cloning can still be cached.
func readFreshAppConfig(path string) ([]byte, error) {
	raw, err := readFile(path)
	if err != nil {
		return nil, err
	}
	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	if cachedConfig != nil && bytes.Equal(cachedConfig, raw) {
		return cachedConfig, nil
	}
	cachedConfig = raw
	cachedApplication = nil
	return raw, nil
}
