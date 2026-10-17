# Content management (CMS)

ACME Inc.'s journal: a read-only post API and two HTML pages, Go standard library only. ACME Inc. and
everyone in it is fictional; post text is lorem ipsum.

- **Standalone:** reads `mock/posts.json` (60 posts, the CMS's own content) and `mock/employees.json`
  (the authors).
- **Connected:** `-hub http://127.0.0.1:8700` (or `ACME_HUB`) loads authors live from the CRUD hub's
  `employees` table. If the hub is unreachable it logs that and uses the snapshot. `/api/source` and the
  page footer say which one is in use.

## Run

```bash
go run .
```

```bash
go run . -hub http://127.0.0.1:8700
```

Then open `http://127.0.0.1:8720/` (`-addr` or `CMS_ADDR` changes it).

| Path | What |
|---|---|
| `GET /` | Published posts, newest first; `?q=` searches |
| `GET /posts/{slug}` | One post with three related posts; a bad slug gets a did-you-mean |
| `GET /api/posts` | Published posts (title, slug, author, date, excerpt) |
| `GET /api/posts/{slug}` | One post; `404` with `did_you_mean` for a near miss |
| `GET /api/posts/{slug}/related` | TF-IDF related posts (`internal/related`) |
| `GET /api/search?q=` | Posts containing every word |
| `GET /api/source` | Where authors came from, and counts |
| `GET /api/health` | Runtime health (`internal/health`) |

Writing posts, auth and storage are not built yet. The earlier SQLite design (needs a cgo driver) is
kept in `reference/sqlite_server.go.txt`.

## Tests

```bash
go vet ./...
```

```bash
go test ./...
```

## License

AGPL-3.0, as the rest of the portfolio.
