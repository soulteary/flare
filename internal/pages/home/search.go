package home

import (
	"github.com/soulteary/flare/config/model"
	searchcore "github.com/soulteary/flare/internal/search"
)

// PopulateSearch exposes only configured search targets and escaped text to templates.
func PopulateSearch(m map[string]any, options model.Application, query, target, message string) {
	opts := searchcore.Normalize(options.Search)
	if err := searchcore.Validate(opts); err != nil {
		opts = searchcore.Normalize(nil)
		if message == "" {
			message = "External search configuration is invalid; local search remains available."
		}
	}
	if target == "" {
		target = opts.DefaultTarget
	}
	engines := make([]model.SearchEngine, 0, len(opts.Engines))
	valid := target == "local"
	for _, engine := range opts.Engines {
		if engine.Enabled {
			engines = append(engines, engine)
			if engine.ID == target {
				valid = true
			}
		}
	}
	if !valid {
		target = "local"
	}
	m["SearchOptions"] = opts
	m["SearchEngines"] = engines
	m["SearchTarget"] = target
	m["SearchQuery"] = query
	m["SearchError"] = message
	m["SearchConfigured"] = options.Search != nil
}
