// ==== ACME Inc. portfolio ===============================================
// Project: content_management_system | File: cli.go
// About: Part of package main in content_management_system.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"acme/cms/internal/app"
	"acme/cms/internal/desktop"
	"acme/cms/internal/install"
)

const exeName = "acme-cms.exe"

type options struct {
	db, addr, root, smoke              string
	mock, serve, version, inst, uninst bool
	desktopShortcut, quiet, purge      bool
}

func parse(args []string, out io.Writer) (options, error) {
	var o options
	fs := flag.NewFlagSet("acme-cms", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&o.db, "db", "", `shared database (default: ACME_DB, the saved setting, or %LOCALAPPDATA%\ACME Inc\acme.db)`)
	fs.BoolVar(&o.mock, "mock", false, "authors from the embedded snapshot, not the database")
	fs.BoolVar(&o.serve, "serve", false, "no window: serve the pages on -addr (for development)")
	fs.StringVar(&o.addr, "addr", "127.0.0.1:8720", "with -serve: listen address")
	fs.BoolVar(&o.version, "version", false, "print the version")
	fs.BoolVar(&o.inst, "install", false, "install for this user (Start menu, Apps & features)")
	fs.BoolVar(&o.desktopShortcut, "desktop-shortcut", false, "with -install: add a desktop shortcut")
	fs.BoolVar(&o.uninst, "uninstall", false, "uninstall (the database is kept)")
	fs.BoolVar(&o.quiet, "quiet", false, "no message boxes")
	fs.BoolVar(&o.purge, "purge", false, "with -uninstall: also remove this program's settings")
	fs.StringVar(&o.root, "root", "", `sandbox: install under this folder (registry goes to registry.json)`)
	fs.StringVar(&o.smoke, "smoke", "", "open the window, write a JSON report to this file, close")
	return o, fs.Parse(args)
}

// tell prints, and also shows a message box unless -quiet (a windowsgui exe has no console).
func tell(o options, msg string) {
	fmt.Println(msg)
	if !o.quiet {
		desktop.MessageBox(app.Name, msg)
	}
}

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
			// a setup made by ACME Wizard Maker installed it: its uninstaller keeps the files changed since
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
	tell(o, fmt.Sprintf("%s %s installed in %s. Find it in the Start menu under ACME Inc.", app.Name, app.Version, plan.InstallDir))
	return 0, "", nil
}

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
	path := app.ResolveDB(o.db, os.Getenv)
	st := newSite(path, o.mock)
	st.settings = func(p string) error { return app.SaveSettings(os.Getenv, map[string]any{"db": p}) }
	handler := routes(st, time.Now())
	if o.serve {
		log.Printf("%s on http://%s/ (authors from %s)", app.Title, o.addr, st.sourceText())
		log.Fatal(http.ListenAndServe(o.addr, handler))
	}
	opts := desktop.Options{Title: app.Title, Width: 1000, Height: 760, DataPath: app.DataDir(os.Getenv) + `\WebView2`, Handler: handler}
	if o.smoke != "" {
		seen := make(chan [2]any, 1)
		opts.Bind = map[string]any{"acmeSmoke": func(title string, cards int) { seen <- [2]any{title, cards} }}
		opts.Ready = func(w desktop.Window) { go smoke(o.smoke, st, w, seen) }
	}
	if err := desktop.Run(opts); err != nil {
		tell(o, app.Name+": "+err.Error())
		os.Exit(1)
	}
}

// smoke waits for the page to show posts, then writes the report and closes the window.
func smoke(file string, st *site, w desktop.Window, seen chan [2]any) {
	time.Sleep(2 * time.Second)
	w.Eval("acmeSmoke(document.title, document.querySelectorAll('article').length)")
	page := [2]any{"", 0}
	select {
	case page = <-seen:
	case <-time.After(10 * time.Second):
	}
	s := st.get()
	source := "mock"
	if s.Source == "database" {
		source = "db"
	}
	report := map[string]any{"app_id": app.ID, "version": app.Version, "title": app.Title, "icon_set": w.IconSet(),
		"source": source, "db": st.dbPath, "counts": s.Counts(), "page_title": page[0], "cards": page[1]}
	b, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(file, b, 0o644)
	w.Close()
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=e1d5521c33a2573d43f5b2d47a4d06475d717b08ec85211368cd597f911838e2 ====
