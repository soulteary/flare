package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type baselineTransport func(*http.Request) (*http.Response, error)

func (f baselineTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type closeFailureBody struct{ io.Reader }

func (closeFailureBody) Close() error { return errors.New("close failed") }

func TestScenarioTransportAndCloseErrors(t *testing.T) {
	client := &http.Client{Transport: baselineTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("transport down") })}
	_, err := performScenarioRequest(client, "http://target.test", scenario{Method: "GET"})
	require.ErrorContains(t, err, "transport down")
	client.Transport = baselineTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: closeFailureBody{strings.NewReader("body")}}, nil
	})
	_, err = performScenarioRequest(client, "http://target.test", scenario{Method: "GET"})
	require.ErrorContains(t, err, "close response")
}

func TestControlStatsReturnsResourceSnapshot(t *testing.T) {
	rec := httptest.NewRecorder()
	newControlMux().ServeHTTP(rec, httptest.NewRequest("GET", "/stats", nil))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "PeakRSSBytes")
	require.Contains(t, rec.Body.String(), "UserCPUSeconds")
	require.Contains(t, rec.Body.String(), "Mem")
}
