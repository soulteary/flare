package home

import (
	"github.com/soulteary/flare/config/model"
	"os"
	"strings"
	"testing"
)

func TestCategoryLinksPreserveSearchAndEscape(t *testing.T) {
	var b strings.Builder
	renderCategoryNavigation(&b, []model.Category{{ID: "a&b", Name: `<unsafe>`}}, "a&b", "x&y", "/bookmarks", "en")
	output := b.String()
	for _, want := range []string{`category=a%26b&amp;search=x%26y`, `&lt;unsafe&gt;`, `aria-current="page"`, `href="/bookmarks?search=x%26y"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "<unsafe>") {
		t.Fatal("unescaped label")
	}
}

func TestCollapsibleCategoryPreservesItemsAndDefaultPlacement(t *testing.T) {
	items := []model.Bookmark{{Name: "Implicit", URL: "https://example.com"}, {Name: "Explicit", URL: "https://example.com/2", Category: "first"}, {Name: "Other", URL: "https://example.com/3", Category: "second"}}
	first := model.Category{ID: "first", Name: "First"}
	var b strings.Builder
	renderBookmarksWithCategories(&b, &items, &first, &first, false, false, "DEFAULT", true)
	output := b.String()
	for _, want := range []string{`<details`, `<summary`, `Implicit`, `Explicit`, `</details>`} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(output, "Other") {
		t.Fatal("other category leaked")
	}
}

func TestCategorySelectionAndSearch(t *testing.T) {
	t.Chdir(t.TempDir())
	fixture := `categories:
- id: first
  title: First
- id: second
  title: Second
links:
- name: Default match
  link: https://example.com/default
- name: Explicit match
  link: https://example.com/explicit
  category: second
`
	if err := os.WriteFile("bookmarks.yml", []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	options := &model.Application{Locale: "en", Homepage: &model.HomepageOptions{CategoryNavigation: "tabs"}}
	output := string(GenerateBookmarkNavigationTemplate("match", "second", "/", options))
	if strings.Contains(output, ">Default match<") || !strings.Contains(output, ">Explicit match<") {
		t.Fatalf("category filter failed: %s", output)
	}
	output = string(GenerateBookmarkNavigationTemplate("match", "missing", "/", options))
	if !strings.Contains(output, ">Default match<") || !strings.Contains(output, ">Explicit match<") {
		t.Fatal("unknown category did not fall back to all")
	}
	options.Homepage.CategoryNavigation = "collapse"
	output = string(GenerateBookmarkNavigationTemplate("match", "", "/", options))
	if strings.Contains(output, "<details") {
		t.Fatal("search results must not be hidden in closed details")
	}
}
