package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/server"
)

type scenario struct {
	Name        string
	Method      string
	Path        string
	Body        string
	ContentType string
}

type metrics struct {
	Scenario string
	Method   string
	Path     string

	Requests    int
	Concurrency int
	Elapsed     time.Duration

	QPS      float64
	P50      time.Duration
	P95      time.Duration
	P99      time.Duration
	AllocsOp float64
	BytesOp  float64
}

func defaultFlags() model.Flags {
	env := define.DefaultEnvVars
	return model.Flags{
		Port:                   env.Port,
		EnableGuide:            env.EnableGuide,
		EnableEditor:           env.EnableEditor,
		EnableOfflineMode:      true, // baseline 不依赖外部网络
		EnableMinimumRequest:   env.EnableMinimumRequest,
		EnableDeprecatedNotice: env.EnableDeprecatedNotice,
		DisableCSP:             env.DisableCSP,
		Visibility:             "DEFAULT",
		DisableLoginMode:       true,
		User:                   env.User,
		Pass:                   env.Pass,
		CookieName:             env.CookieName,
		CookieSecret:           env.CookieSecret,
	}
}

func setupRouter() (http.Handler, func(), error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	tmpDir, err := os.MkdirTemp("", "flare-baseline-*")
	if err != nil {
		return nil, nil, err
	}
	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, nil, err
	}

	cleanup := func() {
		_ = os.Chdir(workDir)
		_ = os.RemoveAll(tmpDir)
		_ = os.Unsetenv("FLARE_BASELINE")
	}

	os.Setenv("FLARE_BASELINE", "1") // 关闭请求日志等，使 baseline 反映纯 handler 吞吐
	define.AppFlags = defaultFlags()

	// 初始化运行时配置和缓存，确保基线可重复执行。
	_, _ = data.GetAllSettingsOptions()
	_ = data.LoadFavoriteBookmarks()
	_ = data.LoadNormalBookmarks()

	handler := server.NewRouter(&define.AppFlags)
	return handler, cleanup, nil
}

func buildScenarios() map[string]scenario {
	encoded := data.Base64EncodeUrl("https://link.example.com")
	return map[string]scenario{
		"home": {
			Name:   "home",
			Method: http.MethodGet,
			Path:   "/",
		},
		"home-search": {
			Name:        "home-search",
			Method:      http.MethodPost,
			Path:        "/",
			Body:        "search=example",
			ContentType: "application/x-www-form-urlencoded",
		},
		"bookmarks": {
			Name:   "bookmarks",
			Method: http.MethodGet,
			Path:   "/bookmarks",
		},
		"applications": {
			Name:   "applications",
			Method: http.MethodGet,
			Path:   "/applications",
		},
		"redir-url": {
			Name:   "redir-url",
			Method: http.MethodGet,
			Path:   "/redir/url?go=" + encoded,
		},
	}
}

func quantile(latencies []int64, q float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(latencies))*q)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return time.Duration(latencies[index])
}

func runScenario(router http.Handler, s scenario, requests int, concurrency int, warmup int) (metrics, error) {
	if requests <= 0 {
		return metrics{}, fmt.Errorf("requests must be > 0")
	}
	if concurrency <= 0 {
		return metrics{}, fmt.Errorf("concurrency must be > 0")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	server := httptest.NewServer(router)
	defer server.Close()

	doReq := func() (time.Duration, error) {
		bodyReader := strings.NewReader(s.Body)
		req, err := http.NewRequest(s.Method, server.URL+s.Path, bodyReader)
		if err != nil {
			return 0, err
		}
		if s.ContentType != "" {
			req.Header.Set("Content-Type", s.ContentType)
		}
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode >= 500 {
			return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}
		return time.Since(start), nil
	}

	for i := 0; i < warmup; i++ {
		if _, err := doReq(); err != nil {
			return metrics{}, err
		}
	}

	latencies := make([]int64, requests)
	jobs := make(chan int, requests)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	startAt := time.Now()

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				latency, err := doReq()
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}
				latencies[idx] = int64(latency)
			}
		}()
	}

	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	elapsed := time.Since(startAt)

	select {
	case err := <-errCh:
		return metrics{}, err
	default:
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	requestCount := float64(requests)
	qps := requestCount / elapsed.Seconds()
	allocsOp := float64(after.Mallocs-before.Mallocs) / requestCount
	bytesOp := float64(after.TotalAlloc-before.TotalAlloc) / requestCount

	return metrics{
		Scenario: s.Name,
		Method:   s.Method,
		Path:     s.Path,
		Requests: requests,
		Elapsed:  elapsed,

		Concurrency: concurrency,
		QPS:         qps,
		P50:         quantile(latencies, 0.50),
		P95:         quantile(latencies, 0.95),
		P99:         quantile(latencies, 0.99),
		AllocsOp:    allocsOp,
		BytesOp:     bytesOp,
	}, nil
}

