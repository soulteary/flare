package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	mu      sync.RWMutex
	bundles = make(map[string]map[string]string)
)

func init() {
	load("zh", "locales/zh.json")
	load("en", "locales/en.json")
}

func load(lang, path string) {
	data, err := localesFS.ReadFile(path)
	if err != nil {
		return
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	mu.Lock()
	bundles[lang] = m
	mu.Unlock()
}

// T 返回指定语言的文案，key 不存在或语言不存在时返回 key 本身。
// 未知 locale 时优先回退到 en，其次 zh。
func T(locale, key string) string {
	mu.RLock()
	m := bundles[locale]
	mu.RUnlock()
	if m == nil {
		mu.RLock()
		if locale != "zh" {
			m = bundles["en"]
		}
		if m == nil {
			m = bundles["zh"]
		}
		mu.RUnlock()
	}
	if m == nil {
		return key
	}
	if s, ok := m[key]; ok {
		return s
	}
	return key
}

// Tf 与 T 相同，但支持 fmt 格式化，args 用于 fmt.Sprintf。
func Tf(locale, key string, args ...any) string {
	return fmt.Sprintf(T(locale, key), args...)
}

// Weekday 返回某语言下的星期几名称（Sunday=0）。
func Weekday(locale string, w time.Weekday) string {
	keys := []string{"weekday_sun", "weekday_mon", "weekday_tue", "weekday_wed", "weekday_thu", "weekday_fri", "weekday_sat"}
	if int(w) < 0 || int(w) > 6 {
		return ""
	}
	return T(locale, keys[w])
}

// DateFormat 返回该语言下日期的格式 layout（用于 time.Format）。
func DateFormat(locale string) string {
	switch locale {
	case "en":
		return "Jan 02, 2006"
	default:
		return "2006年01月02日"
	}
}
