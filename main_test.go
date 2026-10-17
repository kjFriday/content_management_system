package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"acme/cms/internal/store"
)

func server(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st := newSite(filepath.Join(t.TempDir(), "none.db"), false)
	s := st.get()
	srv := httptest.NewServer(routes(st, time.Now()))
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

func TestNoDatabaseMeansSnapshot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.db")
	st := newSite(missing, false)
	if st.get().Source != "mock" || !strings.Contains(st.sourceText(), "mock snapshot") {
		t.Fatalf("source %q / %q", st.get().Source, st.sourceText())
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("a missing database was created")
	}
	if err := st.useDB(missing); err == nil {
		t.Fatal("switched to a missing database")
	}
}

func TestIconsAndAboutPage(t *testing.T) {
	srv, _ := server(t)
	for path, kind := range map[string]string{"/favicon.ico": "image/x-icon", "/icon-32.png": "image/png", "/icon-256.png": "image/png"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil || resp.StatusCode != 200 || resp.Header.Get("Content-Type") != kind {
			t.Fatalf("%s: %v %v", path, err, resp)
		}
		resp.Body.Close()
	}
	if code, body := get(t, srv.URL+"/about"); code != 200 || !strings.Contains(body, "CC BY 2.5") || !strings.Contains(body, "0.2.0") {
		t.Fatalf("about: %d", code)
	}
}
