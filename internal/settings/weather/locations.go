package weather

import (
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	provider "github.com/soulteary/funny-china-weather"
)

//go:embed picker.js
var pickerJS string
var searchProvider = provider.SearchLocations
var locateProvider = provider.GetMyIPLocationContext

// Bound concurrent directory requests across all users.
var directoryBudget struct {
	sync.Mutex
	start time.Time
	count int
}
var directorySlots = make(chan struct{}, 4)
var errInvalidKeyword = errors.New("invalid keyword")
var errDirectoryBusy = errors.New("directory busy or rate limited")

func message(c fiber.Ctx, key string) string {
	options, _ := data.GetAllSettingsOptions()
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	return i18n.T(locale, key)
}
func pickerScript(c fiber.Ctx) error { return c.Type("js").SendString(pickerJS) }
func lookup(c fiber.Ctx, keyword string) ([]model.WeatherLocation, error) {
	keyword = strings.TrimSpace(keyword)
	if !utf8.ValidString(keyword) || keyword == "" || utf8.RuneCountInString(keyword) > 64 {
		return nil, errInvalidKeyword
	}
	if err := acquireDirectory(); err != nil {
		return nil, err
	}
	defer func() { <-directorySlots }()
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()
	values, err := resolveLegacy(ctx, keyword)
	if err != nil {
		return nil, err
	}
	result := make([]model.WeatherLocation, 0, len(values))
	for _, v := range values {
		result = append(result, model.WeatherLocation{ID: v.ID, Province: v.Province, City: v.City, County: v.County})
	}
	return result, nil
}
func acquireDirectory() error {
	directoryBudget.Lock()
	if time.Since(directoryBudget.start) >= time.Minute {
		directoryBudget.start = time.Now()
		directoryBudget.count = 0
	}
	if directoryBudget.count >= 120 {
		directoryBudget.Unlock()
		return errDirectoryBusy
	}
	directoryBudget.count++
	directoryBudget.Unlock()
	select {
	case directorySlots <- struct{}{}:
		return nil
	default:
		return errDirectoryBusy
	}
}
func searchLocations(c fiber.Ctx) error {
	values, err := lookup(c, c.Query("q"))
	c.Set("Cache-Control", "no-store")
	if err != nil {
		return locationFailure(c, err)
	}
	return c.JSON(values)
}

// MigrateLegacyLocation accepts only a unique provider result. It never splits
// Chinese addresses or invents administrative suffixes. Failure leaves YAML intact.
func MigrateLegacyLocation(value string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	values, err := resolveLegacy(ctx, value)
	if err != nil || len(values) != 1 {
		return
	}
	v := values[0]
	selected := &model.WeatherLocation{ID: v.ID, Province: v.Province, City: v.City, County: v.County}
	data.MigrateWeatherLocation(value, selected)
}

// Serialized provider addresses are searched by their most specific field, then matched exactly.
func resolveLegacy(ctx context.Context, value string) ([]provider.Location, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 && len(parts) != 3 {
		// v0.3.0 defaults and documented municipality inputs used the 市 suffix.
		// This compatibility map is deliberately limited to exact old names;
		// accept only the province/city pair confirmed by the provider.
		legacyMunicipalities := map[string]string{"北京市": "北京", "天津市": "天津", "上海市": "上海", "重庆市": "重庆"}
		if name, ok := legacyMunicipalities[value]; ok {
			values, err := searchProvider(ctx, name)
			if err != nil {
				return nil, err
			}
			matches := []provider.Location{}
			for _, candidate := range values {
				if candidate.Province == name && candidate.City == name && candidate.County == "" {
					matches = append(matches, candidate)
				}
			}
			return matches, nil
		}
		return searchProvider(ctx, value)
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	values, err := searchProvider(ctx, parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	matches := []provider.Location{}
	county := ""
	if len(parts) == 3 {
		county = parts[2]
	}
	for _, v := range values {
		if v.Province == parts[0] && v.City == parts[1] && v.County == county {
			matches = append(matches, v)
		}
	}
	return matches, nil
}
func detectLocation(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	if err := acquireDirectory(); err != nil {
		return c.Status(http.StatusTooManyRequests).JSON(fiber.Map{"error": message(c, "weather_detect_failed")})
	}
	defer func() { <-directorySlots }()

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	value, err := locateProvider(ctx)
	if err != nil {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": message(c, "weather_detect_failed")})
	}
	values, err := resolveLegacy(ctx, value)
	if err != nil || len(values) != 1 {
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{"error": message(c, "weather_detect_failed")})
	}
	v := values[0]
	c.Set("Cache-Control", "no-store")
	return c.JSON(model.WeatherLocation{ID: v.ID, Province: v.Province, City: v.City, County: v.County})
}

func locationFailure(c fiber.Ctx, err error) error {
	status, key, code := http.StatusBadGateway, "weather_search_failed", "upstream"
	var networkError *net.OpError
	switch {
	case errors.Is(err, errInvalidKeyword):
		status, key, code = http.StatusBadRequest, "weather_invalid_keyword", "invalid_keyword"
	case errors.Is(err, errDirectoryBusy):
		status, key, code = http.StatusTooManyRequests, "weather_search_busy", "busy"
	case errors.Is(err, context.DeadlineExceeded):
		status, key, code = http.StatusGatewayTimeout, "weather_search_timeout", "timeout"
	case errors.As(err, &networkError):
		key, code = "weather_network_failed", "network"
	}
	slog.Warn("weather location request failed", "keyword", c.Query("q"), "kind", code, "error", err)
	return c.Status(status).JSON(fiber.Map{"error": message(c, key), "code": code})
}
