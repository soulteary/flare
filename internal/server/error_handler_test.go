package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFiberAppErrorResponsesHideInternalDetails(t *testing.T) {
	app := newNativeFiberApp(t, false)
	const privateDetails = "open /private/flare/secret-config.yml: permission denied"
	app.Get("/__internal-error", func(fiber.Ctx) error { return errors.New(privateDetails) })
	app.Get("/__panic", func(fiber.Ctx) error { panic(privateDetails) })
	app.Get("/__not-found-error", func(fiber.Ctx) error { return fiber.ErrNotFound })
	app.Get("/__bad-request", func(fiber.Ctx) error { return fiber.NewError(http.StatusBadRequest, "invalid selection") })
	app.Get("/__wrapped-client-error", func(fiber.Ctx) error {
		return fmt.Errorf("%s: %w", privateDetails, fiber.NewError(http.StatusBadRequest, "invalid selection"))
	})
	app.Get("/__service-error", func(fiber.Ctx) error {
		return fiber.NewError(http.StatusServiceUnavailable, privateDetails)
	})
	app.Head("/__internal-error", func(fiber.Ctx) error { return errors.New(privateDetails) })
	bridge := adaptor.FiberApp(app)
	for _, test := range []struct {
		name, method, path string
		status             int
		message            string
	}{
		{name: "internal error", method: http.MethodGet, path: "/__internal-error", status: http.StatusInternalServerError, message: "Internal Server Error"},
		{name: "recovered panic", method: http.MethodGet, path: "/__panic", status: http.StatusInternalServerError, message: "Internal Server Error"},
		{name: "not found", method: http.MethodGet, path: "/__not-found-error", status: http.StatusNotFound, message: "Not Found"},
		{name: "bad request", method: http.MethodGet, path: "/__bad-request", status: http.StatusBadRequest, message: "invalid selection"},
		{name: "wrapped client error", method: http.MethodGet, path: "/__wrapped-client-error", status: http.StatusBadRequest, message: "invalid selection"},
		{name: "explicit server error", method: http.MethodGet, path: "/__service-error", status: http.StatusServiceUnavailable, message: "Internal Server Error"},
		{name: "HEAD error has no body", method: http.MethodHead, path: "/__internal-error", status: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			bridge.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			assert.Equal(t, test.status, response.Code)
			assert.NotContains(t, response.Body.String(), privateDetails)
			assert.NotContains(t, response.Body.String(), "/private/")
			if test.method == http.MethodHead {
				assert.Empty(t, response.Body.String())
				return
			}
			assert.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "application/json"))
			var payload map[string]string
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			assert.Equal(t, map[string]string{"message": test.message}, payload)
		})
	}
}

type errorResponseLogWriter struct{ records chan []byte }

func (w errorResponseLogWriter) Write(data []byte) (int, error) {
	w.records <- append([]byte(nil), data...)
	return len(data), nil
}

func TestNewFiberAppErrorResponsePreservesOriginalRequestLog(t *testing.T) {
	app := newNativeFiberApp(t, false)
	records := make(chan []byte, 1)
	log := slog.New(slog.NewJSONHandler(errorResponseLogWriter{records: records}, nil))
	app.Use(logger.NewFiber(log))
	internalErr := errors.New("open /private/flare/log-only-secret.yml: permission denied")
	app.Get("/__logged-error", func(fiber.Ctx) error { return internalErr })
	response := httptest.NewRecorder()
	adaptor.FiberApp(app).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/__logged-error", nil))
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), internalErr.Error())
	var payload map[string]string
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	assert.Equal(t, map[string]string{"message": "Internal Server Error"}, payload)
	select {
	case raw := <-records:
		var record map[string]any
		require.NoError(t, json.Unmarshal(raw, &record))
		assert.Equal(t, internalErr.Error(), record["error"])
		assert.Equal(t, internalErr.Error(), record["msg"])
		assert.Equal(t, "ERROR", record["level"])
		assert.Equal(t, float64(http.StatusInternalServerError), record["status"])
	default:
		t.Fatal("internal error was not logged")
	}
}
