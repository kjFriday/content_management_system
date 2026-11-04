// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/install_test.go
// About: Tests for install.go (Install, Uninstall) and the fixtures every test file here shares.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for install.go (Install, Uninstall) and the fixtures every test file here shares.
//
// Every test installs into a sandbox root under t.TempDir and, except TestRealShortcutInSandbox,
// records the PowerShell scripts instead of running them, so nothing on the machine changes.
// Fixtures: recorder, setup, day and noEnv.

package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recorder stands in for PowerShell: it keeps every script it is given and runs none.
type recorder struct{ scripts []string }

// run is the Runner: it records s and succeeds.
func (r *recorder) run(s string) error { r.scripts = append(r.scripts, s); return nil }

// setup writes a fake exe into a temp build folder and returns a test Program for it, plus the
// sandbox root (not yet created) to install under.
func setup(t *testing.T) (Program, string) {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "build", "generic-company-test.exe")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("MZ fake exe"), 0o755)
	return Program{AppID: "GenericCompany.Test", Name: "Generic Company Test App", Version: "0.2.0", Publisher: "Generic Company Inc. (fictional)",
		Exe: exe, ExeName: "generic-company-test.exe", Icon: []byte{0, 0, 1, 0}}, filepath.Join(dir, "sandbox")
}

// day is the fixed install date, so InstallDate reads "20261017" in every run.
var day = time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)

// noEnv is an empty environment. With a sandbox root no variable is read anyway; without one,
// Locations falls back to the user's home folder.
func noEnv(string) string { return "" }

// TestRoundTrip installs twice (a reinstall must work), then uninstalls: the folder and registry
// key go, the database beside the sandbox stays, and three scripts ran.
func TestRoundTrip(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, true, 1, day)
	rec := &recorder{}
	db := filepath.Join(root, "generic_company.db")
	os.MkdirAll(root, 0o755)
	os.WriteFile(db, nil, 0o644)
	if err := Install(p, rec.run); err != nil {
		t.Fatal(err)
	}
	if err := Install(p, rec.run); err != nil { // reinstall
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p.ExeTarget); string(b) != "MZ fake exe" {
		t.Fatal("exe not copied")
	}
	var reg map[string]map[string]any
	b, _ := os.ReadFile(p.RegistryFile)
	json.Unmarshal(b, &reg)
	if reg[p.RegistryKey]["DisplayVersion"] != "0.2.0" {
		t.Fatalf("registry %v", reg)
	}
	removed, later, err := Uninstall(p, rec.run, prog.Exe, "")
	if err != nil || !removed || later != "" {
		t.Fatalf("uninstall: %v %v %q", removed, err, later)
	}
	if _, err := os.Stat(p.InstallDir); !os.IsNotExist(err) {
		t.Fatal("install dir left behind")
	}
	if _, err := os.Stat(db); err != nil {
		t.Fatal("the database was removed")
	}
	b, _ = os.ReadFile(p.RegistryFile)
	if strings.TrimSpace(string(b)) != "{}" {
		t.Fatalf("registry left: %s", b)
	}
	if len(rec.scripts) != 3 {
		t.Fatalf("%d scripts", len(rec.scripts))
	}
}

// TestRunningFromInstallDirDeletesLater uninstalls as the installed exe: only that exe is left,
// and the folder comes back as later.
func TestRunningFromInstallDirDeletesLater(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	rec := &recorder{}
	Install(p, rec.run)
	// Uninstall only names the folder: the caller starts DeleteAfterExit once its message box is gone
	removed, later, err := Uninstall(p, rec.run, p.ExeTarget, "")
	if err != nil || !removed || later != p.InstallDir {
		t.Fatalf("%v %v %q", removed, err, later)
	}
	entries, _ := os.ReadDir(p.InstallDir)
	if len(entries) != 1 || entries[0].Name() != "generic-company-test.exe" {
		t.Fatalf("left: %v", entries)
	}
}

// TestGuardRefusesForeignFolder pins both guards: no uninstall without a marker, no install over
// another app id's marker, and the foreign file survives both.
func TestGuardRefusesForeignFolder(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	os.MkdirAll(p.InstallDir, 0o755)
	os.WriteFile(filepath.Join(p.InstallDir, "keep.txt"), []byte("not ours"), 0o644)
	if _, _, err := Uninstall(p, (&recorder{}).run, prog.Exe, ""); err == nil {
		t.Fatal("removed a folder without a marker")
	}
	os.WriteFile(filepath.Join(p.InstallDir, Marker), []byte(`{"app_id":"GenericCompany.Other"}`), 0o644)
	if err := Install(p, (&recorder{}).run); err == nil {
		t.Fatal("installed over another program")
	}
	if _, err := os.Stat(filepath.Join(p.InstallDir, "keep.txt")); err != nil {
		t.Fatal("foreign file deleted")
	}
}

// TestRealShortcutInSandbox runs the real PowerShell (skipped with -short): the .lnk appears, and
// after uninstall the emptied Start-menu folder is gone too. The sandbox keeps the registry out.
func TestRealShortcutInSandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	if err := Install(p, PowerShell); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Shortcuts[0].Path); err != nil {
		t.Fatal("no shortcut:", err)
	}
	if _, _, err := Uninstall(p, PowerShell, prog.Exe, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(p.Shortcuts[0].Path)); !os.IsNotExist(err) {
		t.Fatal("Start-menu folder left behind")
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=45fb340a288e6b23b4db9f5e3eef2c6a5f8feac953190a9f172bce6334ba94d4 ====
