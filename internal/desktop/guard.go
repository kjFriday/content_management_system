// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/desktop/guard.go
// About: Package desktop shows a Generic Company program's web pages in its own native window.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package desktop shows a Generic Company program's web pages in its own native window.
//
// The window is Edge WebView2. The pages are served by an in-process server on 127.0.0.1 that
// only that window can use.
//
// How a run goes (Run, in window_windows.go):
//  1. Listen on 127.0.0.1 on a free port the system picks, and serve the program's handler there
//     behind Guard (this file), with a fresh NewToken.
//  2. Open the WebView2 window and bind the requested JavaScript functions to Go.
//  3. Navigate to /__open?t=TOKEN: Guard swaps the token for a cookie and redirects to "/".
//  4. Call Ready, then run the window's message loop until it closes.
//
// Files: guard.go (the token and cookie check), options.go (Options), window_windows.go (the
// real window, Window and MessageBox) and window_other.go (stubs so the package builds off
// Windows, where only the program's serve modes work).
package desktop

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// Cookie carries the per-run token after the window's first request. Every Generic Company
// program uses this name; their separate WebView2 profiles keep the cookies apart.
const Cookie = "generic_company_session"

// NewToken is 32 random hex characters, made fresh for every run.
// The 16 bytes (128 bits) come from crypto/rand, so the token cannot be guessed.
func NewToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// no secure randomness means no safe guard: stop rather than serve with a weak token
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Guard lets a request through only from the program's own window: the first navigation to
// /__open?t=TOKEN sets an HttpOnly, SameSite=Strict cookie; after that every request needs the cookie
// and the exact Host. A web page in a browser can neither read the token nor send the cookie, so it
// cannot use (or change anything through) the local server.
//
// host is the server's "127.0.0.1:port". Every refusal is a plain 403 "forbidden".
//
// Why each part:
//   - The Host check stops DNS rebinding: a hostile site whose name is made to resolve to
//     127.0.0.1 still sends its own name in Host, so its requests are refused.
//   - HttpOnly keeps page scripts from reading the cookie; SameSite=Strict keeps the browser from
//     sending it on requests that start on another site.
//   - The token is compared in constant time, so response timing does not leak how many leading
//     characters of a guess were right.
//   - The 303 redirect after /__open leaves the page on plain "/", so the token is not kept in
//     the page's address, where a script could read it.
func Guard(next http.Handler, token, host string) http.Handler {
	ok := func(s string) bool { return subtle.ConstantTimeCompare([]byte(s), []byte(token)) == 1 }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != host {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/__open" {
			if !ok(r.URL.Query().Get("t")) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: Cookie, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		c, err := r.Cookie(Cookie)
		if err != nil || !ok(c.Value) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=fd1df5d5a0cc63347a4fa4c0c93788533d8059aced74ecd23f1d78db45b81d05 ====
