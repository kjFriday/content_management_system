// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: main_test.go
// About: ==== Generic Company Inc. portfolio ===============================================...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// ==== Generic Company Inc. portfolio ===============================================
// Project: content_management_system | File: main_test.go
// About: Tests for package main.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

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

	"generic-company/cms/internal/store"
)

func server(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st := newSite(filepath.Join(t.TempDir(), "none.db"), false)
	s := st.get()
	srv := httptest.NewServer(routes(st, time.Now(), false))
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

func TestPublic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "none.db")
	st := newSite(dbPath, false)
	srv := httptest.NewServer(secure(routes(st, time.Now(), true)))
	t.Cleanup(srv.Close)
	first := st.get().Published()[0]
	for _, path := range []string{"/", "/posts/" + first.Slug, "/about", "/api/posts", "/api/source"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		body := string(b)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		if strings.Contains(body, dbPath) || strings.Contains(body, filepath.Base(dbPath)) || strings.Contains(body, `name="db"`) {
			t.Fatalf("%s shows the database path or its form: %.200s", path, body)
		}
		for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
			if resp.Header.Get(h) == "" {
				t.Fatalf("%s: no %s header", path, h)
			}
		}
	}
	resp, err := http.PostForm(srv.URL+"/about", map[string][]string{"db": {dbPath}})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /about: %d, want 405", resp.StatusCode)
	}
	if code, _ := get(t, srv.URL+"/api/health"); code != 404 {
		t.Fatalf("health: %d, want 404", code)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=5571d854ec1e515b19bcbb3667780500f7f430bc438d109e99c9961c3b3c51cf ====
