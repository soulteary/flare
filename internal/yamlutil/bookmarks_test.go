package yamlutil

import (
	"errors"
	"github.com/soulteary/flare/config/model"
	"strings"
	"testing"
)

func TestNavigationPermissionValidation(t *testing.T) {
	for _, body := range []string{"private: null", "private: 'true'", "private: 1", "private: yes", "private: true\n  private: false", "category: absent"} {
		raw := []byte("links:\n- name: sentinel\n  link: https://private.example\n  " + body + "\n")
		var v model.Bookmarks
		if err := DecodeBookmarks(raw, &v, false); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	for _, raw := range []string{"categories:\n- id: same\n- id: same\nlinks: []\n", "links: []\n---\nlinks: []\n"} {
		var v model.Bookmarks
		if DecodeBookmarks([]byte(raw), &v, false) == nil {
			t.Fatal("invalid document accepted")
		}
	}
	var v model.Bookmarks
	if err := DecodeBookmarks([]byte("links:\n- name: App\n  link: https://example.com\n  category: c\ncategories:\n- id: c\n"), &v, true); err == nil {
		t.Fatal("apps accepted category")
	}
}
func TestNavigationNodeExtensionsSurviveEditAndNativeImport(t *testing.T) {
	raw := []byte("custom_root: {enabled: true}\ncategories:\n- id: c\n  title: Category\n  private: true\n  extension: [one, two]\nlinks:\n- name: Original\n  link: https://example.com\n  category: c\n  private: false\n  custom: {nested: value}\n")
	var v model.Bookmarks
	if err := DecodeBookmarks(raw, &v, false); err != nil {
		t.Fatal(err)
	}
	v.Items[0].Name = "Edited"
	v.Categories[0].Name = "Renamed"
	out, err := MergeBookmarks(raw, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"custom_root:", "extension:", "nested: value", "private: true", "Edited"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("lost %s: %s", want, out)
		}
	}
	// New target retains extensions extracted from the imported source models.
	imported, err := MergeBookmarks([]byte("links: []\n"), v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(imported), "nested: value") || !strings.Contains(string(imported), "extension:") {
		t.Fatalf("import lost extensions: %s", imported)
	}
}

func TestAnchoredNavigationReadCompatibleAndWritesReadOnly(t *testing.T) {
	raw := []byte("private_policy: &private_policy\n  private: true\ncategory: &category\n  id: secret\n  title: Private category\n  <<: *private_policy\nentry: &entry\n  name: Private entry\n  link: https://secret.example\n  category: secret\n  <<: *private_policy\ncategories:\n- *category\nlinks:\n- *entry\n- <<: *entry\n  name: Explicit false child\n  private: false\n")
	var value model.Bookmarks
	if err := DecodeBookmarks(raw, &value, false); err != nil {
		t.Fatal(err)
	}
	if !value.Categories[0].Private || !value.Items[0].Private || value.Items[1].Private {
		t.Fatalf("merge precedence changed: %+v", value)
	}
	if _, err := MergeBookmarks(raw, value); !errors.Is(err, ErrBookmarkAliasesReadOnly) {
		t.Fatalf("aliased write was not read-only: %v", err)
	}
	for _, body := range []string{"policy: &p {private: 'true'}\nlinks:\n- <<: *p\n  name: Bad\n  link: https://secret.example\n", "policy: &p {private: null}\nlinks:\n- <<: *p\n  name: Bad\n  link: https://secret.example\n", "cycle: &c {self: *c}\nlinks: []\n", "links:\n- *missing\n"} {
		var invalid model.Bookmarks
		if DecodeBookmarks([]byte(body), &invalid, false) == nil {
			t.Fatalf("invalid aliases accepted: %s", body)
		}
	}
}