func main() {
	scenarioName := flag.String("scenario", "all", "baseline scenario: all|home|home-search|bookmarks|applications|redir-url")
	requests := flag.Int("requests", 2000, "request count per scenario")
	concurrency := flag.Int("concurrency", 8, "worker count")
	warmup := flag.Int("warmup", 100, "warmup request count")
	cpuProfile := flag.String("cpuprofile", "", "cpu profile output file path")
	memProfile := flag.String("memprofile", "", "heap profile output file path")
	markdown := flag.Bool("markdown", false, "print markdown table rows")
	flag.Parse()

	if *cpuProfile != "" {
		abs, err := filepath.Abs(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve cpuprofile path failed: %v\n", err)
			os.Exit(1)
		}
		*cpuProfile = abs
	}
	if *memProfile != "" {
		abs, err := filepath.Abs(*memProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "resolve memprofile path failed: %v\n", err)
			os.Exit(1)
		}
		*memProfile = abs
	}

	router, cleanup, err := setupRouter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup router failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	scenarios := buildScenarios()
	var selected []scenario
	if *scenarioName == "all" {
		selected = []scenario{
			scenarios["home"],
			scenarios["home-search"],
			scenarios["bookmarks"],
			scenarios["applications"],
			scenarios["redir-url"],
		}
	} else {
		s, ok := scenarios[*scenarioName]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown scenario: %s\n", *scenarioName)
			os.Exit(1)
		}
		selected = []scenario{s}
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create cpuprofile failed: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = f.Close()
		}()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "start cpu profile failed: %v\n", err)
			os.Exit(1)
		}
	}

	results := make([]metrics, 0, len(selected))
	for _, s := range selected {
		result, err := runScenario(router, s, *requests, *concurrency, *warmup)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run scenario %s failed: %v\n", s.Name, err)
			os.Exit(1)
		}
		results = append(results, result)
	}

	if *memProfile != "" {
		runtime.GC()
		f, err := os.Create(*memProfile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create memprofile failed: %v\n", err)
			os.Exit(1)
		}
		if err := pprof.WriteHeapProfile(f); err != nil {
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "write mem profile failed: %v\n", err)
			os.Exit(1)
		}
		_ = f.Close()
	}

	if *markdown {
		for _, item := range results {
			fmt.Printf("| %s | %s %s | %.2f | %s | %s | %s | %.2f | %.2f |\n",
				item.Scenario,
				item.Method,
				item.Path,
				item.QPS,
				item.P50.Round(time.Microsecond).String(),
				item.P95.Round(time.Microsecond).String(),
				item.P99.Round(time.Microsecond).String(),
				item.AllocsOp,
				item.BytesOp,
			)
		}
		return
	}

	for _, item := range results {
		fmt.Printf("scenario=%s method=%s path=%s requests=%d concurrency=%d elapsed=%s qps=%.2f p50=%s p95=%s p99=%s allocs/op=%.2f B/op=%.2f\n",
			item.Scenario,
			item.Method,
			item.Path,
			item.Requests,
			item.Concurrency,
			item.Elapsed.Round(time.Millisecond).String(),
			item.QPS,
			item.P50.Round(time.Microsecond).String(),
			item.P95.Round(time.Microsecond).String(),
			item.P99.Round(time.Microsecond).String(),
			item.AllocsOp,
			item.BytesOp,
		)
	}
}
