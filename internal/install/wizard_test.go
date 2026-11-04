// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/wizard_test.go
// About: Tests for wizard.go: handing a Wizard Maker install to its own uninstall.exe.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for wizard.go: handing a Wizard Maker install to its own uninstall.exe.
//
// The uninstaller is never really run: each test passes a fake Exec or Reader, except
// TestRunAndWaitReturnsTheExitCode, which runs cmd.exe.

package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wizardInstall lays out what a setup made by Generic Company Wizard Maker leaves: its files, uninstall.exe and a
// marker with sha256 and installer, then changes notes.txt as a user would.
func wizardInstall(t *testing.T, p Plan, appID string) {
	t.Helper()
	os.MkdirAll(p.InstallDir, 0o755)
	files := []string{"generic-company-test.exe", "notes.txt", "app.ico", WizardUninstaller}
	for _, f := range files {
		os.WriteFile(filepath.Join(p.InstallDir, f), []byte("as installed"), 0o644)
	}
	b, _ := json.Marshal(map[string]any{"app_id": appID, "name": p.Name, "version": p.Version, "files": files,
		"shortcuts": []string{}, "sha256": map[string]string{"notes.txt": "e3b0"}, "installer": "Generic Company Wizard Maker 0.1.0"})
	os.WriteFile(filepath.Join(p.InstallDir, Marker), b, 0o644)
	os.WriteFile(filepath.Join(p.InstallDir, "notes.txt"), []byte("changed by the user"), 0o644)
}

// TestWizardInstallIsTheUninstallersToRemove pins when Wizard names an uninstaller: not for no
// install, not for the program's own install, not for another app id, only for a wizard-made
// install of this program.
func TestWizardInstallIsTheUninstallersToRemove(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	if Wizard(p) != "" {
		t.Fatal("no install yet")
	}
	Install(p, (&recorder{}).run)
	if Wizard(p) != "" {
		t.Fatal("the program's own install is not wizard-made")
	}
	wizardInstall(t, p, "GenericCompany.Other")
	if Wizard(p) != "" {
		t.Fatal("another program's uninstaller would run")
	}
	wizardInstall(t, p, "GenericCompany.Test")
	if Wizard(p) != filepath.Join(p.InstallDir, "uninstall.exe") {
		t.Fatalf("uninstaller %q", Wizard(p))
	}
}

// TestUninstallArgs pins no flags by default, and the order -quiet, -purge, -root with an
// absolute root.
func TestUninstallArgs(t *testing.T) {
	if a := UninstallArgs(false, false, ""); len(a) != 0 {
		t.Fatalf("%q", a)
	}
	abs, _ := filepath.Abs("sb")
	if a := strings.Join(UninstallArgs(true, true, "sb"), "|"); a != "-quiet|-purge|-root|"+abs {
		t.Fatalf("%q", a)
	}
}

// TestRunUninstallerKeepsChangedFiles pins that RunUninstaller only runs uninstall.exe with the
// given flags and passes its code and words back (4, "already running"), touching no file
// itself; a silent cancel gets "Uninstall cancelled.".
func TestRunUninstallerKeepsChangedFiles(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	wizardInstall(t, p, "GenericCompany.Test")
	var got []string
	fake := func(exe string, args []string) (int, string, error) {
		got = append([]string{exe}, args...)
		return 4, "Setup or uninstall of Generic Company Test App is already running.", nil
	}
	code, said, err := RunUninstaller(p, prog.Exe, UninstallArgs(true, false, root), fake)
	if err != nil || code != 4 || !strings.Contains(said, "already running") {
		t.Fatalf("%d %q %v", code, said, err)
	}
	abs, _ := filepath.Abs(root)
	if strings.Join(got, "|") != filepath.Join(p.InstallDir, "uninstall.exe")+"|-quiet|-root|"+abs {
		t.Fatalf("ran %q", got)
	}
	if b, _ := os.ReadFile(filepath.Join(p.InstallDir, "notes.txt")); string(b) != "changed by the user" {
		t.Fatal("the changed file was touched")
	}
	cancel := func(string, []string) (int, string, error) { return Cancelled, "", nil }
	if code, said, _ := RunUninstaller(p, prog.Exe, nil, cancel); code != Cancelled || said != "Uninstall cancelled." {
		t.Fatalf("cancel: %d %q", code, said)
	}
}

// TestRunUninstallerRefusesFromTheFolderItRemoves pins two refusals that never run anything: the
// running exe lies in the folder (given in upper case, as Windows paths ignore case), and the
// folder holds no wizard-made install.
func TestRunUninstallerRefusesFromTheFolderItRemoves(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	wizardInstall(t, p, "GenericCompany.Test")
	ran := false
	fake := func(string, []string) (int, string, error) { ran = true; return 0, "", nil }
	if _, _, err := RunUninstaller(p, strings.ToUpper(p.ExeTarget), nil, fake); err == nil || ran {
		t.Fatalf("ran the uninstaller from inside: %v", err)
	}
	if _, _, err := RunUninstaller(MakePlan(prog, noEnv, filepath.Join(root, "x"), false, 1, day), prog.Exe, nil, fake); err == nil || ran {
		t.Fatal("ran an uninstaller that is not there")
	}
}

