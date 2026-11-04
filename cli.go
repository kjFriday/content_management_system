// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: cli.go
// About: Func main and the command line: flags, the installer commands, -serve and the window.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Func main and the command line: flags, the installer commands, -serve and the window.
//
// main.go holds the routes this file serves; site.go the store it builds; internal/app the
// program's identity and database path; internal/install the -install and -uninstall work;
// internal/desktop the WebView2 window.
//
// Flow of main:
//  1. parse reads the flags; a bad flag exits with code 2 (the flag package has printed why).
//  2. -version tells the name and version and returns.
//  3. -install or -uninstall runs runInstaller, then, if the install folder must go after this
//     process ends, starts install.DeleteAfterExit as the very last action.
//  4. Otherwise app.ResolveDB picks the database, newSite loads it and routes builds the mux.
//  5. With -serve or -public, listen serves the mux behind secure until Ctrl+C.
//  6. Otherwise desktop.Run opens the window on the mux; -smoke adds a binding and a Ready hook
//     that runs smoke.
//
// Types: options (the parsed flags).

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"generic-company/cms/internal/app"
	"generic-company/cms/internal/desktop"
	"generic-company/cms/internal/install"
)

// exeName is the exe's file name once installed, whatever the running copy is called. setup.json,
// the Wizard Maker setup's description, names the same file.
const exeName = "generic-company-cms.exe"

// options are the parsed command-line flags, one field per flag (see parse for each meaning).
// serve is also true when public is set.
type options struct {
	db, addr, root, smoke                      string
	mock, serve, public, version, inst, uninst bool
	desktopShortcut, quiet, purge              bool
}

// parse reads args (without the program name) into options. Usage and errors go to out.
//
// ContinueOnError makes a bad flag (or -h) come back as an error instead of exiting here, so tests
// can call parse; main turns the error into exit code 2. -public implies -serve. The usage
// strings are the -h output, and the README mirrors them.
func parse(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("generic-company-cms", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&o.db, "db", "", `shared database (default: GENERIC_COMPANY_DB, the saved setting, or %LOCALAPPDATA%\Generic Company Inc\generic_company.db)`)
	fs.BoolVar(&o.mock, "mock", false, "authors from the embedded snapshot, not the database")
	fs.BoolVar(&o.serve, "serve", false, "no window: serve the pages on -addr (for development)")
	fs.BoolVar(&o.public, "public", false, "no window: serve a public, read-only site on -addr (no database form, no local paths)")
	fs.StringVar(&o.addr, "addr", "127.0.0.1:8720", "with -serve or -public: listen address (e.g. :8080 for every interface)")
	fs.BoolVar(&o.version, "version", false, "print the version")
	fs.BoolVar(&o.inst, "install", false, "install for this user (Start menu, Apps & features)")
	fs.BoolVar(&o.desktopShortcut, "desktop-shortcut", false, "with -install: add a desktop shortcut")
	fs.BoolVar(&o.uninst, "uninstall", false, "uninstall (the database is kept)")
	fs.BoolVar(&o.quiet, "quiet", false, "no message boxes")
	fs.BoolVar(&o.purge, "purge", false, "with -uninstall: also remove this program's settings")
	fs.StringVar(&o.root, "root", "", `sandbox: install under this folder (registry goes to registry.json)`)
	fs.StringVar(&o.smoke, "smoke", "", "open the window, write a JSON report to this file, close")
	err := fs.Parse(args)
	o.serve = o.serve || o.public
	return o, err
}

// listen serves handler on addr until Ctrl+C or SIGTERM. The timeouts stop slow clients from holding
// connections open.
//
// Steps:
//  1. Build the server: 5 s to send the headers, 10 s for the whole request, 15 s to write the
//     answer, 60 s for an idle keep-alive connection, and at most 64 KiB (64 << 10) of headers.
//  2. Start ListenAndServe on its own goroutine; its error arrives on done.
//  3. Wait for whichever comes first: a serve error (for example, the port is taken), which is
//     returned, or a signal, after which Shutdown lets open requests finish for up to 5 s.
//
// A clean Shutdown returns nil.
func listen(addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		wait, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(wait)
	}
}

