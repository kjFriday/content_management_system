<img src="icon/app-256.png" width="64" height="64" alt="" style="image-rendering: pixelated">

# Generic Company Journal (content management)

Generic Company Inc.'s journal as a desktop program: posts, search, related posts and a read-only JSON API, shown
in its own window. Generic Company Inc. and everyone in it is fictional; post text is lorem ipsum.

- **Posts** are the CMS's own content, embedded in the exe from `mock/posts.json` (60 posts).
- **Authors** come from the shared Generic Company database's `employees` table, opened **read-only by path**. The
  path is `-db PATH`, the `GENERIC_COMPANY_DB` environment variable, the one saved on the **About** page, or
  `%LOCALAPPDATA%\Generic Company Inc\generic_company.db`. With no usable database the authors come from the embedded
  `mock/employees.json`, and the footer says so. `-mock` forces the snapshot.

The window is Microsoft Edge WebView2 (part of Windows 10/11). The pages are served by the exe itself on
`127.0.0.1`, to that window only: the first request carries a per-run token that becomes an HttpOnly,
SameSite=Strict cookie, and every other request without it, or with another `Host`, is refused.

## How it works

1. `main` in `cli.go` reads the flags (`parse`). `-version`, `-install` and `-uninstall` are handled
   there and the program ends (`runInstaller`, `internal/install`).
2. `app.ResolveDB` (`internal/app`) picks the database path: `-db`, `GENERIC_COMPANY_DB`, the saved
   setting, then the default.
3. `newSite` (`site.go`) builds the store: `db.LoadAuthors` (`internal/db`) opens the file
   read-only, checks the schema and reads `employees`; on any error the authors come from the embedded
   `mock/employees.json` and the reason goes in the footer. The posts always come from the embedded
   `mock/posts.json`.
4. `store.New` (`internal/store`) flattens every post body to text once (`related.BodyText`), fits
   the TF-IDF index (`internal/related`) and maps slugs for the did-you-mean (`internal/suggest`).
5. `routes` (`main.go`) builds the mux. With `-serve` or `-public`, `listen` serves it behind
   `secure` (headers, timeouts); otherwise `desktop.Run` (`internal/desktop`) serves it on 127.0.0.1
   behind `Guard` and opens the WebView2 window on it.
6. Each handler takes the current store with `site.get` and answers JSON (`writeJSON`) or HTML
   (`render` with a template from `pages.go`). `POST /about` builds a new store from another database
   and swaps it in only if it loads (`site.useDB`).

Where state lives:

