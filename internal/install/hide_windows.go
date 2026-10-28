// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: internal/install/hide_windows.go
// About: Part of package install in content_management_system.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
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

// ==== stamp v1 | author=kjFriday | hmac-sha256=001f0e0f3e81fdda525da8844f93cbc8474f297baffddcb89e299250280f328a ====
