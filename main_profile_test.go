package main

// This executable is built with `go test -c` for synthetic local profiling.
// No profiling listener or environment switch is compiled into production.
import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestProfileDaemon(t *testing.T) {
	if os.Getenv("FLARE_PROFILE_TEST_ENABLE") != "1" {
		t.Skip("isolated profiling executable only")
	}
	port, err := strconv.Atoi(os.Getenv("FLARE_PROFILE_TEST_PORT"))
	if err != nil || port < 1024 || port > 65535 {
		t.Fatal("valid dedicated profiling port required")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	for _, name := range []string{"heap", "allocs", "goroutine", "mutex", "block"} {
		mux.Handle("/debug/pprof/"+name, pprof.Handler(name))
	}
	mux.HandleFunc("/runtime", func(w http.ResponseWriter, r *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		var usage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
			http.Error(w, "process accounting unavailable", 500)
			return
		}
		seconds := func(v syscall.Timeval) float64 { return float64(v.Sec) + float64(v.Usec)/1e6 }
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"time": time.Now().UTC(), "cpu_user_seconds": seconds(usage.Utime), "cpu_system_seconds": seconds(usage.Stime),
			"total_alloc_bytes": mem.TotalAlloc, "mallocs": mem.Mallocs, "frees": mem.Frees,
			"heap_alloc_bytes": mem.HeapAlloc, "heap_inuse_bytes": mem.HeapInuse, "heap_objects": mem.HeapObjects,
			"heap_sys_bytes": mem.HeapSys, "heap_released_bytes": mem.HeapReleased,
			"num_gc": mem.NumGC, "pause_total_ns": mem.PauseTotalNs, "goroutines": runtime.NumGoroutine(),
		})
	})
	control := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = control.Serve(listener) }()
	defer control.Close()
	// Retain the standard sampling rate; instrumentation is identical in both
	// profile builds. Profile controls and forced-GC heap capture are outside the
	// production HTTP listener and reported separately from native measurements.
	os.Args = []string{os.Args[0]}
	main()
}
