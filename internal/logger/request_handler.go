package logger

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
)

const requestAttrsCap = 13

var requestAttrsPool = sync.Pool{
	New: func() any {
		attrs := make([]slog.Attr, 0, requestAttrsCap)
		return &attrs
	},
}

func acquireRequestAttrs(pool *sync.Pool) *[]slog.Attr {
	attrs, ok := pool.Get().(*[]slog.Attr)
	if !ok || attrs == nil {
		fresh := make([]slog.Attr, 0, requestAttrsCap)
		attrs = &fresh
	}
	*attrs = (*attrs)[:0]
	return attrs
}

func releaseRequestAttrs(pool *sync.Pool, attrs *[]slog.Attr) {
	clear(*attrs)
	*attrs = (*attrs)[:0]
	pool.Put(attrs)
}

type LoggerConfig struct {
	// Successful health checks and static assets may be skipped. Errors and
	// HTTP 4xx/5xx responses are always logged.
	Skipper func(fiber.Ctx) bool
}

func NewFiber(logger *slog.Logger) fiber.Handler {
	return NewFiberWithConfig(logger, LoggerConfig{})
}

func NewFiberWithConfig(logger *slog.Logger, config LoggerConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		skip := config.Skipper != nil && config.Skipper(c)
		start := time.Now()
		path := c.Path()
		query := c.Request().URI().QueryString()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			// Fiber resolves returned errors after middleware unwinds. Unlike
			// net/http, its buffered response has not been committed at this point.
			status = http.StatusInternalServerError
			var httpErr *fiber.Error
			if errors.As(err, &httpErr) && httpErr != nil {
				status = httpErr.Code
			}
		}
		if skip && err == nil && status < http.StatusBadRequest {
			return nil
		}
		var params map[string]string
		if err != nil || status >= http.StatusBadRequest {
			if names := c.Route().Params; len(names) > 0 {
				params = make(map[string]string, len(names))
				for _, name := range names {
					params[strings.Clone(name)] = strings.Clone(c.Params(name))
				}
			}
		}
		attrs := acquireRequestAttrs(&requestAttrsPool)
		// Fiber strings can alias pooled request buffers. Copy every retained
		// string before the successful request log moves to its goroutine.
		*attrs = append(*attrs,
			slog.Time("time", start),
			slog.String("method", strings.Clone(c.Method())),
			slog.String("host", strings.Clone(string(c.Request().Host()))),
			slog.String("path", strings.Clone(path)),
			slog.String("query", string(query)),
		)
		if params != nil {
			*attrs = append(*attrs, slog.Any("params", params))
		}
		*attrs = append(*attrs,
			slog.String("route", strings.Clone(c.Route().Path)),
			slog.String("ip", strings.Clone(c.IP())),
			slog.String("referer", strings.Clone(c.Get(fiber.HeaderReferer))),
			slog.String("user-agent", strings.Clone(c.Get(fiber.HeaderUserAgent))),
			slog.Duration("latency", time.Since(start)),
			slog.Int("status", status),
		)
		level, message := slog.LevelInfo, "REQUEST"
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			level = slog.LevelWarn
		} else if status >= http.StatusInternalServerError || err != nil {
			level = slog.LevelError
		}
		if err != nil {
			message = err.Error()
			*attrs = append(*attrs, slog.Any("error", err))
		}
		write := func(ctx context.Context) {
			logger.LogAttrs(ctx, level, message, (*attrs)...)
			releaseRequestAttrs(&requestAttrsPool, attrs)
		}
		logContext := c.Context()
		if err == nil && status >= http.StatusOK && status < http.StatusMultipleChoices {
			// Context() supplies the independent user context, unlike Fiber's
			// pooled Ctx/RequestCtx. Retain its values while allowing the log to
			// finish after request cancellation, without capturing c itself.
			logContext = context.WithoutCancel(logContext)
			go write(logContext)
		} else {
			write(logContext)
		}
		return err
	}
}

func DefaultRequestLogSkipper(c fiber.Ctx) bool {
	path := c.Path()
	return path == "/ping" || path == "/favicon.ico" || strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/redir/")
}
