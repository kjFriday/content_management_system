// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/install.go
// About: Package install installs or uninstalls a Generic Company program for the current user.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package install installs or uninstalls a Generic Company program for the current user.
//
// It needs no admin rights: everything goes under the user's own folders and HKCU (the current
// user's half of the registry), never Program Files or HKLM.
//
// Install copies the exe and its icon to %LOCALAPPDATA%\Programs\Generic Company Inc\<name>\ with an install.json
// marker, adds a Start-menu shortcut (optionally a desktop one) and an uninstall entry under
// HKCU\...\CurrentVersion\Uninstall\<app id>, so it shows in Apps & features. Uninstall removes all
// three and never touches the shared database. An install that a setup made by Generic Company Wizard Maker left
// (its marker names the installer), here or in the folder its uninstall entry names, is removed by its
// own uninstall.exe instead, which keeps the files changed since: see Located and Wizard.
//
// Three layers, so tests never touch the real system: Plan (pure), Script (pure PowerShell, fed on
// stdin) and Install/Uninstall with a replaceable Runner. With a sandbox root everything lands under
// it and the registry becomes root\registry.json.
//
// Each layer has its file: plan.go, script.go (with the real Runner and Reader) and install.go (with
// the marker and DeleteAfterExit). wizard.go holds the Wizard Maker hand-off, and hide_windows.go the
// Windows-only process flags (hide_other.go stands in on other systems).
//
// A typical caller (the program's -install and -uninstall flags):
//  1. MakePlan from a Program, the environment and the sandbox root.
//  2. To install: Install(plan, PowerShell).
//  3. To uninstall: Located aims the plan at the folder the uninstall entry names; if Wizard
//     finds a Wizard Maker install there, RunUninstaller hands it over. Otherwise Uninstall, and
//     if it returns a folder, DeleteAfterExit as the process's last action.
//
// Types: Program, Plan, Shortcut, Value (plan.go); Runner, Reader (script.go); Exec (wizard.go);
// marker (this file). Start reading at MakePlan.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Marker is the file in the install folder that records what was installed there.
const Marker = "install.json"

