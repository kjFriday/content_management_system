// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: pages.go
// About: The HTML pages: the layout every page shares (head, inline CSS, header, footer), the four...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The HTML pages: the layout every page shares (head, inline CSS, header, footer), the four page
// templates, and render, which writes one as the answer.

package main

import (
	"html/template"
	"log"
	"net/http"
)

// render writes t, filled from data, as an HTML answer with the status code. An error while it runs
// can only be logged: the status line has gone out by then.
func render(w http.ResponseWriter, code int, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	if err := t.Execute(w, data); err != nil {
		log.Print(err)
	}
}

// layout holds the two blocks every page wraps itself in: "top" (the head with the inline CSS, and the
// header with the search box) and "bottom" (the footer). The inline <style> is why secure's
// Content-Security-Policy allows 'unsafe-inline' styles.
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

// The pages, each parsed on top of layout. routes fills them through page(), which adds Source,
// Version, Name and Public to each page's own keys: Posts, Q and Count (index); Post, Author, Blocks
// and Related (post); Hint (missing); DB, Credit, Publisher, Counts, Saved and Error (about).
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
{{if not .Public}}<form method="post" action="/about"><input class="wide" name="db" value="{{.DB}}" spellcheck="false">
<p><button type="submit">Use this database</button>{{if .Saved}} <span class="meta">Saved.</span>{{end}}</p></form>
{{if .Error}}<p class="error">{{.Error}}</p>{{end}}{{end}}</div>
{{template "bottom" .}}`))
)

// ==== stamp v1 | author=kjFriday | hmac-sha256=f3b71dd66d2a75b2ac3df8167a48ec3dbaeb39e3a8d1c72a508e1e16fe78c639 ====
