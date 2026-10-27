package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
	"github.com/soulteary/flare/internal/security"
	"github.com/soulteary/flare/internal/settings"
	"github.com/soulteary/flare/internal/settings/appearance"
	"github.com/soulteary/flare/internal/settings/others"
	"github.com/soulteary/flare/internal/settings/search"
	"github.com/soulteary/flare/internal/settings/theme"
	"github.com/soulteary/flare/internal/settings/weather"
)

// fiberErrorHandler keeps internal failures out of public error responses.
// Returned errors remain unchanged for the request logger; only explicitly
// supplied client-error messages are exposed to the caller.
func fiberErrorHandler(c fiber.Ctx, err error) error {
	status := http.StatusInternalServerError
	message := http.StatusText(http.StatusInternalServerError)
	var httpErr *fiber.Error
	if errors.As(err, &httpErr) && httpErr != nil {
		status = httpErr.Code
		if status < http.StatusInternalServerError {
			message = httpErr.Message
			if message == "" {
				message = http.StatusText(status)
			}
		}
	}
	if message == "REAUTH_REQUIRED" || message == "AUTH_REQUIRED" || message == "SESSION_EXPIRED" || message == "CSRF_INVALID" {
		// Errors returned by the shared authorization service retain a stable
		// machine code even when raised inside a content commit callback.
		id := c.GetRespHeader("X-Request-ID")
		if id == "" {
			raw := make([]byte, 12)
			if _, err := rand.Read(raw); err == nil {
				id = hex.EncodeToString(raw)
				c.Set("X-Request-ID", id)
			}
		}
		return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": message, "message": message}, "request_id": id})
	}
	if c.Method() == http.MethodHead {
		return c.Status(status).Send(nil)
	}
	return c.Status(status).JSON(fiber.Map{"message": message})
}

// NewFiberApp builds the native Fiber app.
// It returns an error if any required initialization (templates, mdi, editor) fails.
// The given appFlags are synced to define.AppFlags and receive the effective cookie signing key.
func NewFiberApp(appFlags *model.Flags, stores ...*security.Store) (*fiber.App, error) {
	if appFlags != nil {
		define.AppFlags = *appFlags
	}
	if define.AppFlags.StartupStatus != "" && define.AppFlags.StartupStatus != "ready" {
		e := fiber.New(fiber.Config{ErrorHandler: fiberErrorHandler, Immutable: true})
		e.Use(func(c fiber.Ctx) error {
			c.Set("Cache-Control", "private, no-store")
			return c.Status(503).Type("html", "utf-8").SendString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Flare 尚未就绪</title></head><body><main><h1>Flare 尚未就绪</h1>
<p>现有内容未公开。请由实例管理员查看终端输出或容器日志，确认具体原因。</p>
<h2>首次使用：初始化管理员</h2>
<p>先停止服务，再在原工作目录、数据挂载和环境变量下执行初始化。密码需输入两次，终端不回显；要求 15–128 个字符且不能是弱密码。</p>
<h3>源码运行</h3><p>在运行终端按 Ctrl+C，然后执行：</p><pre>go run . security init --username admin
go run .</pre>
<h3>Docker Compose</h3><p>在原 Compose 目录执行（服务名以 flare 为例）：</p><pre>docker compose stop flare
docker compose run --rm --no-deps flare flare security init --username admin
docker compose up -d flare</pre>
<p>初始化成功并重启后，刷新页面，使用设置的账户登录。Docker 必须持久化 /app/.flare-security/（或自定义 FLARE_STATE_DIR）；挂载整个 /app 已包含该目录。</p>
<h2>已经初始化过？</h2><p>请根据实例终端或容器日志检查配置、目录权限和持久化状态。不要重复初始化或删除安全状态目录；损坏或缺失的已提交状态需要恢复完整备份。</p>
<p>完整说明见项目文档 docs/accounts.md（含 docker run 步骤）和 docs/backup-restore.md。</p></main></body></html>`)
		})
		return e, nil
	}
	define.Init()
	views, err := templates.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("初始化模板: %w", err)
	}
	e := fiber.New(fiber.Config{
		Views:                        views,
		ErrorHandler:                 fiberErrorHandler,
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
	if os.Getenv("FLARE_BASELINE") != "1" {
		log := logger.GetLogger()
		e.Use(logger.NewFiberWithConfig(log, logger.LoggerConfig{
			AccessLog: define.AppFlags.AccessLog,
			Skipper:   logger.DefaultRequestLogSkipper,
		}))
	}
	// Recover inside the access logger so panics are recorded as failed requests.
	e.Use(recover.New())

	if len(stores) > 0 {
		guard, err := NewWriteSourceProtection()
		if err != nil {
			return nil, fmt.Errorf("初始化请求来源: %w", err)
		}
		e.Use(guard)
	} else {
		e.Use(protectWriteSource)
	}
	if err := auth.RequestHandle(e, stores...); err != nil {
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

func NewRouter(flags *model.Flags, stores ...*security.Store) (http.Handler, error) {
	router, err := newRouter(flags, stores...)
	if err != nil {
		return nil, err
	}
	return router, nil
}

func newRouter(flags *model.Flags, stores ...*security.Store) (*fiberRouter, error) {
	app, err := NewFiberApp(flags, stores...)
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
