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
	opts := searchcore.Normalize(options.Search)
	return opts.Mode == "hybrid" && opts.DefaultTarget != "local" && searchcore.Validate(opts) == nil
}

func populateHybridSearch(m map[string]any, options model.Application) {
	enabled := hybridEnabled(options) && m["SearchConfigured"] == true
	m["SearchHybrid"] = enabled
	if !enabled {
		return
	}
	keywords := map[string]string{}
	opts := searchcore.Normalize(options.Search)
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
	// A hidden module is outside the search scope. Its missing or malformed
	// backing file must not break suggestions from a visible module.
	var apps, bookmarks model.Bookmarks
	if options.ShowApps {
		apps, err = data.LoadFavoriteBookmarks()
		if err != nil {
			return c.SendStatus(http.StatusInternalServerError)
		}
	}
	if options.ShowBookmarks {
		bookmarks, err = data.LoadNormalBookmarks()
		if err != nil {
			return c.SendStatus(http.StatusInternalServerError)
		}
	}
	// Suggestions are for the homepage, not the endpoint's own pathname. Use request-
	// scoped dynamic URL metadata so simultaneous clients cannot alter one another.
	r := http.Request{Host: string(c.Request().Host()), URL: &url.URL{Path: "/"}, RequestURI: "/"}
	if c.RequestCtx().IsTLS() {
		r.TLS = &tls.ConnectionState{}
	}
	info := fn.ParseRequestURLTo(&r)
	return c.JSON(collectSearchSuggestions(options, query, apps, bookmarks, &info))
}

func collectSearchSuggestions(options model.Application, query string, apps, bookmarks model.Bookmarks, info *fn.DynamicURL) []searchSuggestion {
	result := make([]searchSuggestion, 0, 8)
	query = strings.ToLower(query)
	appendMatches := func(items []model.Bookmark, isApp bool) {
		for _, item := range items {
			if len(result) == 8 {
				return
			}
			link := fn.ParseDynamicUrlWith(item.URL, info)
			matched := strings.Contains(strings.ToLower(item.Name), query) || strings.Contains(strings.ToLower(link), query)
			if isApp {
				matched = matched || strings.Contains(strings.ToLower(item.Desc), query)
			}
			if !matched || !safeSuggestionURL(link) {
				continue
			}
			if !isApp && item.Category != "" && len(bookmarks.Categories) > 0 {
				found := false
				for _, category := range bookmarks.Categories {
					if category.ID == item.Category {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if options.EnableEncryptedLink || strings.HasPrefix(link, "chrome-extension://") {
				link = fn.EncodedNavigationURL(link, info)
			}
			newTab := options.OpenBookmarkNewTab
			if isApp {
				newTab = options.OpenAppNewTab
			}
			result = append(result, searchSuggestion{Name: item.Name, URL: link, NewTab: newTab})
		}
	}
	if options.ShowApps {
		appendMatches(apps.Items, true)
	}
	if options.ShowBookmarks {
		appendMatches(bookmarks.Items, false)
	}
	return result
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
