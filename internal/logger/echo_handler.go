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

const requestAttrsCap = 12

var requestAttrsPool = sync.Pool{
	New: func() any { return make([]slog.Attr, 0, requestAttrsCap) },
}

// LoggerConfig configures the request logging middleware.
type LoggerConfig struct {
	// Skipper returns true to skip logging for the request (e.g. health check, favicon).
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
			if skipper(c) {
				return next(c)
			}
			start := time.Now()
			path := c.Request().URL.Path
			query := c.Request().URL.RawQuery

			err := next(c)

			status := 0
			if rw, _ := echo.UnwrapResponse(c.Response()); rw != nil {
				status = rw.Status
			}
			// Only build path params for error responses to keep 2xx hot path allocation-free.
			var params map[string]string
			if status >= http.StatusBadRequest {
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

			requestAttributes := requestAttrsPool.Get().([]slog.Attr)
			requestAttributes = requestAttributes[:0]
			requestAttributes = append(requestAttributes,
				slog.Time("time", start),
				slog.String("method", method),
				slog.String("host", host),
				slog.String("path", path),
				slog.String("query", query),
			)
			if params != nil {
				requestAttributes = append(requestAttributes, slog.Any("params", params))
			}
			requestAttributes = append(requestAttributes,
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
				if err != nil {
					msg = err.Error()
				}
			} else if status >= http.StatusInternalServerError {
				level = slog.LevelError
				if err != nil {
					msg = err.Error()
				}
			}

			// Async log for 2xx to shorten critical path and improve throughput.
			if status >= 200 && status < 300 {
				go func(attrs []slog.Attr) {
					logger.LogAttrs(context.Background(), level, msg, attrs...)
					attrs = attrs[:0]
					requestAttrsPool.Put(attrs)
				}(requestAttributes)
			} else {
				logger.LogAttrs(c.Request().Context(), level, msg, requestAttributes...)
				requestAttributes = requestAttributes[:0]
				requestAttrsPool.Put(requestAttributes)
			}
			return err
		}
	}
}

// DefaultRequestLogSkipper skips logging for health check, favicon, redirects, and static assets to reduce allocs in hot paths.
func DefaultRequestLogSkipper(c *echo.Context) bool {
	path := c.Request().URL.Path
	return path == "/ping" || path == "/favicon.ico" || strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/redir/")
}
