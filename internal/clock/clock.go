// Package clock implements display-only clock settings, without changing time.Local.
package clock

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Keep IANA zones available in standalone binaries and minimal containers.

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

//go:embed scripts/clock.js
var script string

//go:embed scripts/settings.js
var settingsScript string

// Scripts are immutable embedded sources. The exact same bytes are hashed and
// rendered after HTML minification, so formatting cannot invalidate the CSP hash.
func Script() template.JS         { return template.JS(script) }
func SettingsScript() template.JS { return template.JS(settingsScript) }
func scriptPolicy(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "script-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'; script-src-attr 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script';"
}

var policy = scriptPolicy(script)
var settingsPolicy = scriptPolicy(settingsScript)

func Policy() string         { return policy }
func SettingsPolicy() string { return settingsPolicy }

// Validate accepts geographical IANA names and UTC, never OS paths or Local.
func Validate(c model.ClockOptions) error {
	switch c.TimezoneMode {
	case "", "server", "browser":
	case "custom":
		if _, err := loadZone(c.Timezone); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid clock timezone mode")
	}
	return nil
}
func loadZone(name string) (*time.Location, error) {
	if name != "UTC" && (!strings.Contains(name, "/") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.ContainsAny(name, "\\\x00\r\n\t ")) {
		return nil, fmt.Errorf("use an IANA time zone such as Asia/Shanghai or UTC")
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown IANA time zone %q", name)
	}
	return loc, nil
}

var invalidWarning sync.Once
var opaqueWarning sync.Once

type Resolved struct {
	Options  model.ClockOptions
	Location *time.Location
	Zone     string
	Enhance  bool
}

// Resolve tolerates invalid hand-edited configuration but logs only once.
// An opaque server Local zone retains its original static rendering; a fixed
// offset is never substituted for a geographical zone with daylight saving.
func Resolve(input *model.ClockOptions, visible bool) Resolved {
	c := model.ClockOptions{TimezoneMode: "server"}
	if input != nil {
		c = *input
	}
	if c.TimezoneMode == "" {
		c.TimezoneMode = "server"
	}
	if err := Validate(c); err != nil {
		invalidWarning.Do(func() { log.Printf("clock settings invalid; using server time zone: %v", err) })
		c.TimezoneMode = "server"
	}
	loc := time.Local
	if c.TimezoneMode == "custom" {
		if customLoc, err := loadZone(c.Timezone); err == nil {
			loc = customLoc
		} else {
			c.TimezoneMode = "server"
			invalidWarning.Do(func() { log.Printf("clock time zone unavailable; using server zone: %v", err) })
		}
	}
	zone := loc.String()
	enhance := visible && (c.Dynamic || c.TimezoneMode == "browser")
	if enhance && c.TimezoneMode != "browser" {
		if _, err := loadZone(zone); err != nil {
			enhance = false
			opaqueWarning.Do(func() {
				log.Printf("clock: server time zone %q cannot be used by the browser; retaining static time, configure TZ with an IANA zone", zone)
			})
		}
	}
	return Resolved{c, loc, zone, enhance}
}

func Greeting(greeting, locale string, hour int) string {
	words := strings.Split(greeting, ";")
	fallback := i18n.T(locale, "greetings_placeholder")
	if len(words) == 1 {
		if words[0] != "" {
			return words[0]
		}
		return fallback
	}
	index := 3
	if hour >= 5 && hour <= 10 {
		index = 0
	} else if hour >= 11 && hour <= 13 {
		index = 1
	} else if hour >= 14 && hour <= 18 {
		index = 2
	}
	if index < len(words) && words[index] != "" {
		return words[index]
	}
	return fallback
}

// Populate uses one instant and one location for every server-rendered component.
func Populate(m map[string]any, opts model.Application, now time.Time, updateGreeting bool) Resolved {
	r := Resolve(opts.Clock, opts.ShowDateTime)
	now = now.In(r.Location)
	layout := "15:04"
	if r.Options.ShowSeconds() {
		layout += ":05"
	}
	m["HeroDate"] = now.Format(i18n.DateFormat(opts.Locale))
	m["HeroDay"] = i18n.Weekday(opts.Locale, now.Weekday())
	m["HeroTime"] = now.Format(layout)
	m["ClockEnhance"] = r.Enhance
	m["ClockDynamic"] = r.Options.Dynamic
	m["ClockSeconds"] = r.Options.ShowSeconds()
	m["ClockMode"] = r.Options.TimezoneMode
	m["ClockZone"] = r.Zone
	m["ClockScript"] = Script()
	if updateGreeting {
		m["Greetings"] = Greeting(opts.Greetings, opts.Locale, now.Hour())
		if opts.ShowTitle {
			m["ClockGreetings"] = opts.Greetings
			m["ClockGreetingFallback"] = i18n.T(opts.Locale, "greetings_placeholder")
		}
	}
	return r
}