| What | Where |
|---|---|
| Posts and the fallback authors | Embedded in the exe from `mock/` (`//go:embed` in `site.go`); in memory as one `store.Store` |
| Authors | The shared Generic Company database (`employees` table), opened read-only; the CMS never writes or creates it |
| The chosen database | `%LOCALAPPDATA%\Generic Company Inc\GenericCompany.CMS\settings.json` (`{"db": ..., "version": 1}`, `internal/app`) |
| Window profile (cookies, cache) | `WebView2\` in the same folder |
| An install | `%LOCALAPPDATA%\Programs\Generic Company Inc\Generic Company Journal\` with `install.json`, a Start-menu shortcut and `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\GenericCompany.CMS`; with `-root DIR` all of it lands in DIR and the registry becomes `DIR\registry.json` |

## Code map

| File | What |
|---|---|
| `cli.go` | Entry point: `func main`, the flags, the installer commands, `-serve`/`-public` (`listen`) and `-smoke` |
| `main.go` | The package comment, `routes` (every page and API route), `writeJSON`, `notFound`, `secure` |
| `site.go` | The site state: the embedded snapshot, `load`, `newSite`, `get`, `useDB` |
| `pages.go` | `render`, the shared `layout` (inline CSS, header, footer) and the four page templates |
| `internal/app/app.go` | Identity (`Version`, `ID`, `Name`, `Title`, `Credit`), `ResolveDB`, `DataDir`, settings.json |
| `internal/db/db.go` | `Open`, `CheckSchema`, `Authors`, `LoadAuthors`: the read-only database |
| `internal/store/store.go` | The in-memory store: lists, `Get` with did-you-mean, related, search, counts |
| `internal/related/related.go`, `body.go` | TF-IDF and cosine similarity; `BodyText` flattens Editor.js and TipTap bodies |
| `internal/suggest/suggest.go` | `EditDistance` (Levenshtein, in runes) and `Suggest` |
| `internal/textfmt/textfmt.go` | `Truncate` for excerpts, `Plural`, and other rune-counted helpers |
| `internal/health/health.go` | `GET /api/health`: runtime statistics |
| `internal/desktop/guard.go`, `options.go`, `window_windows.go`, `window_other.go` | The token-and-cookie guard, the window options, the WebView2 window, and the stub off Windows |
| `internal/install/plan.go`, `script.go`, `install.go`, `wizard.go`, `hide_*.go` | Install in layers: `MakePlan` (pure data), `Script` (the PowerShell), `Install`/`Uninstall` (run it), the hand-off to a Wizard Maker `uninstall.exe`, and the hidden-console flags |
| `main_test.go` | End-to-end: routes, pages and API over the snapshot on an `httptest` server |
| `icon_test.go` | The icons are pixel-exact enlargements and match the hashes in `icon/README.md` |
| `internal/*/*_test.go` | One test file per package; `internal/install` has one per layer (`plan_test.go`, `script_test.go`, `install_test.go`, `wizard_test.go`) |
| `mock/` | The snapshot (`posts.json`, `employees.json`, `products.json`, `schema.sql`); see `mock/README.md` |
| `icon/`, `rsrc_windows_amd64.syso` | The icon files and the linked icon and manifest; see `icon/README.md` |
| `setup.json` | The Wizard Maker setup description |
| `reference/sqlite_server.go.txt` | The earlier cgo SQLite design, kept for reference, not built |

## Build and run

```bash
go build -trimpath -ldflags "-H windowsgui" -o generic-company-cms.exe .
```

```bash
generic-company-cms.exe
```

`go run . -serve` serves the same pages without a window on `http://127.0.0.1:8720/` (`-addr` changes
it), for development. `rsrc_windows_amd64.syso` embeds the pixel icon and a DPI-aware manifest; it is
generated from `icon/app.res` by the portfolio's `tools/pixel_icons.py --syso`. `-trimpath` keeps the
build machine's paths out of the exe.

## Host it as a website

```bash
generic-company-cms.exe -public -addr :8080
```

`-public` serves the same pages and API without a window, for anyone who can reach `-addr` (`:8080` is
every interface; the default stays `127.0.0.1:8720`). It is read-only and leaves out what only the
owner should see: the About page has no database form (`POST /about` gets 405), the footer and the log
name no local paths, and `/api/health` is not served. Every response carries a Content-Security-Policy,
`X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` and `Referrer-Policy: same-origin`, and the
server has read, write and idle timeouts and stops cleanly on Ctrl+C or SIGTERM.

The exe speaks plain HTTP: put it behind something that does HTTPS (Caddy, nginx, or the host's own
TLS). It also builds for Linux, where only `-serve` and `-public` work:

```bash
GOOS=linux go build -trimpath -o generic-company-cms .
```

With no shared database there (no `-db`, `GENERIC_COMPANY_DB` or saved setting), authors come from the
embedded snapshot.

## Install

```bash
generic-company-cms.exe -install
```

Installs for the current user (no admin rights): copies the exe to
`%LOCALAPPDATA%\Programs\Generic Company Inc\Generic Company Journal\`, adds **Generic Company Inc → Generic Company Journal** to the Start menu
(`-desktop-shortcut` for the desktop too) and lists it in **Apps & features**. `generic-company-cms.exe -uninstall`
(or Apps & features) removes it again; the database is never deleted (`-purge` also removes the CMS's
own settings and WebView2 profile). `-root DIR` does it all inside DIR instead. Other flags: `-version`,
`-quiet`, `-smoke FILE`.

`wizard-maker.exe build setup.json` (Generic Company Wizard Maker, `company/wizard_maker`), run in this
folder after the build, makes the setup, `dist\Generic Company Journal-0.2.0-setup.exe`.

| Path | What |
|---|---|
| `GET /` | Published posts, newest first; `?q=` searches |
| `GET /posts/{slug}` | One post with three related posts; a bad slug gets a did-you-mean |
| `GET /about` | Version, icon credit, and the shared database path (`POST` to change it; neither with `-public`) |
| `GET /api/posts` | Published posts (title, slug, author, date, excerpt) |
| `GET /api/posts/{slug}` | One post; `404` with `did_you_mean` for a near miss |
| `GET /api/posts/{slug}/related` | TF-IDF related posts (`internal/related`) |
| `GET /api/search?q=` | Posts containing every word |
| `GET /api/source` | Where authors came from, counts and version |
| `GET /api/health` | Runtime health (`internal/health`); not served with `-public` |

Writing posts and moving them into the shared database are not built yet. The earlier SQLite design is
kept in `reference/sqlite_server.go.txt`.

## Dependencies

- `github.com/jchv/go-webview2`: the native window. It drives WebView2 from pure Go, so no C compiler
  (cgo) is needed.
- `modernc.org/sqlite` (pinned at v1.46.1, the last release for Go 1.24): reads the shared SQLite file,
  also without cgo.

## Developing

Everything runs from this folder (`company/journal`), with Go 1.24.

| Step | Command |
|---|---|
| Run without a window | `go run . -serve` (add `-mock` to ignore the database), then open `http://127.0.0.1:8720/` |
| Vet | `go vet ./...` |
| Test | `go test ./...` |
| Smoke-check the window | `generic-company-cms.exe -mock -smoke smoke.json -quiet`: opens the window, writes a JSON report (title, `icon_set`, `source`, version, counts, cards on the page) and closes |
| Build | the `go build` line under **Build and run** |
| Package | `wizard-maker.exe build setup.json` (see **Install**) |

`go test -short ./...` skips the one test that runs PowerShell to make a real shortcut in a temp folder.
The `internal/db` tests read `mock/schema.sql` and the mock rows, and skip if they are missing.

Where to add a typical feature:

- **A new API answer**: a method on `Store` in `internal/store/store.go` (test in `store_test.go`), a
  `mux.HandleFunc` in `routes` in `main.go`, a row in the route table below, and a case in
  `main_test.go`. If the owner alone should see it, register it inside `routes`' `if !public` part,
  as `/api/health` is.
- **A new page**: a template in `pages.go` that wraps itself in `{{template "top" .}}` and
  `{{template "bottom" .}}`, and a route in `main.go` that fills its data and calls `render`.
- **A new flag**: a field in `options` and a line in `parse` (`cli.go`); the usage strings are the
  `-h` output, and this README mirrors them.

Twins: the Notify Hub (`company/notify_hub`) keeps copies of `internal/app`, `internal/desktop` and
`internal/install`, file for file, that differ only in the app id and name. Change both together.

Traps the code comments warn about:

- `Version = "0.2.0"` in `internal/app/app.go` must stay the first line there that starts with
  `Version` and `=`: the Dev Console (`tools/dev_console`) rewrites it on a version bump.
  `setup.json` repeats the id, name, version and publisher and must agree.
- `icon/` and `rsrc_windows_amd64.syso` are generated by `tools/pixel_icons.py`, and `mock/` by the
  CRUD hub's `export_mock.py`; do not edit them by hand. `icon_test.go` checks the icon files against
  the hashes in `icon/README.md`.
- The `-smoke` report keys are read by `tools/verify.ps1` and the Dev Console; keep them. The
  `genericCompanySmoke` binding name in `main` must match the name `smoke` evaluates.
- `secure` allows `'unsafe-inline'` styles only because `layout` in `pages.go` has an inline
  `<style>` block; moving the CSS out lets that go.
- Every source file starts with a portfolio header and ends with a stamp line. After editing one,
  run `python tools/stamp.py --restamp <path>` from the portfolio root; `--check` verifies.

## Tests

```bash
go vet ./...
```

```bash
go test ./...
```

Run both from this folder. `go test -short ./...` skips the shortcut test (see **Developing**).

## Icon and license

Icon: Silk "newspaper" by Mark James, CC BY 2.5, enlarged by nearest-neighbour (`icon/README.md`).
Code: AGPL-3.0, as the rest of the portfolio.
