package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// Native targets bypass the HTTP compatibility adapter used by integration
// tests. Production Fiber runs on fasthttp; generic test targets use net/http.
type nativeTarget interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
}

type targetServer struct {
	URL      string
	shutdown func(context.Context) error
	done     chan error
}

func startTarget(handler http.Handler) (*targetServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	target := &targetServer{URL: "http://" + listener.Addr().String(), done: make(chan error, 1)}
	if native, ok := handler.(nativeTarget); ok {
		target.shutdown = native.Shutdown
		go func() { target.done <- native.Serve(listener) }()
	} else {
		server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
		target.shutdown = server.Shutdown
		go func() { target.done <- server.Serve(listener) }()
	}
	return target, nil
}

func (target *targetServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = target.shutdown(ctx)
	select {
	case <-target.done:
	case <-ctx.Done():
	}
}

func newControlMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		user, system, peakRSS := processResources()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Mem              runtime.MemStats
			UserCPUSeconds   float64
			SystemCPUSeconds float64
			PeakRSSBytes     uint64
		}{mem, user, system, peakRSS})
	})
	// These diagnostic profiles cover only this isolated target process.
	// Register only the needed handlers on the loopback control mux.
	mux.HandleFunc("GET /debug/pprof/profile", httppprof.Profile)
	mux.Handle("GET /debug/pprof/heap", httppprof.Handler("heap"))
	return mux
}

func serveTarget(handler http.Handler) error {
	target, err := startTarget(handler)
	if err != nil {
		return err
	}
	defer target.Close()

	// Instrumentation lives on a separate loopback-only listener. It is present
	// only in this benchmark binary, never in the application daemon.
	controlListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	mux := newControlMux()
	control := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer control.Close()
	go func() { _ = control.Serve(controlListener) }()
	info := map[string]any{
		"url": target.URL, "control": "http://" + controlListener.Addr().String(), "pid": os.Getpid(),
	}
	if configured, ok := handler.(interface{ ReadBufferSize() int }); ok {
		info["read_buffer_bytes"] = configured.ReadBufferSize()
	}
	if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-ctx.Done():
		return nil
	case err := <-target.done:
		return fmt.Errorf("target stopped: %w", err)
	}
}
