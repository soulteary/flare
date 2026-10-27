package fn

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingResponseBody struct{}

func (failingResponseBody) Read([]byte) (int, error) { return 0, errors.New("response interrupted") }
func (failingResponseBody) Close() error             { return nil }

type networkRoundTrip func(*http.Request) (*http.Response, error)

func (f networkRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNetworkHelpers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"name":"Flare"}`)
		case "/bad-json":
			_, _ = io.WriteString(w, "{")
		case "/unavailable":
			w.WriteHeader(503)
		default:
			_, _ = io.WriteString(w, "<html>Flare</html>")
		}
	}))
	defer server.Close()
	var value struct{ Name string }
	require.NoError(t, GetJSON(server.URL+"/json", &value))
	require.Equal(t, "Flare", value.Name)
	require.Error(t, GetJSON(server.URL+"/bad-json", &value))
	body, err := GetHTML(server.URL)
	require.NoError(t, err)
	require.Equal(t, "<html>Flare</html>", body)
	body, err = GetHTML(server.URL + "/unavailable")
	require.EqualError(t, err, "503")
	require.Empty(t, body)
	require.Error(t, GetJSON("://bad", &value))
	_, err = GetHTML("://bad")
	require.Error(t, err)
	original := client
	t.Cleanup(func() { client = original })
	client = &http.Client{Transport: networkRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingResponseBody{}, Header: make(http.Header)}, nil
	})}
	_, err = GetHTML("http://upstream.test")
	require.ErrorContains(t, err, "response interrupted")
	require.Error(t, GetJSON("http://upstream.test", &value))
}
