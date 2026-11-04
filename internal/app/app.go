// ==== Generic Company Inc. portfolio ====================================
// Project: journal | File: internal/app/app.go
// About: Package app is the CMS's identity (name, version, credit) and where it keeps its files.
// Copyright (c) 2026 kjFriday | SPDX-License-Identifier: AGPL-3.0-only
// You may change this file under the AGPL-3.0. The stamp at the end marks
// the version kjFriday wrote. Verify with the portfolio's tools/stamp.py --check.
// Reused code, if any, is mapped in PROVENANCE.md.
// ========================================================================

// Package app is the CMS's identity (name, version, credit) and where it keeps its files.
//
// The shared database is the only link between the Generic Company programs. Its path is chosen in this order:
// the -db flag, the GENERIC_COMPANY_DB environment variable, the program's saved setting, then the default
// %LOCALAPPDATA%\Generic Company Inc\generic_company.db. The setting lives in %LOCALAPPDATA%\Generic Company Inc\<app id>\settings.json.
//
// cli.go reads the constants for the window title, the installer and -version; site.go saves the
// About page's database choice through SaveSettings. Functions take an Env instead of reading the
// environment, so tests can point every path at a temporary folder.
//
// settings.json is a JSON object such as {"db": "D:\\data\\generic_company.db", "version": 1}.
// Keys this program does not know are kept on every save.
package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// The program's identity. setup.json (the Wizard Maker setup) repeats ID, Name, Version and
// Publisher and must agree; the Dev Console rewrites the Version line on a version bump, so it
// stays the first line in this file that starts with Version and an "=".
const (
	// Version is the release, shown in the title, the About page and Apps & features.
	Version = "0.2.0"
	// ID names the uninstall registry key and the program's data folder.
	ID = "GenericCompany.CMS"
	// Name is the human name: Start-menu shortcut, install folder, message-box titles.
	Name = "Generic Company Journal"
	// Title is the window and page title; it ends with " "+Version (pinned by a test).
	Title = "Generic Company Inc. — " + Name + " " + Version
	// Publisher is the Publisher value in the uninstall entry.
	Publisher = "Generic Company Inc. (fictional)"
	// Credit is the icon's attribution, which CC BY 2.5 requires wherever the icon is shown.
	Credit = "Icon: Silk by Mark James (famfamfam.com/lab/icons/silk), CC BY 2.5 " +
		"(creativecommons.org/licenses/by/2.5), enlarged by nearest-neighbour."
	// company is the folder name under %LOCALAPPDATA% that every Generic Company program shares.
	company = "Generic Company Inc"
	// settingsVersion is written into settings.json as "version", so a later format can tell
	// an old file apart.
	settingsVersion = 1
)

// Env looks up an environment variable; tests pass a map instead of the real environment.
type Env func(string) string

// LocalAppData is %LOCALAPPDATA%, or ~\AppData\Local when it is unset.
func LocalAppData(env Env) string {
	if v := env("LOCALAPPDATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Local")
}

// BaseDir is %LOCALAPPDATA%\Generic Company Inc: the shared database and every program's data folder.
func BaseDir(env Env) string { return filepath.Join(LocalAppData(env), company) }

// DefaultDB is the shared database's default path.
func DefaultDB(env Env) string { return filepath.Join(BaseDir(env), "generic_company.db") }

// DataDir is this program's own folder (settings, WebView2 profile).
func DataDir(env Env) string { return filepath.Join(BaseDir(env), ID) }

// settingsPath is DataDir\settings.json, the one settings file this program keeps.
func settingsPath(env Env) string { return filepath.Join(DataDir(env), "settings.json") }

// LoadSettings returns the saved settings, or an empty map when missing or not a JSON object.
// It never fails: a broken file is treated as no file. JSON numbers come back as float64.
func LoadSettings(env Env) map[string]any {
	out := map[string]any{}
	b, err := os.ReadFile(settingsPath(env))
	// the file "null" decodes without an error but leaves out nil, hence the last check
	if err != nil || json.Unmarshal(b, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// SaveSettings merges changes into settings.json, keeping unknown keys; written to a temp file, then renamed.
//
// Steps:
//  1. Load the current settings and overwrite the changed keys; stamp "version".
//  2. Create the data folder if needed.
//  3. Write settings.json.tmp, then rename it over settings.json. A crash part way leaves the old
//     file whole instead of a half-written one; on Windows os.Rename replaces an existing file.
//
// Errors come from the file system (folder, write, rename). Not safe for two writers at once.
func SaveSettings(env Env, changes map[string]any) error {
	data := LoadSettings(env)
	for k, v := range changes {
		data[k] = v
	}
	data["version"] = settingsVersion
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	path := settingsPath(env)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ResolveDB picks the shared database: flag, then GENERIC_COMPANY_DB, then the saved setting, then the default.
//
// A choice that is empty or only white space is skipped. The first real one is made absolute
// (relative to the working folder), or returned as given if that fails. The file is not checked
// here: site.go falls back to the snapshot when it cannot be opened.
func ResolveDB(flag string, env Env) string {
	saved, _ := LoadSettings(env)["db"].(string)
	for _, choice := range []string{flag, env("GENERIC_COMPANY_DB"), saved} {
		if c := strings.TrimSpace(choice); c != "" {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return DefaultDB(env)
}

// ==== stamp v1 | author=kjFriday | hmac-sha256=a9fc6ce689ad2e2a51234329739f8372c87371d1946d5bacbbdccf5dcf626b85 ====
