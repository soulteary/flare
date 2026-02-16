package data

import (
	"sync"
)

var (
	fileCacheMu     sync.RWMutex
	cachedConfig    []byte
	cachedApps      []byte
	cachedBookmarks []byte
)

func readFileCached(name string, readDisk func() ([]byte, error)) ([]byte, error) {
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
		return b, nil
	}
	fileCacheMu.RUnlock()

	fileCacheMu.Lock()
	defer fileCacheMu.Unlock()
	switch name {
	case "config":
		if cachedConfig != nil {
			return cachedConfig, nil
		}
		b, err := readDisk()
		if err != nil {
			return nil, err
		}
		cachedConfig = b
		return cachedConfig, nil
	case "apps":
		if cachedApps != nil {
			return cachedApps, nil
		}
		b, err := readDisk()
		if err != nil {
			return nil, err
		}
		cachedApps = b
		return cachedApps, nil
	case "bookmarks":
		if cachedBookmarks != nil {
			return cachedBookmarks, nil
		}
		b, err := readDisk()
		if err != nil {
			return nil, err
		}
		cachedBookmarks = b
		return cachedBookmarks, nil
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