// marker is the content of install.json. It proves which program owns a folder before anything
// in it is overwritten or deleted, and lists the shortcuts to remove even if the plan changed
// (say, a desktop shortcut added at install but not asked for at uninstall).
type marker struct {
	AppID     string   `json:"app_id"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Files     []string `json:"files"`               // file names in the folder, relative to it
	Shortcuts []string `json:"shortcuts"`           // full paths of the .lnk files
	Installer string   `json:"installer,omitempty"` // written by Generic Company Wizard Maker setups, with a sha256 map
}

// readMarker reads dir's install.json. ok is false when the file is missing or not valid JSON;
// the marker is then whatever was decoded, usually empty.
func readMarker(dir string) (marker, bool) {
	var m marker
	b, err := os.ReadFile(filepath.Join(dir, Marker))
	return m, err == nil && json.Unmarshal(b, &m) == nil
}

// registryValues is the plan's registry values as a name -> value map, the shape a sandbox's
// registry.json stores per key (DWORDs as numbers, strings as strings).
func registryValues(p Plan) map[string]any {
	out := map[string]any{}
	for _, v := range p.Registry {
		if v.IsDWord {
			out[v.Name] = v.Number
		} else {
			out[v.Name] = v.Text
		}
	}
	return out
}

// editRegistryFile is the sandbox's stand-in for the registry: it loads the JSON object at path
// (registry key -> values; missing or broken reads as empty), lets edit change it and writes it
// back, creating the folder if needed.
func editRegistryFile(path string, edit func(map[string]any)) error {
	data := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &data)
	}
	edit(data)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

// copyFile copies src to dst whole, through memory (an exe of a few MB). Fails if dst is in use,
// as a running exe is on Windows.
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

// Install copies the exe and icon, writes the marker, then the shortcuts and registry entry.
// Reinstalling over the same program is fine; a folder owned by another program is refused.
//
// Steps:
//  1. Refuse if the folder's marker names another app id.
//  2. Create the folder and copy the exe, unless it already runs from there (a reinstall from
//     the installed copy); the paths are compared case-insensitively, as Windows does.
//  3. Write app.ico and the marker.
//  4. In a sandbox, record the registry values in registry.json.
//  5. Run the script that makes the shortcuts and, outside a sandbox, the uninstall entry.
//
// A failure part way leaves what was done so far; running Install again finishes it.
func Install(p Plan, run Runner) error {
	if m, ok := readMarker(p.InstallDir); ok && m.AppID != p.AppID {
		return fmt.Errorf("%s belongs to %s", p.InstallDir, m.AppID)
	}
	if err := os.MkdirAll(p.InstallDir, 0o755); err != nil {
		return err
	}
	same, _ := filepath.Abs(p.ExeSource)
	if !strings.EqualFold(same, p.ExeTarget) {
		if err := copyFile(p.ExeSource, p.ExeTarget); err != nil {
			return fmt.Errorf("copy %s: %w (is it running?)", filepath.Base(p.ExeTarget), err)
		}
	}
	if err := os.WriteFile(p.IconTarget, p.Icon, 0o644); err != nil {
		return err
	}
	var links []string
	for _, s := range p.Shortcuts {
		links = append(links, s.Path)
	}
	m := marker{AppID: p.AppID, Name: p.Name, Version: p.Version,
		Files: []string{filepath.Base(p.ExeTarget), "app.ico"}, Shortcuts: links}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(p.InstallDir, Marker), b, 0o644); err != nil {
		return err
	}
	if p.RegistryFile != "" {
		if err := editRegistryFile(p.RegistryFile, func(d map[string]any) { d[p.RegistryKey] = registryValues(p) }); err != nil {
			return err
		}
	}
	return run(Script(p, false))
}

// SafeToRemove reports whether InstallDir is under ProgramsDir and its marker names this program.
// Both checks guard the recursive delete in Uninstall against a wrong path.
func SafeToRemove(p Plan) bool {
	dir, _ := filepath.Abs(p.InstallDir)
	programs, _ := filepath.Abs(p.ProgramsDir)
	// "." is ProgramsDir itself and ".." a way out of it: neither is a program's own folder.
	// Rel fails when the two are on different drives.
	// Known limitation: a sibling folder whose name starts with "..", such as "..x", is refused too.
	rel, err := filepath.Rel(programs, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return false
	}
	m, ok := readMarker(dir)
	return ok && m.AppID == p.AppID
}

// Uninstall removes shortcuts, registry entry and install folder. When the running exe lives in that
// folder it removes the rest and returns the folder as later: the caller passes it to DeleteAfterExit
// as its very last action, after any message box. purge, when set, is this program's data folder
// (never the shared database). Returns whether a folder was removed.
//
// running is the path of the exe doing the uninstall (os.Executable).
//
// Steps:
//  1. Add the shortcuts the marker lists that the plan lacks, so all of them go.
//  2. Run the removal script: shortcuts, an emptied Start-menu folder and, outside a sandbox,
//     the uninstall entry. These go even when the folder is already gone, which clears a broken
//     install.
//  3. In a sandbox, drop the key from registry.json. Then purge the data folder if asked
//     (errors ignored: the program's data is optional to remove).
//  4. No folder: report removed=false. A folder that SafeToRemove rejects: refuse with an error.
//  5. If running lives in the folder, delete everything else and return the folder as later.
//     Otherwise delete the whole folder.
func Uninstall(p Plan, run Runner, running string, purge string) (removed bool, later string, err error) {
	m, _ := readMarker(p.InstallDir)
	known := map[string]bool{}
	for _, s := range p.Shortcuts {
		known[s.Path] = true
	}
	for _, extra := range m.Shortcuts {
		if !known[extra] {
			p.Shortcuts = append(p.Shortcuts, Shortcut{Path: extra})
		}
	}
	if err := run(Script(p, true)); err != nil {
		return false, "", err
	}
	if p.RegistryFile != "" {
		if err := editRegistryFile(p.RegistryFile, func(d map[string]any) { delete(d, p.RegistryKey) }); err != nil {
			return false, "", err
		}
	}
	if purge != "" {
		_ = os.RemoveAll(purge)
	}
	if _, err := os.Stat(p.InstallDir); errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if !SafeToRemove(p) {
		return false, "", fmt.Errorf("refusing to delete %s: no matching %s", p.InstallDir, Marker)
	}
	exe, _ := filepath.Abs(running)
	if strings.EqualFold(filepath.Dir(exe), p.InstallDir) {
		// this exe can't delete itself while it runs: remove the rest now, the folder once it exits
		entries, _ := os.ReadDir(p.InstallDir)
		names := []string{}
		for _, e := range entries {
			if !strings.EqualFold(e.Name(), filepath.Base(exe)) {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			_ = os.RemoveAll(filepath.Join(p.InstallDir, n))
		}
		return true, p.InstallDir, nil
	}
	return true, "", os.RemoveAll(p.InstallDir)
}

// DeleteAfterExit starts a hidden cmd that waits a moment, then removes dir (retrying for about 4
// seconds). Start it as this process's very last action: it gives up while the exe still runs.
//
// It returns once cmd has started, without waiting. Windows locks a running exe's file, so the
// exe cannot delete its own folder; a separate process outlives it and does the deleting.
func DeleteAfterExit(dir string) error {
	// rd /s removes the folder and everything in it; /q skips the "are you sure" prompt
	rd := fmt.Sprintf(`rd /s /q "%s"`, dir)
	// cmd has no sleep command, so "ping -n N 127.0.0.1" stands in: N echo requests about a
	// second apart, so about N-1 seconds. the schedule is wait ~2 s, try, wait ~1 s, try, wait
	// ~1 s, try. /d skips cmd's AutoRun registry commands; the "&" runs each step whatever the
	// last one did
	line := fmt.Sprintf(`cmd.exe /d /c "ping -n 3 127.0.0.1 >nul & %s & ping -n 2 127.0.0.1 >nul & %s & ping -n 2 127.0.0.1 >nul & %s"`, rd, rd, rd)
	cmd := exec.Command("cmd.exe")
	hide(cmd, line)
	// start in the temp folder: a process's working folder cannot be deleted while it runs there
	cmd.Dir = os.TempDir()
	return cmd.Start()
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=488dea020f9ba8af9b18d93369a65b7862d87f9a4df229ef5c2756338c5d682d ====
