// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: site.go
// About: The site state: the store every page and API answer reads, where its authors came from...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The site state: the store every page and API answer reads, where its authors came from, and the
// switch to another database from the About page.

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
func mockFS() fs.FS { sub, _ := fs.Sub(snapshot, "mock"); return sub }

// site is the current store and where its authors came from; the About page can swap it.
type site struct {
	mu       sync.RWMutex
	s        *store.Store
	dbPath   string
	why      string // when on the snapshot: why the database was not used
	settings func(path string) error
}

// load reads authors from the database at path, or falls back to the snapshot (and says why).
func load(path string, forceMock bool) (*store.Store, string) {
	why := "-mock"
	if !forceMock {
		authors, err := db.LoadAuthors(path)
		if err == nil {
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
// database cannot be used.
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

// sourceText is the footer's "Authors from ..." line.
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=c2eabcee636b49173357d069dfd1a68291901b39df5216a3da41d1fc9b9b4d9f ====
