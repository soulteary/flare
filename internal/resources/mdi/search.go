package mdi

import (
	"sort"
	"strings"
	"sync"
)

var iconNamesOnce sync.Once
var sortedIconNames []string

// SearchIconNames returns a bounded, deterministic page without exposing mutable
// package data. Case and punctuation match the existing bundled icon names.
func SearchIconNames(query string, offset, limit int) ([]string, int) {
	iconNamesOnce.Do(func() {
		sortedIconNames = make([]string, 0, len(iconMap))
		for name := range iconMap {
			sortedIconNames = append(sortedIconNames, name)
		}
		for name := range legacyIconMap {
			if _, present := iconMap[name]; !present {
				sortedIconNames = append(sortedIconNames, name)
			}
		}
		sort.Strings(sortedIconNames)
	})
	query = strings.ToLower(strings.TrimSpace(query))
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 48 {
		limit = 48
	}
	result := make([]string, 0, limit)
	total := 0
	for _, name := range sortedIconNames {
		if !strings.Contains(name, query) {
			continue
		}
		if total >= offset && len(result) < limit {
			result = append(result, name)
		}
		total++
	}
	return result, total
}
