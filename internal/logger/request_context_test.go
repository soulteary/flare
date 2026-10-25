package logger

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

type requestContextTestKey struct{}

type loggedRequestContext struct {
	value       any
	err         error
	done        <-chan struct{}
	hasDeadline bool
	record      slog.Record
}

type delayedContextHandler struct {
	started chan struct{}
	release chan struct{}
	records chan loggedRequestContext
}

func (*delayedContextHandler) Enabled(ctx context.Context, _ slog.Level) bool {
	return ctx.Err() == nil
}
func (h *delayedContextHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *delayedContextHandler) WithGroup(string) slog.Handler      { return h }
func (h *delayedContextHandler) Handle(ctx context.Context, record slog.Record) error {
	h.started <- struct{}{}
	<-h.release
	_, deadline := ctx.Deadline()
	h.records <- loggedRequestContext{
		value: ctx.Value(requestContextTestKey{}), err: ctx.Err(), done: ctx.Done(),
		hasDeadline: deadline, record: record.Clone(),
	}
	return nil
}

func TestNewFiber_AsyncContextSurvivesCancellationAndRequestReuse(t *testing.T) {
	handler := &delayedContextHandler{
		started: make(chan struct{}, 2), release: make(chan struct{}), records: make(chan loggedRequestContext, 2),
	}
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// The context value owns its string, independently of Fiber's buffers.
		value := string(c.Request().Header.Peek("X-Request-ID"))
		ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), requestContextTestKey{}, value), time.Minute)
		defer cancel()
		c.SetContext(ctx)
		if value == "first" {
			// Cancellation before the logger runs must not suppress a completed
			// successful request's log, including in Handler.Enabled.
			cancel()
		}
		return c.Next()
	})
	app.Use(NewFiber(slog.New(handler)))
	app.Get("/:name", func(c fiber.Ctx) error { return c.SendString("ok") })
	serve := app.Handler()
	var requestCtx fasthttp.RequestCtx
	for _, value := range []string{"first", "other"} {
		requestCtx.Request.Reset()
		requestCtx.Response.Reset()
		requestCtx.Request.Header.SetMethod(http.MethodGet)
		requestCtx.Request.SetRequestURI("http://" + value + ".test/" + value)
		requestCtx.Request.Header.Set("X-Request-ID", value)
		serve(&requestCtx)
		if requestCtx.Response.StatusCode() != http.StatusOK {
			t.Fatalf("response status = %d", requestCtx.Response.StatusCode())
		}
		select {
		case <-handler.started:
		case <-time.After(time.Second):
			close(handler.release)
			t.Fatal("asynchronous logger dropped a canceled request's log")
		}
	}
	// Both requests have completed and their Fiber contexts have been released;
	// the same fasthttp context and request buffers now hold the second request.
	close(handler.release)
	want := map[string]string{"/first": "first", "/other": "other"}
	for range 2 {
		select {
		case logged := <-handler.records:
			path := ""
			logged.record.Attrs(func(attr slog.Attr) bool {
				if attr.Key == "path" {
					path = attr.Value.String()
				}
				return true
			})
			expected, ok := want[path]
			if !ok || logged.value != expected {
				t.Fatalf("context value = %v for path %q, expected %q", logged.value, path, expected)
			}
			delete(want, path)
			if logged.err != nil || logged.done != nil || logged.hasDeadline {
				t.Fatalf("async log retained request cancellation: error=%v done=%v deadline=%v", logged.err, logged.done, logged.hasDeadline)
			}
		case <-time.After(time.Second):
			t.Fatal("asynchronous log did not finish after request cancellation")
		}
	}
}

func TestAcquireRequestAttrsHandlesUnexpectedPoolValues(t *testing.T) {
	var nilAttrs *[]slog.Attr
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"empty", nil},
		{"wrong type", "unexpected"},
		{"typed nil", nilAttrs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := sync.Pool{New: func() any { return tc.value }}
			attrs := acquireRequestAttrs(&pool)
			if attrs == nil || len(*attrs) != 0 || cap(*attrs) < requestAttrsCap {
				t.Fatalf("invalid fallback buffer: %v", attrs)
			}
			*attrs = append(*attrs, slog.String("path", "/example"))
			releaseRequestAttrs(&pool, attrs)
		})
	}
}

func TestRequestAttrsPoolReuseClearsRetainedValues(t *testing.T) {
	storage := make([]slog.Attr, 0, requestAttrsCap)
	pool := sync.Pool{New: func() any { return &storage }}
	first := acquireRequestAttrs(&pool)
	*first = append(*first, slog.String("path", "/first"), slog.Any("params", map[string]string{"name": "first"}))
	previous := *first
	releaseRequestAttrs(&pool, first)
	for _, attr := range previous {
		if !attr.Equal(slog.Attr{}) {
			t.Fatalf("released buffer retained an attribute: %v", attr)
		}
	}
	next := acquireRequestAttrs(&pool)
	if next != first || len(*next) != 0 {
		t.Fatal("the cleared attribute buffer was not reused")
	}
	*next = append(*next, slog.String("path", "/other"))
	if len(*next) != 1 || (*next)[0].Value.String() != "/other" {
		t.Fatalf("unexpected reused attributes: %v", *next)
	}
	releaseRequestAttrs(&pool, next)
}
