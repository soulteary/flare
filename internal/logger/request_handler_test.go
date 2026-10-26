package logger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

type delayedRequestHandler struct {
	started chan struct{}
	release chan struct{}
	records chan slog.Record
}

func (h *delayedRequestHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *delayedRequestHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *delayedRequestHandler) WithGroup(string) slog.Handler            { return h }
func (h *delayedRequestHandler) Handle(_ context.Context, record slog.Record) error {
	h.started <- struct{}{}
	<-h.release
	h.records <- record.Clone()
	return nil
}

func TestNewFiberWithConfig_AsyncLogOwnsPooledStrings(t *testing.T) {
	handler := &delayedRequestHandler{
		started: make(chan struct{}, 2), release: make(chan struct{}), records: make(chan slog.Record, 2),
	}
	app := fiber.New() // Exercise borrowed strings even without Immutable mode.
	app.Use(NewFiberWithConfig(slog.New(handler), LoggerConfig{AccessLog: "all"}))
	app.Get("/:name", func(c fiber.Ctx) error { return c.SendString("ok") })
	bridge := adaptor.FiberApp(app)
	for _, value := range []string{"first", "other"} {
		request := httptest.NewRequest(http.MethodGet, "http://"+value+".test/"+value+"?q="+value, nil)
		request.Header.Set("User-Agent", value+" agent")
		request.Header.Set("Referer", "http://"+value+".test/")
		response := httptest.NewRecorder()
		bridge.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("response status = %d", response.Code)
		}
		select {
		case <-handler.started:
		case <-time.After(time.Second):
			t.Fatal("asynchronous logger did not start")
		}
	}
	close(handler.release)
	want := map[string]bool{"first": true, "other": true}
	for range 2 {
		var record slog.Record
		select {
		case record = <-handler.records:
		case <-time.After(time.Second):
			t.Fatal("asynchronous logger did not finish")
		}
		attrs := make(map[string]string)
		record.Attrs(func(attr slog.Attr) bool {
			if attr.Value.Kind() == slog.KindString {
				attrs[attr.Key] = attr.Value.String()
			}
			return true
		})
		value := strings.TrimPrefix(attrs["path"], "/")
		if !want[value] {
			t.Fatalf("unexpected or overwritten request log: %v", attrs)
		}
		delete(want, value)
		for key, expected := range map[string]string{
			"host": value + ".test", "query": "q=" + value,
			"user-agent": value + " agent", "referer": "http://" + value + ".test/",
		} {
			if attrs[key] != expected {
				t.Errorf("retained %s = %q, want %q", key, attrs[key], expected)
			}
		}
	}
}

// requestLogWriter delivers complete JSON log records without sharing a buffer
// with the middleware's asynchronous success logger.
type requestLogWriter struct {
	records chan []byte
}

func (w requestLogWriter) Write(data []byte) (int, error) {
	w.records <- append([]byte(nil), data...)
	return len(data), nil
}

func TestNewFiberWithConfig_OffPreservesApplicationLogs(t *testing.T) {
	records := make(chan []byte, 2)
	log := slog.New(slog.NewJSONHandler(requestLogWriter{records: records}, nil))
	app := fiber.New()
	app.Use(NewFiberWithConfig(log, LoggerConfig{AccessLog: "off"}))
	app.Get("/example", func(c fiber.Ctx) error {
		log.Info("application event")
		return fiber.ErrBadRequest
	})
	response := httptest.NewRecorder()
	adaptor.FiberApp(app).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/example", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	select {
	case raw := <-records:
		var record map[string]any
		require.NoError(t, json.Unmarshal(raw, &record))
		if record["msg"] != "application event" || record["level"] != "INFO" {
			t.Fatalf("application log changed: %s", raw)
		}
	default:
		t.Fatal("application log was suppressed")
	}
	select {
	case raw := <-records:
		t.Fatalf("off mode produced an access log: %s", raw)
	default:
	}
}

