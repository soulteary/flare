//go:build !fiber

package logger

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v5"
)

const requestAttrsCap = 13

var requestAttrsPool = sync.Pool{
	New: func() any {
		attrs := make([]slog.Attr, 0, requestAttrsCap)
		return &attrs
	},
}

// LoggerConfig configures the request logging middleware.
type LoggerConfig struct {
	// Skipper selects requests whose successful responses do not need logging
	// (e.g. health checks and static assets). Errors and HTTP 4xx/5xx are always logged.
	Skipper func(c *echo.Context) bool
}

// NewEcho returns an Echo middleware that logs each request with slog.
func NewEcho(logger *slog.Logger) echo.MiddlewareFunc {
	return NewEchoWithConfig(logger, LoggerConfig{})
}

// NewEchoWithConfig returns the middleware with optional skipper and tuning.
func NewEchoWithConfig(logger *slog.Logger, config LoggerConfig) echo.MiddlewareFunc {
	skipper := config.Skipper
	if skipper == nil {
		skipper = func(*echo.Context) bool { return false }
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			skip := skipper(c)
			start := time.Now()
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery

			err := next(c)

			// Echo handles returned errors after this middleware unwinds. Resolve
			// their status without writing a response or invoking the error handler.
			// An already committed response keeps the status sent to the client.
			_, status := echo.ResolveResponseStatus(c.Response(), err)
			if skip && err == nil && status < http.StatusBadRequest {
				return nil
			}
			// Only build path params for failures to avoid allocations on success.
			var params map[string]string
			if err != nil || status >= http.StatusBadRequest {
				if pv := c.PathValues(); len(pv) > 0 {
					params = make(map[string]string, len(pv))
					for _, v := range pv {
						params[v.Name] = v.Value
					}
				}
			}
			method := c.Request().Method
			host := c.Request().Host
			route := c.Path()
			latency := time.Since(start)
			userAgent := c.Request().UserAgent()
			ip := c.RealIP()
			referer := c.Request().Referer()

			attrsPtr, ok := requestAttrsPool.Get().(*[]slog.Attr)
			if !ok || attrsPtr == nil {
				attrsPtr = &[]slog.Attr{}
			}
			requestAttributes := attrsPtr
			*requestAttributes = (*requestAttributes)[:0]
			*requestAttributes = append(*requestAttributes,
				slog.Time("time", start),
				slog.String("method", method),
				slog.String("host", host),
				slog.String("path", path),
				slog.String("query", query),
			)
			if params != nil {
				*requestAttributes = append(*requestAttributes, slog.Any("params", params))
			}
			*requestAttributes = append(*requestAttributes,
				slog.String("route", route),
				slog.String("ip", ip),
				slog.String("referer", referer),
				slog.String("user-agent", userAgent),
				slog.Duration("latency", latency),
				slog.Int("status", status),
			)

			level := slog.LevelInfo
			msg := "REQUEST"
			if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
				level = slog.LevelWarn
			} else if status >= http.StatusInternalServerError || err != nil {
				level = slog.LevelError
			}
			if err != nil {
				msg = err.Error()
				*requestAttributes = append(*requestAttributes, slog.Any("error", err))
			}

			// Async log for successful 2xx to shorten the critical path. A handler
			// can fail after committing a 2xx response; keep those errors synchronous.
			if err == nil && status >= 200 && status < 300 {
				go func(attrs *[]slog.Attr) {
					logger.LogAttrs(context.Background(), level, msg, (*attrs)...)
					*attrs = (*attrs)[:0]
					requestAttrsPool.Put(attrs)
				}(requestAttributes)
			} else {
				logger.LogAttrs(c.Request().Context(), level, msg, (*requestAttributes)...)
				*requestAttributes = (*requestAttributes)[:0]
				requestAttrsPool.Put(requestAttributes)
			}
			return err
		}
	}
}

// DefaultRequestLogSkipper selects health checks, favicon, redirects, and static
// assets for skipping successful request logs. NewEchoWithConfig still logs failures.
func DefaultRequestLogSkipper(c *echo.Context) bool {
	path := c.Request().URL.Path
	return path == "/ping" || path == "/favicon.ico" || strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/redir/")
}
