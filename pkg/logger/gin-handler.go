package logger

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func New(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		params := map[string]string{}
		for _, p := range c.Params {
			params[p.Key] = p.Value
		}

		c.Next()

		status := c.Writer.Status()
		method := c.Request.Method
		host := c.Request.Host
		route := c.FullPath()
		end := time.Now()
		latency := end.Sub(start)
		userAgent := c.Request.UserAgent()
		ip := c.ClientIP()
		referer := c.Request.Referer()

		requestAttributes := []slog.Attr{
			slog.Time("time", start),
			slog.String("method", method),
			slog.String("host", host),
			slog.String("path", path),
			slog.String("query", query),
			slog.Any("params", params),
			slog.String("route", route),
			slog.String("ip", ip),
			slog.String("referer", referer),
			slog.String("user-agent", userAgent),

			slog.Duration("latency", latency),
			slog.Int("status", status),
		}

		level := slog.LevelInfo
		msg := "REQUEST"
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			level = slog.LevelWarn
			msg = c.Errors.String()
		} else if status >= http.StatusInternalServerError {
			level = slog.LevelError
			msg = c.Errors.String()
		}

		logger.LogAttrs(c.Request.Context(), level, msg, requestAttributes...)
	}
}
