// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/health/health.go
// About: Package health reports basic Go runtime statistics for a health endpoint.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package health reports basic Go runtime statistics for a health endpoint.
//
// main.go mounts Handler at /api/health and leaves it out with -public. Take makes one Snapshot;
// Handler serves it as JSON. The JSON field names are a contract with readers and are pinned by
// a test.
package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// Snapshot is one reading of the running process. Byte counts come from
// runtime.MemStats.
type Snapshot struct {
	UptimeSeconds  int64  `json:"uptime_seconds"`   // whole seconds since the start time
	Goroutines     int    `json:"goroutines"`       // goroutines alive now
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"` // bytes of live heap objects (MemStats.HeapAlloc)
	SysBytes       uint64 `json:"sys_bytes"`        // bytes the runtime got from the OS (MemStats.Sys)
	NumGC          uint32 `json:"num_gc"`           // finished garbage collections
	GoVersion      string `json:"go_version"`       // the toolchain that built the exe, e.g. "go1.24.2"
}

// Take reads the runtime now. Uptime is measured from start in whole
// seconds and never goes below zero. runtime.ReadMemStats briefly stops
// the world, so poll at a modest rate rather than on every hot request.
func Take(start time.Time) Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// integer division truncates toward zero, so 1.9 s reads as 1
	up := int64(time.Since(start) / time.Second)
	// a start time in the future (a clock change, or a caller's mistake) reads as 0, not negative
	if up < 0 {
		up = 0
	}
	return Snapshot{
		UptimeSeconds:  up,
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: m.HeapAlloc,
		SysBytes:       m.Sys,
		NumGC:          m.NumGC,
		GoVersion:      runtime.Version(),
	}
}

// Handler serves a Snapshot as JSON, with uptime measured from start. It
// answers GET and HEAD, replies 405 to any other method, and marks the
// response no-store so a cache never serves an old reading.
func Handler(start time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		// for HEAD, net/http discards the body, so the same code answers both methods; an encode
		// error means the client has gone, and there is no one left to tell
		json.NewEncoder(w).Encode(Take(start))
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=544f07f3e5df86555f0d60695b2ac420b936b48f462b54f24b16bc7cf857453b ====