// tell prints, and also shows a message box unless -quiet (a windowsgui exe has no console).
func tell(o options, msg string) {
	fmt.Println(msg)
	if !o.quiet {
		desktop.MessageBox(app.Name, msg)
	}
}

// program describes this exe for package install: the identity from internal/app, the running
// exe's path (it is copied as-is) and the embedded app.ico. It fails only when the OS cannot say
// where the exe is. The icon's read error is dropped: the file is embedded, so only a broken
// build could lack it.
func program() (install.Program, error) {
	exe, err := os.Executable()
	if err != nil {
		return install.Program{}, err
	}
	ico, _ := icons.ReadFile("icon/app.ico")
	return install.Program{AppID: app.ID, Name: app.Name, Version: app.Version, Publisher: app.Publisher,
		Exe: exe, ExeName: exeName, Icon: ico}, nil
}

// runInstaller installs or uninstalls: -> the exit code, and a folder to remove once this exe has exited.
//
// The exit code is non-zero only when a Wizard Maker uninstaller ran and returned one. An error
// means nothing was done, or the work stopped part way; main tells it and exits with 1.
//
// Steps:
//  1. Describe the program and size the exe (the registry's EstimatedSize; 0 when it cannot be
//     read), then make the plan for -root and -desktop-shortcut.
//  2. -uninstall: aim the plan at the folder the uninstall entry names (install.Located).
//  3. If a Wizard Maker setup made that install, run its uninstall.exe and pass its exit code on.
//  4. Otherwise uninstall here: shortcuts, registry entry, folder, and with -purge the program's
//     data folder. The shared database is never removed.
//  5. Without -uninstall: install, then tell where.
func runInstaller(o options) (int, string, error) {
	prog, err := program()
	if err != nil {
		return 0, "", err
	}
	info, _ := os.Stat(prog.Exe)
	var size int64
	if info != nil {
		size = info.Size()
	}
	plan := install.MakePlan(prog, os.Getenv, o.root, o.desktopShortcut, size, time.Now())
	if o.uninst {
		if plan, err = install.Located(plan, install.PowerShellOutput); err != nil {
			return 0, "", err
		}
		if install.Wizard(plan) != "" {
			// a setup made by Generic Company Wizard Maker installed it: its uninstaller keeps the files changed
			// since the install, so it does the work. a cancel (install.Cancelled) is not worth a message box
			code, said, err := install.RunUninstaller(plan, prog.Exe, install.UninstallArgs(o.quiet, o.purge, o.root), install.RunAndWait)
			switch {
			case err != nil:
				return 0, "", err
			case code != 0 && code != install.Cancelled:
				tell(o, app.Name+": "+said)
			case said != "":
				fmt.Println(said) // its window, or its quiet output, has said how it went
			}
			return code, "", nil
		}
		purge := ""
		if o.purge {
			purge = app.DataDir(os.Getenv)
		}
		removed, later, err := install.Uninstall(plan, install.PowerShell, prog.Exe, purge)
		if err != nil {
			return 0, "", err
		}
		msg := app.Name + " uninstalled. The database is kept."
		if !removed {
			msg = app.Name + " was not installed; shortcuts and the uninstall entry are cleared."
		}
		tell(o, msg)
		return 0, later, nil
	}
	if err := install.Install(plan, install.PowerShell); err != nil {
		return 0, "", err
	}
	tell(o, fmt.Sprintf("%s %s installed in %s. Find it in the Start menu under Generic Company Inc.", app.Name, app.Version, plan.InstallDir))
	return 0, "", nil
}

