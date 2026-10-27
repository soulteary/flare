package home

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

// These benchmarks warm parsed YAML but vary only rendered HTML reuse. Cold
// includes resetting the bounded HTML map, not process startup or YAML parsing.
func BenchmarkNavigationFragmentCache(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		for _, mode := range []string{"uncached", "cold", "hot"} {
			label := mode
			if size == 10000 && mode == "hot" {
				label = "oversize-bypass"
			}
			b.Run(fmt.Sprintf("entries-%d/%s", size, label), func(b *testing.B) {
				b.Chdir(b.TempDir())
				fragmentFixture(b, size)
				oldCache := navigationFragments
				navigationFragments = newNavigationHTMLCache()
				b.Cleanup(func() { navigationFragments = oldCache })
				view, _, err := data.LoadNavigationViews(true, false, true, false)
				if err != nil {
					b.Fatal(err)
				}
				opts := &model.Application{Locale: "en"}
				expected, _ := generateApplicationsSnapshot("", opts, view.Copy())
				if got := navigationFragment(view, opts, true, "", "", "/", nil); got != expected {
					b.Fatal("output mismatch")
				}
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					if mode == "cold" {
						navigationFragments = newNavigationHTMLCache()
					}
					if mode == "uncached" {
						got, _ := generateApplicationsSnapshot("", opts, view.Copy())
						if len(got) != len(expected) {
							b.Fatal("output mismatch")
						}
					} else if got := navigationFragment(view, opts, true, "", "", "/", nil); len(got) != len(expected) {
						b.Fatal("output mismatch")
					}
				}
			})
		}
	}
}

// Native TCP loopback, sequential keep-alive requests, same fixture and response
// bytes in cold/hot cases. Includes HTTP client/server allocations and durable
// auth I/O. It is not a browser benchmark or a multi-host network measurement.
func BenchmarkNativeHomepageFragmentCache(b *testing.B) {
	for _, identity := range []string{"disabled", "anonymous", "authenticated"} {
		for _, mode := range []string{"cold", "hot"} {
			b.Run(identity+"/"+mode, func(b *testing.B) {
				app, _, token := fragmentApp(b, identity == "disabled", 1000)
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					b.Fatal(err)
				}
				done := make(chan error, 1)
				go func() { done <- app.Listener(listener, fiber.ListenConfig{DisableStartupMessage: true}) }()
				b.Cleanup(func() {
					if err := app.Shutdown(); err != nil {
						b.Error(err)
					}
					if err := <-done; err != nil {
						b.Error(err)
					}
				})
				transport := &http.Transport{MaxIdleConnsPerHost: 1, DisableCompression: true}
				b.Cleanup(transport.CloseIdleConnections)
				client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
				url := "http://" + listener.Addr().String() + "/"
				request := func() int {
					req, err := http.NewRequest("GET", url, nil)
					if err != nil {
						b.Fatal(err)
					}
					if identity == "authenticated" {
						req.AddCookie(token)
					}
					r, err := client.Do(req)
					if err != nil {
						b.Fatal(err)
					}
					n, err := io.Copy(io.Discard, r.Body)
					r.Body.Close()
					if err != nil {
						b.Fatal(err)
					}
					if r.StatusCode != 200 {
						b.Fatalf("HTTP %d", r.StatusCode)
					}
					return int(n)
				}
				expected := request()
				samples := make([]time.Duration, 0, 10000)
				b.ReportAllocs()
				b.SetBytes(int64(expected))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if mode == "cold" {
						navigationFragments.mu.Lock()
						clear(navigationFragments.entries)
						navigationFragments.order = nil
						navigationFragments.bytes = 0
						navigationFragments.mu.Unlock()
					}
					start := time.Now()
					if n := request(); n != expected {
						b.Fatalf("response changed: %d != %d", n, expected)
					}
					if len(samples) < cap(samples) {
						samples = append(samples, time.Since(start))
					}
				}
				b.StopTimer()
				sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
				if len(samples) > 0 {
					b.ReportMetric(float64(samples[len(samples)/2].Nanoseconds())/1e6, "p50-ms")
					b.ReportMetric(float64(samples[(len(samples)-1)*95/100].Nanoseconds())/1e6, "p95-ms")
				}
				b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "req/s")
			})
		}
	}
}
