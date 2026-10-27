package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

const bufferedPrivateSentinel = "PRIVATE-BUFFERED-READ-SENTINEL"

func requireBufferedReadRejected(t *testing.T, response *http.Response, status int) {
	t.Helper()
	require.Equal(t, status, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), bufferedPrivateSentinel)
	for _, values := range response.Header {
		for _, value := range values {
			require.NotContains(t, value, bufferedPrivateSentinel, "rejected buffered reads must not retain private response headers")
		}
	}
}
func preparePrivateJSON(c fiber.Ctx) error {
	c.Set("Location", "https://private.example/"+bufferedPrivateSentinel)
	c.Set("X-Private-Metadata", bufferedPrivateSentinel)
	return c.JSON(fiber.Map{"private": bufferedPrivateSentinel})
}

func TestBufferedReadEntryFaultDoesNotRunHandler(t *testing.T) {
	store, browser, _ := identityFixture(t)
	called := false
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error { called = true; return preparePrivateJSON(c) })
	require.NoError(t, store.Close())
	response := browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"})
	requireBufferedReadRejected(t, response, 503)
	require.False(t, called)
}

func TestBufferedReadRejectsPreparedJSONAndHeadersAfterAuthorityLoss(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, path := range []string{"/", "/applications", "/bookmarks"} {
			for _, fault := range []bool{false, true} {
				name := method + path + "/revoke"
				if fault {
					name = method + path + "/fault"
				}
				t.Run(name, func(t *testing.T) {
					store, browser, _ := identityFixture(t)
					browser.app.Add([]string{method}, path, AuthRequired, func(c fiber.Ctx) error {
						if err := preparePrivateJSON(c); err != nil {
							return err
						}
						if fault {
							return store.Close()
						}
						return store.RevokeAll()
					})
					status := 401
					if fault {
						status = 503
					}
					requireBufferedReadRejected(t, browser.request(method, path, nil, map[string]string{"Accept": "application/json"}), status)
				})
			}
		}
	}
}

func TestBufferedReadFinalAuthorizationRunsAfterHandlerError(t *testing.T) {
	store, browser, _ := identityFixture(t)
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		if err := preparePrivateJSON(c); err != nil {
			return err
		}
		if err := store.RevokeAll(); err != nil {
			return err
		}
		return fiber.NewError(500, "simulated renderer failure")
	})
	requireBufferedReadRejected(t, browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"}), 401)
}

func TestBufferedReadTimeoutErrorDoesNotExtendIdleActivity(t *testing.T) {
	_, browser, statePath := identityFixture(t)
	type stateEnvelope struct {
		State struct {
			HighWater time.Time
			Sessions  map[string]security.Session
		}
	}
	readState := func() stateEnvelope {
		raw, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var value stateEnvelope
		require.NoError(t, json.Unmarshal(raw, &value))
		return value
	}
	before := readState()
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		if err := preparePrivateJSON(c); err != nil {
			return err
		}
		return context.DeadlineExceeded
	})
	response := browser.request("GET", "/", nil, nil)
	require.Equal(t, 500, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), bufferedPrivateSentinel)
	after := readState()
	require.Equal(t, before.State.Sessions, after.State.Sessions, "failed rendering must not count as foreground activity")
	require.True(t, after.State.HighWater.After(before.State.HighWater), "the final clock decision must still become durable before returning an error")
}

func TestBufferedReadConcurrentSameTokenCannotOutliveRevocation(t *testing.T) {
	store, browser, _ := identityFixture(t)
	entered := make(chan *security.SessionProof, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		if err := preparePrivateJSON(c); err != nil {
			return err
		}
		entered <- persistent(c).identity(c).proof
		<-release
		return nil
	})
	var cookie *http.Cookie
	for _, value := range browser.cookies {
		copy := *value
		cookie = &copy
	}
	type result struct {
		response *http.Response
		err      error
	}
	completed := make(chan result, 2)
	for range 2 {
		go func() {
			request, err := http.NewRequest("GET", "http://flare.test/", nil)
			if err != nil {
				completed <- result{err: err}
				return
			}
			request.AddCookie(cookie)
			request.Header.Set("Accept", "application/json")
			response, err := browser.app.Test(request, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
			completed <- result{response, err}
		}()
	}
	proofs := make([]*security.SessionProof, 0, 2)
	for len(proofs) < 2 {
		select {
		case proof := <-entered:
			proofs = append(proofs, proof)
		case early := <-completed:
			require.NoError(t, early.err)
			t.Fatal("request completed before rendering barrier")
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent rendering barrier timed out")
		}
	}
	require.NotSame(t, proofs[0], proofs[1], "same-token requests must never share authorization proof state")
	require.NoError(t, store.RevokeAll())
	unblock()
	for range 2 {
		outcome := <-completed
		require.NoError(t, outcome.err)
		requireBufferedReadRejected(t, outcome.response, 401)
	}
	requireBufferedReadRejected(t, browser.request("GET", "/", nil, nil), 401)
}

type privateBufferedStream struct{ read, closed bool }

