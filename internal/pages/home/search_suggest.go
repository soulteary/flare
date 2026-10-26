package home

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/fn"
	searchcore "github.com/soulteary/flare/internal/search"
	"github.com/soulteary/flare/internal/searchui"
)

func hybridEnabled(options model.Application) bool {
	if !options.ShowSearchComponent || options.Search == nil {
		return false
	}
	opts, _ := searchcore.Upgrade(options.Search)
	return (opts.Mode == "local-first" || (opts.Mode == "hybrid" && opts.DefaultTarget != "local")) && searchcore.Validate(opts) == nil
}

func populateHybridSearch(m map[string]any, options model.Application) {
	enabled := hybridEnabled(options) && m["SearchConfigured"] == true
	m["SearchHybrid"] = enabled
	if !enabled {
		return
	}
	keywords := map[string]string{}
	opts, _ := searchcore.Upgrade(options.Search)
	if opts.EnableKeywords {
		keywords["local"] = "local"
		for _, engine := range opts.Engines {
			if engine.Enabled && engine.Keyword != "" {
				keywords[strings.ToLower(engine.Keyword)] = engine.ID
			}
		}
	}
	encoded, err := json.Marshal(keywords)
	if err != nil {
		m["SearchHybrid"] = false
		return
	}
	m["SearchHybridKeywords"] = string(encoded)
}

// Both optional homepage scripts share a single policy: no hash overwrites another.
func applyHomeScriptPolicy(c fiber.Ctx, withClock, withSearch bool) {
	if define.AppFlags.DisableCSP {
		return
	}
	if !withSearch {
		if withClock {
			c.Set("Content-Security-Policy", clock.Policy())
		}
		return
	}
	hashes := searchui.Hash()
	if withClock {
		hashes += " " + strings.TrimPrefix(strings.SplitN(clock.Policy(), ";", 2)[0], "script-src ")
	}
	c.Set("Content-Security-Policy", "script-src "+hashes+"; script-src-attr 'none'; connect-src 'self'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script';")
}

func registerSearchSuggestions(e *fiber.App) {
	noStore := func(c fiber.Ctx) error {
		c.Set("Cache-Control", "no-store")
		c.Set("X-Content-Type-Options", "nosniff")
		return c.Next()
	}
	if define.AppFlags.Visibility == "PRIVATE" {
		e.Post("/search/suggest", noStore, auth.AuthRequired, pageSearchSuggestions)
	} else {
		e.Post("/search/suggest", noStore, pageSearchSuggestions)
	}
}

type searchSuggestion struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	NewTab bool   `json:"newTab"`
}

func pageSearchSuggestions(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.SendStatus(http.StatusInternalServerError)
	}
	if !hybridEnabled(options) {
		return c.SendStatus(http.StatusNotFound)
	}
	query, err := searchcore.ValidateQuery(c.FormValue("search"))
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON([]searchSuggestion{})
	}
	if query == "" {
		return c.JSON([]searchSuggestion{})
	}
	// Suggestions are for the homepage, not the endpoint's own pathname. Use request-
	// scoped dynamic URL metadata so simultaneous clients cannot alter one another.
	r := http.Request{Host: string(c.Request().Host()), URL: &url.URL{Path: "/"}, RequestURI: "/"}
	if c.RequestCtx().IsTLS() {
		r.TLS = &tls.ConnectionState{}
	}
	info := fn.ParseRequestURLTo(&r)
	matches, err := collectSearchSuggestions(options, query, &info)
	if err != nil {
		return c.SendStatus(http.StatusInternalServerError)
	}
	return c.JSON(matches)
}

func collectSearchSuggestions(options model.Application, query string, info *fn.DynamicURL) ([]searchSuggestion, error) {
	matches, err := data.SearchNavigationMatches(query, options.ShowApps, options.ShowBookmarks, 8, func(link string) string { return fn.ParseDynamicUrlWith(link, info) }, safeSuggestionURL)
	if err != nil {
		return nil, err
	}
	result := make([]searchSuggestion, 0, len(matches))
	for _, match := range matches {
		link := match.URL
		if options.EnableEncryptedLink || strings.HasPrefix(link, "chrome-extension://") {
			link = fn.EncodedNavigationURL(link, info)
		}
		newTab := options.OpenBookmarkNewTab
		if match.IsApp {
			newTab = options.OpenAppNewTab
		}
		result = append(result, searchSuggestion{Name: match.Bookmark.Name, URL: link, NewTab: newTab})
	}
	return result, nil
}

func safeSuggestionURL(value string) bool {
	if strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme == "" {
		return value != "" && !strings.HasPrefix(value, "//") && !strings.Contains(value, "\\")
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https" || parsed.Scheme == "chrome-extension") && parsed.Host != "" && parsed.User == nil
}
