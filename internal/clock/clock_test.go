package clock

import (
	"strings"
	"testing"
	"time"

	"github.com/soulteary/flare/config/model"
	"go.yaml.in/yaml/v3"
)

func TestOldConfigurationDefaults(t *testing.T) {
	for _, input := range []string{"ShowDateTime: true", "ShowDateTime: true\nClock: {}", "ShowDateTime: true\nClock:\n  Seconds: false"} {
		var app model.Application
		if err := yaml.Unmarshal([]byte(input), &app); err != nil {
			t.Fatal(err)
		}
		r := Resolve(app.Clock, app.ShowDateTime)
		if r.Enhance || r.Options.Dynamic || r.Options.TimezoneMode != "server" {
			t.Fatalf("old defaults changed: %+v", r)
		}
		if r.Options.ShowSeconds() == strings.Contains(input, "false") {
			t.Fatal("seconds default lost")
		}
	}
}

func TestTimezoneValidation(t *testing.T) {
	for _, zone := range []string{"UTC", "Asia/Shanghai", "America/New_York", "Asia/Kathmandu", "Etc/GMT+5"} {
		if err := Validate(model.ClockOptions{TimezoneMode: "custom", Timezone: zone}); err != nil {
			t.Fatal(err)
		}
	}
	for _, zone := range []string{"", "Local", "CST", "/etc/localtime", "../Asia/Shanghai", "Asia/Unknown", "Asia/Shanghai\n", "UTC+8"} {
		if Validate(model.ClockOptions{TimezoneMode: "custom", Timezone: zone}) == nil {
			t.Fatalf("accepted %q", zone)
		}
	}
	if Validate(model.ClockOptions{TimezoneMode: "typo"}) == nil {
		t.Fatal("invalid mode accepted")
	}
	r := Resolve(&model.ClockOptions{TimezoneMode: "custom", Timezone: "no-such-zone", Dynamic: true}, true)
	if r.Location != time.Local || r.Options.TimezoneMode != "server" {
		t.Fatal("invalid config did not fall back")
	}
}

func TestPopulateCalendarAndGreetings(t *testing.T) {
	fixtures := []struct{ instant, zone, date, day, clock, greeting string }{
		{"2026-12-31T15:59:59Z", "Asia/Shanghai", "2026年12月31日", "星期四", "23:59:59", "night"},
		{"2026-12-31T16:00:00Z", "Asia/Shanghai", "2027年01月01日", "星期五", "00:00:00", "night"},
		{"2024-02-28T16:00:00Z", "Asia/Shanghai", "2024年02月29日", "星期四", "00:00:00", "night"},
		{"2026-03-08T06:59:59Z", "America/New_York", "2026年03月08日", "星期日", "01:59:59", "night"},
		{"2026-03-08T07:00:00Z", "America/New_York", "2026年03月08日", "星期日", "03:00:00", "night"},
		{"2026-11-01T05:59:59Z", "America/New_York", "2026年11月01日", "星期日", "01:59:59", "night"},
		{"2026-11-01T06:00:00Z", "America/New_York", "2026年11月01日", "星期日", "01:00:00", "night"},
		{"2026-10-25T00:00:00Z", "Asia/Kathmandu", "2026年10月25日", "星期日", "05:45:00", "morning"},
	}
	for _, f := range fixtures {
		t.Run(f.instant+f.zone, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, f.instant)
			if err != nil {
				t.Fatal(err)
			}
			m := map[string]any{}
			app := model.Application{Locale: "zh", ShowDateTime: true, ShowTitle: true, Greetings: "morning;noon;afternoon;night", Clock: &model.ClockOptions{Dynamic: true, TimezoneMode: "custom", Timezone: f.zone}}
			Populate(m, app, now, true)
			for key, want := range map[string]string{"HeroDate": f.date, "HeroDay": f.day, "HeroTime": f.clock, "Greetings": f.greeting} {
				if m[key] != want {
					t.Errorf("%s: got %v want %s", key, m[key], want)
				}
			}
			m = map[string]any{}
			Populate(m, app, now, false)
			if _, ok := m["ClockGreetings"]; ok {
				t.Fatal("help title may be overwritten")
			}
		})
	}
	no := false
	m := map[string]any{}
	Populate(m, model.Application{Locale: "en", Clock: &model.ClockOptions{Seconds: &no, TimezoneMode: "custom", Timezone: "UTC"}}, time.Date(2026, 10, 25, 10, 4, 5, 0, time.UTC), true)
	if m["HeroDate"] != "Oct 25, 2026" || m["HeroTime"] != "10:04" {
		t.Fatal(m)
	}
}

func TestGreetingBoundariesAndShortLists(t *testing.T) {
	for h := 0; h < 24; h++ {
		want := "night"
		if h >= 5 && h <= 10 {
			want = "morning"
		} else if h >= 11 && h <= 13 {
			want = "noon"
		} else if h >= 14 && h <= 18 {
			want = "afternoon"
		}
		if got := Greeting("morning;noon;afternoon;night", "en", h); got != want {
			t.Errorf("hour %d: %s", h, got)
		}
	}
	if Greeting("morning;noon", "en", 15) == "" {
		t.Fatal("missing segment has no fallback")
	}
	if Greeting("fixed", "en", 10) != "fixed" {
		t.Fatal("fixed greeting changed")
	}
}

func TestEnhancementAndOpaqueServerZone(t *testing.T) {
	orig := time.Local
	defer func() { time.Local = orig }()
	time.Local = time.FixedZone("Local", 8*3600)
	if Resolve(&model.ClockOptions{Dynamic: true}, true).Enhance {
		t.Fatal("opaque Local enhanced with fixed offset")
	}
	if !Resolve(&model.ClockOptions{TimezoneMode: "browser"}, true).Enhance {
		t.Fatal("browser initialization missing")
	}
	if Resolve(&model.ClockOptions{Dynamic: true, TimezoneMode: "custom", Timezone: "UTC"}, false).Enhance {
		t.Fatal("hidden clock enhanced")
	}
}
