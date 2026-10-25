package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestControlHeapProfileReturnsBinary(t *testing.T) {
	rec := httptest.NewRecorder()
	newControlMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("heap profile status: %d, body: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("heap profile Content-Type: %q", got)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x1f, 0x8b}) {
		t.Fatal("heap profile is not gzip-encoded pprof data")
	}
	reader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) == 0 {
		t.Fatal("heap profile payload is empty")
	}
}
