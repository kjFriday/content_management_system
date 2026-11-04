// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/store/store.go
// About: Package store holds the CMS's posts in memory and answers the read API.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package store holds the CMS's posts in memory and answers the read API.
//
// It serves published lists, lookups by slug with a did-you-mean, related posts and a plain-text
// search. Posts come from the snapshot's posts.json; authors from its employees.json or from the
// shared database's employees table (package db reads that and hands the authors to FromAuthors).
//
// Callers: package main builds a Store at start-up and again when the About page switches database
// (site.go), then calls its methods from the HTTP handlers (main.go). This package calls related for
// body text and similarity, suggest for the did-you-mean, and textfmt for excerpts.
//
// Flow:
//  1. FromMock or FromAuthors reads posts.json (and employees.json) from an fs.FS.
//  2. New flattens every body to plain text once, maps slugs to positions and fits the related
//     index over the published posts.
//  3. Handlers call Published, Get, Related, Search, Author and Counts; none of them changes the
//     Store.
//
// Types: Post (a stored post), Author (an employee), Summary (a post as lists show it) and Store.
package store

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"

	"generic-company/cms/internal/related"
	"generic-company/cms/internal/suggest"
	"generic-company/cms/internal/textfmt"
)

// Post is one CMS post. Body is the editor's JSON block content, kept as-is.
// Status is "published" or "draft"; only published posts are ever served. PublishedAt is nil for a
// draft and otherwise an ISO 8601 local time such as "2026-01-05T09:47"; CreatedAt has the same form.
type Post struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	Body        json.RawMessage `json:"body"`
	Status      string          `json:"status"`
	AuthorID    int             `json:"author_id"`
	PublishedAt *string         `json:"published_at"`
	CreatedAt   string          `json:"created_at"`
}

// Author is an Generic Company Inc. employee (the hub's employees table).
type Author struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Summary is a post as the list endpoints show it. Author is the resolved name, PublishedAt is ""
// when the post has no date, and Excerpt is the start of the plain body text, at most 140 runes.
type Summary struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Author      string `json:"author"`
	PublishedAt string `json:"published_at"`
	Excerpt     string `json:"excerpt"`
}

// Store is read-only after New and safe for concurrent use. To change what is served, build a new
// Store and swap it in (site.useDB in package main does that).
type Store struct {
	Source  string         // where the authors came from: "mock" or "database"
	posts   []Post         // every post, drafts included, in file order
	bySlug  map[string]int // slug -> position in posts
	authors map[int]Author // employee id -> author
	text    []string       // plain text per post, for search and excerpts
	index   *related.Index // similarity index over the published posts only
}

// New indexes posts and authors. source is stored as Source; package main compares it with
// "database" and "mock", so pass one of those.
//
// Steps:
//  1. Map each slug to its position; for a repeated slug the last post wins here, while the related
//     index keeps the first.
//  2. Flatten each body to plain text with related.BodyText, once, for search and excerpts.
//  3. Hand each published post (title plus text, keyed by slug) to the related index; drafts never
//     appear as related posts.
//  4. Map authors by id.
func New(posts []Post, authors []Author, source string) *Store {
	s := &Store{Source: source, posts: posts, bySlug: map[string]int{}, authors: map[int]Author{}}
	docs := make([]related.Doc, 0, len(posts))
	for i, p := range posts {
		s.bySlug[p.Slug] = i
		t := related.BodyText(p.Body)
		s.text = append(s.text, t)
		if p.Status == "published" {
			docs = append(docs, related.Doc{Name: p.Slug, Text: p.Title + " " + t})
		}
	}
	for _, a := range authors {
		s.authors[a.ID] = a
	}
	s.index = related.New(docs)
	return s
}

