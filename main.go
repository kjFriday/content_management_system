// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: main.go
// About: ==== Generic Company Inc. portfolio ===============================================...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// ==== Generic Company Inc. portfolio ===============================================
// Project: content_management_system | File: main.go
// About: Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts: a desktop window over a small set of...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Command generic-company-cms is Generic Company Journal, Generic Company Inc.'s posts: a desktop window over a small set of pages
// and a read-only JSON API, served in-process to that window only.
//
// Posts come from the embedded snapshot (the CMS owns them); post authors come from the shared Generic Company
// database's employees table, opened read-only by path, or from the snapshot when there is no database.
// See cli.go for the flags (-install, -uninstall, -serve, ...).
package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"generic-company/cms/internal/app"
	"generic-company/cms/internal/db"
	"generic-company/cms/internal/health"
	"generic-company/cms/internal/store"
	"generic-company/cms/internal/textfmt"
)

//go:embed mock/posts.json mock/employees.json
var snapshot embed.FS

//go:embed icon/app.ico icon/app-32.png icon/app-256.png
var icons embed.FS

func mockFS() fs.FS { sub, _ := fs.Sub(snapshot, "mock"); return sub }

// site is the current store and where its authors came from; the Database page can swap it.
type site struct {
	mu       sync.RWMutex
	s        *store.Store
	dbPath   string
	forced   bool   // -mock: never use the database
	why      string // when on the snapshot: why the database was not used
	settings func(path string) error
}

// load reads authors from the database at path, or falls back to the snapshot (and says why).
func load(path string, forceMock bool) (*store.Store, string) {
	why := "-mock"
	if !forceMock {
		authors, err := db.LoadAuthors(path)
		if err == nil {
			if s, err := store.FromAuthors(mockFS(), authors, "database"); err == nil {
				return s, ""
			}
		}
		why = err.Error()
	}
	s, err := store.FromMock(mockFS())
	if err != nil { // the snapshot is embedded, so only a broken build gets here
		panic(err)
	}
	return s, why
}
func newSite(path string, forceMock bool) *site {
	s, why := load(path, forceMock)
	return &site{s: s, dbPath: path, forced: forceMock, why: why}
}

func (st *site) get() *store.Store {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.s
}

// sourceText is the footer's "Authors from ..." line.
func (st *site) sourceText() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.s.Source == "database" {
		return "the shared database (" + st.dbPath + ")"
	}
	return "the mock snapshot (" + st.why + ")"
}

func (st *site) useDB(path string) error {
	s, why := load(path, false)
	if why != "" {
		return &dbError{why}
	}
	st.mu.Lock()
	st.s, st.dbPath, st.forced, st.why = s, path, false, ""
	st.mu.Unlock()
	if st.settings != nil {
		return st.settings(path)
	}
	return nil
}

type dbError struct{ why string }

func (e *dbError) Error() string { return e.why }

func routes(st *site, start time.Time) *http.ServeMux {
	mux := http.NewServeMux()
	page := func(extra map[string]any) map[string]any {
		data := map[string]any{"Source": st.sourceText(), "Version": app.Version, "Name": app.Name}
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
	mux.Handle("/api/health", health.Handler(start))

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
			"Count": textfmt.Plural(len(posts), "1 post", itoa(len(posts))+" posts")}))
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
		st.mu.RLock()
		path := st.dbPath
		st.mu.RUnlock()
		render(w, http.StatusOK, aboutPage, page(map[string]any{"DB": path, "Credit": app.Credit,
			"Publisher": app.Publisher, "Counts": st.get().Counts(), "Saved": r.URL.Query().Get("saved") != ""}))
	})
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

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter, hint string) {
	body := map[string]string{"error": "not found"}
	if hint != "" {
		body["did_you_mean"] = hint
	}
	writeJSON(w, http.StatusNotFound, body)
}

func render(w http.ResponseWriter, code int, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := t.Execute(w, data); err != nil {
		log.Print(err)
	}
}

