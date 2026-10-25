package search

import (
	"github.com/soulteary/flare/config/model"
	"net/url"
	"strings"
	"testing"
)

func TestLegacyDefaultsAndCopy(t *testing.T) {
	options := Normalize(nil)
	if options.Mode != "select" || options.DefaultTarget != "local" {
		t.Fatal(options)
	}
	if err := Validate(options); err != nil {
		t.Fatal(err)
	}
	source := model.SearchOptions{Engines: Presets()}
	copy := Normalize(&source)
	copy.Engines[0].Name = "changed"
	if source.Engines[0].Name == "changed" {
		t.Fatal("normalization aliases caller data")
	}
}

func TestBuildURLPreservesQuery(t *testing.T) {
	engine := model.SearchEngine{URLTemplate: "https://example.com:8443/search?fixed=a%2Bb&q={query}#results"}
	for _, query := range []string{"机器人 C++ & Go", "a#b%? +", "🦾", "a  b"} {
		result, err := BuildURL(engine, query)
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(result)
		if err != nil {
			t.Fatal(err)
		}
		if u.Query().Get("q") != query || u.Query().Get("fixed") != "a+b" || u.Fragment != "results" {
			t.Fatal(result)
		}
	}
}

func TestTemplateValidation(t *testing.T) {
	invalid := []string{
		"javascript:alert('{query}')", "//example.com/?q={query}", "https://u:p@example.com/?q={query}",
		"https://{query}.example.com/", "https://example.com/{query}", "https://example.com/?{query}=a",
		"https://example.com/?q=x#{query}", "https://example.com/?q={query}&x={query}", "https://example.com/?q=x",
		"https://example.com/?q={query}&bad=%zz", "https://example.com/?q={query}\r\nLocation:x",
	}
	for _, template := range invalid {
		t.Run(template, func(t *testing.T) {
			if _, err := BuildURL(model.SearchEngine{URLTemplate: template}, "q"); err == nil {
				t.Fatal("accepted unsafe template")
			}
		})
	}
	for _, template := range []string{"http://192.168.1.1:8080/?q={query}", "http://[::1]:8080/?q={query}", "https://docs.lan/?q={query}"} {
		if _, err := BuildURL(model.SearchEngine{URLTemplate: template}, "q"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolve(t *testing.T) {
	options := model.SearchOptions{DefaultTarget: "bing", EnableKeywords: true, Engines: Presets()}
	for _, tc := range []struct{ query, target, wantTarget, wantQuery string }{
		{"hello", "", "bing", "hello"}, {"hello", "local", "local", "hello"},
		{"!G 机器人", "local", "google", "机器人"}, {"!LOCAL NAS", "bing", "local", "NAS"},
		{"!unknown test", "local", "local", "!unknown test"},
	} {
		result, err := Resolve(options, tc.query, tc.target)
		if err != nil {
			t.Fatal(err)
		}
		if result.Target != tc.wantTarget || result.Query != tc.wantQuery {
			t.Fatal(result)
		}
	}
	if _, err := Resolve(options, "!g", "local"); err == nil {
		t.Fatal("accepted shortcut without query")
	}
	if _, err := Resolve(options, "test", "https://evil.example/"); err == nil {
		t.Fatal("accepted arbitrary target")
	}
	options.Engines[0].Enabled = false
	if _, err := Resolve(options, "test", "bing"); err == nil {
		t.Fatal("accepted disabled default")
	}
}

func TestValidateOptions(t *testing.T) {
	for _, mutate := range []func(*model.SearchOptions){
		func(o *model.SearchOptions) { o.Mode = "invalid" },
		func(o *model.SearchOptions) { o.Mode = "hybrid" },
		func(o *model.SearchOptions) { o.DefaultTarget = "missing" },
		func(o *model.SearchOptions) { o.Engines[0].ID = "local" },
		func(o *model.SearchOptions) { o.Engines[1].ID = o.Engines[0].ID },
		func(o *model.SearchOptions) { o.Engines[1].Keyword = "B" },
		func(o *model.SearchOptions) { o.Engines[0].Keyword = "LOCAL" },
		func(o *model.SearchOptions) { o.Engines[0].Name = " " },
	} {
		options := Normalize(&model.SearchOptions{Engines: Presets()})
		mutate(&options)
		if err := Validate(options); err == nil {
			t.Fatal("accepted invalid options", options)
		}
	}
}

func TestQueryBoundaries(t *testing.T) {
	if query, err := ValidateQuery("  " + strings.Repeat("中", 512) + "  "); err != nil || len([]rune(query)) != 512 {
		t.Fatal(query, err)
	}
	for _, query := range []string{" ", strings.Repeat("中", 513), "a\nb", string([]byte{0xff})} {
		if _, err := ValidateQuery(query); err == nil {
			t.Fatal("accepted invalid query")
		}
	}
}

func TestLocalFirstDoesNotUseExternalDefaultImplicitly(t *testing.T) {
	options := model.SearchOptions{Mode: "local-first", DefaultTarget: "bing", Engines: Presets()}
	result, err := Resolve(options, "hello", "")
	if err != nil || result.Target != "local" || result.URL != "" {
		t.Fatalf("implicit search: %+v, %v", result, err)
	}
	result, err = Resolve(options, "hello", "bing")
	if err != nil || result.Target != "bing" || result.URL == "" {
		t.Fatalf("explicit web search: %+v, %v", result, err)
	}
}
