// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/hide_windows.go
// About: Part of package install in journal.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package install

import (
	"os/exec"
	"syscall"
)

// hide starts cmd with no console window; a non-empty line is the exact command line Windows gets.
func hide(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=2f50928d91e19470359ccfade10177464f13c1da1caeffb7e0d3c1385ad1e435 ====
