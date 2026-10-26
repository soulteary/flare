package yamlutil_test

import (
	"reflect"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/yamlutil"
	"go.yaml.in/yaml/v3"
)

func TestStrictBookmarkDecoding(t *testing.T) {
	for _, input := range []string{
		"unknown: true\nlinks: []\n",
		"links:\n- name: Test\n  link: https://example.com\n  unknown: true\n",
		"links: []\nlinks: []\n",
		"links:\n- name: Test\n  name: Other\n  link: https://example.com\n",
		"links: []\n---\nlinks: []\n",
		"links: []\n---\n",
		"links: []\n---\n[broken\n",
	} {
		t.Run(input, func(t *testing.T) {
			var bookmarks model.Bookmarks
			if err := yamlutil.UnmarshalStrict([]byte(input), &bookmarks); err == nil {
				t.Fatal("invalid bookmark document accepted")
			}
		})
	}
}

func TestLegacyBookmarkRoundTrip(t *testing.T) {
	// YAML 1.1 booleans remain supported in typed fields; text such as on and
	// numeric-looking identifiers must retain their original string values.
	input := []byte("categories:\n- id: 001\n  title: on\nlinks:\n- name: yes\n  link: https://example.com/?a=1&b=2\n  category: 001\n  desc: |\n    第一行\n    second line\n  private: YES\n")
	want := model.Bookmarks{
		Categories: []model.Category{{ID: "001", Name: "on"}},
		Items:      []model.Bookmark{{Name: "yes", URL: "https://example.com/?a=1&b=2", Category: "001", Desc: "第一行\nsecond line\n", Private: true}},
	}
	var got model.Bookmarks
	if err := yamlutil.UnmarshalStrict(input, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy values changed: %+v", got)
	}
	raw, err := yaml.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip model.Bookmarks
	if err := yamlutil.UnmarshalStrict(raw, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, want) {
		t.Fatalf("round trip changed values: %+v", roundTrip)
	}
}

func TestEmptyInputPreservesDefaults(t *testing.T) {
	for _, input := range []string{"", "# comment\n", "null\n"} {
		app := model.Application{Title: "existing", ShowTitle: true}
		if err := yamlutil.UnmarshalStrict([]byte(input), &app); err != nil {
			t.Fatal(err)
		}
		if app.Title != "existing" || !app.ShowTitle {
			t.Fatalf("empty input changed defaults: %+v", app)
		}
	}
}