// readJSON reads the file name from fsys and decodes it into v. A missing file or bad JSON comes
// back as the error.
func readJSON(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// FromMock loads posts and authors from a snapshot (posts.json, employees.json at the root of fsys).
// The Store's Source is "mock". A missing or malformed file is returned as the error.
func FromMock(fsys fs.FS) (*Store, error) {
	var authors []Author
	if err := readJSON(fsys, "employees.json", &authors); err != nil {
		return nil, err
	}
	return FromAuthors(fsys, authors, "mock")
}

// FromAuthors loads posts from the snapshot (the CMS owns them) with the given authors, e.g. the
// shared database's employees; source says where the authors came from.
func FromAuthors(fsys fs.FS, authors []Author, source string) (*Store, error) {
	var posts []Post
	if err := readJSON(fsys, "posts.json", &posts); err != nil {
		return nil, err
	}
	return New(posts, authors, source), nil
}

// Author returns the post's author name, or "Generic Company Inc." when the id is unknown. The
// fallback happens when the database's employees do not include the snapshot's author ids.
func (s *Store) Author(p Post) string {
	if a, ok := s.authors[p.AuthorID]; ok {
		return a.Name
	}
	return "Generic Company Inc."
}

// summary builds the list view of the post at position i: author name, date ("" when unset) and an
// excerpt of at most 140 runes, ending in "…" when cut.
func (s *Store) summary(i int) Summary {
	p := s.posts[i]
	at := ""
	if p.PublishedAt != nil {
		at = *p.PublishedAt
	}
	return Summary{Title: p.Title, Slug: p.Slug, Author: s.Author(p), PublishedAt: at, Excerpt: textfmt.Truncate(s.text[i], 140)}
}

// Published lists published posts, newest first. A published post without PublishedAt is left
// out here and in Search, although Get still serves it.
func (s *Store) Published() []Summary {
	var idx []int
	for i, p := range s.posts {
		if p.Status == "published" && p.PublishedAt != nil {
			idx = append(idx, i)
		}
	}
	// ISO 8601 times in one fixed format sort as plain strings; a post with a differently written
	// date would sort out of place. The stable sort keeps file order for equal dates.
	sort.SliceStable(idx, func(a, b int) bool { return *s.posts[idx[a]].PublishedAt > *s.posts[idx[b]].PublishedAt })
	out := make([]Summary, len(idx))
	for k, i := range idx {
		out[k] = s.summary(i)
	}
	return out
}

// Get returns a published post by slug. When there is none, hint is the closest published slug.
// A draft's slug counts as missing, so a draft is never served. hint is "" when no published slug
// is close (see suggest.Suggest for what counts as close). The vocabulary is rebuilt on every
// miss, which is cheap for a blog-sized list.
func (s *Store) Get(slug string) (p Post, hint string, ok bool) {
	if i, found := s.bySlug[slug]; found && s.posts[i].Status == "published" {
		return s.posts[i], "", true
	}
	var vocab []string
	for _, q := range s.posts {
		if q.Status == "published" {
			vocab = append(vocab, q.Slug)
		}
	}
	hint, _ = suggest.Suggest(slug, vocab)
	return Post{}, hint, false
}

// Text is a post's plain body text, as related.BodyText made it, or "" for an unknown slug. It does
// not check the status, so it also answers for drafts.
func (s *Store) Text(slug string) string {
	if i, ok := s.bySlug[slug]; ok {
		return s.text[i]
	}
	return ""
}

// Related lists up to k published posts like slug, most similar first. It is nil when slug is not
// a published post or nothing shares a term with it; the JSON API then answers null, not [].
func (s *Store) Related(slug string, k int) []Summary {
	var out []Summary
	for _, r := range s.index.Related(slug, k) {
		out = append(out, s.summary(s.bySlug[r.Name]))
	}
	return out
}

// Search lists published posts whose title or text contains every word of q (case-insensitive).
// Words match as substrings, so "rev" finds "revenue". Results keep Published's newest-first order.
// A blank q or no match gives nil, which the JSON API answers as null.
func (s *Store) Search(q string) []Summary {
	words := strings.Fields(strings.ToLower(q))
	var out []Summary
	if len(words) == 0 {
		return out
	}
	for _, sum := range s.Published() {
		hay := strings.ToLower(sum.Title + " " + s.Text(sum.Slug))
		all := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				all = false
				break
			}
		}
		if all {
			out = append(out, sum)
		}
	}
	return out
}

// Counts reports posts by status and how many authors are known: keys "published", "draft" (any
// status that occurs) and "authors". The About page and /api/source show it.
func (s *Store) Counts() map[string]int {
	c := map[string]int{"authors": len(s.authors)}
	for _, p := range s.posts {
		c[p.Status]++
	}
	return c
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=434986d12cf346aa5faddadaa245d26e02eaa731f88076516992edb131a531cc ====
