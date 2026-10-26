// Package search validates configured search targets and resolves queries without making network requests.
package search

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soulteary/flare/config/model"
)

const MaxQueryRunes = 512
const MaxURLBytes = 8192
const placeholder = "{query}"

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// Result is either a local query (URL is empty) or an external navigation.
type Result struct{ Query, Target, URL string }

// Normalize applies defaults and returns an independent copy of the configuration.
func Normalize(options *model.SearchOptions) model.SearchOptions {
	result := model.SearchOptions{Mode: "select", DefaultTarget: "local"}
	if options == nil {
		return result
	}
	result = *options
	result.Engines = append([]model.SearchEngine(nil), options.Engines...)
	if result.Mode == "" {
		result.Mode = "select"
	}
	if result.DefaultTarget == "" {
		result.DefaultTarget = "local"
	}
	return result
}

// Presets returns editable templates, not a list automatically enabled on upgrade.
func Presets() []model.SearchEngine {
	return []model.SearchEngine{
		{ID: "bing", Name: "Bing", URLTemplate: "https://www.bing.com/search?q={query}", Enabled: true},
		{ID: "google", Name: "Google", URLTemplate: "https://www.google.com/search?q={query}", Enabled: true},
		{ID: "baidu", Name: "百度", URLTemplate: "https://www.baidu.com/s?wd={query}", Enabled: true},
		{ID: "duckduckgo", Name: "DuckDuckGo", URLTemplate: "https://duckduckgo.com/?q={query}", Enabled: true},
	}
}

func Validate(options model.SearchOptions) error {
	options = Normalize(&options)
	if options.Mode != "select" && options.Mode != "hybrid" && options.Mode != "local-first" {
		return fmt.Errorf("unsupported search mode")
	}
	if len(options.Engines) > 32 {
		return fmt.Errorf("at most 32 search engines are allowed")
	}
	ids := map[string]bool{"local": true}
	defaultFound := options.DefaultTarget == "local"
	for _, engine := range options.Engines {
		if !identifier.MatchString(engine.ID) || ids[engine.ID] {
			return fmt.Errorf("invalid or duplicate engine ID: %q", engine.ID)
		}
		ids[engine.ID] = true
		if strings.TrimSpace(engine.Name) == "" || utf8.RuneCountInString(engine.Name) > 64 || hasControl(engine.Name) || !utf8.ValidString(engine.Name) {
			return fmt.Errorf("engine %q requires a name of at most 64 characters", engine.ID)
		}
		if _, err := parseTemplate(engine.URLTemplate); err != nil {
			return fmt.Errorf("engine %q: %w", engine.ID, err)
		}
		if engine.ID == options.DefaultTarget && engine.Enabled {
			defaultFound = true
		}
	}
	if !defaultFound {
		return fmt.Errorf("default search target must be local or an enabled engine")
	}
	if options.Mode == "hybrid" && options.DefaultTarget == "local" {
		return fmt.Errorf("hybrid search requires an enabled external default target")
	}
	return nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// ValidateQuery trims surrounding whitespace but preserves the query's internal spacing.
func ValidateQuery(query string) (string, error) {
	if !utf8.ValidString(query) {
		return "", fmt.Errorf("search query must be valid UTF-8")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "", fmt.Errorf("enter a search query")
	}
	if utf8.RuneCountInString(query) > MaxQueryRunes {
		return "", fmt.Errorf("search query exceeds %d characters", MaxQueryRunes)
	}
	if hasControl(query) {
		return "", fmt.Errorf("search query contains control characters")
	}
	return query, nil
}

func parseTemplate(template string) (*url.URL, error) {
	if len(template) > 4096 || !utf8.ValidString(template) || hasControl(template) || strings.TrimSpace(template) != template {
		return nil, fmt.Errorf("invalid search URL template")
	}
	if strings.Count(template, placeholder) != 1 {
		return nil, fmt.Errorf("search URL must contain exactly one {query}")
	}
	// Replace the marker before parsing so braces cannot be mistaken for malformed URL syntax.
	const marker = "FLARESEARCHQUERYPLACEHOLDER"
	if strings.Contains(template, marker) {
		return nil, fmt.Errorf("invalid search URL template")
	}
	u, err := url.Parse(strings.Replace(template, placeholder, marker, 1))
	if err != nil || u == nil {
		return nil, fmt.Errorf("invalid search URL")
	}
	if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return nil, fmt.Errorf("search URL must be absolute HTTP(S) without credentials")
	}
	// The token may occur only within one query parameter value, never in a key, host, path, or fragment.
	if strings.Contains(u.Scheme+u.Host+u.Path+u.Fragment, marker) {
		return nil, fmt.Errorf("{query} must occur in a query parameter value")
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid search URL query")
	}
	found := false
	for key, items := range values {
		if strings.Contains(key, marker) {
			return nil, fmt.Errorf("{query} cannot occur in a parameter name")
		}
		for _, value := range items {
			if strings.Contains(value, marker) {
				found = true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("{query} must occur in a query parameter value")
	}
	return u, nil
}

func BuildURL(engine model.SearchEngine, query string) (string, error) {
	query, err := ValidateQuery(query)
	if err != nil {
		return "", err
	}
	if _, err = parseTemplate(engine.URLTemplate); err != nil {
		return "", err
	}
	result := strings.Replace(engine.URLTemplate, placeholder, url.QueryEscape(query), 1)
	if len(result) > MaxURLBytes {
		return "", fmt.Errorf("search URL is too long")
	}
	return result, nil
}

// Resolve accepts a configured engine ID, never a caller-provided destination URL.
func Resolve(options model.SearchOptions, query, target string) (Result, error) {
	options = Normalize(&options)
	if err := Validate(options); err != nil {
		return Result{}, err
	}
	query, err := ValidateQuery(query)
	if err != nil {
		return Result{}, err
	}
	if target == "" {
		target = options.DefaultTarget
		if options.Mode == "local-first" {
			target = "local"
		}
	}
	result := Result{Query: query, Target: target}
	if target == "local" {
		return result, nil
	}
	for _, engine := range options.Engines {
		if engine.ID == target && engine.Enabled {
			result.URL, err = BuildURL(engine, query)
			return result, err
		}
	}
	return Result{}, fmt.Errorf("search target is unavailable")
}