// entry writes the sandbox uninstall entry with InstallLocation dir, as a setup would.
func entry(t *testing.T, p Plan, dir string) {
	t.Helper()
	if err := editRegistryFile(p.RegistryFile, func(d map[string]any) { d[p.RegistryKey] = map[string]any{"InstallLocation": dir} }); err != nil {
		t.Fatal(err)
	}
}

// TestLocatedFindsAWizardInstallElsewhere walks a sandbox through no entry, a stale entry and a
// wizard install in another folder, which Located follows and RunUninstaller then runs. The
// Reader fails the test if called: a sandbox must use registry.json.
func TestLocatedFindsAWizardInstallElsewhere(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	none := func(string) (string, error) { t.Fatal("read the real registry in a sandbox"); return "", nil }
	if got, err := Located(p, none); err != nil || got.InstallDir != p.InstallDir {
		t.Fatalf("no entry: %q %v", got.InstallDir, err)
	}
	elsewhere := p
	elsewhere.InstallDir = filepath.Join(root, "Apps", "Journal")
	entry(t, p, elsewhere.InstallDir)
	if got, err := Located(p, none); err != nil || got.InstallDir != p.InstallDir {
		t.Fatalf("a stale entry: %q %v", got.InstallDir, err)
	}
	wizardInstall(t, elsewhere, "GenericCompany.Test")
	got, err := Located(p, none)
	if err != nil || got.InstallDir != elsewhere.InstallDir || Wizard(got) != filepath.Join(elsewhere.InstallDir, "uninstall.exe") {
		t.Fatalf("%q %v", got.InstallDir, err)
	}
	var ran []string
	fake := func(exe string, args []string) (int, string, error) {
		ran = append([]string{exe}, args...)
		return 0, "", nil
	}
	if code, _, err := RunUninstaller(got, prog.Exe, UninstallArgs(true, false, root), fake); err != nil || code != 0 || ran[0] != Wizard(got) {
		t.Fatalf("%d %v %q", code, err, ran)
	}
}

// TestLocatedNeverClearsAnInstallElsewhere pins the refusal for a folder elsewhere that holds
// another app id's install or no marker at all, and that the entry is left in place.
func TestLocatedNeverClearsAnInstallElsewhere(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	elsewhere := p
	elsewhere.InstallDir = filepath.Join(root, "Apps", "Journal")
	entry(t, p, elsewhere.InstallDir)
	none := func(string) (string, error) { return "", nil }
	for _, id := range []string{"GenericCompany.Other", ""} {
		wizardInstall(t, elsewhere, id)
		if id == "" {
			os.Remove(filepath.Join(elsewhere.InstallDir, Marker))
		}
		if got, err := Located(p, none); err == nil || got.InstallDir != p.InstallDir {
			t.Fatalf("%q: %q %v", id, got.InstallDir, err)
		}
	}
	if b, _ := os.ReadFile(p.RegistryFile); !strings.Contains(string(b), "InstallLocation") {
		t.Fatal("the entry was cleared")
	}
}

// TestLocatedReadsTheRealRegistry pins, with a fake Reader, the script used outside a sandbox,
// that a read error is passed on, and that a relative InstallLocation is ignored.
func TestLocatedReadsTheRealRegistry(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, "", false, 1, day)
	dir := filepath.Join(root, "Apps", "Journal")
	var script string
	read := func(s string) (string, error) { script = s; return dir, nil }
	wizardInstall(t, Plan{InstallDir: dir, Name: p.Name, Version: p.Version}, "GenericCompany.Test")
	got, err := Located(p, read)
	if err != nil || got.InstallDir != dir {
		t.Fatalf("%q %v", got.InstallDir, err)
	}
	if !strings.Contains(script, `Get-ItemProperty -LiteralPath 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\GenericCompany.Test' -Name InstallLocation`) {
		t.Fatalf("script %s", script)
	}
	if _, err := Located(p, func(string) (string, error) { return "", os.ErrPermission }); err == nil {
		t.Fatal("a failed read was ignored")
	}
	if got, _ := Located(p, func(string) (string, error) { return "Apps", nil }); got.InstallDir != p.InstallDir {
		t.Fatal("followed a relative InstallLocation")
	}
}

// TestRunAndWaitReturnsTheExitCode pins that exit code 4 and the output come back without an
// error, and that a missing exe is an error.
func TestRunAndWaitReturnsTheExitCode(t *testing.T) {
	code, said, err := RunAndWait("cmd.exe", []string{"/d", "/c", "echo kept & exit 4"})
	if err != nil || code != 4 || said != "kept" {
		t.Fatalf("%d %q %v", code, said, err)
	}
	if _, _, err := RunAndWait(filepath.Join(t.TempDir(), "uninstall.exe"), nil); err == nil {
		t.Fatal("a missing exe ran")
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=c75257bee28162c5d4f9cf05c3591363b8af99c359e7eb6bb5d64909b2129af2 ====
