// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/script.go
// About: Layer two: Script writes a Plan's PowerShell; PowerShell and PowerShellOutput run it.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Layer two: Script writes a Plan's PowerShell; PowerShell and PowerShellOutput run it.
//
// PowerShell does the two jobs Go's standard library cannot: making .lnk shortcuts (through the
// WScript.Shell COM object, whose CreateShortcut returns a shell link to fill in and Save) and
// writing the registry (through PowerShell's HKCU: drive). Every value is put in as a Literal,
// and the script goes in on stdin, so a path with quotes or spaces cannot break out of its string.

package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Literal is a PowerShell single-quoted string: nothing inside is expanded; ' is doubled.
// So O'Brien becomes 'O”Brien', and $env:X or a backtick stays as typed.
//
// Known limitation: PowerShell also treats the typographic quotes U+2018 to U+201B as single
// quotes, and they are not doubled here.
func Literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Script is the PowerShell that creates (or removes) the shortcuts and, unless sandboxed, the registry entry.
//
// remove=false: create each shortcut's folder, then fill and save each shortcut; then create the
// uninstall key and write each value with its type (String or DWord). -Force overwrites what a
// previous install left.
//
// remove=true: delete each shortcut, then the first shortcut's folder (the Start-menu folder)
// only if it is now empty, since other programs may share it; then the uninstall key with its
// values. Missing items are not errors.
//
// The removal script assumes at least one shortcut (MakePlan always makes the Start-menu one).
func Script(p Plan, remove bool) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	// any failing command stops the script with an error, so the Runner reports it
	line("$ErrorActionPreference = 'Stop'")
	if remove {
		for _, s := range p.Shortcuts {
			line("Remove-Item -LiteralPath %s -Force -ErrorAction SilentlyContinue", Literal(s.Path))
		}
		line("$f = %s; if ((Test-Path -LiteralPath $f) -and -not (Get-ChildItem -LiteralPath $f)) { Remove-Item -LiteralPath $f }",
			Literal(filepath.Dir(p.Shortcuts[0].Path)))
		if p.RegistryFile == "" {
			line("Remove-Item -LiteralPath %s -Recurse -ErrorAction SilentlyContinue", Literal(p.RegistryKey))
		}
		return b.String()
	}
	line("$shell = New-Object -ComObject WScript.Shell")
	for _, s := range p.Shortcuts {
		line("New-Item -ItemType Directory -Force -Path %s | Out-Null", Literal(filepath.Dir(s.Path)))
		line("$l = $shell.CreateShortcut(%s)", Literal(s.Path))
		line("$l.TargetPath = %s", Literal(s.Target))
		line("$l.Arguments = %s", Literal(s.Args))
		line("$l.WorkingDirectory = %s", Literal(s.WorkDir))
		// "file,index": the icon at index 0 in the file
		line("$l.IconLocation = %s", Literal(s.Icon+",0"))
		line("$l.Description = %s", Literal(s.Description))
		line("$l.Save()")
	}
	if p.RegistryFile == "" {
		key := Literal(p.RegistryKey)
		line("New-Item -Path %s -Force | Out-Null", key)
		for _, v := range p.Registry {
			kind, val := "String", v.Text
			if v.IsDWord {
				kind, val = "DWord", fmt.Sprint(v.Number)
			}
			line("New-ItemProperty -Path %s -Name %s -Value %s -PropertyType %s -Force | Out-Null", key, Literal(v.Name), Literal(val), kind)
		}
	}
	return b.String()
}

// Runner runs a PowerShell script. Tests pass a recorder; the program passes PowerShell.
type Runner func(script string) error

// PowerShell is the real Runner: the script goes on stdin, so no quoting reaches the command line.
// It discards what the script prints.
func PowerShell(script string) error {
	_, err := PowerShellOutput(script)
	return err
}

// Reader runs a PowerShell script: -> what it printed.
type Reader func(script string) (string, error)

// PowerShellOutput is the real Reader, run like PowerShell.
//
// It returns stdout with surrounding white space trimmed. A non-zero exit becomes an error that
// carries both stdout and stderr, so the PowerShell message reaches the user. The process runs
// with no console window (hide).
//
// Text comes back in the console's code page unless the script first sets
// [Console]::OutputEncoding to UTF-8, as InstallLocation's script does.
func PowerShellOutput(script string) (string, error) {
	// Windows PowerShell 5.1, present on every supported Windows. -NoProfile skips the user's
	// profile scripts; -NonInteractive fails instead of prompting; -ExecutionPolicy Bypass lets
	// the script run whatever the machine's policy; "-Command -" reads the commands from stdin
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "-")
	cmd.Stdin = strings.NewReader(script)
	hide(cmd, "")
	var out, errs strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errs
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("PowerShell: %v: %s", err, strings.TrimSpace(out.String()+errs.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=4884513a87b286a1f2d137654ecc39c8780a7b05aec3d06c98e8fe90eb8c5efa ====
