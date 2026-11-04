// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/hide_windows.go
// About: The Windows-only process flags behind hide; hide_other.go stands in on other systems.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The Windows-only process flags behind hide; hide_other.go stands in on other systems.
//
// The _windows.go file name is the build constraint. syscall.SysProcAttr has these fields only
// on Windows, which is why they live in a file of their own.

package install

import (
	"os/exec"
	"syscall"
)

// hide starts cmd with no console window; a non-empty line is the exact command line Windows gets.
//
// Go normally builds the command line from cmd.Args, quoting each argument its own way. cmd.exe
// parses its /c text by its own rules, so DeleteAfterExit passes the finished line here instead.
// HideWindow sets SW_HIDE for the started window; CREATE_NO_WINDOW (0x08000000) keeps a console
// program from getting a console at all, so no black box flashes up.
func hide(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=82b1c9f60f6779eb29940faaa38cec5b8c4e5e33af72b2fc7fa338f9e3a8c22c ====
