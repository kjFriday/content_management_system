// ==== ACME Inc. portfolio ===============================================
// Project: content_management_system | File: internal/db/db.go
// About: Package db opens the shared ACME database by path (read-only: the CMS only reads post...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package db opens the shared ACME database by path (read-only: the CMS only reads post authors from
// it). The CRUD hub owns the schema; this package never creates or changes tables.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"acme/cms/internal/store"

	_ "modernc.org/sqlite" // pure-Go SQLite: no cgo, so the exe builds with the Go toolchain alone
)

// Needs is every table and column the CMS reads.
var Needs = map[string][]string{"employees": {"id", "name", "role"}}

// Open opens path read-only. A missing file is an error; it is never created.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(path)
	dsn := "file:" + (&url.URL{Path: filepath.ToSlash(abs)}).EscapedPath() +
		"?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// CheckSchema fails unless every table and column in Needs exists.
func CheckSchema(conn *sql.DB) error {
	for table, cols := range Needs {
		rows, err := conn.Query("SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			return err
		}
		have := map[string]bool{}
		for rows.Next() {
			var n string
			rows.Scan(&n)
			have[n] = true
		}
		rows.Close()
		if len(have) == 0 {
			return fmt.Errorf("no %s table", table)
		}
		var missing []string
		for _, c := range cols {
			if !have[c] {
				missing = append(missing, c)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s lacks %s", table, strings.Join(missing, ", "))
		}
	}
	return nil
}

// Authors reads every employee as a post author.
func Authors(conn *sql.DB) ([]store.Author, error) {
	rows, err := conn.Query("SELECT id, name, COALESCE(role, '') FROM employees ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.Author
	for rows.Next() {
		var a store.Author
		if err := rows.Scan(&a.ID, &a.Name, &a.Role); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LoadAuthors opens path, checks the schema and reads the authors in one go.
func LoadAuthors(path string) ([]store.Author, error) {
	conn, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := CheckSchema(conn); err != nil {
		return nil, err
	}
	authors, err := Authors(conn)
	if err == nil && len(authors) == 0 {
		err = errors.New("no employees")
	}
	return authors, err
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=3d0914ecedb8551d4149f8bc220c29c8c0fba6c1e0bfd1647b448c511baf4285 ====
