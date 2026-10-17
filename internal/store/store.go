// Package store holds the CMS's posts in memory and answers the read API: published lists, lookups by
// slug with a did-you-mean, related posts and a plain-text search. Posts come from the snapshot's
// posts.json; authors from its employees.json or from the shared database's employees table.
package store

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"

	"acme/cms/internal/related"
	"acme/cms/internal/suggest"
	"acme/cms/internal/textfmt"
)

// Post is one CMS post. Body is the editor's JSON block content, kept as-is.
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

// Author is an ACME Inc. employee (the hub's employees table).
type Author struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Summary is a post as the list endpoints show it.
type Summary struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Author      string `json:"author"`
	PublishedAt string `json:"published_at"`
	Excerpt     string `json:"excerpt"`
}

// Store is read-only after New and safe for concurrent use.
type Store struct {
	Source  string // where the authors came from: "mock" or "database"
	posts   []Post
	bySlug  map[string]int
	authors map[int]Author
	text    []string // plain text per post, for search and excerpts
	index   *related.Index
}

// New indexes posts and authors.
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

func readJSON(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// FromMock loads posts and authors from a snapshot (posts.json, employees.json at the root of fsys).
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

// Author returns the post's author name, or "ACME Inc." when the id is unknown.
func (s *Store) Author(p Post) string {
	if a, ok := s.authors[p.AuthorID]; ok {
		return a.Name
	}
	return "ACME Inc."
}

func (s *Store) summary(i int) Summary {
	p := s.posts[i]
	at := ""
	if p.PublishedAt != nil {
		at = *p.PublishedAt
	}
	return Summary{Title: p.Title, Slug: p.Slug, Author: s.Author(p), PublishedAt: at, Excerpt: textfmt.Truncate(s.text[i], 140)}
}

// Published lists published posts, newest first.
func (s *Store) Published() []Summary {
	var idx []int
	for i, p := range s.posts {
		if p.Status == "published" && p.PublishedAt != nil {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return *s.posts[idx[a]].PublishedAt > *s.posts[idx[b]].PublishedAt })
	out := make([]Summary, len(idx))
	for k, i := range idx {
		out[k] = s.summary(i)
	}
	return out
}

// Get returns a published post by slug. When there is none, hint is the closest published slug.
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

// Text is a post's plain body text.
func (s *Store) Text(slug string) string {
	if i, ok := s.bySlug[slug]; ok {
		return s.text[i]
	}
	return ""
}

// Related lists up to k published posts like slug.
func (s *Store) Related(slug string, k int) []Summary {
	var out []Summary
	for _, r := range s.index.Related(slug, k) {
		out = append(out, s.summary(s.bySlug[r.Name]))
	}
	return out
}

// Search lists published posts whose title or text contains every word of q (case-insensitive).
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

// Counts reports posts by status and how many authors are known.
func (s *Store) Counts() map[string]int {
	c := map[string]int{"authors": len(s.authors)}
	for _, p := range s.posts {
		c[p.Status]++
	}
	return c
}
