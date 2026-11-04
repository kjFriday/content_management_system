// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/install/hide_other.go
// About: The stand-in for hide_windows.go, so the package also builds off Windows.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The stand-in for hide_windows.go, so the package also builds off Windows.

//go:build !windows

package install

import "os/exec"

// hide does nothing off Windows, where installing is not supported (it is all PowerShell and the registry).
// line is ignored; DeleteAfterExit's cmd.exe does not exist off Windows anyway, so its Start fails.
func hide(cmd *exec.Cmd, line string) {}

// ==== stamp v1 | author=kjFriday | hmac-sha256=55295aae96c15227a603a1f94ae69c72f31cb81947d46b6fdb3bc6ab795a0add ====
