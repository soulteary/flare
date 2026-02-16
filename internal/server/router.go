package FlareServer

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlareLogger "github.com/soulteary/flare/internal/logger"
	FlareDeprecated "github.com/soulteary/flare/internal/misc/deprecated"
	FlareHealth "github.com/soulteary/flare/internal/misc/health"
	FlareRedir "github.com/soulteary/flare/internal/misc/redir"
	FlareEditor "github.com/soulteary/flare/internal/pages/editor"
	FlareGuide "github.com/soulteary/flare/internal/pages/guide"
	FlareHome "github.com/soulteary/flare/internal/pages/home"
	FlareAssets "github.com/soulteary/flare/internal/resources/assets"
	FlareMDI "github.com/soulteary/flare/internal/resources/mdi"
	FlareTemplates "github.com/soulteary/flare/internal/resources/templates"
	FlareSettings "github.com/soulteary/flare/internal/settings"
	FlareAppearance "github.com/soulteary/flare/internal/settings/appearance"
	FlareOthers "github.com/soulteary/flare/internal/settings/others"
	FlareSearch "github.com/soulteary/flare/internal/settings/search"
	FlareTheme "github.com/soulteary/flare/internal/settings/theme"
	FlareWeather "github.com/soulteary/flare/internal/settings/weather"
)

// NewRouter builds the Echo app and returns an http.Handler for the server.
func NewRouter(_ *FlareModel.Flags) http.Handler {
	FlareDefine.Init()
	e := echo.New()
	e.Use(middleware.Recover())
	if os.Getenv("FLARE_BASELINE") != "1" {
		log := FlareLogger.GetLogger()
		e.Use(FlareLogger.NewEchoWithConfig(log, FlareLogger.LoggerConfig{Skipper: FlareLogger.DefaultRequestLogSkipper}))
	}
	FlareAuth.RequestHandle(e)
	FlareTemplates.RegisterRouting(e)
	FlareAssets.RegisterRouting(e)
	FlareHealth.RegisterRouting(e)
	FlareHome.RegisterRouting(e)
	FlareSettings.RegisterRouting(e)
	FlareTheme.RegisterRouting(e)
	FlareWeather.RegisterRouting(e)
	FlareSearch.RegisterRouting(e)
	FlareAppearance.RegisterRouting(e)
	FlareOthers.RegisterRouting(e)
	FlareMDI.Init()
	FlareMDI.RegisterRouting(e)
	FlareRedir.RegisterRouting(e)
	if FlareDefine.AppFlags.EnableGuide {
		FlareGuide.Init()
		FlareGuide.RegisterRouting(e)
	}
	if FlareDefine.AppFlags.EnableEditor {
		FlareEditor.Init()
		FlareEditor.RegisterRouting(e)
	}
	FlareDeprecated.RegisterRouting(e)
	return e
}
