// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/store/store_test.go
// About: Tests for the in-memory store.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for the in-memory store.
//
// They cover list order and draft hiding, author fallback, excerpts, the
// did-you-mean on a missed slug, related posts, search, and loading the
// real mock snapshot. sample is a three-post fixture: two published
// posts that share "lorem" and "tempor", and one draft with words no published
// post has, so any leak of the draft shows up.

package store

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// ptr returns a pointer to a copy of s, for the optional PublishedAt field.
func ptr(s string) *string { return &s }

// sample builds a Store over three posts: author 7 is known, author 8 is not, so the second post
// falls back to the company name. The newer post is listed second, so a list in file order fails.
func sample() *Store {
	// body wraps t in a one-block JSON body; its "type" key is structural and must not reach the text.
	body := func(t string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"blocks": []map[string]string{{"type": "paragraph", "text": t}}})
		return b
	}
	posts := []Post{
		{ID: 1, Title: "Lorem ipsum dolor", Slug: "lorem-ipsum-dolor-1", Body: body("lorem ipsum dolor sit amet tempor"), Status: "published", AuthorID: 7, PublishedAt: ptr("2026-01-02T10:00")},
		{ID: 2, Title: "Tempor incididunt", Slug: "tempor-incididunt-2", Body: body("tempor incididunt lorem labore"), Status: "published", AuthorID: 8, PublishedAt: ptr("2026-03-01T10:00")},
		{ID: 3, Title: "Draft only", Slug: "draft-only-3", Body: body("secret draft words"), Status: "draft", AuthorID: 7},
	}
	return New(posts, []Author{{ID: 7, Name: "Maria Garcia"}}, "mock")
}

// TestPublishedIsNewestFirstWithoutDrafts pins the list order, the draft's absence, the author
// fallback and that the excerpt holds body text without the block's "type" value.
func TestPublishedIsNewestFirstWithoutDrafts(t *testing.T) {
	got := sample().Published()
	if len(got) != 2 || got[0].Slug != "tempor-incididunt-2" {
		t.Fatalf("published = %+v", got)
	}
	if got[1].Author != "Maria Garcia" || got[0].Author != "Generic Company Inc." {
		t.Fatalf("authors = %q, %q", got[1].Author, got[0].Author)
	}
	if !strings.HasPrefix(got[1].Excerpt, "lorem ipsum") || strings.Contains(got[1].Excerpt, "paragraph") {
		t.Fatalf("excerpt = %q", got[1].Excerpt)
	}
}

// TestGetHidesDraftsAndSuggests pins that a draft slug is not served, that a one-letter typo gets
// the right hint, and that a published slug is found.
func TestGetHidesDraftsAndSuggests(t *testing.T) {
	s := sample()
	if _, _, ok := s.Get("draft-only-3"); ok {
		t.Fatal("a draft was served")
	}
	if _, hint, ok := s.Get("lorem-ipsum-dolr-1"); ok || hint != "lorem-ipsum-dolor-1" {
		t.Fatalf("hint = %q ok = %v", hint, ok)
	}
	if p, _, ok := s.Get("lorem-ipsum-dolor-1"); !ok || p.ID != 1 {
		t.Fatal("published post not found")
	}
}

// TestRelatedAndSearch pins that the only other published post is related, that search is
// case-insensitive and needs every word, that drafts are not searched and that blank finds nothing.
func TestRelatedAndSearch(t *testing.T) {
	s := sample()
	rel := s.Related("lorem-ipsum-dolor-1", 3)
	if len(rel) != 1 || rel[0].Slug != "tempor-incididunt-2" {
		t.Fatalf("related = %+v", rel)
	}
	if got := s.Search("LOREM tempor"); len(got) != 2 {
		t.Fatalf("search = %+v", got)
	}
	if got := s.Search("secret"); len(got) != 0 {
		t.Fatal("search found a draft")
	}
	if got := s.Search("  "); len(got) != 0 {
		t.Fatal("blank search matched")
	}
}

// TestFromMockSnapshot loads the real snapshot (60 posts) and pins that every published post's
// author is in employees.json. It skips when the snapshot folder is absent.
func TestFromMockSnapshot(t *testing.T) {
	s, err := FromMock(os.DirFS("../../mock"))
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	c := s.Counts()
	if c["published"]+c["draft"] != 60 || c["authors"] == 0 {
		t.Fatalf("counts = %v", c)
	}
	for _, p := range s.Published() {
		if p.Author == "Generic Company Inc." {
			t.Fatalf("post %s has no known author", p.Slug)
		}
	}
}

// TestFromAuthorsKeepsSnapshotPosts pins that database authors replace the snapshot's authors but
// not its posts, and that Source records "database".
func TestFromAuthorsKeepsSnapshotPosts(t *testing.T) {
	s, err := FromAuthors(os.DirFS("../../mock"), []Author{{ID: 1, Name: "Kenji Sato"}}, "database")
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	if s.Source != "database" || s.Counts()["authors"] != 1 || len(s.Published()) == 0 {
		t.Fatalf("source %q counts %v", s.Source, s.Counts())
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=61807ad0707bc7514fa976c91cf5ac0705835e1ae97b12872ba93231f5c7f3c7 ====
