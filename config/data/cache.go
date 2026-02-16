package FlareData

import (
	"sync"
)

var (
	fileCacheMu     sync.RWMutex
	cachedConfig    []byte
	cachedApps      []byte
	cachedBookmarks []byte
)

func readFileCached(name string, readDisk func() []byte) []byte {
	fileCacheMu.RLock()
	var cached *[]byte
	switch name {
	case "config":
		cached = &cachedConfig
	case "apps":
		cached = &cachedApps
	case "bookmarks":
		cached = &cachedBookmarks
	default:
		fileCacheMu.RUnlock()
		return readDisk()
	}
	if *cached != nil {
		b := *cached
		fileCacheMu.RUnlock()
		return b
	}
	fileCacheMu.RUnlock()

	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	switch name {
	case "config":
		if cachedConfig != nil {
			return cachedConfig
		}
		cachedConfig = readDisk()
		return cachedConfig
	case "apps":
		if cachedApps != nil {
			return cachedApps
		}
		cachedApps = readDisk()
		return cachedApps
	case "bookmarks":
		if cachedBookmarks != nil {
			return cachedBookmarks
		}
		cachedBookmarks = readDisk()
		return cachedBookmarks
	}
	return readDisk()
}

func invalidateFileCache(name string) {
	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	switch name {
	case "config":
		cachedConfig = nil
	case "apps":
		cachedApps = nil
	case "bookmarks":
		cachedBookmarks = nil
	}
}
