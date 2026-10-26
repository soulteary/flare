package search

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	searchcore "github.com/soulteary/flare/internal/search"
)

func RegisterRouting(e *fiber.App) {
	guard := newSearchSecurity()
	e.Get(define.SettingPages.Search.Path, auth.AuthRequired, guard.attach, pageSearch)
	e.Post(define.SettingPages.Search.Path, auth.AuthRequired, guard.attach, guard.protect, updateSearchOptions)
}

func updateSearchOptions(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return recoverSearchForm(c, err)
	}
	options.ShowSearchComponent = c.FormValue("show-search-component") == "1"
	options.DisabledSearchAutoFocus = c.FormValue("disabled-search-auto-focus") == "1"
	if c.FormValue("auto-focus-setting") == "1" {
		options.DisabledSearchAutoFocus = c.FormValue("search-auto-focus") != "1"
	}
	if c.FormValue("engine-count") == "" {
		if !data.UpdateSearch(options.ShowSearchComponent, options.DisabledSearchAutoFocus) {
			return renderSettings(c, options, "Unable to save search settings", false)
		}
		return renderSettings(c, options, "", true)
	}
	opts, err := parseSearchForm(c)
	options.Search = &opts
	if err != nil {
		return renderSettings(c, options, err.Error(), false)
	}
	return applySearchAction(c, options, opts)
}

func parseSearchForm(c fiber.Ctx) (model.SearchOptions, error) {
	opts := model.SearchOptions{Mode: c.FormValue("search-mode", "select"), DefaultTarget: c.FormValue("default-target", "local"), OpenNewTab: c.FormValue("open-new-tab") == "1"}
	count, err := strconv.Atoi(c.FormValue("engine-count", "0"))
	if err != nil || count < 0 || count > 32 {
		return opts, fmt.Errorf("search supports at most 32 engines")
	}
	for i := 0; i < count; i++ {
		key := "engine-" + strconv.Itoa(i) + "-"
		opts.Engines = append(opts.Engines, model.SearchEngine{ID: c.FormValue(key + "id"), Name: c.FormValue(key + "name"), URLTemplate: c.FormValue(key + "url"), Enabled: c.FormValue(key+"enabled") == "1"})
	}
	return opts, nil
}

func pageSearch(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	return renderSettings(c, options, "", false)
}

func renderSettings(c fiber.Ctx, options model.Application, message string, saved bool) error {
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	if guard, ok := c.Locals(searchSecurityKey{}).(*searchSecurity); ok {
		m["SearchCSRFToken"] = guard.formToken(c)
	}
	m["SearchConfigRevision"] = options.ConfigRevision
	if pending := c.FormValue("config-revision"); pending != "" && !saved {
		m["SearchConfigRevision"] = pending
	}
	if saved {
		if current, err := data.GetAllSettingsOptions(); err == nil {
			m["SearchConfigRevision"] = current.ConfigRevision
		}
	}
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageName"] = "Search"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["ShowSearchComponent"] = options.ShowSearchComponent
	m["DisabledSearchAutoFocus"] = options.DisabledSearchAutoFocus
	m["OptionTitle"] = options.Title
	opts := searchcore.Normalize(options.Search)
	upgraded := false
	// Submitted forms are working state, not legacy persisted configuration.
	if c.Method() == http.MethodGet {
		opts, upgraded = searchcore.Upgrade(options.Search)
	}
	m["SearchCompatibility"] = (upgraded || options.Search == nil) && !saved && c.Method() == http.MethodGet
	m["SearchVersionDowngrade"] = options.ConfigVersionState == "downgrade"
	m["SearchConfigVersion"] = options.AppVersion
	m["SearchOptions"] = opts
	m["SearchModes"] = []struct{ ID, Title, Help string }{
		{"hybrid", "search_mode_hybrid", "search_mode_hybrid_help"},
		{"local-first", "search_mode_local_first", "search_mode_local_first_help"},
		{"select", "search_mode_select", "search_mode_select_help"},
	}
	defaultPresent := opts.DefaultTarget == "local"
	for _, engine := range opts.Engines {
		if engine.ID == opts.DefaultTarget {
			defaultPresent = true
		}
	}
	m["SearchDefaultUnavailable"] = !defaultPresent
	m["SearchPresets"] = searchcore.Presets()
	m["SearchError"] = message
	m["SearchSaved"] = saved
	return c.Status(http.StatusOK).Render("settings.html", m)
}

