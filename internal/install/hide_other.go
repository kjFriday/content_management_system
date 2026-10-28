// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: internal/install/hide_other.go
// About: Part of package install in content_management_system.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

//go:build !windows

package install

import "os/exec"

// hide does nothing off Windows, where installing is not supported (it is all PowerShell and the registry).
func hide(cmd *exec.Cmd, line string) {}

// ==== stamp v1 | author=kjFriday | hmac-sha256=b0ae0865722ed539c87261844c0e8b91cbccf86b19d1e883df1bfe86ff6c95eb ====
