// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/plan.go
// About: Layer one: MakePlan works out, as plain data, where an install goes and what it adds.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Layer one: MakePlan works out, as plain data, where an install goes and what it adds.
//
// Nothing here touches the disk or the registry, so tests can check every path and value
// directly. Script (script.go) turns a Plan into PowerShell; Install and Uninstall (install.go)
// carry it out.
//
// The uninstall entry follows the Windows convention for Apps & features: one key per program
// under UninstallKey, with values such as DisplayName, UninstallString and EstimatedSize (in KB).

package install

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UninstallKey holds the current user's uninstall entries, the ones Apps & features lists.
// It is written in PowerShell's registry-drive form (HKCU:\...), as Script uses it.
const UninstallKey = `HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall`

// Program is what gets installed.
type Program struct {
	AppID, Name, Version, Publisher string
	Exe                             string // the running exe, copied as-is
	ExeName                         string // its name once installed, e.g. generic-company-cms.exe
	Icon                            []byte // app.ico, written beside the exe
}

// Shortcut is one .lnk file. Path is where the .lnk goes; Target, Args, WorkDir, Icon and
// Description become the shell link's TargetPath, Arguments, WorkingDirectory, IconLocation
// (first icon in the file) and tooltip.
type Shortcut struct {
	Path, Target, Args, WorkDir, Icon, Description string
}

// Value is one registry value: a string, or a DWORD when IsDWord.
// DWORD is the registry's 32-bit number type (REG_DWORD); Text is used otherwise.
type Value struct {
	Name    string
	Text    string
	Number  int
	IsDWord bool
}

// Plan is everything Install and Uninstall will do, as plain data.
// The paths are absolute in normal use.
type Plan struct {
	AppID, Name, Version      string
	InstallDir, ProgramsDir   string // the program's folder, and the shared parent it must lie under
	ExeSource, ExeTarget      string // the running exe, and where it is copied
	IconTarget                string
	Icon                      []byte
	Shortcuts                 []Shortcut
	RegistryKey, RegistryFile string // RegistryFile set only in a sandbox
	Registry                  []Value
}

// Locations returns the programs, Start-menu and desktop folders, and the sandbox registry file ("" for real).
//
// With a root (the -root sandbox) all four lie under it. Otherwise they are the per-user
// folders: %LOCALAPPDATA%\Programs (the usual place for per-user programs), the roaming Start
// menu and the user's desktop, each falling back to its usual place under the home folder when
// its variable is unset.
//
// Known limitation: the desktop is taken as %USERPROFILE%\Desktop, so a desktop moved elsewhere
// (for example into OneDrive) is not found.
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

// quote wraps s in double quotes when it holds a space or tab and is not quoted already, so a
// path in the uninstall command line stays one argument.
func quote(s string) string {
	if strings.ContainsAny(s, " \t") && !strings.HasPrefix(s, `"`) {
		return `"` + s + `"`
	}
	return s
}

// MakePlan computes the plan. today is passed in so tests are stable.
//
// env reads environment variables, root is the sandbox ("" for real), desktop adds a desktop
// shortcut and size is the exe's size in bytes.
//
// Steps:
//  1. Find the folders (Locations); the program goes in ProgramsDir\<Name>.
//  2. Build the uninstall command: the installed exe with -uninstall, plus -root in a sandbox so
//     the uninstall finds the same sandbox.
//  3. One Start-menu shortcut, and a desktop one when asked; all point at the installed exe.
//  4. The uninstall entry's values. NoModify and NoRepair hide those buttons in Apps & features;
//     InstallDate is yyyymmdd; EstimatedSize is in KB, rounded up by adding 1 (so an exact
//     multiple of 1024 bytes reads 1 KB high).
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=79845effadf8b78f54c69117f2bb532adae7ceae4214a0784430f076b01d0bc8 ====
