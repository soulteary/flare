package i18n

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	kit "github.com/soulteary/i18n-kit/v4"
)

func TestLocales(t *testing.T) {
	for _, tc := range []struct {
		input, normalized, html string
		valid                   bool
	}{
		{"", "zh", "zh-CN", false}, {" zh-CN ", "zh", "zh-CN", true},
		{"zh_Hans", "zh", "zh-CN", true}, {"EN_us", "en", "en", true},
		{"en-GB", "en", "en", true}, {"fr", "en", "en", false}, {"unknown", "en", "en", false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := Normalize(tc.input); got != tc.normalized {
				t.Fatalf("Normalize = %q", got)
			}
			if got := HTMLLang(tc.input); got != tc.html {
				t.Fatalf("HTMLLang = %q", got)
			}
			if got, ok := ParseLocale(tc.input); ok != tc.valid || (ok && got != tc.normalized) {
				t.Fatalf("ParseLocale = %q, %v", got, ok)
			}
			if DateFormat(tc.input) != DateFormat(tc.normalized) || Weekday(tc.input, time.Monday) != Weekday(tc.normalized, time.Monday) {
				t.Fatal("date helpers disagree with normalized locale")
			}
		})
	}
	if Weekday("en", time.Weekday(-1)) != "" || Weekday("en", time.Weekday(7)) != "" {
		t.Fatal("invalid weekday")
	}
}

func TestBundleFallbackAndLoadErrors(t *testing.T) {
	source := fstest.MapFS{
		"locales/zh.json": {Data: []byte(`{"local":"中文"}`)},
		"locales/en.json": {Data: []byte(`{"local":"English","fallback":"Fallback %s"}`)},
	}
	b, err := loadBundle(source)
	if err != nil {
		t.Fatal(err)
	}
	tr := kit.NewTranslator(b)
	if got := tr.TfWithLang(kit.LangZH, "fallback", "value"); got != "Fallback value" {
		t.Fatal(got)
	}
	if got := tr.TWithLang(kit.LangZH, "missing"); got != "missing" {
		t.Fatal(got)
	}
	delete(source, "locales/en.json")
	if _, err := loadBundle(source); err == nil {
		t.Fatal("missing file accepted")
	}
	source["locales/en.json"] = &fstest.MapFile{Data: []byte(`{"broken":`)}
	if _, err := loadBundle(source); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestFormatting(t *testing.T) {
	if got := Tf("en", "missing_%s", "private value"); got != "missing_%s" {
		t.Fatalf("missing key leaked args: %q", got)
	}
	if got := Format("en", "missing_{value}", map[string]any{"value": "private"}); got != "missing_{value}" {
		t.Fatalf("missing key substituted: %q", got)
	}
	if got := Tf("en", "search_result", "example"); got != "Search: example" {
		t.Fatal(got)
	}
	// Use an isolated formatter so this test never mutates the application's bundle.
	b := kit.NewBundle(kit.LangEN)
	b.AddTranslation(kit.LangEN, "named", "Hello {name}: {count}")
	if got := kit.NewFormatter(b).Format(kit.LangZH, "named", map[string]any{"name": "<>&\"'", "count": 2}); got != "Hello <>&\"': 2" {
		t.Fatal(got)
	}
}

func TestConcurrentTranslations(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				if T("zh-CN", "back") != "返回" || T("en-US", "back") != "Back" {
					t.Error("request languages affected one another")
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestCatalogParity(t *testing.T) {
	catalogs := make(map[string]map[string]string)
	for _, lang := range []string{"zh", "en"} {
		data, err := localesFS.ReadFile("locales/" + lang + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(data, &catalog); err != nil {
			t.Fatal(err)
		}
		catalogs[lang] = catalog
	}
	placeholder := regexp.MustCompile(`\{[[:alnum:]_]+\}|%[-+#0-9.*]*[sdvfg]`)
	signature := func(value string) string {
		matches := placeholder.FindAllString(value, -1)
		sort.Strings(matches)
		return strings.Join(matches, "|")
	}
	for lang, catalog := range catalogs {
		for key, value := range catalog {
			other := "en"
			if lang == "en" {
				other = "zh"
			}
			translated, ok := catalogs[other][key]
			if !ok {
				t.Error(fmt.Sprintf("%s missing %s", other, key))
				continue
			}
			if signature(value) != signature(translated) {
				t.Errorf("placeholder mismatch for %s: %q / %q", key, value, translated)
			}
		}
	}
}
