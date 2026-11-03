// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/install.go
// About: Package install installs or uninstalls a Generic Company program for the current user (no...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package install installs or uninstalls a Generic Company program for the current user (no admin rights).
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

type marker struct {
	AppID     string   `json:"app_id"`
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Files     []string `json:"files"`
	Shortcuts []string `json:"shortcuts"`
	Installer string   `json:"installer,omitempty"` // written by Generic Company Wizard Maker setups, with a sha256 map
}

func readMarker(dir string) (marker, bool) {
	var m marker
	b, err := os.ReadFile(filepath.Join(dir, Marker))
	return m, err == nil && json.Unmarshal(b, &m) == nil
}

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

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}

// Install copies the exe and icon, writes the marker, then the shortcuts and registry entry.
// Reinstalling over the same program is fine; a folder owned by another program is refused.
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
func SafeToRemove(p Plan) bool {
	dir, _ := filepath.Abs(p.InstallDir)
	programs, _ := filepath.Abs(p.ProgramsDir)
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
		// this exe can't delete itself while it runs: remove the rest now, the folder once we exit
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
func DeleteAfterExit(dir string) error {
	rd := fmt.Sprintf(`rd /s /q "%s"`, dir)
	line := fmt.Sprintf(`cmd.exe /d /c "ping -n 3 127.0.0.1 >nul & %s & ping -n 2 127.0.0.1 >nul & %s & ping -n 2 127.0.0.1 >nul & %s"`, rd, rd, rd)
	cmd := exec.Command("cmd.exe")
	hide(cmd, line)
	cmd.Dir = os.TempDir()
	return cmd.Start()
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=b10f75a7b4dcfb6a427c8c4a64220b6b6f61571165d773371ab810025d9ca6cf ====
