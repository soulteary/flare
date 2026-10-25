package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

type nativeProbe struct{ server *http.Server }

func (*nativeProbe) ServeHTTP(http.ResponseWriter, *http.Request) {
	panic("benchmark used the compatibility adapter instead of the native listener")
}

func (probe *nativeProbe) Serve(listener net.Listener) error  { return probe.server.Serve(listener) }
func (probe *nativeProbe) Shutdown(ctx context.Context) error { return probe.server.Shutdown(ctx) }

func TestTargetUsesNativeListener(t *testing.T) {
	probe := &nativeProbe{server: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "native target")
	})}}
	target, err := startTarget(probe)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	response, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "native target" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestScenarioRejectsUnexpectedStatus(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.NotFound(w, nil) })
	_, err := runScenario(handler, "", scenario{Name: "missing", Method: http.MethodGet, Path: "/"}, 1, 1, 1, 0)
	if err == nil || !strings.Contains(err.Error(), "unexpected status code: 404") {
		t.Fatalf("a missing route must fail the benchmark, got %v", err)
	}
}

func TestScenarioRejectsTruncatedResponse(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "short")
	})
	_, err := runScenario(handler, "", scenario{Name: "truncated", Method: http.MethodGet, Path: "/"}, 1, 1, 1, 0)
	if err == nil || !strings.Contains(err.Error(), "read response") {
		t.Fatalf("a truncated response must fail the benchmark, got %v", err)
	}
}
