// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/script.go
// About: Layer two: Script writes a Plan's PowerShell; PowerShell and PowerShellOutput run it.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Layer two: Script writes a Plan's PowerShell; PowerShell and PowerShellOutput run it.

package install

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Literal is a PowerShell single-quoted string: nothing inside is expanded; ' is doubled.
func Literal(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// Script is the PowerShell that creates (or removes) the shortcuts and, unless sandboxed, the registry entry.
func Script(p Plan, remove bool) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
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

// Runner runs a PowerShell script.
type Runner func(script string) error

// PowerShell is the real Runner: the script goes on stdin, so no quoting reaches the command line.
func PowerShell(script string) error {
	_, err := PowerShellOutput(script)
	return err
}

// Reader runs a PowerShell script: -> what it printed.
type Reader func(script string) (string, error)

// PowerShellOutput is the real Reader, run like PowerShell.
func PowerShellOutput(script string) (string, error) {
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=d1134240d02509bae4e85b7ba311b3eec38971ad3e5f6443a5b57baa23d7c3f7 ====
