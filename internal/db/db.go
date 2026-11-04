// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/db/db.go
// About: Package db reads post authors from the shared Generic Company SQLite database, read-only.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package db reads post authors from the shared Generic Company SQLite database, read-only.
//
// It opens the database by path (the CMS only reads post authors from it). The CRUD hub owns the
// schema; this package never creates or changes tables, and never creates the file.
//
// site.go calls LoadAuthors, which runs the other three in order:
//  1. Open opens the file read-only and checks it answers.
//  2. CheckSchema confirms every table and column in Needs exists, so a database from another
//     program, or an older schema, gives a clear error instead of a failed query later.
//  3. Authors reads the employees table as store.Author records.
//
// Any error makes site.go fall back to the embedded snapshot and show the error text in the footer.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"generic-company/cms/internal/store"

	_ "modernc.org/sqlite" // pure-Go SQLite: no cgo, so the exe builds with the Go toolchain alone
)

// Needs is every table and column the CMS reads, checked by CheckSchema. Extra columns (the hub's
// employees table also has email, for one) are fine.
var Needs = map[string][]string{"employees": {"id", "name", "role"}}

// Open opens path read-only. A missing file is an error; it is never created.
//
// The caller closes the returned handle. Errors: the file is missing or unreadable, or is not a
// SQLite database (found by the Ping, since sql.Open itself only checks the name).
//
// Steps:
//  1. Stat first, so a missing file fails with the operating system's plain "cannot find"
//     error before SQLite is involved. Nothing here ever creates the file.
//  2. Build a SQLite URI filename ("file:" + path + "?" + options). The path is made absolute,
//     uses forward slashes and is percent-escaped, so a space or "#" or "?" in a folder name is
//     not read as part of the options.
//  3. Open and Ping, which makes the driver actually open the file.
func Open(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(path)
	// mode=ro: SQLite refuses every write on this connection.
	// busy_timeout(5000): when another program holds a write lock, wait up to 5000 ms for it
	// instead of failing at once with SQLITE_BUSY.
	// foreign_keys(1): SQLite leaves foreign-key checks off by default and this turns them on;
	// a read-only connection never writes, so it changes nothing here.
	// _pragma=NAME(VALUE) is the modernc driver's way to run "PRAGMA NAME = VALUE" on each new
	// connection in the pool.
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
//
// The errors read "no employees table" or "employees lacks name, role" (the missing columns in
// Needs order); the footer shows them as they are. Map order makes the table order random, which
// matters only once Needs holds more than one table.
//
// Known limitation: a failed rows.Scan or a rows.Err after the loop is ignored, so a read error
// shows up as missing columns rather than as itself.
//
// Steps, for each table in Needs:
//  1. List its columns through pragma_table_info, the table-valued form of PRAGMA table_info:
//     one row per column, and no rows at all for a table that does not exist. Unlike the plain
//     PRAGMA statement it takes the table name as a bound parameter, so no SQL is built by hand.
//  2. No columns means no table.
//  3. Collect the needed columns that are absent.
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

// Authors reads every employee as a post author, ordered by id. A NULL role reads as "". A row
// that does not scan (for example a NULL name) fails the whole read.
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
// The connection is closed before it returns. An empty employees table is an error ("no
// employees"): posts with no one to credit are better served from the snapshot.
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=28aa44258a96c7519b8a06de495ab1fcb8f81a729ac1daae146b42ef526c2566 ====
