// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/plan.go
// About: Layer one: MakePlan works out, as plain data, where an install goes and what it adds.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Layer one: MakePlan works out, as plain data, where an install goes and what it adds.

package install

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UninstallKey holds the current user's uninstall entries, the ones Apps & features lists.
const UninstallKey = `HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall`

// Program is what gets installed.
type Program struct {
	AppID, Name, Version, Publisher string
	Exe                             string // the running exe, copied as-is
	ExeName                         string // its name once installed, e.g. generic-company-cms.exe
	Icon                            []byte // app.ico, written beside the exe
}

// Shortcut is one .lnk file.
type Shortcut struct {
	Path, Target, Args, WorkDir, Icon, Description string
}

// Value is one registry value: a string, or a DWORD when IsDWord.
type Value struct {
	Name    string
	Text    string
	Number  int
	IsDWord bool
}

// Plan is everything Install and Uninstall will do, as plain data.
type Plan struct {
	AppID, Name, Version      string
	InstallDir, ProgramsDir   string
	ExeSource, ExeTarget      string
	IconTarget                string
	Icon                      []byte
	Shortcuts                 []Shortcut
	RegistryKey, RegistryFile string // RegistryFile set only in a sandbox
	Registry                  []Value
}

// Locations returns the programs, Start-menu and desktop folders, and the sandbox registry file ("" for real).
func Locations(env func(string) string, root string) (programs, startMenu, desktop, registry string) {
	if root != "" {
		root, _ = filepath.Abs(root)
		return filepath.Join(root, "Programs", "Generic Company Inc"), filepath.Join(root, "Start Menu", "Generic Company Inc"),
			filepath.Join(root, "Desktop"), filepath.Join(root, "registry.json")
	}
	home, _ := os.UserHomeDir()
	or := func(k, def string) string {
		if v := env(k); v != "" {
			return v
		}
		return def
	}
	local := or("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	roaming := or("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	return filepath.Join(local, "Programs", "Generic Company Inc"),
		filepath.Join(roaming, "Microsoft", "Windows", "Start Menu", "Programs", "Generic Company Inc"),
		filepath.Join(or("USERPROFILE", home), "Desktop"), ""
}

func quote(s string) string {
	if strings.ContainsAny(s, " \t") && !strings.HasPrefix(s, `"`) {
		return `"` + s + `"`
	}
	return s
}

// MakePlan computes the plan. today is passed in so tests are stable.
func MakePlan(p Program, env func(string) string, root string, desktop bool, size int64, today time.Time) Plan {
	programs, startMenu, desk, registry := Locations(env, root)
	dir := filepath.Join(programs, p.Name)
	exe := filepath.Join(dir, p.ExeName)
	icon := filepath.Join(dir, "app.ico")
	uninstall := quote(exe) + " -uninstall"
	if root != "" {
		abs, _ := filepath.Abs(root)
		uninstall += " -root " + quote(abs)
	}
	links := []string{filepath.Join(startMenu, p.Name+".lnk")}
	if desktop {
		links = append(links, filepath.Join(desk, p.Name+".lnk"))
	}
	plan := Plan{AppID: p.AppID, Name: p.Name, Version: p.Version, InstallDir: dir, ProgramsDir: programs,
		ExeSource: p.Exe, ExeTarget: exe, IconTarget: icon, Icon: p.Icon,
		RegistryKey: UninstallKey + `\` + p.AppID, RegistryFile: registry}
	for _, l := range links {
		plan.Shortcuts = append(plan.Shortcuts, Shortcut{Path: l, Target: exe, WorkDir: dir, Icon: icon,
			Description: p.Name + " " + p.Version})
	}
	plan.Registry = []Value{
		{Name: "DisplayName", Text: p.Name + " " + p.Version},
		{Name: "DisplayVersion", Text: p.Version},
		{Name: "Publisher", Text: p.Publisher},
		{Name: "DisplayIcon", Text: icon},
		{Name: "InstallLocation", Text: dir},
		{Name: "UninstallString", Text: uninstall},
		{Name: "QuietUninstallString", Text: uninstall + " -quiet"},
		{Name: "InstallDate", Text: today.Format("20060102")},
		{Name: "NoModify", Number: 1, IsDWord: true},
		{Name: "NoRepair", Number: 1, IsDWord: true},
		{Name: "EstimatedSize", Number: int(size/1024) + 1, IsDWord: true},
	}
	return plan
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=4284f2a5e80981e71917fe668c5f009db9d98083c549d1fe6f0e91f4271aadc0 ====
