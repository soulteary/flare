package main

import (
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestQuantilesUseNearestRankAndClampEndpoints(t *testing.T) {
	for _, tc := range []struct {
		q    float64
		want time.Duration
	}{{-1, 10}, {0, 10}, {.50, 20}, {.95, 40}, {.99, 40}, {2, 40}} {
		require.Equal(t, tc.want, quantile([]int64{10, 20, 30, 40}, tc.q))
	}
	require.Zero(t, quantile(nil, .95))
}

func TestScenarioMeasuresAllRequestsAndPreservesPayload(t *testing.T) {
	var requests atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != http.MethodPost || r.URL.Path != "/search" || string(body) != "search=personal" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			http.Error(w, "wrong request", http.StatusBadRequest)
			return
		}
		requests.Add(1)
		w.WriteHeader(http.StatusAccepted)
	})
	s := scenario{Name: "payload", Method: http.MethodPost, Path: "/search", Body: "search=personal", ContentType: "application/x-www-form-urlencoded", ExpectedStatus: http.StatusAccepted}
	result, err := runScenario(handler, "", s, 12, 3, 2, 200)
	require.NoError(t, err)
	require.Equal(t, int32(14), requests.Load())
	require.Equal(t, "payload", result.Scenario)
	require.Equal(t, 12, result.Requests)
	require.Equal(t, 3, result.Concurrency)
	require.Positive(t, result.QPS)
	require.Positive(t, result.P50)
	require.LessOrEqual(t, result.P50, result.P95)
	require.LessOrEqual(t, result.P95, result.P99)
}

func TestScenarioRejectsInvalidInputsAndWorkerFailures(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		url                   string
		method                string
		requests, concurrency int
		want                  string
	}{
		{"requests", "", "GET", 0, 1, "requests must be > 0"},
		{"concurrency", "", "GET", 1, 0, "concurrency must be > 0"},
		{"method", "http://localhost", "invalid method", 1, 1, "invalid method"},
		{"URL", "://bad", "GET", 1, 1, "missing protocol scheme"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runScenario(nil, tc.url, scenario{Method: tc.method}, tc.requests, tc.concurrency, 0, 0)
			require.ErrorContains(t, err, tc.want)
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	_, err := runScenario(nil, server.URL, scenario{Method: http.MethodGet}, 10, 3, 0, 0)
	require.ErrorContains(t, err, "unexpected status code: 503")
	// Redirects must be measured at their own status, without following the target.
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/elsewhere", http.StatusFound) }))
	defer redirect.Close()
	_, err = runScenario(nil, redirect.URL, scenario{Method: http.MethodGet, ExpectedStatus: http.StatusFound}, 1, 1, 0, 0)
	require.NoError(t, err)
}

func TestIsolatedRouterCleansOnlyItsTemporaryDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	workingDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("config.yml", []byte("precious user config"), 0600))
	oldFlags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = oldFlags })
	t.Setenv("FLARE_BASELINE", "")
	handler, cleanup, err := setupRouter()
	require.NoError(t, err)
	t.Cleanup(cleanup)
	temporary, err := os.Getwd()
	require.NoError(t, err)
	require.NotEqual(t, workingDir, temporary)
	require.True(t, define.AppFlags.EnableOfflineMode)
	require.True(t, define.AppFlags.DisableLoginMode)
	for name, s := range buildScenarios() {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(s.Method, s.Path, strings.NewReader(s.Body))
			req.Header.Set("Content-Type", s.ContentType)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			expected := s.ExpectedStatus
			if expected == 0 {
				expected = http.StatusOK
			}
			require.Equal(t, expected, rec.Code)
		})
	}
	cleanup()
	after, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, workingDir, after)
	_, err = os.Stat(temporary)
	require.ErrorIs(t, err, os.ErrNotExist)
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, "precious user config", string(raw))
}

// Successful CLI invocations exercise report/profile output without a separate
// uninstrumented binary, so their execution contributes to Go coverage.
func baselineCLI(t *testing.T, args ...string) string {
	t.Helper()
	oldArgs, oldFlags, oldOut := os.Args, flag.CommandLine, os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Args, flag.CommandLine, os.Stdout = oldArgs, oldFlags, oldOut
		require.NoError(t, reader.Close())
	})
	flag.CommandLine = flag.NewFlagSet("baseline-test", flag.ContinueOnError)
	os.Args = append([]string{"baseline-test"}, args...)
	os.Stdout = writer
	output := make(chan []byte, 1)
	go func() { raw, _ := io.ReadAll(reader); output <- raw }()
	main()
	require.NoError(t, writer.Close())
	os.Stdout = oldOut
	return string(<-output)
}

func TestCLIReportsAndProfiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redir/url" {
			w.WriteHeader(http.StatusFound)
		}
	}))
	defer server.Close()
	for _, format := range []string{"json", "markdown", "text"} {
		t.Run(format, func(t *testing.T) {
			args := []string{"-url", server.URL, "-requests", "2", "-concurrency", "1", "-warmup", "0"}
			if format != "text" {
				args = append(args, "-"+format)
			}
			dir := t.TempDir()
			if format == "json" {
				args = append(args, "-cpuprofile", filepath.Join(dir, "cpu.pprof"), "-memprofile", filepath.Join(dir, "heap.pprof"))
			} else {
				args = append(args, "-scenario", "home")
			}
			output := baselineCLI(t, args...)
			switch format {
			case "json":
				var results []metrics
				require.NoError(t, json.Unmarshal([]byte(output), &results))
				require.Len(t, results, 5)
				for _, name := range []string{"cpu.pprof", "heap.pprof"} {
					raw, err := os.ReadFile(filepath.Join(dir, name))
					require.NoError(t, err)
					require.Greater(t, len(raw), 2)
					require.Equal(t, []byte{0x1f, 0x8b}, raw[:2])
				}
			case "markdown":
				require.Contains(t, output, "| home | GET / |")
			default:
				require.Contains(t, output, "scenario=home method=GET path=/ requests=2")
			}
		})
	}
}
