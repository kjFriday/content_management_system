// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/desktop/options.go
// About: The window's settings, shared by window_windows.go and the window_other.go stubs.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The window's settings, shared by window_windows.go and the window_other.go stubs.

package desktop

import "net/http"

// Options describe the window.
type Options struct {
	Title         string
	Width, Height uint
	DataPath      string       // WebView2 profile folder
	Handler       http.Handler // the program's pages
	// Bind adds JavaScript functions (window.<name>) that call into Go.
	Bind map[string]any
	// Ready, when set, runs on the UI thread once the window exists (e.g. for a smoke check).
	Ready func(w Window)
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=f402f71353cb6f871d762773d03026f8b44a6c6548a62c232f26bd0765edf063 ====
