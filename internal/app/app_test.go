// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/app/app_test.go
// About: Tests for package app: the shared default path, the database precedence and settings.json.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for package app: the shared default path, the database precedence and settings.json.

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeEnv is an Env with LOCALAPPDATA set to dir and other variables read from extra, which the
// caller may change between calls.
func fakeEnv(dir string, extra map[string]string) Env {
	return func(k string) string {
		if k == "LOCALAPPDATA" {
			return dir
		}
		return extra[k]
	}
}

// TestDefaultIsShared pins the default database and data folder under LOCALAPPDATA, the paths the
// other Generic Company programs also use.
func TestDefaultIsShared(t *testing.T) {
	dir := t.TempDir()
	env := fakeEnv(dir, nil)
	if got, want := ResolveDB("", env), filepath.Join(dir, "Generic Company Inc", "generic_company.db"); got != want {
		t.Fatalf("default = %q, want %q", got, want)
	}
	if got := DataDir(env); got != filepath.Join(dir, "Generic Company Inc", "GenericCompany.CMS") {
		t.Fatalf("data dir = %q", got)
	}
}

// TestPrecedence pins flag over environment over saved setting; the last case checks that a
// white-space environment value is skipped.
func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	extra := map[string]string{}
	env := fakeEnv(dir, extra)
	if err := SaveSettings(env, map[string]any{"db": filepath.Join(dir, "saved.db")}); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ flag, envDB, want string }{
		{"", "", "saved.db"},
		{"", filepath.Join(dir, "env.db"), "env.db"},
		{filepath.Join(dir, "flag.db"), filepath.Join(dir, "env.db"), "flag.db"},
		{"", "   ", "saved.db"},
	}
	for _, c := range cases {
		extra["GENERIC_COMPANY_DB"] = c.envDB
		if got := ResolveDB(c.flag, env); !strings.HasSuffix(got, c.want) {
			t.Errorf("flag %q env %q -> %q, want *%s", c.flag, c.envDB, got, c.want)
		}
	}
}

// TestSettingsKeepUnknownKeysAndTolerateJunk pins that a broken file reads as empty, that a save
// keeps keys it did not change, and that no .tmp file is left behind after the rename.
func TestSettingsKeepUnknownKeysAndTolerateJunk(t *testing.T) {
	env := fakeEnv(t.TempDir(), nil)
	if err := os.MkdirAll(DataDir(env), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(settingsPath(env), []byte("{not json"), 0o644)
	if len(LoadSettings(env)) != 0 {
		t.Fatal("junk settings were not ignored")
	}
	SaveSettings(env, map[string]any{"db": "a.db", "theme": "dark"})
	SaveSettings(env, map[string]any{"db": "b.db"})
	s := LoadSettings(env)
	if s["db"] != "b.db" || s["theme"] != "dark" || s["version"] != float64(1) {
		t.Fatalf("settings = %v", s)
	}
	entries, _ := os.ReadDir(DataDir(env))
	if len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
}

// TestTitleCarriesTheVersion pins the version at the end of the title and the licence name in
// the icon credit.
func TestTitleCarriesTheVersion(t *testing.T) {
	if !strings.HasSuffix(Title, " "+Version) || !strings.Contains(Credit, "CC BY 2.5") {
		t.Fatalf("title %q credit %q", Title, Credit)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=fb29f0881c2444b71ea505650f8acffdb95794242d93d3c788c6b06f7b8bf10a ====
