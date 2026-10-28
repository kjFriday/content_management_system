// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: internal/db/db_test.go
// About: ==== Generic Company Inc. portfolio ===============================================...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// ==== Generic Company Inc. portfolio ===============================================
// Project: content_management_system | File: internal/db/db_test.go
// About: Tests for package db.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package db

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// mockDB builds generic_company.db from the hub's schema (mock/schema.sql) and the mock employees.
func mockDB(t *testing.T) string {
	t.Helper()
	schema, err := os.ReadFile("../../mock/schema.sql")
	if err != nil {
		t.Skip("no mock/schema.sql:", err)
	}
	path := filepath.Join(t.TempDir(), "generic_company.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	var employees []map[string]any
	b, _ := os.ReadFile("../../mock/employees.json")
	json.Unmarshal(b, &employees)
	for _, e := range employees {
		if _, err := conn.Exec("INSERT INTO employees (id, name, role, email) VALUES (?, ?, ?, ?)", e["id"], e["name"], e["role"], e["email"]); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestAuthorsFromTheSharedDatabase(t *testing.T) {
	authors, err := LoadAuthors(mockDB(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(authors) != 120 || authors[0].ID != 1 || authors[0].Name == "" {
		t.Fatalf("%d authors, first %+v", len(authors), authors[0])
	}
}

func TestReadOnlyAndNeverCreated(t *testing.T) {
	path := mockDB(t)
	conn, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec("DELETE FROM employees"); err == nil {
		t.Fatal("a read-only connection wrote")
	}
	missing := filepath.Join(t.TempDir(), "none.db")
	if _, err := Open(missing); err == nil {
		t.Fatal("opened a missing file")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("a missing database was created")
	}
}

func TestSchemaCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.db")
	conn, _ := sql.Open("sqlite", path)
	conn.Exec("CREATE TABLE employees (id INTEGER, name TEXT)")
	conn.Close()
	if _, err := LoadAuthors(path); err == nil || err.Error() != "employees lacks role" {
		t.Fatalf("err = %v", err)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=d76f7036aadf90ee02d4ad972580c3f86b6332158d69c3714a42baebcaecbb6d ====
