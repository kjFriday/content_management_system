// Command cms serves ACME Inc.'s posts: a read-only JSON API and two HTML pages, standard library only.
//
// Standalone (default) it reads mock/posts.json and mock/employees.json. With -hub (or ACME_HUB) post
// authors come live from the CRUD hub's employees table; if the hub is unreachable it says so and
// falls back to the snapshot. The earlier SQLite design is kept in reference/ for later.
package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"acme/cms/internal/health"
	"acme/cms/internal/store"
	"acme/cms/internal/textfmt"
)

func main() {
	addr := flag.String("addr", envOr("CMS_ADDR", "127.0.0.1:8720"), "listen address")
	mock := flag.String("mock", "mock", "snapshot folder (posts.json, employees.json)")
	hub := flag.String("hub", os.Getenv("ACME_HUB"), "CRUD hub base URL for live authors (empty: snapshot only)")
	flag.Parse()

	s, err := load(*mock, strings.TrimRight(*hub, "/"))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ACME Inc. CMS on http://%s/ (authors from %s, %v)", *addr, s.Source, s.Counts())
	log.Fatal(http.ListenAndServe(*addr, routes(s, time.Now())))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func load(mock, hub string) (*store.Store, error) {
	if hub != "" {
		s, err := store.FromHub(mock, hub)
		if err == nil {
			return s, nil
		}
		log.Printf("hub unreachable at %s (%v); using the mock snapshot", hub, err)
	}
	return store.FromMock(mock)
}

func routes(s *store.Store, start time.Time) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/posts", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.Published())
	})
	mux.HandleFunc("GET /api/posts/{slug}", func(w http.ResponseWriter, r *http.Request) {
		p, hint, ok := s.Get(r.PathValue("slug"))
		if !ok {
			notFound(w, hint)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"post": p, "author": s.Author(p)})
	})
	mux.HandleFunc("GET /api/posts/{slug}/related", func(w http.ResponseWriter, r *http.Request) {
		if _, hint, ok := s.Get(r.PathValue("slug")); !ok {
			notFound(w, hint)
			return
		}
		writeJSON(w, http.StatusOK, s.Related(r.PathValue("slug"), 3))
	})
	mux.HandleFunc("GET /api/search", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.Search(r.URL.Query().Get("q")))
	})
	mux.HandleFunc("GET /api/source", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"authors_from": s.Source, "counts": s.Counts()})
	})
	mux.Handle("/api/health", health.Handler(start))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		posts := s.Published()
		if q != "" {
			posts = s.Search(q)
		}
		render(w, http.StatusOK, indexPage, map[string]any{"Posts": posts, "Q": q, "Source": s.Source,
			"Count": textfmt.Plural(len(posts), "1 post", itoa(len(posts))+" posts")})
	})
	mux.HandleFunc("GET /posts/{slug}", func(w http.ResponseWriter, r *http.Request) {
		p, hint, ok := s.Get(r.PathValue("slug"))
		if !ok {
			render(w, http.StatusNotFound, missingPage, map[string]any{"Hint": hint, "Source": s.Source})
			return
		}
		var body struct {
			Blocks []struct {
				Text string `json:"text"`
			} `json:"blocks"`
		}
		_ = json.Unmarshal(p.Body, &body)
		render(w, http.StatusOK, postPage, map[string]any{"Post": p, "Author": s.Author(p), "Blocks": body.Blocks,
			"Related": s.Related(p.Slug, 3), "Source": s.Source})
	})
	return mux
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
<meta name="viewport" content="width=device-width, initial-scale=1"><title>ACME Inc. Journal</title>
<style>
body{margin:0;font:16px/1.55 system-ui,sans-serif;color:#1d2330;background:#f6f7f9}
@media (prefers-color-scheme:dark){body{color:#e6e8ec;background:#14171c}a{color:#8fb0ff}.card{background:#1d2128!important;border-color:#2e343e!important}}
header,main,footer{max-width:760px;margin:0 auto;padding:16px}
header{display:flex;gap:12px;align-items:center;flex-wrap:wrap}header h1{font-size:20px;margin:0;flex:1}
a{color:#2f5bd3;text-decoration:none}a:hover{text-decoration:underline}
.card{background:#fff;border:1px solid #dde1e7;border-radius:8px;padding:14px 16px;margin-bottom:12px}
.meta{color:#667085;font-size:13px}h2{font-size:18px;margin:0 0 4px}
input{font:inherit;padding:4px 8px;border:1px solid #c3c8d0;border-radius:6px}
footer{color:#667085;font-size:12px}
</style></head><body><header><h1><a href="/">ACME Inc. Journal</a></h1>
<form action="/"><input name="q" type="search" value="{{.Q}}" placeholder="Search posts"></form></header><main>{{end}}
{{define "bottom"}}</main><footer>Fictional company; text is lorem ipsum. Authors from {{.Source}}. JSON at <a href="/api/posts">/api/posts</a>.</footer></body></html>{{end}}`

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
)
