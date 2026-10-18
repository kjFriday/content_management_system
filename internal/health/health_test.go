// ==== ACME Inc. portfolio ===============================================
// Project: content_management_system | File: internal/health/health_test.go
// About: Tests for package health.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package health

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestTakeReadsTheRuntime(t *testing.T) {
	runtime.GC()
	start := time.Now().Add(-90 * time.Second)
	s := Take(start)

	if s.UptimeSeconds < 90 || s.UptimeSeconds > 95 {
		t.Errorf("uptime = %ds, want about 90", s.UptimeSeconds)
	}
	if s.Goroutines < 1 {
		t.Errorf("goroutines = %d, want at least 1", s.Goroutines)
	}
	if s.HeapAllocBytes == 0 {
		t.Error("heap_alloc_bytes is 0")
	}
	if s.SysBytes < s.HeapAllocBytes {
		t.Errorf("sys_bytes %d is less than heap_alloc_bytes %d", s.SysBytes, s.HeapAllocBytes)
	}
	if s.NumGC < 1 {
		t.Errorf("num_gc = %d after a forced GC", s.NumGC)
	}
	if s.GoVersion != runtime.Version() {
		t.Errorf("go_version = %q, want %q", s.GoVersion, runtime.Version())
	}
}

func TestTakeNeverReportsNegativeUptime(t *testing.T) {
	if s := Take(time.Now().Add(time.Hour)); s.UptimeSeconds != 0 {
		t.Errorf("a start time in the future gave uptime %d, want 0", s.UptimeSeconds)
	}
}

func TestHandlerServesJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler(time.Now()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	// The field names are the contract with the dashboard; pin them.
	var fields map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, rec.Body)
	}
	var keys []string
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "go_version,goroutines,heap_alloc_bytes,num_gc,sys_bytes,uptime_seconds"
	if got := strings.Join(keys, ","); got != want {
		t.Errorf("fields = %s, want %s", got, want)
	}
}

func TestHandlerRejectsOtherMethods(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		Handler(time.Now()).ServeHTTP(rec, httptest.NewRequest(method, "/api/health", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, want 405", method, rec.Code)
		}
		if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
			t.Errorf("%s: Allow = %q", method, allow)
		}
	}
}

// Wired into a ServeMux the way main.go registers its routes.
func TestHandlerOverHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", Handler(time.Now().Add(-5*time.Second)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body %s", res.StatusCode, body)
	}
	var s Snapshot
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("decode: %v\n%s", err, body)
	}
	if s.UptimeSeconds < 5 || s.Goroutines < 1 || s.GoVersion == "" {
		t.Errorf("implausible snapshot: %+v", s)
	}

	head, err := http.Head(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	head.Body.Close()
	if head.StatusCode != http.StatusOK {
		t.Errorf("HEAD status = %d, want 200", head.StatusCode)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=83401cb04649db443ae2208df13b12ab5d780ce128a7bbd547fdbea4def97a06 ====
