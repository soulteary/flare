package bookmarks

import (
	"github.com/soulteary/flare/config/model"
	"strings"
	"testing"
)

func TestBrowserDirectoriesAndRoundTrip(t *testing.T) {
	input := `<!DOCTYPE NETSCAPE-Bookmark-file-1><DL><p><DT><H3>书签栏</H3><DL><p><DT><H3>开发</H3><DL><p><DT><A HREF="http://nas:8080/a?x=1&amp;y=2#part">工具 &amp; 文档</A></DL><p><DT><A HREF="https://example.com">站点</A></DL><p></DL>`
	entries, err := Parse([]byte(input), "html")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Build(model.Bookmarks{}, entries, Options{Strategy: "skip", StripRoot: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Added != 2 || plan.Result.Categories[0].Name != "开发" || plan.Result.Items[0].URL != "http://nas:8080/a?x=1&y=2#part" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	out, err := HTML(plan.Result)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(out, "html")
	if err != nil || len(again) != 2 || again[0].Bookmark.Name != "工具 & 文档" {
		t.Fatalf("roundtrip: %+v %v", again, err)
	}
}

func TestMergeAndURLSafety(t *testing.T) {
	current := model.Bookmarks{Categories: []model.Category{{ID: "old", Name: "开发"}}, Items: []model.Bookmark{{Name: "原名", URL: "https://EXAMPLE.com/a?q=1#x", Category: "old", Icon: "go", Desc: "keep", Private: true}}}
	entries := []Entry{{Bookmark: model.Bookmark{Name: "新名", URL: "https://example.com/a?q=1#x"}, Path: []string{"开发"}}, {Bookmark: model.Bookmark{URL: "javascript:alert(1)"}}, {Bookmark: model.Bookmark{URL: "/local"}}}
	plan, err := Build(current, entries, Options{Strategy: "update"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Updated != 1 || plan.Invalid != 2 || plan.Result.Items[0].Icon != "go" || !plan.Result.Items[0].Private || current.Items[0].Name != "原名" {
		t.Fatalf("bad merge: %+v", plan)
	}
	plan, err = Build(current, entries, Options{Strategy: "skip", AllowOther: true})
	if err != nil || plan.Skipped != 1 || plan.Added != 1 || plan.Invalid != 1 {
		t.Fatalf("bad options: %+v %v", plan, err)
	}
}

func TestNativeFieldsAndLimits(t *testing.T) {
	input := []byte("categories:\n- id: go\n  title: Go\nlinks:\n- name: Test\n  link: https://example.com\n  category: go\n  icon: go\n  desc: details\n  private: true\n")
	entries, err := Parse(input, "yaml")
	if err != nil || !entries[0].Bookmark.Private || entries[0].Path[0] != "Go" {
		t.Fatalf("native: %+v %v", entries, err)
	}
	if _, err = Parse([]byte("unknown: true"), "yaml"); err == nil {
		t.Fatal("unknown YAML accepted")
	}
	if _, err = Parse([]byte(strings.Repeat("x", MaxBytes+1)), "html"); err == nil {
		t.Fatal("oversize accepted")
	}
	if _, err = Parse([]byte(strings.Repeat("<DL>", MaxDepth+2)), "html"); err == nil {
		t.Fatal("depth accepted")
	}
	if _, err = Parse([]byte(`<DL><DT><A HREF="https://example.com">broken`), "html"); err == nil {
		t.Fatal("truncated anchor accepted")
	}
}

func TestImportPoliciesAndApplications(t *testing.T) {
	entries := []Entry{{Bookmark: model.Bookmark{Name: "one", URL: "https://example.com/a?q=1#x"}, Path: []string{"Bar", "Tools"}}, {Bookmark: model.Bookmark{Name: "two", URL: "https://example.com/a?q=2#x"}, Path: []string{"Other"}}}
	first, err := Build(model.Bookmarks{}, entries, Options{Strategy: "skip", Flatten: true, Folder: "All"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(first.Result, entries, Options{Strategy: "skip", Flatten: true, Folder: "All"})
	if err != nil || second.Skipped != 2 || second.Added != 0 {
		t.Fatalf("not idempotent: %+v %v", second, err)
	}
	keep, err := Build(first.Result, entries, Options{Strategy: "keep", Flatten: true, Folder: "All"})
	if err != nil || keep.Added != 2 || len(keep.Result.Items) != 4 {
		t.Fatalf("keep: %+v %v", keep, err)
	}
	apps, err := Build(model.Bookmarks{}, entries, Options{Strategy: "skip", Apps: true})
	if err != nil || len(apps.Result.Categories) != 0 || apps.Result.Items[0].Category != "" {
		t.Fatalf("apps: %+v %v", apps, err)
	}
	if _, err = Build(model.Bookmarks{}, entries, Options{Strategy: "replace"}); err == nil {
		t.Fatal("unsupported policy accepted")
	}
}

func TestURLComparisonAndForbiddenProtocols(t *testing.T) {
	for _, raw := range []string{"JAVASCRIPT:alert(1)", "data:text/html,hi", "vbscript:foo", "file:///etc/passwd", "https://example.com/\nunsafe", "http:///missing", "https://example.com\\@evil.com"} {
		if _, err := URLKey(raw, true); err == nil {
			t.Errorf("unsafe URL accepted: %q", raw)
		}
	}
	for _, raw := range []string{"/local?a=1#x", "mailto:user@example.com", "ssh://nas:22", "smb://nas/share", "magnet:?xt=urn:btih:123", "https://[::1]:8443/a?q=1#x"} {
		if _, err := URLKey(raw, true); err != nil {
			t.Errorf("valid URL rejected %q: %v", raw, err)
		}
	}
	a, _ := URLKey(" HTTPS://EXAMPLE.COM:443/A?Q=1#X ", false)
	b, _ := URLKey("https://example.com:443/A?Q=1#X", false)
	if a != b {
		t.Fatalf("host normalization: %q != %q", a, b)
	}
	c, _ := URLKey("https://example.com:443/a?Q=1#X", false)
	if a == c {
		t.Fatal("path case was normalized")
	}
}
