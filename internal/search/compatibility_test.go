package search

import (
	"github.com/soulteary/flare/config/model"
	"reflect"
	"testing"
)

func TestUpgradePreservesLocalIntentAndEngineData(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opts         *model.SearchOptions
		mode, target string
		changed      bool
	}{
		{"missing block", nil, "select", "local", false},
		{"legacy fields", &model.SearchOptions{}, "select", "local", true},
		{"hybrid local", &model.SearchOptions{Mode: "hybrid", DefaultTarget: "local", Engines: Presets()}, "local-first", "local", true},
		{"disabled default", &model.SearchOptions{Mode: "hybrid", DefaultTarget: "missing", Engines: Presets()}, "local-first", "local", true},
		{"valid hybrid", &model.SearchOptions{Mode: "hybrid", DefaultTarget: "bing", Engines: Presets()}, "hybrid", "bing", false},
		{"future mode", &model.SearchOptions{Mode: "future", DefaultTarget: "local"}, "future", "local", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before model.SearchOptions
			if tc.opts != nil {
				before = *tc.opts
				before.Engines = append([]model.SearchEngine(nil), tc.opts.Engines...)
			}
			got, changed := Upgrade(tc.opts)
			if got.Mode != tc.mode || got.DefaultTarget != tc.target || changed != tc.changed {
				t.Fatalf("%+v changed=%v", got, changed)
			}
			if tc.opts != nil && !reflect.DeepEqual(before, *tc.opts) {
				t.Fatal("upgrade mutated source")
			}
			again, repeated := Upgrade(&got)
			if repeated || !reflect.DeepEqual(again, got) {
				t.Fatal("upgrade is not idempotent")
			}
		})
	}
	broken := model.SearchOptions{Mode: "hybrid", DefaultTarget: "local", Engines: []model.SearchEngine{{ID: "custom", Name: "custom", URLTemplate: "invalid", Enabled: true}}}
	if _, changed := Upgrade(&broken); changed {
		t.Fatal("silently upgraded malformed engine")
	}
}
