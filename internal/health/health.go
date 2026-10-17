// Package health reports basic Go runtime statistics for a service
// health endpoint such as GET /api/health.
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
	UptimeSeconds  int64  `json:"uptime_seconds"`
	Goroutines     int    `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	SysBytes       uint64 `json:"sys_bytes"`
	NumGC          uint32 `json:"num_gc"`
	GoVersion      string `json:"go_version"`
}

// Take reads the runtime now. Uptime is measured from start in whole
// seconds and never goes below zero. runtime.ReadMemStats briefly stops
// the world, so poll at a modest rate rather than on every hot request.
func Take(start time.Time) Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	up := int64(time.Since(start) / time.Second)
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
		json.NewEncoder(w).Encode(Take(start))
	}
}
