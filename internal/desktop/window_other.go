// ==== Generic Company Inc. portfolio ====================================
// Project: content_management_system | File: internal/desktop/window_other.go
// About: Off Windows there is no window: the program can still serve its pages with -serve or...
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

//go:build !windows

// Off Windows there is no window: the program can still serve its pages with -serve or -public.
package desktop

import "errors"

// Window is the open window; off Windows there never is one.
type Window struct{}

// Close does nothing off Windows.
func (Window) Close() {}

// Eval does nothing off Windows.
func (Window) Eval(string) {}

// IconSet is false off Windows.
func (Window) IconSet() bool { return false }

// Run fails off Windows: the window is Edge WebView2.
func Run(Options) error {
	return errors.New("the window needs Windows (Edge WebView2); use -serve or -public to serve the pages")
}

// MessageBox does nothing off Windows; callers print the message as well.
func MessageBox(title, text string) {}

// ==== stamp v1 | author=kjFriday | hmac-sha256=7f74d20b26d584beb0bd9e871b6f8154a4a3a7e2cc1f3e83ee8f439a22268988 ====
