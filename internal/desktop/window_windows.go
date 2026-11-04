// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/desktop/window_windows.go
// About: The real window on Windows: an Edge WebView2 view over the Guard-protected local server.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// The real window on Windows: an Edge WebView2 view over the Guard-protected local server.
//
// The _windows.go file name is the build constraint; window_other.go stands in elsewhere.
// go-webview2 wraps the WebView2 COM API without cgo; the Microsoft Edge WebView2 Runtime must be
// installed (it ships with current Windows).
//
// The Win32 calls here go through syscall.NewLazyDLL: the DLL is loaded and the function looked
// up on first use, and Call passes every argument as a uintptr.

package desktop

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"syscall"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

// init pins the main goroutine to its OS thread before main runs.
func init() {
	// WebView2 and the Win32 message loop must stay on the thread that created the window.
	// init runs on the main goroutine, so this pins main, and Run, which main calls, to the
	// process's first thread; without it the Go scheduler could move them between threads.
	runtime.LockOSThread()
}

// Window is the open window. It is passed by value; every copy drives the same view.
type Window struct{ view webview2.WebView }

// Close ends Run. Safe from any goroutine: Dispatch queues the call onto the UI thread.
func (w Window) Close() { w.view.Dispatch(func() { w.view.Terminate() }) }

// Eval runs JavaScript in the page. Safe from any goroutine; it does not wait for the script or
// return its value (use a Bind function to get a value back).
func (w Window) Eval(js string) { w.view.Dispatch(func() { w.view.Eval(js) }) }

// IconSet reports whether the window has its own icon (the exe's icon resource 1 loaded as the class icon).
//
// Both must hold: the window class has an icon handle, and the exe really carries icon group 1
// (linked in from rsrc_windows_amd64.syso), so a stock icon alone does not count. The -smoke
// report uses it to check the build.
func (w Window) IconSet() bool {
	user32 := syscall.NewLazyDLL("user32.dll")
	hwnd := uintptr(w.view.Window())
	// GCLP_HICON is -14; as a uintptr that is ^uintptr(13), the two's-complement bit pattern of -14
	icon, _, _ := user32.NewProc("GetClassLongPtrW").Call(hwnd, ^uintptr(13)) // GCLP_HICON (-14)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	// GetModuleHandleW(NULL) is the running exe's own module
	module, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	// FindResourceW(module, name, type): name 1 is MAKEINTRESOURCE(1), an integer id passed in the
	// pointer argument; type 14 is RT_GROUP_ICON, the directory of an icon's sizes
	group, _, _ := kernel32.NewProc("FindResourceW").Call(module, 1, 14) // RT_GROUP_ICON 1
	return icon != 0 && group != 0
}

// Run serves Handler on 127.0.0.1 behind Guard, opens the window on it and returns when it closes.
//
// It must be called from the main goroutine (see init) and blocks for the window's lifetime.
// Errors: no free port, no WebView2 Runtime, or a Bind name that cannot be bound. The server is
// closed when Run returns.
//
// Steps:
//  1. Listen on 127.0.0.1 port 0, which asks the system for any free port, so two programs (or
//     two copies) never clash; the address it got becomes the Host that Guard requires.
//  2. Serve the handler behind Guard on its own goroutine.
//  3. Create the WebView2 window: its profile in DataPath, centred, with icon resource 1.
//  4. Bind the JavaScript functions, then navigate to /__open with the token.
//  5. Call Ready (before the message loop, so it must not block), then run the loop until Close
//     or the user closes the window.
func Run(o Options) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	token, host := NewToken(), ln.Addr().String()
	srv := &http.Server{Handler: Guard(o.Handler, token, host)}
	go srv.Serve(ln)
	defer srv.Close()

	view := webview2.NewWithOptions(webview2.WebViewOptions{
		DataPath:      o.DataPath,
		AutoFocus:     true,
		WindowOptions: webview2.WindowOptions{Title: o.Title, Width: o.Width, Height: o.Height, IconId: 1, Center: true},
	})
	if view == nil {
		return fmt.Errorf("could not open a WebView2 window (is the Microsoft Edge WebView2 Runtime installed?)")
	}
	defer view.Destroy()
	for name, fn := range o.Bind {
		if err := view.Bind(name, fn); err != nil {
			return err
		}
	}
	view.Navigate("http://" + host + "/__open?t=" + token)
	if o.Ready != nil {
		o.Ready(Window{view})
	}
	view.Run()
	return nil
}

// MessageBox shows a plain Windows message box (GUI programs have no console to print to).
// It blocks until the user clicks OK.
func MessageBox(title, text string) {
	// the W (wide) Win32 functions take UTF-16 text ending in a NUL; a string holding a NUL fails
	// to convert and shows as empty
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	// MessageBoxW(owner, text, caption, type): no owner window; 0x40 is MB_ICONINFORMATION with
	// the default MB_OK (0) button
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x40)
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=edef8746a0151de7eb3edfb074869357949a9eca63a03307c3bab5fa9fa057fc ====