// main runs the program; the file comment above lists its flow.
//
// Exit codes: 0 done, 1 an error (told in a message box unless -quiet), 2 a bad flag, or the
// Wizard Maker uninstaller's own code. In a -H windowsgui build there is no console, so the
// message box is the only place an error shows.
func main() {
	o, err := parse(os.Args[1:], os.Stderr)
	if err != nil {
		os.Exit(2)
	}
	if o.version {
		tell(o, app.Name+" "+app.Version)
		return
	}
	if o.inst || o.uninst {
		code, later, err := runInstaller(o)
		if err == nil && later != "" {
			// the very last action, after the message box: the cmd removes the folder once this exe has exited
			err = install.DeleteAfterExit(later)
		}
		if err != nil {
			tell(o, app.Name+": "+err.Error())
			os.Exit(1)
		}
		os.Exit(code)
	}
	// the pages: the same mux serves the window and -serve
	path := app.ResolveDB(o.db, os.Getenv)
	st := newSite(path, o.mock)
	st.settings = func(p string) error { return app.SaveSettings(os.Getenv, map[string]any{"db": p}) }
	handler := routes(st, time.Now(), o.public)
	if o.serve {
		if o.public {
			log.Printf("%s on http://%s/ (public; authors from %s)", app.Title, o.addr, st.publicSource())
		} else {
			log.Printf("%s on http://%s/ (authors from %s)", app.Title, o.addr, st.sourceText())
		}
		if err := listen(o.addr, secure(handler)); err != nil {
			log.Fatal(err)
		}
		return
	}
	// the window: 1000 x 760 px, with its WebView2 profile (cookies, cache) in the program's data folder
	opts := desktop.Options{Title: app.Title, Width: 1000, Height: 760, DataPath: app.DataDir(os.Getenv) + `\WebView2`, Handler: handler}
	if o.smoke != "" {
		// the page calls genericCompanySmoke (the Bind name and the name smoke evaluates must
		// match) to hand its title and card count back to Go; the buffer of 1 means the call
		// never blocks the UI thread
		seen := make(chan [2]any, 1)
		opts.Bind = map[string]any{"genericCompanySmoke": func(title string, cards int) { seen <- [2]any{title, cards} }}
		opts.Ready = func(w desktop.Window) { go smoke(o.smoke, st, w, seen) }
	}
	if err := desktop.Run(opts); err != nil {
		tell(o, app.Name+": "+err.Error())
		os.Exit(1)
	}
}

// smoke waits for the page to show posts, then writes the report and closes the window.
//
// It runs on its own goroutine (Ready must return so the window's message loop can start).
// tools/verify.ps1 and the Dev Console read the report: title, icon_set, source ("db" or "mock"),
// version and counts.authors are checked there, so keep the keys.
//
// Steps:
//  1. Give the page 2 s to load, then ask it for its title and number of <article> cards.
//  2. Wait up to 10 s for the answer; on a timeout the report says "" and 0.
//  3. Read the store's source and the database path under the site's lock.
//  4. Write the JSON report to file and close the window, which ends desktop.Run.
//
// A failed write is not reported: the missing file is the failure the checker sees.
func smoke(file string, st *site, w desktop.Window, seen chan [2]any) {
	time.Sleep(2 * time.Second)
	w.Eval("genericCompanySmoke(document.title, document.querySelectorAll('article').length)")
	page := [2]any{"", 0}
	select {
	case page = <-seen:
	case <-time.After(10 * time.Second):
	}
	s := st.get()
	st.mu.RLock()
	dbPath := st.dbPath
	st.mu.RUnlock()
	source := "mock"
	if s.Source == "database" {
		source = "db"
	}
	report := map[string]any{"app_id": app.ID, "version": app.Version, "title": app.Title, "icon_set": w.IconSet(),
		"source": source, "db": dbPath, "counts": s.Counts(), "page_title": page[0], "cards": page[1]}
	b, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(file, b, 0o644)
	w.Close()
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=65160687087c95dc4a78feaf6c7b1d5e1563c6c67b081b283fafd0b58eb8df4a ====
