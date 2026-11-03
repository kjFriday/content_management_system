// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/hide_windows.go
// About: The Windows-only process flags behind hide; hide_other.go stands in on other systems.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The Windows-only process flags behind hide; hide_other.go stands in on other systems.

package install

import (
	"os/exec"
	"syscall"
)

// hide starts cmd with no console window; a non-empty line is the exact command line Windows gets.
func hide(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=31f783d90ad2ee399f67d8d5a74528216f197d2e45606fffb51193de59eeb8eb ====
