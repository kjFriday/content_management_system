// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: main_test.go
// About: End-to-end tests for the routes, pages and API.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// End-to-end tests for the routes, pages and API.
//
// Each test starts routes on an httptest server over the embedded snapshot
// and checks status codes and bodies. The
// database path always points at a file that does not exist in a temporary
// folder, so the site runs on the snapshot and a test can also check that no
// database file is created.

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

// server starts the owner's (non-public) routes over the snapshot and returns the server and the
// store it serves, which the test reads for expected values. The server closes when the test ends.
func server(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st := newSite(filepath.Join(t.TempDir(), "none.db"), false)
	s := st.get()
	srv := httptest.NewServer(routes(st, time.Now(), false))
	t.Cleanup(srv.Close)
	return srv, s
}

// get fetches url and returns the status code and the body; a transport error fails the test.
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

// TestAPI pins every JSON route: the list matches the store, a post and its related list answer
// 200, a slug with its last letter changed to "x" answers 404 with a did_you_mean, the source is
// the snapshot, and health answers outside -public.
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

// TestPages pins the HTML pages: the index lists the newest post, a post page shows its author, a
// search for "<script>" comes back escaped, and an unknown slug answers 404.
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

// TestNoDatabaseMeansSnapshot pins the fallback: a missing database means snapshot authors, the
// footer says so, the file is not created by the attempt, and switching to it fails.
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

// TestIconsAndAboutPage pins the three icon routes and their content types, and that the About page
// shows the credit line (it contains "CC BY 2.5") and the version "0.2.0".
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

// TestPublic pins the -public site behind secure: no page or API answer shows the database path or
// its form, every answer carries the security headers, POST /about answers 405 and /api/health 404.
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=03ff7b590302299b2266beacced4fdc84b7e891efc94f059131215ad28ab8474 ====
