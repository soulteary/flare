package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityErrorsHaveStableJSONCodes(t *testing.T) {
	for _, tc := range []struct {
		err         error
		status      int
		code, retry string
	}{
		{security.ErrDeploymentManaged, 403, "CREDENTIAL_EXTERNALLY_MANAGED", ""},
		{security.ErrPasswordPolicy, 400, "INVALID_INPUT", ""},
		{security.ErrBusy, 429, "RATE_LIMITED", "10"},
		{errors.New("private storage detail"), 503, "STORAGE_UNAVAILABLE", ""},
	} {
		t.Run(tc.code, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error { return securityError(c, fmt.Errorf("wrapped: %w", tc.err)) })
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Accept", "application/json")
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, tc.status, resp.StatusCode)
			require.Equal(t, tc.retry, resp.Header.Get("Retry-After"))
			require.NotEmpty(t, resp.Header.Get("X-Request-ID"))
			raw, readErr := io.ReadAll(resp.Body)
			require.NoError(t, readErr)
			var payload struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
				RequestID string `json:"request_id"`
			}
			require.NoError(t, json.Unmarshal(raw, &payload))
			require.Equal(t, tc.code, payload.Error.Code)
			require.Equal(t, tc.code, payload.Error.Message)
			require.Equal(t, resp.Header.Get("X-Request-ID"), payload.RequestID)
			require.NotContains(t, string(raw), "private storage detail")
		})
	}
}

func TestPersistentRecentAuthenticationAndGuardedWrites(t *testing.T) {
	store, b := persistentFixture(t, "local")
	b.app.Get("/recent", func(c fiber.Ctx) error { return RequireRecentAuthentication(c) })
	b.app.Post("/guarded", AuthRequired, func(c fiber.Ctx) error {
		return AuthorizedWrite(c, func() error {
			assert.True(t, Revalidate(c))
			assert.True(t, RecentAuthentication(c))
			return c.SendStatus(204)
		})
	})
	requirePersistentResponseStatus(t, 403, b.request("GET", "/recent", nil, nil))
	b.login()
	requirePersistentResponseStatus(t, 200, b.request("GET", "/recent", nil, nil))
	requirePersistentResponseStatus(t, 204, b.request("POST", "/guarded", url.Values{"csrf-token": {b.csrf()}}, nil))
	require.NoError(t, store.Close())
	requirePersistentResponseStatus(t, 503, b.request("POST", "/account/reauthenticate", url.Values{"password": {"legacy short"}}, nil))
}

func TestPersistentRejectsInvalidAccountOperations(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.login()
	csrf := b.csrf()
	for _, path := range []string{"/account/reauthenticate", "/account/password", "/account/local"} {
		requirePersistentResponseStatus(t, 403, b.request("POST", path, nil, nil))
	}
	r := b.request("POST", "/account/password", url.Values{"csrf-token": {csrf}, "password": {"legacy short"}, "new_password": {"long replacement password 42"}, "confirm_password": {"different"}}, nil)
	require.Equal(t, 400, r.StatusCode)
	require.NoError(t, r.Body.Close())
	r = b.request("POST", "/account/reauthenticate", url.Values{"csrf-token": {csrf}, "password": {"wrong"}}, nil)
	require.Equal(t, 401, r.StatusCode)
	require.NoError(t, r.Body.Close())
	r = b.request("POST", "/account/password", url.Values{"csrf-token": {csrf}, "password": {"legacy short"}, "new_password": {"short"}, "confirm_password": {"short"}}, nil)
	require.Equal(t, 400, r.StatusCode)
	require.NoError(t, r.Body.Close())
	r = b.request("POST", "/account/local", url.Values{"csrf-token": {csrf}, "password": {"legacy short"}, "confirm": {"local"}}, nil)
	require.Equal(t, 503, r.StatusCode)
	require.NoError(t, r.Body.Close())
	requirePersistentResponseStatus(t, 400, b.request("POST", "/account/reauthenticate?password=leak", nil, nil))
	requirePersistentResponseStatus(t, 200, b.request("GET", "/protected", nil, nil))
}

func TestPersistentLimiterBoundsAndBackoff(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.app.Get("/limiter", func(c fiber.Ctx) error {
		a := persistent(c)
		now := time.Now()
		keys := a.bucketKeys(c)
		a.buckets["expired"] = loginBucket{last: now.Add(-16 * time.Minute)}
		assert.True(t, a.allow(c))
		assert.NotContains(t, a.buckets, "expired")
		failures := make([]time.Time, 9)
		for i := range failures {
			failures[i] = now
		}
		for _, key := range keys {
			a.buckets[key] = loginBucket{failures: failures, last: now, strikes: 6}
		}
		a.failed(c, security.ErrInvalidCredentials)
		for _, key := range keys {
			assert.WithinDuration(t, now.Add(15*time.Minute), a.buckets[key].until, 2*time.Second)
		}
		assert.False(t, a.allow(c))
		assert.NotEmpty(t, c.GetRespHeader("Retry-After"))
		first := a.buckets[keys[0]]
		a.failed(c, security.ErrInvalidCredentials)
		assert.Equal(t, first, a.buckets[keys[0]])
		a.failed(c, security.ErrBusy)
		assert.Equal(t, first, a.buckets[keys[0]])
		a.buckets = map[string]loginBucket{}
		for i := 0; i < 10000; i++ {
			a.buckets[fmt.Sprint(i)] = loginBucket{last: now}
		}
		assert.False(t, a.allow(c))
		assert.Equal(t, "30", c.GetRespHeader("Retry-After"))
		a.failed(c, security.ErrInvalidCredentials)
		assert.Len(t, a.buckets, 10000)
		return c.SendStatus(204)
	})
	requirePersistentResponseStatus(t, 204, b.request("GET", "/limiter", nil, nil))
}

func TestPersistentSetupRejectsMissingStoreAndInvalidPolicy(t *testing.T) {
	require.Error(t, RequestHandleWithSecurity(fiber.New(), nil))
	store, _ := persistentFixture(t, "local")
	define.AppFlags.ExternalURL = "https://user:pass@example.com"
	require.Error(t, RequestHandleWithSecurity(fiber.New(), store))
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		assert.False(t, RecentAuthentication(c))
		assert.Error(t, RequireRecentAuthentication(c))
		return c.SendStatus(204)
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 204, resp.StatusCode)
}

func TestAccountOperationsRespectSharedRateLimit(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.login()
	csrf := b.csrf()
	b.app.Get("/install-limit", func(c fiber.Ctx) error {
		a := persistent(c)
		now := time.Now()
		for _, key := range a.bucketKeys(c) {
			a.buckets[key] = loginBucket{last: now, until: now.Add(time.Minute)}
		}
		return c.SendStatus(204)
	})
	response := b.request("GET", "/install-limit", nil, nil)
	require.Equal(t, 204, response.StatusCode)
	require.NoError(t, response.Body.Close())
	for _, path := range []string{"/account/reauthenticate", "/account/password", "/account/local"} {
		r := b.request("POST", path, url.Values{"csrf-token": {csrf}}, nil)
		require.Equal(t, 429, r.StatusCode, path)
		require.NotEmpty(t, r.Header.Get("Retry-After"))
		require.NoError(t, r.Body.Close())
	}
	r := b.request("GET", "/protected", nil, nil)
	require.Equal(t, 200, r.StatusCode)
	require.NoError(t, r.Body.Close())
}

func requirePersistentResponseStatus(t *testing.T, want int, response *http.Response) {
	t.Helper()
	defer response.Body.Close()
	require.Equal(t, want, response.StatusCode)
}
