// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/wizard.go
// About: Hand-off of a Wizard Maker install to its own uninstall.exe, wherever it was put.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Hand-off of a Wizard Maker install to its own uninstall.exe, wherever it was put.
//
// The same program can be installed two ways: by its own -install (install.go), or by a setup
// that Generic Company Wizard Maker built from it. The setup lets the user pick the folder, adds
// an "installer" key and a sha256 map to install.json, and leaves uninstall.exe beside the
// program. That uninstaller removes only files whose hash still matches, so a file the user
// changed survives; -uninstall therefore hands such an install over rather than deleting the
// folder itself.
//
// Flow for -uninstall:
//  1. Located reads the uninstall entry's InstallLocation and aims the plan there.
//  2. Wizard says whether that folder holds a wizard-made install of this program.
//  3. RunUninstaller runs its uninstall.exe with UninstallArgs, through an Exec (RunAndWait).

package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The Wizard Maker uninstaller's file name and the exit code it uses for a cancel.
const (
	// WizardUninstaller is what a setup made by Generic Company Wizard Maker leaves in the install folder.
	WizardUninstaller = "uninstall.exe"
	// Cancelled is that uninstaller's exit code when the user cancels it in its window.
	Cancelled = 3
)

// Wizard is the uninstaller of an install that a setup made by Generic Company Wizard Maker left in p.InstallDir
// (its marker names this program and an installer), or "" for this program's own install. Such an
// install is removed by that uninstaller, which takes away only the files still as installed.
func Wizard(p Plan) string {
	m, ok := readMarker(p.InstallDir)
	if !ok || m.AppID != p.AppID || m.Installer == "" {
		return ""
	}
	return filepath.Join(p.InstallDir, WizardUninstaller)
}

// InstallLocation is the folder the uninstall entry names, or "": from root\registry.json in a sandbox,
// else read from the registry through PowerShell.
//
// A missing key or value reads as "" without an error (SilentlyContinue); only a failed
// PowerShell run is an error. The script switches the output to UTF-8 so a folder name with
// non-ASCII letters comes back intact.
func InstallLocation(p Plan, read Reader) (string, error) {
	if p.RegistryFile != "" {
		data := map[string]map[string]any{}
		if b, err := os.ReadFile(p.RegistryFile); err == nil {
			_ = json.Unmarshal(b, &data)
		}
		loc, _ := data[p.RegistryKey]["InstallLocation"].(string)
		return loc, nil
	}
	return read(fmt.Sprintf("[Console]::OutputEncoding = [Text.Encoding]::UTF8\n"+
		"(Get-ItemProperty -LiteralPath %s -Name InstallLocation -ErrorAction SilentlyContinue).InstallLocation\n",
		Literal(p.RegistryKey)))
}

// Located is p aimed at the install its uninstall entry names when that lies outside p.InstallDir: a
// setup made by Generic Company Wizard Maker installs where the user picks. Only such an install (see Wizard) is
// handed on; any other folder still there is refused, so its entry and shortcuts are never cleared.
//
// Steps:
//  1. Read InstallLocation. A read error is returned with p unchanged; an empty or relative
//     location is ignored (it cannot be trusted to name a folder).
//  2. The same folder as the plan's (compared case-insensitively): nothing to change.
//  3. A folder that no longer exists: a stale entry, so keep the plan and let Uninstall clear it.
//  4. A wizard-made install of this program there: the plan, aimed at that folder.
//  5. Anything else there: an error telling the user to use Apps & features.
func Located(p Plan, read Reader) (Plan, error) {
	loc, err := InstallLocation(p, read)
	if err != nil || !filepath.IsAbs(loc) {
		return p, err
	}
	loc = filepath.Clean(loc)
	if strings.EqualFold(loc, filepath.Clean(p.InstallDir)) {
		return p, nil
	}
	if _, err := os.Stat(loc); errors.Is(err, os.ErrNotExist) {
		return p, nil // a stale entry: clearing it orphans nothing
	}
	there := p
	there.InstallDir = loc
	if Wizard(there) == "" {
		return p, fmt.Errorf("%s is installed in %s, which this copy cannot uninstall: use Apps & features", p.Name, loc)
	}
	return there, nil
}

// UninstallArgs are the flags -uninstall passes on to that uninstaller.
// -quiet and -purge pass through as they are; -root is made absolute, since the uninstaller runs
// from the temp folder (see RunAndWait).
func UninstallArgs(quiet, purge bool, root string) []string {
	args := []string{}
	if quiet {
		args = append(args, "-quiet")
	}
	if purge {
		args = append(args, "-purge")
	}
	if root != "" {
		abs, _ := filepath.Abs(root)
		args = append(args, "-root", abs)
	}
	return args
}

// Exec runs a program and waits for it: -> its exit code and what it printed.
// The error is for a program that could not run at all; a non-zero exit is not an error.
type Exec func(exe string, args []string) (int, string, error)

// RunAndWait is the real Exec. The program starts in the temp folder, so it never holds the folder it removes.
// stdout and stderr come back together, trimmed. Unlike PowerShellOutput it does not hide a
// window: the uninstaller shows its own unless it was given -quiet.
func RunAndWait(exe string, args []string) (int, string, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
	// an ExitError means the program ran and exited non-zero: that is a result, not a failure
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), strings.TrimSpace(string(out)), nil
	}
	return 0, strings.TrimSpace(string(out)), err
}

// RunUninstaller removes a wizard-made install (see Wizard) through its uninstaller, run with args;
// -> its exit code (0 done, Cancelled, ...) and how it went. It refuses while running lies in the
// install folder: Windows cannot delete a running exe, so the uninstaller would stop half-way.
func RunUninstaller(p Plan, running string, args []string, run Exec) (int, string, error) {
	exe := Wizard(p)
	if exe == "" {
		return 0, "", fmt.Errorf("%s holds no wizard-made install of %s", p.InstallDir, p.Name)
	}
	self, _ := filepath.Abs(running)
	// IsLocal is true for a relative path that stays inside its base (no "..", not absolute), so
	// this asks whether running lies anywhere under InstallDir. On Windows Rel ignores letter
	// case, as the file system does
	if rel, err := filepath.Rel(p.InstallDir, self); err == nil && filepath.IsLocal(rel) {
		return 0, "", fmt.Errorf("this copy runs from %s, which its uninstaller removes: uninstall %s from Apps & features, or run %s",
			p.InstallDir, p.Name, exe)
	}
	code, said, err := run(exe, args)
	if err != nil {
		return 0, "", fmt.Errorf("run %s: %w", exe, err)
	}
	// a silent exit still gets words, so the caller always has something to tell the user
	if said == "" && code == Cancelled {
		said = "Uninstall cancelled."
	} else if said == "" && code != 0 {
		said = fmt.Sprintf("%s exited with %d", WizardUninstaller, code)
	}
	return code, said, nil
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=2d8f87a3cd3c89f0b7a7889108ea57ada1585dae803bbdd8bd99895dc9ded996 ====
