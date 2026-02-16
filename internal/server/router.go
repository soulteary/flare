package server

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/misc/deprecated"
	"github.com/soulteary/flare/internal/misc/health"
	"github.com/soulteary/flare/internal/misc/redir"
	"github.com/soulteary/flare/internal/pages/editor"
	"github.com/soulteary/flare/internal/pages/guide"
	"github.com/soulteary/flare/internal/pages/home"
	"github.com/soulteary/flare/internal/resources/assets"
	"github.com/soulteary/flare/internal/resources/mdi"
	"github.com/soulteary/flare/internal/resources/templates"
	"github.com/soulteary/flare/internal/settings"
	"github.com/soulteary/flare/internal/settings/appearance"
	"github.com/soulteary/flare/internal/settings/others"
	"github.com/soulteary/flare/internal/settings/search"
	"github.com/soulteary/flare/internal/settings/theme"
	"github.com/soulteary/flare/internal/settings/weather"
)

// NewRouter builds the Echo app and returns an http.Handler for the server.
func NewRouter(_ *model.Flags) http.Handler {
	define.Init()
	e := echo.New()
	e.Use(middleware.Recover())
	if os.Getenv("FLARE_BASELINE") != "1" {
		log := logger.GetLogger()
		e.Use(logger.NewEchoWithConfig(log, logger.LoggerConfig{Skipper: logger.DefaultRequestLogSkipper}))
	}
	auth.RequestHandle(e)
	home.InitWeatherIfNeeded()
	templates.RegisterRouting(e)
	assets.RegisterRouting(e)
	health.RegisterRouting(e)
	home.RegisterRouting(e)
	settings.RegisterRouting(e)
	theme.RegisterRouting(e)
	weather.RegisterRouting(e)
	search.RegisterRouting(e)
	appearance.RegisterRouting(e)
	others.RegisterRouting(e)
	mdi.Init()
	mdi.RegisterRouting(e)
	redir.RegisterRouting(e)
	if define.AppFlags.EnableGuide {
		guide.Init()
		guide.RegisterRouting(e)
	}
	if define.AppFlags.EnableEditor {
		editor.Init()
		editor.RegisterRouting(e)
	}
	deprecated.RegisterRouting(e)
	return e
}
