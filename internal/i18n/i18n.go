package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"time"

	kit "github.com/soulteary/i18n-kit/v4"
)

//go:embed locales/*.json
var localesFS embed.FS

var bundle = mustLoadBundle(localesFS)
var translator = kit.NewTranslator(bundle)
var formatter = kit.NewFormatter(bundle)

func mustLoadBundle(source fs.FS) *kit.Bundle {
	b, err := loadBundle(source)
	if err != nil {
		panic(err)
	}
	return b
}

func loadBundle(source fs.FS) (*kit.Bundle, error) {
	b := kit.NewBundle(kit.LangEN)
	for _, lang := range []kit.Language{kit.LangZH, kit.LangEN} {
		path := "locales/" + lang.String() + ".json"
		data, err := fs.ReadFile(source, path)
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", path, err)
		}
		if err := b.LoadJSON(lang, data); err != nil {
			return nil, fmt.Errorf("i18n: load %s: %w", path, err)
		}
	}
	return b, nil
}

// ParseLocale accepts aliases only for languages translated by this project.
func ParseLocale(locale string) (string, bool) {
	lang, ok := kit.ParseLanguage(locale)
	if !ok || (lang != kit.LangZH && lang != kit.LangEN) {
		return "", false
	}
	return lang.String(), true
}

// Normalize uses the site's Chinese default for an empty locale and English
// for unsupported locales, matching the historical translation fallback.
func Normalize(locale string) string {
	if strings.TrimSpace(locale) == "" {
		return "zh"
	}
	if lang, ok := ParseLocale(locale); ok {
		return lang
	}
	return "en"
}

// HTMLLang returns the language tag for the rendered document.
func HTMLLang(locale string) string {
	if Normalize(locale) == "zh" {
		return "zh-CN"
	}
	return "en"
}

// T returns a translation, falling back to English and then the key itself.
func T(locale, key string) string {
	return translator.TWithLang(kit.Language(Normalize(locale)), key)
}

// Tf applies printf arguments only when the translation exists.
func Tf(locale, key string, args ...any) string {
	return translator.TfWithLang(kit.Language(Normalize(locale)), key, args...)
}

// Format substitutes named {parameters}. Callers must escape output for their
// rendering context, just as with T and Tf.
func Format(locale, key string, params map[string]any) string {
	lang := kit.Language(Normalize(locale))
	if _, found := bundle.LookupTranslation(lang, key); !found {
		return key
	}
	return formatter.Format(lang, key, params)
}

// Weekday returns a translated weekday name (Sunday=0).
func Weekday(locale string, w time.Weekday) string {
	keys := []string{"weekday_sun", "weekday_mon", "weekday_tue", "weekday_wed", "weekday_thu", "weekday_fri", "weekday_sat"}
	if int(w) < 0 || int(w) > 6 {
		return ""
	}
	return T(locale, keys[w])
}

// DateFormat returns a layout suitable for time.Format.
func DateFormat(locale string) string {
	if Normalize(locale) == "en" {
		return "Jan 02, 2006"
	}
	return "2006年01月02日"
}
