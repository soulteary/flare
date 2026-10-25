//go:build fiber

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/recover"

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

// NewFiberApp builds the experimental native Fiber app.
// It returns an error if any required initialization (templates, mdi, guide, editor) fails.
// The given appFlags are synced to define.AppFlags and receive the effective cookie signing key.
func NewFiberApp(appFlags *model.Flags) (*fiber.App, error) {
	define.Init()
	if appFlags != nil {
		define.AppFlags = *appFlags
	}
	views, err := templates.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("初始化模板: %w", err)
	}
	e := fiber.New(fiber.Config{
		Views:                        views,
		Immutable:                    true,
		CaseSensitive:                true,
		StrictRouting:                true,
		BodyLimit:                    32 << 20,
		ReadBufferSize:               64 << 10,
		DisablePreParseMultipartForm: true,
		ReadTimeout:                  5 * time.Second,
		WriteTimeout:                 10 * time.Second,
		IdleTimeout:                  60 * time.Second,
	})
	e.Use(recover.New())
	if os.Getenv("FLARE_BASELINE") != "1" {
		log := logger.GetLogger()
		e.Use(logger.NewFiberWithConfig(log, logger.LoggerConfig{Skipper: logger.DefaultRequestLogSkipper}))
	}
	if err := auth.RequestHandle(e); err != nil {
		return nil, fmt.Errorf("初始化认证: %w", err)
	}
	if appFlags != nil {
		appFlags.CookieSecret = define.AppFlags.CookieSecret
	}
	home.InitWeatherIfNeeded()
	assets.RegisterRouting(e)
	health.RegisterRouting(e)
	home.RegisterRouting(e)
	settings.RegisterRouting(e)
	theme.RegisterRouting(e)
	weather.RegisterRouting(e)
	search.RegisterRouting(e)
	appearance.RegisterRouting(e)
	others.RegisterRouting(e)
	if err := mdi.Init(); err != nil {
		return nil, fmt.Errorf("初始化 MDI 资源: %w", err)
	}
	mdi.RegisterRouting(e)
	redir.RegisterRouting(e)
	if define.AppFlags.EnableGuide {
		if err := guide.Init(); err != nil {
			return nil, fmt.Errorf("初始化引导页: %w", err)
		}
		guide.RegisterRouting(e)
	}
	if define.AppFlags.EnableEditor {
		if err := editor.Init(); err != nil {
			return nil, fmt.Errorf("初始化编辑器: %w", err)
		}
		editor.RegisterRouting(e)
	}
	deprecated.RegisterRouting(e)
	return e, nil
}

// fiberRouter bridges net/http only for existing HTTP integration tests and
// embedders. Serve starts the native fasthttp listener used by the daemon and
// benchmark tool; measured network requests never pass through the adaptor.
type fiberRouter struct {
	app     *fiber.App
	handler http.Handler
}

func NewRouter(flags *model.Flags) (http.Handler, error) {
	app, err := NewFiberApp(flags)
	if err != nil {
		return nil, err
	}
	return &fiberRouter{app: app, handler: adaptor.FiberApp(app)}, nil
}

func (r *fiberRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

func (r *fiberRouter) Serve(listener net.Listener) error {
	return r.app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true})
}

func (r *fiberRouter) Shutdown(ctx context.Context) error {
	return r.app.ShutdownWithContext(ctx)
}

// ReadBufferSize lets the benchmark record the effective native configuration.
func (r *fiberRouter) ReadBufferSize() int {
	return r.app.Config().ReadBufferSize
}