func recoverSearchForm(c fiber.Ctx, err error) error {
	// Retain the submitted form for recovery, but never use fallback data to write.
	options := model.Application{Locale: "zh", ShowSearchComponent: c.FormValue("show-search-component") == "1", DisabledSearchAutoFocus: c.FormValue("disabled-search-auto-focus") == "1"}
	if c.FormValue("auto-focus-setting") == "1" {
		options.DisabledSearchAutoFocus = c.FormValue("search-auto-focus") != "1"
	}
	if c.FormValue("engine-count") != "" {
		pending, parseErr := parseSearchForm(c)
		options.Search = &pending
		if parseErr != nil {
			err = fmt.Errorf("%w; invalid submitted form: %v", err, parseErr)
		}
	}
	return renderSettings(c, options, "Unable to save search settings: "+err.Error(), false)
}

func applySearchAction(c fiber.Ctx, options model.Application, opts model.SearchOptions) error {
	action := c.FormValue("action", "save")
	// Editing actions change the working form only. Save explicitly commits all rows.
	switch {
	case action == "add":
		if len(opts.Engines) >= 32 {
			return renderSettings(c, options, "At most 32 search engines are allowed", false)
		}
		opts.Engines = append(opts.Engines, model.SearchEngine{Enabled: true})
	case strings.HasPrefix(action, "preset:"):
		if err := addSearchPreset(&opts, strings.TrimPrefix(action, "preset:")); err != nil {
			return renderSettings(c, options, err.Error(), false)
		}
	case strings.HasPrefix(action, "delete:") || strings.HasPrefix(action, "up:") || strings.HasPrefix(action, "down:"):
		if err := editSearchRow(&opts, action); err != nil {
			return renderSettings(c, options, err.Error(), false)
		}
	case strings.HasPrefix(action, "test:"):
		index, e := strconv.Atoi(strings.TrimPrefix(action, "test:"))
		if e != nil || index < 0 || index >= len(opts.Engines) {
			return renderSettings(c, options, "Invalid engine row", false)
		}
		target, e := searchcore.BuildURL(opts.Engines[index], "Flare search test")
		if e != nil {
			return renderSettings(c, options, e.Error(), false)
		}
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Cache-Control", "no-store")
		c.Set("Location", target)
		return c.SendStatus(http.StatusSeeOther)
	case action == "save":
		return saveSearchForm(c, options, opts)
	default:
		return renderSettings(c, options, "Unknown form action", false)
	}
	options.Search = &opts
	return renderSettings(c, options, "", false)
}

func addSearchPreset(opts *model.SearchOptions, id string) error {
	for _, preset := range searchcore.Presets() {
		if preset.ID == id {
			exists := false
			for _, e := range opts.Engines {
				if e.ID == id {
					exists = true
				}
			}
			if !exists {
				if len(opts.Engines) >= 32 {
					return fmt.Errorf("At most 32 search engines are allowed")
				}
				opts.Engines = append(opts.Engines, preset)
			}
		}
	}
	return nil
}

func editSearchRow(opts *model.SearchOptions, action string) error {
	parts := strings.SplitN(action, ":", 2)
	index, e := strconv.Atoi(parts[1])
	if e != nil || index < 0 || index >= len(opts.Engines) {
		return fmt.Errorf("Invalid engine row")
	}
	switch parts[0] {
	case "delete":
		opts.Engines = append(opts.Engines[:index], opts.Engines[index+1:]...)
	case "up":
		if index > 0 {
			opts.Engines[index-1], opts.Engines[index] = opts.Engines[index], opts.Engines[index-1]
		}
	case "down":
		if index+1 < len(opts.Engines) {
			opts.Engines[index+1], opts.Engines[index] = opts.Engines[index], opts.Engines[index+1]
		}
	}
	return nil
}

func saveSearchForm(c fiber.Ctx, options model.Application, opts model.SearchOptions) error {
	// Choosing web-submit explicitly authorizes an external default, but never
	// adds or enables an engine on the user's behalf.
	if opts.Mode == "hybrid" && opts.DefaultTarget == "local" {
		for _, engine := range opts.Engines {
			if engine.Enabled {
				opts.DefaultTarget = engine.ID
				break
			}
		}
		if opts.DefaultTarget == "local" {
			return renderSettings(c, options, i18n.T(options.Locale, "search_engine_required"), false)
		}
	}
	options.Search = &opts
	if err := searchcore.Validate(opts); err != nil {
		return renderSettings(c, options, err.Error(), false)
	}
	if err := data.UpdateSearchOptions(options.ShowSearchComponent, options.DisabledSearchAutoFocus, &opts, c.FormValue("config-revision")); err != nil {
		return renderSettings(c, options, "Unable to save search settings: "+err.Error(), false)
	}
	options.Search = &opts
	return renderSettings(c, options, "", true)
}