const layout = `{{define "top"}}<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>Generic Company Inc. Journal</title>
<link rel="icon" href="/favicon.ico">
<style>
body{margin:0;font:16px/1.55 system-ui,sans-serif;color:#1d2330;background:#f6f7f9}
header,main,footer{max-width:760px;margin:0 auto;padding:16px}
header{display:flex;gap:12px;align-items:center;flex-wrap:wrap}header h1{font-size:20px;margin:0;flex:1;display:flex;gap:10px;align-items:center}
.px{image-rendering:pixelated;image-rendering:crisp-edges}
a{color:#2f5bd3;text-decoration:none}a:hover{text-decoration:underline}
.card{background:#fff;border:1px solid #dde1e7;border-radius:8px;padding:14px 16px;margin-bottom:12px}
.meta{color:#667085;font-size:13px}h2{font-size:18px;margin:0 0 4px}
input{font:inherit;padding:4px 8px;border:1px solid #c3c8d0;border-radius:6px}
.wide{width:100%;box-sizing:border-box}.error{color:#b42318}
footer{color:#667085;font-size:12px}
@media (prefers-color-scheme:dark){body{color:#e6e8ec;background:#14171c}a{color:#8fb0ff}.card{background:#1d2128!important;border-color:#2e343e!important}}
</style></head><body><header><h1><img class="px" src="/icon-32.png" width="32" height="32" alt=""><a href="/">Generic Company Inc. Journal</a></h1>
<form action="/"><input name="q" type="search" value="{{.Q}}" placeholder="Search posts"></form><a href="/about">About</a></header><main>{{end}}
{{define "bottom"}}</main><footer>{{.Name}} {{.Version}} · Fictional company; text is lorem ipsum. Authors from {{.Source}}.</footer></body></html>{{end}}`

var (
	indexPage = template.Must(template.Must(template.New("index").Parse(layout)).Parse(`{{template "top" .}}
<p class="meta">{{.Count}}{{if .Q}} matching "{{.Q}}"{{end}}</p>
{{range .Posts}}<article class="card"><h2><a href="/posts/{{.Slug}}">{{.Title}}</a></h2>
<div class="meta">{{.Author}} · {{.PublishedAt}}</div><p>{{.Excerpt}}</p></article>{{end}}
{{template "bottom" .}}`))
	postPage = template.Must(template.Must(template.New("post").Parse(layout)).Parse(`{{template "top" .}}
<article class="card"><h2>{{.Post.Title}}</h2><div class="meta">{{.Author}} · {{.Post.PublishedAt}}</div>
{{range .Blocks}}<p>{{.Text}}</p>{{end}}</article>
{{if .Related}}<h3>Related</h3>{{range .Related}}<div class="card"><a href="/posts/{{.Slug}}">{{.Title}}</a> <span class="meta">{{.Author}}</span></div>{{end}}{{end}}
{{template "bottom" .}}`))
	missingPage = template.Must(template.Must(template.New("missing").Parse(layout)).Parse(`{{template "top" .}}
<div class="card"><h2>Post not found</h2>{{if .Hint}}<p>Did you mean <a href="/posts/{{.Hint}}">{{.Hint}}</a>?</p>{{end}}</div>
{{template "bottom" .}}`))
	aboutPage = template.Must(template.Must(template.New("about").Parse(layout)).Parse(`{{template "top" .}}
<div class="card" style="text-align:center"><img class="px" src="/icon-256.png" width="128" height="128" alt="">
<h2>{{.Name}} {{.Version}}</h2><p class="meta">{{.Publisher}}</p><p class="meta">{{.Credit}}<br>Code: AGPL-3.0. All data is fictional.</p></div>
<div class="card"><h2>Shared database</h2><p class="meta">Post authors come from its employees table ({{.Counts.authors}} known). Posts stay in the CMS's own snapshot.</p>
<form method="post" action="/about"><input class="wide" name="db" value="{{.DB}}" spellcheck="false">
<p><button type="submit">Use this database</button>{{if .Saved}} <span class="meta">Saved.</span>{{end}}</p></form>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}</div>
{{template "bottom" .}}`))
)

// ==== stamp v1 | author=kjFriday | hmac-sha256=e3cb0091c9bd2b30ad1148d51e9a2855612c1b4799154b0bffaa071bd40c38ce ====