func TestNewFiberWithConfig_RequestOutcomes(t *testing.T) {
	internalErr := errors.New("file does not implement io.ReadSeeker")
	wrappedNotFound := fmt.Errorf("lookup failed: %w", fiber.NewError(http.StatusNotFound, "Not Found"))
	tests := []requestOutcome{
		{
			name: "successful asset is skipped", path: "/assets/mdi/icon.svg",
			handler: func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("svg") },
			status:  http.StatusOK, body: "svg",
		},
		{
			name: "successful health check is skipped", path: "/ping",
			handler: func(c fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) },
			status:  http.StatusNoContent,
		},
		{
			name: "successful favicon is skipped", path: "/favicon.ico",
			handler: func(c fiber.Ctx) error { return c.SendString("icon") },
			status:  http.StatusOK, body: "icon",
		},
		{
			name: "successful redirect is skipped", path: "/redir/example",
			handler: func(c fiber.Ctx) error { return c.Redirect().Status(http.StatusFound).To("/") },
			status:  http.StatusFound,
		},
		{
			name: "asset not found is logged", path: "/assets/mdi/missing.svg", route: "/assets/mdi/:name",
			handler: func(fiber.Ctx) error { return fiber.ErrNotFound },
			status:  http.StatusNotFound, body: fiber.ErrNotFound.Error(), level: "WARN", errMessage: fiber.ErrNotFound.Error(),
		},
		{
			name: "asset failure is logged", path: "/assets/mdi/broken.svg",
			handler: func(fiber.Ctx) error { return internalErr },
			status:  http.StatusInternalServerError, body: internalErr.Error(), level: "ERROR", errMessage: internalErr.Error(),
		},
		{
			name: "failed health check is logged", path: "/ping",
			handler: func(c fiber.Ctx) error { return c.Status(http.StatusServiceUnavailable).Send(nil) },
			status:  http.StatusServiceUnavailable, level: "ERROR",
		},
		{
			name: "client failure without returned error is logged", path: "/assets/missing.svg",
			handler: func(c fiber.Ctx) error { return c.Status(http.StatusNotFound).Send(nil) },
			status:  http.StatusNotFound, level: "WARN",
		},
		{
			name: "wrapped HTTP error uses its status", path: "/missing",
			handler: func(fiber.Ctx) error { return wrappedNotFound },
			status:  http.StatusNotFound, body: wrappedNotFound.Error(), level: "WARN", errMessage: wrappedNotFound.Error(),
		},
		{
			name: "ordinary success is logged", path: "/example",
			handler: func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") },
			status:  http.StatusOK, body: "ok", level: "INFO",
		},
		{
			name: "implicit success uses status 200", path: "/empty",
			handler: func(fiber.Ctx) error { return nil },
			status:  http.StatusOK, level: "INFO",
		},
		{
			name: "ordinary redirect is preserved", path: "/redirect",
			handler: func(c fiber.Ctx) error { return c.Redirect().Status(http.StatusFound).To("/") },
			status:  http.StatusFound, level: "INFO",
		},
		{
			name: "buffered success is replaced by returned error", path: "/assets/mdi/partial.svg",
			handler: func(c fiber.Ctx) error {
				if err := c.Status(http.StatusOK).SendString("partial"); err != nil {
					return err
				}
				return internalErr
			},
			status: http.StatusInternalServerError, body: internalErr.Error(), level: "ERROR", errMessage: internalErr.Error(),
		},
	}
	for _, mode := range []struct {
		name, accessLog string
		logsSuccess     bool
		disabled        bool
	}{
		{name: "default"},
		{name: "errors", accessLog: "errors"},
		{name: "all", accessLog: "all", logsSuccess: true},
		{name: "off", accessLog: "off", disabled: true},
		{name: "invalid falls back to errors", accessLog: "invalid"},
	} {
		for _, tt := range tests {
			t.Run(mode.name+"/"+tt.name, func(t *testing.T) {
				wantLevel := tt.level
				if mode.disabled || (!mode.logsSuccess && tt.errMessage == "" && tt.status < http.StatusBadRequest) {
					wantLevel = ""
				}
				records := make(chan []byte, 4)
				log := slog.New(slog.NewJSONHandler(requestLogWriter{records: records}, nil))
				errorHandlerCalls := 0
				e := fiber.New(fiber.Config{ErrorHandler: func(c fiber.Ctx, err error) error {
					errorHandlerCalls++
					return fiber.DefaultErrorHandler(c, err)
				}})
				var returnedErr error
				e.Use(func(c fiber.Ctx) error {
					returnedErr = c.Next()
					return returnedErr
				})
				e.Use(NewFiberWithConfig(log, LoggerConfig{AccessLog: mode.accessLog, Skipper: DefaultRequestLogSkipper}))
				route := tt.route
				if route == "" {
					route = tt.path
				}
				handlerCalls := 0
				var handlerErr error
				e.Get(route, func(c fiber.Ctx) error {
					handlerCalls++
					handlerErr = tt.handler(c)
					return handlerErr
				})
				rec := httptest.NewRecorder()
				adaptor.FiberApp(e).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

				require.Equal(t, tt.status, rec.Code)
				assert.Equal(t, 1, handlerCalls)
				assert.True(t, returnedErr == handlerErr, "middleware must return the original error")
				assert.Equal(t, tt.body, strings.TrimSpace(rec.Body.String()))
				if tt.status == http.StatusFound && rec.Header().Get("Location") != "/" {
					t.Errorf("redirect location = %q, want /", rec.Header().Get("Location"))
				}
				wantErrorHandlerCalls := 0
				if tt.errMessage != "" {
					wantErrorHandlerCalls = 1
				}
				assert.Equal(t, wantErrorHandlerCalls, errorHandlerCalls)
				if wantLevel == "" {
					select {
					case record := <-records:
						t.Fatalf("disabled request log was written: %s", record)
					case <-time.After(20 * time.Millisecond):
					}
					return
				}

				assertRequestLog(t, records, tt, wantLevel, route)
			})
		}
	}
}

type requestOutcome struct {
	name       string
	path       string
	route      string
	handler    fiber.Handler
	status     int
	body       string
	level      string
	errMessage string
}

func assertRequestLog(t *testing.T, records <-chan []byte, tt requestOutcome, wantLevel, route string) {
	t.Helper()
	var data []byte
	select {
	case data = <-records:
	case <-time.After(time.Second):
		t.Fatal("request log was not written")
	}
	var record map[string]any
	require.NoError(t, json.Unmarshal(data, &record))
	assert.Equal(t, float64(tt.status), record["status"])
	if record["level"] != wantLevel {
		t.Errorf("logged level = %v, want %s", record["level"], wantLevel)
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
}
