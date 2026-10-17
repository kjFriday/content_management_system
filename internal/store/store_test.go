package store

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

func sample() *Store {
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

func TestPublishedIsNewestFirstWithoutDrafts(t *testing.T) {
	got := sample().Published()
	if len(got) != 2 || got[0].Slug != "tempor-incididunt-2" {
		t.Fatalf("published = %+v", got)
	}
	if got[1].Author != "Maria Garcia" || got[0].Author != "ACME Inc." {
		t.Fatalf("authors = %q, %q", got[1].Author, got[0].Author)
	}
	if !strings.HasPrefix(got[1].Excerpt, "lorem ipsum") || strings.Contains(got[1].Excerpt, "paragraph") {
		t.Fatalf("excerpt = %q", got[1].Excerpt)
	}
}

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

func TestFromMockSnapshot(t *testing.T) {
	s, err := FromMock("../../mock")
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	c := s.Counts()
	if c["published"]+c["draft"] != 60 || c["authors"] == 0 {
		t.Fatalf("counts = %v", c)
	}
	for _, p := range s.Published() {
		if p.Author == "ACME Inc." {
			t.Fatalf("post %s has no known author", p.Slug)
		}
	}
}

func TestFromHubPagesEmployees(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/employees" || r.URL.Query().Get("limit") != "500" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`[{"id": 7, "name": "Kenji Sato", "role": "Sales"}]`))
	}))
	defer srv.Close()
	s, err := FromHub("../../mock", srv.URL)
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	if calls != 1 || s.Source != srv.URL || s.Counts()["authors"] != 1 {
		t.Fatalf("calls %d source %q counts %v", calls, s.Source, s.Counts())
	}
	if _, err := FromHub("../../mock", "http://127.0.0.1:1"); err == nil {
		t.Fatal("an unreachable hub did not fail")
	}
}
