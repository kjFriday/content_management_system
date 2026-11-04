// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: main.go
// About: Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts.
//
// It is a desktop window over a small set of pages and a read-only JSON API, served in-process to
// that window only. With -serve or -public it serves the same pages over HTTP instead, without a
// window.
//
// Posts come from the embedded snapshot (the CMS owns them); post authors come from the shared Generic Company
// database's employees table, opened read-only by path, or from the snapshot when there is no database.
//
// The package is split by concern: cli.go holds func main and the flags (-install, -uninstall, -serve, ...);
// site.go the current store and its database fallback; main.go the routes and HTTP helpers; pages.go the
// HTML layout and page templates.
//
// Request flow:
//  1. main (cli.go) resolves the database path, builds a site with newSite and gets a mux from routes.
//  2. The desktop window (package desktop) or, with -serve, an http.Server wrapped in secure, sends
//     each request to that mux.
//  3. A handler takes the current store with site.get and answers JSON through writeJSON or HTML
//     through render.
//  4. POST /about swaps the store for one built from another database (site.useDB).
//
// API routes: GET /api/posts, /api/posts/{slug}, /api/posts/{slug}/related, /api/search?q=,
// /api/source and, outside -public, /api/health. Pages: /, /posts/{slug} and /about.
//
// Start reading at routes.
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
// start is the process start time, for the health readout's uptime.
//
// The patterns use the Go 1.22 ServeMux syntax: a method before the path ("GET /api/posts"), a
// {slug} wildcard read back with r.PathValue, and {$}, which makes "GET /{$}" match "/" only
// instead of every path. A path whose pattern exists for another method answers 405 Method Not
// Allowed; a path with no pattern at all answers 404.
//
// The post and API handlers call st.get() once and keep that store for the whole request, so a
// database switch in the middle cannot mix two stores in one answer.
func routes(st *site, start time.Time, public bool) *http.ServeMux {
	mux := http.NewServeMux()
	// page merges the keys every page template reads (Source, Version, Name, Public) with the
	// page's own keys; a page key with the same name wins.
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

	// ---- JSON API ----
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
		// the Get check gives drafts and unknown slugs the same 404 and hint as the post route
		if _, hint, ok := s.Get(r.PathValue("slug")); !ok {
			notFound(w, hint)
			return
		}
		// Known limitation: a post with nothing related answers null, not [].
		writeJSON(w, http.StatusOK, s.Related(r.PathValue("slug"), 3))
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		// Known limitation: a blank q or no match answers null, not [].
		writeJSON(w, http.StatusOK, st.get().Search(r.URL.Query().Get("q")))
	})
	mux.HandleFunc("GET /api/source", func(w http.ResponseWriter, r *http.Request) {
		s := st.get()
		writeJSON(w, http.StatusOK, map[string]any{"authors_from": s.Source, "counts": s.Counts(), "version": app.Version})
	})
	if !public {
		// no method in the pattern: health.Handler checks GET and HEAD itself and answers 405 to
		// the rest
		mux.Handle("/api/health", health.Handler(start))
	}

	// ---- icons ----
	// icon serves one embedded icon file with its content type. The files are part of the build, so
	// the read cannot fail at run time, and a day of browser caching (86400 s) is safe.
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

	// ---- pages ----
	// the index lists every published post, or the search results when the header's box sent q
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
		// the page shows only blocks[].text, the snapshot's simplified block shape; a body in
		// another shape (a full Editor.js data object, TipTap) shows no paragraphs, while search and
		// related still read all of it through related.BodyText. A decode error just leaves Blocks
		// empty.
		var body struct {
			Blocks []struct {
				Text string `json:"text"`
			} `json:"blocks"`
		}
		_ = json.Unmarshal(p.Body, &body)
		render(w, http.StatusOK, postPage, page(map[string]any{"Post": p, "Author": s.Author(p), "Blocks": body.Blocks,
			"Related": s.Related(p.Slug, 3)}))
	})
	// the About page shows the program, the author count and, outside -public, the database form;
	// ?saved=1 is the redirect target after a successful POST
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
		return mux // no POST /about: the mux answers 405, because GET /about exists
	}
	// POST /about switches to the database typed into the form. A failure redraws the page with
	// status 400 and the error; success redirects with 303 See Other, so a reload does not post the
	// form again (the post/redirect/get pattern).
	mux.HandleFunc("POST /about", func(w http.ResponseWriter, r *http.Request) {
		path := r.FormValue("db")
		// a relative path would depend on the working directory; store the absolute one
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		// Known limitation: useDB also fails when only saving the setting fails, after the switch;
		// the page then reports an error although the new database is already in use.
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

// writeJSON answers v as JSON with the status code. Headers must be set before WriteHeader, and an
// encoding error cannot change the status once it is sent, so it is ignored. A nil slice encodes as
// null.
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

// secure adds the headers a site served beyond the desktop window should send. main wraps the mux in
// it for -serve and -public; the desktop window serves the mux directly.
//
// The headers:
//   - X-Content-Type-Options: nosniff stops a browser guessing a type other than Content-Type.
//   - X-Frame-Options: DENY and CSP frame-ancestors 'none' stop other sites framing the pages
//     (clickjacking); the first is for older browsers.
//   - Referrer-Policy: same-origin sends no Referer to other sites.
//   - Content-Security-Policy allows scripts, images, forms and the rest only from this origin.
//     Styles also allow 'unsafe-inline' because layout uses an inline <style> block and the About
//     page a style= attribute. base-uri 'none' blocks an injected <base> tag from redirecting
//     relative links.
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=ba981f461563dd2ac65b5fe20276070944fcf1d0283b85f5dd82b740e2d2cd2b ====
