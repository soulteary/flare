//go:build fiber

package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterRouting_HealthCheck(t *testing.T) {
	e := fiber.New()
	RegisterRouting(e)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	rec, err := e.Test(req)
	require.NoError(t, err)
	defer rec.Body.Close()
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, rec.StatusCode)
	assert.Contains(t, string(body), "pong")
	assert.Contains(t, rec.Header.Get("Content-Type"), "application/json")
}
