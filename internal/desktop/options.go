// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/desktop/options.go
// About: The window's settings, shared by window_windows.go and the window_other.go stubs.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The window's settings, shared by window_windows.go and the window_other.go stubs.
//
// Options lives in its own file because both builds need it: window_windows.go reads it, and
// window_other.go's Run takes it and refuses.

package desktop

import "net/http"

// Options describe the window. The caller fills them and passes them to Run.
type Options struct {
	Title         string       // the window's title bar text
	Width, Height uint         // the window's starting size in pixels; it opens centred
	DataPath      string       // WebView2 profile folder
	Handler       http.Handler // the program's pages
	// Bind adds JavaScript functions (window.<name>) that call into Go.
	// The page's call returns a Promise; the arguments arrive as JSON-decoded Go values.
	Bind map[string]any
	// Ready, when set, runs on the UI thread once the window exists (e.g. for a smoke check).
	Ready func(w Window)
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=c3f3ccc3d2147fe48868c101039f0f5ec9f995ffe429f946f22fff4670e38a48 ====
