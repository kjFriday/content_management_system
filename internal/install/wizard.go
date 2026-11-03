// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/wizard.go
// About: Hand-off of a Wizard Maker install to its own uninstall.exe, wherever it was put.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Hand-off of a Wizard Maker install to its own uninstall.exe, wherever it was put.

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
type Exec func(exe string, args []string) (int, string, error)

// RunAndWait is the real Exec. The program starts in the temp folder, so it never holds the folder it removes.
func RunAndWait(exe string, args []string) (int, string, error) {
	cmd := exec.Command(exe, args...)
	cmd.Dir = os.TempDir()
	out, err := cmd.CombinedOutput()
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
	if rel, err := filepath.Rel(p.InstallDir, self); err == nil && filepath.IsLocal(rel) {
		return 0, "", fmt.Errorf("this copy runs from %s, which its uninstaller removes: uninstall %s from Apps & features, or run %s",
			p.InstallDir, p.Name, exe)
	}
	code, said, err := run(exe, args)
	if err != nil {
		return 0, "", fmt.Errorf("run %s: %w", exe, err)
	}
	if said == "" && code == Cancelled {
		said = "Uninstall cancelled."
	} else if said == "" && code != 0 {
		said = fmt.Sprintf("%s exited with %d", WizardUninstaller, code)
	}
	return code, said, nil
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=acb8e74412c433b2a395f83e12a13b691e531943d2258dd43299dfcc6f1e40cb ====