func (s *privateBufferedStream) Read(output []byte) (int, error) {
	s.read = true
	return copy(output, bufferedPrivateSentinel), io.EOF
}
func (s *privateBufferedStream) Close() error { s.closed = true; return nil }

func TestBufferedReadRejectsUnexpectedStreamWithoutReadingIt(t *testing.T) {
	_, browser, _ := identityFixture(t)
	stream := &privateBufferedStream{}
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		c.Set("Location", "https://private.example/"+bufferedPrivateSentinel)
		c.Response().SetBodyStream(stream, -1)
		return nil
	})
	requireBufferedReadRejected(t, browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"}), 500)
	require.False(t, stream.read, "an unapproved streaming callback must not release private content")
	require.True(t, stream.closed)
}

func TestBufferedReadPanicCannotBypassFinalAuthorityGate(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoke", true: "fault"}[fault], func(t *testing.T) {
			store, browser, _ := identityFixture(t)
			browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
				if err := preparePrivateJSON(c); err != nil {
					return err
				}
				if fault {
					if err := store.Close(); err != nil {
						return err
					}
				} else {
					if err := store.RevokeAll(); err != nil {
						return err
					}
				}
				panic(bufferedPrivateSentinel)
			})
			status := 401
			if fault {
				status = 503
			}
			requireBufferedReadRejected(t, browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"}), status)
		})
	}
}

func TestBufferedReadNonSuccessStillChecksAuthority(t *testing.T) {
	for _, status := range []int{302, 403, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			store, browser, _ := identityFixture(t)
			browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
				if err := preparePrivateJSON(c); err != nil {
					return err
				}
				c.Status(status)
				return store.RevokeAll()
			})
			requireBufferedReadRejected(t, browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"}), 401)
		})
	}
}

func TestBufferedReadEligibilityExcludesOnlineHomeAndMutations(t *testing.T) {
	_, browser, _ := identityFixture(t)
	paths := []string{"/", "/applications", "/bookmarks", "/login", "/account/cookie-check", "/editor", "/editor/transfer/export", "/icons/example.svg"}
	browser.app.Use(func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"eligible": bufferedReadEligible(c)})
	})
	for _, offline := range []bool{false, true} {
		define.AppFlags.EnableOfflineMode = offline
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
			for _, path := range paths {
				// Authentication routes are installed ahead of this diagnostic handler.
				if path == "/login" || path == "/account/cookie-check" {
					continue
				}
				response := browser.request(method, path, nil, nil)
				if method == http.MethodHead {
					continue
				}
				var body struct{ Eligible bool }
				require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
				expected := method == http.MethodGet && (path == "/applications" || path == "/bookmarks" || path == "/" && offline)
				require.Equal(t, expected, body.Eligible, method+" "+path)
			}
		}
	}
}

func TestBufferedReadCanceledContextCannotBecomeSuccessfulActivity(t *testing.T) {
	for _, expiredDeadline := range []bool{false, true} {
		name := "canceled"
		if expiredDeadline {
			name = "expired-deadline"
		}
		t.Run(name, func(t *testing.T) {
			_, browser, statePath := identityFixture(t)
			type envelope struct {
				State struct {
					HighWater time.Time
					Sessions  map[string]security.Session
				}
			}
			read := func() envelope {
				raw, err := os.ReadFile(statePath)
				require.NoError(t, err)
				var state envelope
				require.NoError(t, json.Unmarshal(raw, &state))
				return state
			}
			before := read()
			browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
				if err := preparePrivateJSON(c); err != nil {
					return err
				}
				if expiredDeadline {
					ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
					defer cancel()
					c.SetContext(ctx)
				} else {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					c.SetContext(ctx)
				}
				// A canceled handler can mistakenly return nil; that must not touch idle
				// activity or release the response it prepared before cancellation.
				return nil
			})
			requireBufferedReadRejected(t, browser.request("GET", "/", nil, map[string]string{"Accept": "application/json"}), 500)
			after := read()
			require.Equal(t, before.State.Sessions, after.State.Sessions)
			require.True(t, after.State.HighWater.After(before.State.HighWater))
		})
	}
}

func TestBufferedReadAuthenticatedPanicDoesNotTouchIdleActivity(t *testing.T) {
	_, browser, statePath := identityFixture(t)
	type envelope struct {
		State struct {
			HighWater time.Time
			Sessions  map[string]security.Session
		}
	}
	read := func() envelope {
		raw, err := os.ReadFile(statePath)
		require.NoError(t, err)
		var state envelope
		require.NoError(t, json.Unmarshal(raw, &state))
		return state
	}
	before := read()
	browser.app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		if err := preparePrivateJSON(c); err != nil {
			return err
		}
		panic("simulated rendering panic")
	})
	response := browser.request("GET", "/", nil, nil)
	require.Equal(t, 500, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NotContains(t, string(body), bufferedPrivateSentinel)
	after := read()
	require.Equal(t, before.State.Sessions, after.State.Sessions)
	require.True(t, after.State.HighWater.After(before.State.HighWater))
}
