package search

import "github.com/soulteary/flare/config/model"

// Upgrade repairs obsolete target combinations without enabling engines or
// changing a local query into an external submission. It never edits its input.
// Unknown modes and malformed engines remain untouched for validation to report.
func Upgrade(source *model.SearchOptions) (model.SearchOptions, bool) {
	next := Normalize(source)
	if source == nil {
		return next, false
	}
	if next.Mode != "select" && next.Mode != "hybrid" && next.Mode != "local-first" {
		return next, false
	}
	changed := source.Mode == "" || source.DefaultTarget == ""
	available := next.DefaultTarget == "local"
	for _, engine := range next.Engines {
		if engine.Enabled && engine.ID == next.DefaultTarget {
			available = true
		}
	}
	if !available {
		next.DefaultTarget = "local"
		changed = true
	}
	if next.Mode == "hybrid" && next.DefaultTarget == "local" {
		next.Mode = "local-first"
		changed = true
	}
	if Validate(next) != nil {
		return Normalize(source), false
	}
	return next, changed
}
