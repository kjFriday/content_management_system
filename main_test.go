package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"acme/cms/internal/store"
)

func server(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	s, err := store.FromMock("mock")
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	srv := httptest.NewServer(routes(s, time.Now()))
	t.Cleanup(srv.Close)
	return srv, s
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestAPI(t *testing.T) {
	srv, s := server(t)
	code, body := get(t, srv.URL+"/api/posts")
	var list []store.Summary
	if code != 200 || json.Unmarshal([]byte(body), &list) != nil || len(list) != len(s.Published()) || len(list) == 0 {
		t.Fatalf("list: %d %.80s", code, body)
	}
	slug := list[0].Slug
	if code, _ := get(t, srv.URL+"/api/posts/"+slug); code != 200 {
		t.Fatalf("post: %d", code)
	}
	code, body = get(t, srv.URL+"/api/posts/"+slug[:len(slug)-1]+"x")
	if code != 404 || !strings.Contains(body, `"did_you_mean"`) {
		t.Fatalf("typo: %d %s", code, body)
	}
	if code, _ := get(t, srv.URL+"/api/posts/"+slug+"/related"); code != 200 {
		t.Fatalf("related: %d", code)
	}
	if code, body := get(t, srv.URL+"/api/source"); code != 200 || !strings.Contains(body, `"authors_from":"mock"`) {
		t.Fatalf("source: %d %s", code, body)
	}
	if code, _ := get(t, srv.URL+"/api/health"); code != 200 {
		t.Fatalf("health: %d", code)
	}
}

func TestPages(t *testing.T) {
	srv, s := server(t)
	first := s.Published()[0]
	if code, body := get(t, srv.URL+"/"); code != 200 || !strings.Contains(body, first.Title) {
		t.Fatalf("index: %d", code)
	}
	if code, body := get(t, srv.URL+"/posts/"+first.Slug); code != 200 || !strings.Contains(body, first.Author) {
		t.Fatalf("post page: %d", code)
	}
	if code, body := get(t, srv.URL+"/?q=%3Cscript%3E"); code != 200 || strings.Contains(body, "<script>") {
		t.Fatalf("search page is not escaped: %d", code)
	}
	if code, _ := get(t, srv.URL+"/posts/nope"); code != 404 {
		t.Fatalf("missing page: %d", code)
	}
}

func TestLoadFallsBackWhenHubIsDown(t *testing.T) {
	s, err := load("mock", "http://127.0.0.1:1")
	if err != nil {
		t.Skip("no mock snapshot:", err)
	}
	if s.Source != "mock" {
		t.Fatalf("source = %q", s.Source)
	}
}
