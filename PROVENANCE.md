# Provenance

The packages under `internal/` were adapted from code the owner staged,
read-only, in `D:\Portfolio\_extracted`. Each source is identified below by
its SHA-256, which is the key to its row in
`D:\Portfolio\_extracted\MANIFEST.tsv`; that row gives the staged path, the
symbols used and the licence decision (owner reuse). The hashes were checked
against the staged files on 2026-10-16. Only the listed symbols were taken.
No licence header or narrative comment was copied; a few one-line technical
comments keep close to the original wording.

## Files

### `go.mod` (new)

`module acme/cms`, `go 1.24`, no `require` lines. Go 1.22 or later is
needed for the method-and-path patterns and `r.PathValue` in `main.go`.

`main.go` is unchanged and does not build as is. It needs
`go get github.com/mattn/go-sqlite3`, which uses cgo and therefore a C
compiler (for example gcc from MinGW-w64 on Windows). The `internal/`
packages use only the standard library and build and test without cgo:

    go vet ./internal/...
    go test ./internal/...

### `internal/related/related.go`, `related_test.go`

- Source `E2E1C5CCEAF971D6E0D84B632CFDF764136A288BF9D3A0AB4B4DD8834915EEE4`:
  `stopWords`, `tokenize`, `corpusDoc`, `vector`, `index`, `fit`,
  `countTerms`, `weigh`, `normalize`, `cosine`, `centroid`, `sharedTerms`,
  `recommendation`, `rank`. Nothing from the corpus builder or the
  functions after `rank`.
- Test source `47897C7BD2F5ADA950479A93C9C1FB60EF2C2912DB8DBDA7CF51457F95B3B1DD`:
  the four self-contained tests (tokenize, smoothed IDF, cosine, centroid),
  with new fixture words. The other five were rewritten over a fixture of
  ACME Inc. posts. The determinism, tie-order, combining-mark and
  link-card tests are new.
- Changes:
  - `corpusDoc` became the exported `Doc{Name, Text}`, `index` became
    `Index` (built with `New`), and `recommendation` became
    `Recommendation{Name, Score, Shared}` with JSON tags.
  - New `Related(name, k)` (item to item) and `RelatedToSet(names, k)`,
    which uses `centroid`. Both return nil for unknown names or `k <= 0`.
  - Name lookup goes through a map instead of a linear scan, and `New`
    copies its input slice.
  - `tokenize` now keeps any Unicode letter or digit, keeps a combining
    mark inside the word it follows, and measures token length in runes.
    It used to split words such as "café" at the first non-ASCII letter.
    Input is expected in NFC: the standard library has no normalizer, so
    an NFD spelling is a different term from the NFC one.
  - `normalize` and `cosine` now add in sorted term order instead of map
    order, so scores are bit-identical on every call and every build (the
    original could differ in the last bit). `cosine` sums only the shared
    terms. `rank` uses a stable sort, so equal scores order by Name, then
    by position.
  - New `BodyText` flattens a JSON block body (Editor.js, TipTap) into
    text for indexing. The original had no equivalent. It skips structural
    keys (including the Editor.js link-card and embed fields `link`,
    `source`, `embed` and `service`) and any value that is a bare http,
    https or mailto link.

### `internal/suggest/suggest.go`, `suggest_test.go`

- Source `CA5E1F3D5330539C4DCE51140153B59C335E7FCDDE6240FFA92853D64FBD83F7`:
  `editDistance` and `suggest` only. The vocabulary builder and the printer
  after `suggest` were not taken.
- Changes:
  - `EditDistance` is exported and unchanged; it was already rune-based.
  - `Suggest(word, vocab)` takes the vocabulary as a parameter and returns
    the single best entry and a bool, instead of up to three entries.
  - Both sides are lowercased.
  - The 3-character cutoffs (the short-word edit limit and the minimum
    prefix length) now count runes, not bytes.
  - The entry is returned as written in `vocab`.
- The tests are new and use CMS slugs.

### `internal/textfmt/textfmt.go`, `textfmt_test.go`

| Function | Source SHA-256 | Original symbol |
|---|---|---|
| `Truncate` | `CBB0D2CA4549EA4795376B3FD0B611FB468BE02DF7067124B01E9F5290D8748C` | `slakeClip` |
| `Plural` | `CBB0D2CA4549EA4795376B3FD0B611FB468BE02DF7067124B01E9F5290D8748C` | `plural` |
| `ShortDuration` | `42C9F35A789309F71FFD2D6FFE2B6A94AF90CCD0325570646FABDEA70E5E1DA5` | `hostSpan` |
| `OneLine` | `BCF3F07ED405606D844AA4213B39EAD98C44805CD8BD12FBEBB2C43BC19C3558` | `wordsField` |
| `Wrap` | `1387603E26524C6680E4245D5D767206771BB3B52F93CAE15702C3F8E13F1FF5` | `wrapAt` |
| `PadRight` | `370993B950A21A8C4A367C0CB68A4426858C0FCD3CE6138AD9B64480F23E2DFA` | `padRunes` (plus `runeLen`) |

- `clipRunes` from the last source overlapped with `slakeClip`, so only one
  truncator was kept: `Truncate`, which marks the cut with "…".
- `Truncate` returns "" for `n <= 0`. The original panicked on `n == 0`.
- `ShortDuration` uses integer arithmetic instead of float, handles negative
  durations (including the minimum `time.Duration`), and returns "0s" for
  anything under a second.
- `Wrap` drops the indent parameter (the original indented every line),
  measures runes instead of bytes, and treats `width <= 0` as no limit. It
  still never breaks a word longer than `width`; that word gets its own
  line, and Wrap's doc comment says so.
- The tests are new.

### `internal/health/health.go`, `health_test.go`

- Reference only; the code was rewritten rather than copied:
  - `A8B4C7503AAC71552EEAA668E9B40AFF0339548DD853642FCAAAF6C621E175AF`
    (the runtime-stats reader named in its MANIFEST row)
  - `A81D496E4CC1BD11CC9B136FEFBA668918ADF50413D7663B73A3CA1B0510FA65`
    (`startTime`)
- `Snapshot` is a JSON struct with the fields `uptime_seconds`, `goroutines`,
  `heap_alloc_bytes`, `sys_bytes`, `num_gc` and `go_version`.
- `Take(start)` reads it.
- `Handler(start)` serves it for GET and HEAD with `Cache-Control: no-store`.
  Any other method gets 405.
- The terminal-size reading and the formatted text lines were dropped. The
  process start time is a parameter instead of a package global.
- Not done: wiring `GET /api/health` into `main.go` (and adding a
  `db.PingContext` check there). `main.go` was left untouched on purpose.
