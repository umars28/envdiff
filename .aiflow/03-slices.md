# Slices — envdiff

Source: `.aiflow/02-design.md`. Test command for every slice, copied from `.aiflow/project.yaml`:
`go test ./...`

| # | Slice | Primary files | Test that proves it |
|---|---|---|---|
| 1 | Module skeleton and the `.env` parser. `go.mod` (module `github.com/umars28/envdiff`, `go 1.24`), the `Entry`/`Duplicate`/`File`/`ParseError` types, `Parse`, `ParseFile`, `Lookup`, `Keys`. Implements the whole parser contract: `export` prefix, matching outer quotes stripped with opaque remainder, `#` as comment only at line start, BOM and trailing `\r` stripped, blank lines skipped, malformed line is a `ParseError`, duplicate keys last-wins and recorded. | `go.mod`, `internal/envfile/envfile.go`, `internal/envfile/envfile_test.go`, `internal/envfile/testdata/*.env` | `go test ./...` — table-driven cases per contract clause: `PASSWORD=abc#123` parses to value `abc#123`, `export A=1` yields key `A`, CRLF and BOM fixtures round-trip, a duplicate key lands in `File.Duplicates` with both line numbers and `Lookup` returns the last, a junk line returns `*ParseError` whose `Error()` contains the path, line and reason but **not** the offending bytes. |
| 2 | Salted fingerprints. `Print` as a distinct string type, `Unknown`, `Masker`, `New` (32 bytes from `crypto/rand`), `NewWithSalt`, `Fingerprint` as `hex(HMAC-SHA256(salt, value))[:6]`, `Print.String`. Depends on nothing but stdlib — only needs `go.mod` from slice 1. | `internal/mask/mask.go`, `internal/mask/mask_test.go` | `go test ./...` — with a fixed salt via `NewWithSalt`, the same value fingerprints identically and two different salts diverge for the same value; output always matches `<masked:[0-9a-f]{6}>`; a sentinel-value test asserts no substring of the input of length ≥ 3 appears in the rendered `Print`. |
| 3 | Key-set comparison. `Kind` with `Added`/`Removed`/`Changed`, `Kind.Sigil`, `diff.Entry` holding `mask.Print` only, `Result` with `LeftPath`/`RightPath`/`Entries`/`SameCount`, `Compare`, `HasChanges`. `Compare` is the sole reader of `envfile.Entry.Value` and converts to `Print` on read; entries sorted lexicographically by key. | `internal/diff/diff.go`, `internal/diff/diff_test.go` | `go test ./...` — fed two hand-built `*envfile.File` and a fixed-salt `Masker`: a key only on the right is `Added`, only on the left is `Removed`, present on both with different values is `Changed` with both `Old` and `New` set, equal values increment `SameCount` and emit no entry; two keys sharing a value get the same `Print`; `Entries` come back sorted; empty-vs-empty and empty-vs-populated edges hold; `HasChanges` is false exactly when `Entries` is empty. |
| 4 | Text rendering. `report.Text` writing the `--- / +++` header, sigil lines with key column and fingerprints, the `N changed, M unchanged` summary and the salt-caveat footer; `report.Warnings` writing `warning: <path>:<line>: duplicate key <K> (first seen line <n>)` per duplicate. Both take an `io.Writer` and return `error`. | `internal/report/report.go`, `internal/report/report_test.go` | `go test ./...` — golden-string comparison of `Text` against a `bytes.Buffer` for a `Result` containing one added, one removed and one changed entry, matching the design's sample output byte for byte including alignment and footer; `Warnings` golden for a `File` with two duplicates; both assert the empty-input case writes a sensible minimal output and returns nil. |
| 5 | The binary and the leak test. `main.go` parses `-q` and `-version` plus two positional paths, calls `ParseFile` twice, `mask.New`, `Compare`, `Warnings` to stderr, `Text` to stdout unless `-q`, and is the only place `os.Exit` is called: 0 no differences, 1 differences, 2 error. | `cmd/envdiff/main.go`, `cmd/envdiff/main_test.go`, `cmd/envdiff/testdata/*.env` | `go test ./...` — exit-code table over fixture pairs (identical → 0, differing → 1, missing/malformed file → 2) driven through the run function with captured stdout/stderr buffers; `-q` produces empty stdout while keeping the exit code; and the leak test runs every fixture pair, including the deliberately malformed ones that force the error path, over files seeded with `hunter2-TOTALLY-SECRET` and asserts the sentinel appears in neither stdout nor stderr. |

Ordering follows the dependency arrows and nothing else. Slice 1 ships `go.mod` because every
later slice needs a module to compile inside, and it ships the parser types because `diff` and
`report` both name them in their signatures. Slice 2 sits second only because it is cheap and
leaf — it touches stdlib alone and could equally be first; slice 3 needs both 1 and 2 before it
can be written at all. Slice 4 consumes `diff.Result` and so follows 3, and slice 5 is last
because it is pure wiring over the four packages beneath it. Each slice is a compiling package
with its own tests, so `go test ./...` is green at every boundary, and each is mergeable the
moment it is done.

Slice 5 carries the most risk, and it is risk of the kind the design was built to catch rather
than risk of getting stuck. It is the first point where all four packages are wired together and
the first time the error path runs end to end, so it is where the sentinel leak test can fail for
a cause that actually lives in slice 1 — a `ParseError` that quotes the offending line, or a
`fmt.Errorf` in `main` that wraps a value. Budget for a correction landing back in the parser
rather than in `main`. Slice 1 is the second-largest risk for the opposite reason: its grammar
decisions are frozen by the design and re-opening them mid-build would invalidate fixtures in
slices 3 through 5, so the inline-`#` and opaque-quoting rules need to be implemented exactly as
written, not improved upon.
