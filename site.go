// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: site.go
// About: The site state: the current store, where its authors came from, and the database switch.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The site state: the current store, where its authors came from, and the database switch.
//
// Every page and API answer reads the store held here; the About page's form switches it to another
// database. A Store never changes, so switching means building a new one and swapping the pointer
// under a lock.
//
// Flow:
//  1. main (cli.go) calls newSite with the resolved database path and the -mock flag, then sets
//     site.settings so a switch is remembered.
//  2. load tries the database's employees as authors; when that fails it uses the snapshot's and
//     keeps the reason, which the footer shows.
//  3. Handlers call get for the store and sourceText or publicSource for the footer.
//  4. POST /about calls useDB, which loads the new database and swaps it in only on success.

package main

import (
	"embed"
	"errors"
	"io/fs"
	"sync"

	"generic-company/cms/internal/db"
	"generic-company/cms/internal/store"
)

// snapshot is the embedded mock/ folder: the CMS's own posts, and the authors used when there is no
// database.
//
//go:embed mock/posts.json mock/employees.json
var snapshot embed.FS

// mockFS is the snapshot with mock/ as its root, the layout store.FromMock and store.FromAuthors read.
// fs.Sub fails only on an invalid name, and "mock" is valid, so the error is dropped.
func mockFS() fs.FS { sub, _ := fs.Sub(snapshot, "mock"); return sub }

// site is the current store and where its authors came from; the About page can swap it. mu guards
// s, dbPath and why: handlers read under RLock and useDB writes under Lock. settings is set once
// before serving and not guarded.
type site struct {
	mu       sync.RWMutex
	s        *store.Store
	dbPath   string                  // the database path in use, or tried, shown on the About page
	why      string                  // when on the snapshot: why the database was not used
	settings func(path string) error // saves a switched-to path; nil in tests, so nothing is saved
}

// load reads authors from the database at path, or falls back to the snapshot (and says why).
// The second result is "" on the database, "-mock" when forceMock is set, or the database error
// text, such as a missing file, a missing table or column, or "no employees".
//
// Steps:
//  1. Unless forceMock, read the authors with db.LoadAuthors, which opens the file read-only and
//     never creates it.
//  2. On success, pair them with the snapshot's posts and return.
//  3. Otherwise build the store from the snapshot alone and return the reason.
func load(path string, forceMock bool) (*store.Store, string) {
	why := "-mock"
	if !forceMock {
		authors, err := db.LoadAuthors(path)
		if err == nil {
			// Known limitation: this err shadows the outer one. If FromAuthors failed here, the
			// outer err would still be nil and err.Error() below would panic. FromAuthors reads
			// only the embedded posts.json, so only a broken build could get there.
			if s, err := store.FromAuthors(mockFS(), authors, "database"); err == nil {
				return s, ""
			}
		}
		why = err.Error()
	}
	s, err := store.FromMock(mockFS())
	if err != nil { // the snapshot is embedded, so only a broken build gets here
		panic(err)
	}
	return s, why
}

// newSite starts on the database at path, or on the snapshot when forceMock (-mock) is set or the
// database cannot be used. dbPath keeps path either way, so the About page shows the path that was
// tried.
func newSite(path string, forceMock bool) *site {
	s, why := load(path, forceMock)
	return &site{s: s, dbPath: path, why: why}
}

// get is the current store. A Store never changes, so a handler can keep it for the whole request.
func (st *site) get() *store.Store {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.s
}

// sourceText is the footer's "Authors from ..." line: the database path, or the reason the
// snapshot is in use. It shows local paths and errors, so -public uses publicSource instead.
func (st *site) sourceText() string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	if st.s.Source == "database" {
		return "the shared database (" + st.dbPath + ")"
	}
	return "the mock snapshot (" + st.why + ")"
}

// publicSource is the footer line for -public: where authors came from, without paths or errors.
func (st *site) publicSource() string {
	if st.get().Source == "database" {
		return "the shared database"
	}
	return "the mock snapshot"
}

// useDB switches to the database at path, then saves path as the setting. A database that cannot be
// used changes nothing and comes back as the error; a failed save comes back too, after the switch.
// The caller cannot tell the two apart, so POST /about reports a failed save as a failed switch.
//
// The database is read before the lock is taken, so pages keep answering from the old store while
// the new one loads.
func (st *site) useDB(path string) error {
	s, why := load(path, false)
	if why != "" {
		return errors.New(why)
	}
	st.mu.Lock()
	st.s, st.dbPath, st.why = s, path, ""
	st.mu.Unlock()
	if st.settings != nil {
		return st.settings(path)
	}
	return nil
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=f5567e9ffe1a9987d35ea2755968c83b465f729bcb7c2c61586cf8a362f29ace ====
