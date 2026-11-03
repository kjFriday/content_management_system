// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/plan_test.go
// About: Tests for plan.go: MakePlan and Locations.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Tests for plan.go: MakePlan and Locations.

package install

import (
	"path/filepath"
	"strings"
	"testing"
)

func value(p Plan, name string) Value {
	for _, v := range p.Registry {
		if v.Name == name {
			return v
		}
	}
	return Value{}
}

func TestPlan(t *testing.T) {
	prog, root := setup(t)
	p := MakePlan(prog, noEnv, root, true, 4096, day)
	if p.InstallDir != filepath.Join(root, "Programs", "Generic Company Inc", "Generic Company Test App") {
		t.Fatalf("install dir %q", p.InstallDir)
	}
	if len(p.Shortcuts) != 2 || p.Shortcuts[0].Target != p.ExeTarget || !strings.HasSuffix(p.Shortcuts[0].Icon, "app.ico") {
		t.Fatalf("shortcuts %+v", p.Shortcuts)
	}
	if v := value(p, "DisplayVersion"); v.Text != "0.2.0" {
		t.Fatalf("version %+v", v)
	}
	if v := value(p, "InstallDate"); v.Text != "20261017" {
		t.Fatalf("date %+v", v)
	}
	if v := value(p, "NoModify"); !v.IsDWord || v.Number != 1 {
		t.Fatalf("NoModify %+v", v)
	}
	if v := value(p, "EstimatedSize"); v.Number != 5 {
		t.Fatalf("size %+v", v)
	}
	if u := value(p, "UninstallString").Text; !strings.Contains(u, " -uninstall -root ") {
		t.Fatalf("uninstall %q", u)
	}
	if p.RegistryKey != UninstallKey+`\GenericCompany.Test` {
		t.Fatalf("key %q", p.RegistryKey)
	}
}

func TestRealLocations(t *testing.T) {
	env := map[string]string{"LOCALAPPDATA": `C:\L`, "APPDATA": `C:\R`, "USERPROFILE": `C:\U`}
	programs, start, _, reg := Locations(func(k string) string { return env[k] }, "")
	if programs != `C:\L\Programs\Generic Company Inc` || start != `C:\R\Microsoft\Windows\Start Menu\Programs\Generic Company Inc` || reg != "" {
		t.Fatalf("%q %q %q", programs, start, reg)
	}
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=e3741482a8803e36b70d7308a6ce5de3ca2593e3b105de46ea5ef427d253dec2 ====
