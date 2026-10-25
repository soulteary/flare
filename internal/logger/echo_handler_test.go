package logger

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

// requestLogWriter delivers complete JSON log records without sharing a buffer
// with the middleware's asynchronous success logger.
type requestLogWriter struct {
	records chan []byte
}

func (w requestLogWriter) Write(data []byte) (int, error) {
	w.records <- append([]byte(nil), data...)
	return len(data), nil
}

func TestNewEchoWithConfig_RequestOutcomes(t *testing.T) {
	internalErr := errors.New("file does not implement io.ReadSeeker")
	wrappedNotFound := fmt.Errorf("lookup failed: %w", echo.NewHTTPError(http.StatusNotFound, "Not Found"))
	tests := []struct {
		name       string
		path       string
		route      string
		handler    echo.HandlerFunc
		status     int
		body       string
		level      string
		errMessage string
	}{
		{
			name: "successful asset is skipped", path: "/assets/mdi/icon.svg",
			handler: func(c *echo.Context) error { return c.String(http.StatusOK, "svg") },
			status:  http.StatusOK, body: "svg",
		},
		{
			name: "successful health check is skipped", path: "/ping",
			handler: func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) },
			status:  http.StatusNoContent,
		},
		{
			name: "successful redirect is skipped", path: "/redir/example",
			handler: func(c *echo.Context) error { return c.Redirect(http.StatusFound, "/") },
			status:  http.StatusFound,
		},
		{
			name: "asset not found is logged", path: "/assets/mdi/missing.svg", route: "/assets/mdi/:name",
			handler: func(*echo.Context) error { return echo.ErrNotFound },
			status:  http.StatusNotFound, body: `{"message":"Not Found"}`, level: "WARN", errMessage: echo.ErrNotFound.Error(),
		},
		{
			name: "asset failure is logged", path: "/assets/mdi/broken.svg",
			handler: func(*echo.Context) error { return internalErr },
			status:  http.StatusInternalServerError, body: `{"message":"Internal Server Error"}`, level: "ERROR", errMessage: internalErr.Error(),
		},
		{
			name: "failed health check is logged", path: "/ping",
			handler: func(c *echo.Context) error { return c.NoContent(http.StatusServiceUnavailable) },
			status:  http.StatusServiceUnavailable, level: "ERROR",
		},
		{
			name: "wrapped HTTP error uses its status", path: "/missing",
			handler: func(*echo.Context) error { return wrappedNotFound },
			status:  http.StatusNotFound, body: `{"message":"Not Found"}`, level: "WARN", errMessage: wrappedNotFound.Error(),
		},
		{
			name: "ordinary success is logged", path: "/example",
			handler: func(c *echo.Context) error { return c.String(http.StatusOK, "ok") },
			status:  http.StatusOK, body: "ok", level: "INFO",
		},
		{
			name: "implicit success uses status 200", path: "/empty",
			handler: func(*echo.Context) error { return nil },
			status:  http.StatusOK, level: "INFO",
		},
		{
			name: "ordinary redirect is preserved", path: "/redirect",
			handler: func(c *echo.Context) error { return c.Redirect(http.StatusFound, "/") },
			status:  http.StatusFound, level: "INFO",
		},
		{
			name: "committed success keeps wire status and logs later error", path: "/assets/mdi/partial.svg",
			handler: func(c *echo.Context) error {
				if err := c.String(http.StatusOK, "partial"); err != nil {
					return err
				}
				return internalErr
			},
			status: http.StatusOK, body: "partial", level: "ERROR", errMessage: internalErr.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records := make(chan []byte, 4)
			log := slog.New(slog.NewJSONHandler(requestLogWriter{records: records}, nil))
			e := echo.New()
			e.Use(NewEchoWithConfig(log, LoggerConfig{Skipper: DefaultRequestLogSkipper}))
			errorHandlerCalls := 0
			defaultErrorHandler := e.HTTPErrorHandler
			e.HTTPErrorHandler = func(c *echo.Context, err error) {
				errorHandlerCalls++
				defaultErrorHandler(c, err)
			}
			route := tt.route
			if route == "" {
				route = tt.path
			}
			e.GET(route, tt.handler)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := strings.TrimSpace(rec.Body.String()); got != tt.body {
				t.Errorf("body = %q, want %q", got, tt.body)
			}
			if tt.status == http.StatusFound && rec.Header().Get("Location") != "/" {
				t.Errorf("redirect location = %q, want /", rec.Header().Get("Location"))
			}
			wantErrorHandlerCalls := 0
			if tt.errMessage != "" {
				wantErrorHandlerCalls = 1
			}
			if errorHandlerCalls != wantErrorHandlerCalls {
				t.Errorf("error handler called %d times, want %d", errorHandlerCalls, wantErrorHandlerCalls)
			}
			if tt.level == "" {
				select {
				case record := <-records:
					t.Fatalf("successful skipped request produced a log: %s", record)
				case <-time.After(20 * time.Millisecond):
				}
				return
			}

			var data []byte
			select {
			case data = <-records:
			case <-time.After(time.Second):
				t.Fatal("request log was not written")
			}
			var record map[string]any
			if err := json.Unmarshal(data, &record); err != nil {
				t.Fatal(err)
			}
			if record["status"] != float64(tt.status) {
				t.Errorf("logged status = %v, want %d", record["status"], tt.status)
			}
			if record["level"] != tt.level {
				t.Errorf("logged level = %v, want %s", record["level"], tt.level)
			}
			if record["path"] != tt.path || record["route"] != route {
				t.Errorf("logged path/route = %v/%v, want %s/%s", record["path"], record["route"], tt.path, route)
			}
			if tt.errMessage != "" {
				if record["error"] != tt.errMessage || record["msg"] != tt.errMessage {
					t.Errorf("logged error/message = %v/%v, want %s", record["error"], record["msg"], tt.errMessage)
				}
			} else if record["error"] != nil || record["msg"] != "REQUEST" {
				t.Errorf("unexpected error/message: %v/%v", record["error"], record["msg"])
			}
			if tt.route != "" {
				params, ok := record["params"].(map[string]any)
				if !ok || params["name"] != "missing.svg" {
					t.Errorf("logged route params = %v", record["params"])
				}
			}
			select {
			case duplicate := <-records:
				t.Errorf("duplicate request log: %s", duplicate)
			default:
			}
		})
	}
}
