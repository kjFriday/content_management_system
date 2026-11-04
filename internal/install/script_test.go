// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/script_test.go
// About: Tests for script.go: the PowerShell that Script writes, and PowerShellOutput.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for script.go: the PowerShell that Script writes, and PowerShellOutput.

package install

import (
	"strings"
	"testing"
)

// TestScriptQuotingAndRegistry pins the quoting with a folder named O'Brien, the DWord and String
// value types and the real HKCU key; the removal script must remove the key and make no shortcut.
func TestScriptQuotingAndRegistry(t *testing.T) {
	prog, _ := setup(t)
	env := map[string]string{"LOCALAPPDATA": `C:\O'Brien`, "APPDATA": `C:\R`}
	p := MakePlan(prog, func(k string) string { return env[k] }, "", false, 1, day)
	s := Script(p, false)
	for _, want := range []string{
		`'C:\O''Brien\Programs\Generic Company Inc\Generic Company Test App\app.ico,0'`,
		`-Name 'NoModify' -Value '1' -PropertyType DWord`,
		`-Name 'DisplayVersion' -Value '0.2.0' -PropertyType String`,
		`New-Item -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\GenericCompany.Test' -Force`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script lacks %s", want)
		}
	}
	r := Script(p, true)
	if !strings.Contains(r, "Remove-Item -LiteralPath 'HKCU:") || strings.Contains(r, "CreateShortcut") {
		t.Fatal("removal script is wrong")
	}
}

// TestSandboxNeverTouchesRegistry pins that neither sandbox script mentions HKCU.
func TestSandboxNeverTouchesRegistry(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, false, 1, day)
	if strings.Contains(Script(p, false), "HKCU") || strings.Contains(Script(p, true), "HKCU") {
		t.Fatal("sandbox script writes the registry")
	}
}

// TestPowerShellOutput runs the real PowerShell (skipped with -short): a non-ASCII character
// (U+00F6) survives the round trip as UTF-8, and a missing uninstall key reads as "", not an
// error.
func TestPowerShellOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("runs PowerShell")
	}
	if out, err := PowerShellOutput("[Console]::OutputEncoding = [Text.Encoding]::UTF8\n'M' + [char]0xF6 + 'tley'\n"); err != nil || out != "Mötley" {
		t.Fatalf("%q %v", out, err)
	}
	prog, _ := setup(t)
	p := MakePlan(prog, noEnv, "", false, 1, day)
	p.RegistryKey = UninstallKey + `\GenericCompany.NoSuchProgram.Test`
	if loc, err := InstallLocation(p, PowerShellOutput); err != nil || loc != "" {
		t.Fatalf("%q %v", loc, err)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=3d1b1dcb04d1303fbb610d7bbf049734cbacae6c680090ddedfae8911dd5d772 ====
