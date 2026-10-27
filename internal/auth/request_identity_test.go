package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func identityFixture(t *testing.T) (*security.Store, *persistentBrowser, string) {
	t.Helper()
	previous := define.AppFlags
	t.Cleanup(func() { define.AppFlags = previous })
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "identity"
	define.AppFlags.Port = 5005
	define.AppFlags.CookieSecure = false
	define.AppFlags.ExternalURL = ""
	define.AppFlags.AllowedHosts = ""
	define.AppFlags.TrustedProxies = ""
	directory := t.TempDir()
	store, err := security.Open(directory)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.Initialize("identity-admin", "short", "local"))
	token, err := store.Authenticate("identity-admin", "short")
	require.NoError(t, err)
	app := fiber.New()
	require.NoError(t, RequestHandleWithSecurity(app, store))
	name := RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port) + "_" + store.Info().InstanceID
	browser := &persistentBrowser{t, app, map[string]*http.Cookie{name: {Name: name, Value: token}}}
	return store, browser, filepath.Join(directory, "state.json")
}

func TestRequestIdentityReusesDurableAuthorizationOnlyWithinRequest(t *testing.T) {
	_, browser, statePath := identityFixture(t)
	var proofs []*security.SessionProof
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		before, err := os.ReadFile(statePath)
		require.NoError(t, err)
		identity := persistent(c).identity(c)
		require.NotNil(t, identity.proof)
		proofs = append(proofs, identity.proof)
		for i := 0; i < 25; i++ {
			require.True(t, CheckUserIsLogin(c))
			require.Equal(t, "identity-admin", GetUserName(c))
			require.NotEmpty(t, GetUserLoginDate(c))
			require.Same(t, identity, persistent(c).identity(c))
		}
		after, err := os.ReadFile(statePath)
		require.NoError(t, err)
		require.Equal(t, before, after, "repeated read helpers must not advance durable authorization time or fsync state")
		return c.SendString("authorized")
	})
	require.Equal(t, 200, browser.request("GET", "/", nil, nil).StatusCode)
	require.Equal(t, 200, browser.request("GET", "/", nil, nil).StatusCode)
	require.Len(t, proofs, 2)
	require.NotSame(t, proofs[0], proofs[1], "a new HTTP request must durably authorize again")
}

func TestRequestIdentityImmediatelyLosesAuthorityAfterRevocation(t *testing.T) {
	store, browser, _ := identityFixture(t)
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		require.True(t, CheckUserIsLogin(c))
		require.NoError(t, store.RevokeAll())
		require.False(t, CheckUserIsLogin(c))
		require.Empty(t, GetUserName(c))
		require.Empty(t, GetUserLoginDate(c))
		require.False(t, Revalidate(c))
		wrote := false
		err := AuthorizedWrite(c, func() error { wrote = true; return nil })
		require.Error(t, err)
		require.False(t, wrote)
		return c.SendStatus(204)
	})
	require.Equal(t, 401, browser.request("GET", "/", nil, nil).StatusCode)
	require.Equal(t, 401, browser.request("GET", "/", nil, nil).StatusCode)
}

func TestRequestIdentityCannotSurviveStoreFaultBeforeOutput(t *testing.T) {
	store, browser, _ := identityFixture(t)
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		require.True(t, CheckUserIsLogin(c))
		require.NoError(t, store.Close())
		require.False(t, CheckUserIsLogin(c))
		require.Empty(t, GetUserName(c))
		require.False(t, Revalidate(c))
		return c.SendString("PRIVATE-RESPONSE-MUST-BE-DISCARDED")
	})
	response := browser.request("GET", "/", nil, nil)
	require.Equal(t, 503, response.StatusCode, "final durable foreground touch must replace output after a store fault")
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), "PRIVATE-RESPONSE-MUST-BE-DISCARDED")
}

func TestRequestIdentityDiscardsPreparedReadAfterRevocation(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			store, browser, _ := identityFixture(t)
			browser.app.Add([]string{method}, "/", AuthRequired, func(c fiber.Ctx) error {
				require.True(t, CheckUserIsLogin(c))
				require.NoError(t, c.SendString("PRIVATE-RESPONSE-MUST-BE-DISCARDED"))
				require.NoError(t, store.RevokeAll())
				return nil
			})
			response := browser.request(method, "/", nil, nil)
			require.Equal(t, 401, response.StatusCode)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.NotContains(t, string(body), "PRIVATE-RESPONSE-MUST-BE-DISCARDED")
		})
	}
}

func TestRequestIdentityDoesNotTurnAnonymousBadCookieIntoSiteWide401(t *testing.T) {
	_, browser, _ := identityFixture(t)
	for _, cookie := range browser.cookies {
		cookie.Value = "expired-or-forged-session"
	}
	browser.app.Get("/", func(c fiber.Ctx) error {
		require.False(t, CheckUserIsLogin(c))
		return c.SendString("PUBLIC-RESPONSE")
	})
	response := browser.request("GET", "/", nil, nil)
	require.Equal(t, 200, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "PUBLIC-RESPONSE")
}

func TestRequestIdentityFinalHeadUsesFreshDurableAuthorization(t *testing.T) {
	_, browser, statePath := identityFixture(t)
	type diskState struct {
		State struct {
			HighWater time.Time
			Sessions  map[string]security.Session
		}
	}
	readState := func() diskState {
		raw, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var state diskState
		require.NoError(t, json.Unmarshal(raw, &state))
		return state
	}
	var before diskState
	browser.app.Head("/", AuthRequired, func(c fiber.Ctx) error {
		require.True(t, CheckUserIsLogin(c))
		before = readState()
		return c.SendString("authorized")
	})
	require.Equal(t, 200, browser.request("HEAD", "/", nil, nil).StatusCode)
	after := readState()
	require.True(t, after.State.HighWater.After(before.State.HighWater), "the final HEAD check must observe and durably record a fresh clock point, not reuse the request proof")
	require.Equal(t, before.State.Sessions, after.State.Sessions, "HEAD validation must not extend idle activity")
}
