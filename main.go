// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: main.go
// About: Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts: a...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts: a desktop window over a small set of pages
// and a read-only JSON API, served in-process to that window only.
//
// Posts come from the embedded snapshot (the CMS owns them); post authors come from the shared Generic Company
// database's employees table, opened read-only by path, or from the snapshot when there is no database.
//
// The package is split by concern: cli.go holds func main and the flags (-install, -uninstall, -serve, ...);
// site.go the current store and its database fallback; main.go the routes and HTTP helpers; pages.go the
// HTML layout and page templates.
package main

import (
	"embed"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"generic-company/cms/internal/app"
	"generic-company/cms/internal/health"
	"generic-company/cms/internal/textfmt"
)

// icons are the program's icon in the sizes routes serves (favicon, header, About page); cli.go also
// installs app.ico beside the exe.
//
//go:embed icon/app.ico icon/app-32.png icon/app-256.png
var icons embed.FS

// routes are the pages and API. public (-public) leaves out what only the owner should see or do: the
// database path and its form, local paths and errors in the footer, and the runtime health readout.
func routes(st *site, start time.Time, public bool) *http.ServeMux {
	mux := http.NewServeMux()
	page := func(extra map[string]any) map[string]any {
		source := st.sourceText()
		if public {
			source = st.publicSource()
		}
		data := map[string]any{"Source": source, "Version": app.Version, "Name": app.Name, "Public": public}
		for k, v := range extra {
			data[k] = v
		}
		return data
	}
	mux.HandleFunc("GET /api/posts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, st.get().Published())
	})
	mux.HandleFunc("GET /api/posts/{slug}", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		p, hint, ok := s.Get(r.PathValue("slug"))
		if !ok {
			notFound(w, hint)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"post": p, "author": s.Author(p)})
	})
	mux.HandleFunc("GET /api/posts/{slug}/related", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		if _, hint, ok := s.Get(r.PathValue("slug")); !ok {
			notFound(w, hint)
			return
		}
		writeJSON(w, http.StatusOK, s.Related(r.PathValue("slug"), 3))
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, st.get().Search(r.URL.Query().Get("q")))
	})
	mux.HandleFunc("GET /api/source", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		writeJSON(w, http.StatusOK, map[string]any{"authors_from": s.Source, "counts": s.Counts(), "version": app.Version})
	})
	if !public {
		mux.Handle("/api/health", health.Handler(start))
	}

	icon := func(name, kind string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, _ := icons.ReadFile("icon/" + name)
			w.Header().Set("Content-Type", kind)
			w.Header().Set("Cache-Control", "max-age=86400")
			w.Write(b)
		}
	}
	mux.HandleFunc("GET /favicon.ico", icon("app.ico", "image/x-icon"))
	mux.HandleFunc("GET /icon-32.png", icon("app-32.png", "image/png"))
	mux.HandleFunc("GET /icon-256.png", icon("app-256.png", "image/png"))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		q := r.URL.Query().Get("q")
		posts := s.Published()
		if q != "" {
			posts = s.Search(q)
		}
		render(w, http.StatusOK, indexPage, page(map[string]any{"Posts": posts, "Q": q,
			"Count": textfmt.Plural(len(posts), "1 post", strconv.Itoa(len(posts))+" posts")}))
	})
	mux.HandleFunc("GET /posts/{slug}", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		p, hint, ok := s.Get(r.PathValue("slug"))
		if !ok {
			render(w, http.StatusNotFound, missingPage, page(map[string]any{"Hint": hint}))
			return
		}
		var body struct {
			Blocks []struct {
				Text string `json:"text"`
			} `json:"blocks"`
		}
		_ = json.Unmarshal(p.Body, &body)
		render(w, http.StatusOK, postPage, page(map[string]any{"Post": p, "Author": s.Author(p), "Blocks": body.Blocks,
			"Related": s.Related(p.Slug, 3)}))
	})
	mux.HandleFunc("GET /about", func(w http.ResponseWriter, r *http.Request) {
		path := ""
		if !public {
			st.mu.RLock()
			path = st.dbPath
			st.mu.RUnlock()
		}
		render(w, http.StatusOK, aboutPage, page(map[string]any{"DB": path, "Credit": app.Credit,
			"Publisher": app.Publisher, "Counts": st.get().Counts(), "Saved": r.URL.Query().Get("saved") != ""}))
	})
	if public {
		return mux // no POST /about: the mux answers 405
	}
	mux.HandleFunc("POST /about", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("db")
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if err := st.useDB(path); err != nil {
			st.mu.RLock()
			current := st.dbPath
			st.mu.RUnlock()
			render(w, http.StatusBadRequest, aboutPage, page(map[string]any{"DB": current, "Credit": app.Credit,
				"Publisher": app.Publisher, "Counts": st.get().Counts(), "Error": path + ": " + err.Error()}))
			return
		}
		http.Redirect(w, r, "/about?saved=1", http.StatusSeeOther)
	})
	return mux
}

// writeJSON answers v as JSON with the status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// notFound is the API's 404: {"error": "not found"}, plus "did_you_mean" when a published slug is close.
func notFound(w http.ResponseWriter, hint string) {
	body := map[string]string{"error": "not found"}
	if hint != "" {
		body["did_you_mean"] = hint
	}
	writeJSON(w, http.StatusNotFound, body)
}

// secure adds the headers a site served beyond the desktop window should send.
func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self'; "+
			"form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		next.ServeHTTP(w, r)
	})
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=15c429c77957223f9bcc40fbf1b196fec5f2244f820cc68f6411e538ba02ab2f ====
