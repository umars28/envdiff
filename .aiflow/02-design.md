# Design — envdiff

Empty repository: no existing feature to imitate, so this document defines the template every
later slice follows. Baseline test run skipped — there is no Go code yet.

**Template decisions (binding):**

- Module path `github.com/umars28/envdiff`, `go.mod` declaring `go 1.24` (CI resolves the
  toolchain via `go-version-file`).
- Layout: one binary under `cmd/envdiff`, all logic under `internal/` packages that know
  nothing about `os.Exit`, `os.Args`, or each other's internals.
- **Zero external dependencies.** stdlib only: `flag`, `bufio`, `crypto/rand`, `crypto/hmac`,
  `crypto/sha256`, `sort`, `io`, `fmt`.
- Errors are values returned up to `main`; only `main` chooses an exit code. Error strings never
  contain file content.
- Tests are table-driven `_test.go` files beside the package, with `testdata/*.env` fixtures for
  the parser and golden strings for the renderer.

## Public surface

CLI:

```
envdiff [-q] [-version] <old.env> <new.env>
```

Exit codes: `0` no differences, `1` differences found, `2` error (unreadable or unparseable file).
`-q` suppresses stdout and reports via exit code only.

Output on stdout:

```
--- .env.example
+++ .env.production

+ NEW_RELIC_KEY        <masked:3f9a1c>
- LEGACY_SMTP_HOST     <masked:7b02de>
~ DATABASE_URL         <masked:1c4e88> -> <masked:a90f21>

3 changed, 12 unchanged
fingerprints are salted per run; comparable only within this output
```

Warnings on stderr: `warning: .env.production:19: duplicate key DATABASE_URL (first seen line 4)`.

`internal/envfile`:

```go
type Entry struct { Key, Value string; Line int }
type Duplicate struct { Key string; FirstLine, SecondLine int }
type File struct { Path string; Entries []Entry; Duplicates []Duplicate }
type ParseError struct { Path string; Line int; Reason string }

func Parse(path string, r io.Reader) (*File, error)
func ParseFile(path string) (*File, error)
func (f *File) Lookup(key string) (value string, ok bool)
func (f *File) Keys() []string
func (e *ParseError) Error() string
```

`internal/mask`:

```go
type Print string
const Unknown Print = "<masked:------>"

type Masker struct{ /* salt */ }

func New() (*Masker, error)
func NewWithSalt(salt []byte) *Masker
func (m *Masker) Fingerprint(value string) Print
func (p Print) String() string
```

`internal/diff`:

```go
type Kind int
const (Added Kind = iota; Removed; Changed)

type Entry struct { Key string; Kind Kind; Old, New mask.Print }
type Result struct { LeftPath, RightPath string; Entries []Entry; SameCount int }

func Compare(left, right *envfile.File, m *mask.Masker) *Result
func (r *Result) HasChanges() bool
func (k Kind) Sigil() string
```

`internal/report`:

```go
func Text(w io.Writer, r *diff.Result) error
func Warnings(w io.Writer, files ...*envfile.File) error
```

Parser contract, decided here so later slices do not relitigate it:

- `KEY=value` and `export KEY=value` (the `export ` prefix is stripped).
- Matching outer `"` or `'` are stripped; the remaining bytes are **opaque**. No escape-sequence
  interpretation, no `${VAR}` expansion.
- `#` starts a comment **only** at the start of a line (after leading whitespace). Inline `#` is
  value data — `PASSWORD=abc#123` has the value `abc#123`.
- Leading UTF-8 BOM and trailing `\r` are stripped.
- Blank lines skipped. A line that is neither blank, comment, nor `KEY=...` is a `ParseError`,
  not a silent skip.
- Duplicate keys: last wins, recorded in `File.Duplicates`, warned on stderr.
- `Compare` sorts output keys lexicographically so runs are deterministic.

Masking: `Fingerprint` is `hex(HMAC-SHA256(salt, value))[:6]`, salt = 32 bytes from `crypto/rand`
generated once per process and never printed. Added, removed, and changed keys all get
fingerprints, so a reviewer can spot two keys sharing a value without seeing either.

## Modules touched

All files are created; nothing exists to change.

| File | Why |
| --- | --- |
| `go.mod` | CI's `setup-go` reads `go-version-file: go.mod`; without it every job fails. |
| `cmd/envdiff/main.go` | Flag parsing, wiring, and the only place `os.Exit` is called. |
| `cmd/envdiff/main_test.go` | Holds the leak test — the one assertion that spans every package. |
| `internal/envfile/envfile.go` | The `.env` grammar lives in exactly one place. |
| `internal/envfile/envfile_test.go` | Table-driven cases for quoting, `export`, inline `#`, CRLF, BOM, duplicates, malformed lines. |
| `internal/envfile/testdata/*.env` | Real fixture files; parser bugs are easiest to pin with real bytes. |
| `internal/mask/mask.go` | Single chokepoint where a plaintext value becomes a `mask.Print`. |
| `internal/mask/mask_test.go` | Asserts determinism within a salt, divergence across salts, and that no input substring survives into the output. |
| `internal/diff/diff.go` | Set comparison over keys, producing a struct that holds no plaintext. |
| `internal/diff/diff_test.go` | Added/removed/changed/same classification, sort order, empty-file edges. |
| `internal/report/report.go` | Rendering separated from comparison so the format is golden-testable. |
| `internal/report/report_test.go` | Golden strings for the text format and the stderr warnings. |

## Failure mode

**A secret reaches the ticket through a path that was never the masked one.**

This is the expensive failure: the output's entire promise is "safe to paste", so a single leak
means a credential sitting in a tracker that is indexed, emailed on every comment, and visible to
contractors. Rotation is the cheapest remedy and it is not cheap.

Masking at print time is what makes this happen. The `Text` renderer masks correctly, and then
something else prints: a `fmt.Errorf("bad value %q on line 12", line)` that quotes the offending
line verbatim, a `-format json` flag added in three months that marshals the struct directly, a
`log.Printf("%+v", result)` left in during debugging. Each of those is a one-line change by someone
who reasonably assumes masking already happened upstream, and each dumps plaintext — the parse-error
case into stderr, which people paste alongside stdout without thinking about it.

The design removes the opportunity rather than relying on discipline:

1. **Plaintext never enters a type that escapes the parser.** `diff.Result` stores `mask.Print`,
   never `string` values. `Compare` is the only consumer of `envfile.Entry.Value`, and it converts
   on read. Marshal `Result` any way you like — there is nothing in it to leak.
2. **`mask.Print` is a distinct type, not an alias.** Assigning a raw value where a fingerprint
   belongs is a compile error, so the future `-format json` author cannot make this mistake quietly;
   the build stops them.
3. **`ParseError` carries `Path`, `Line`, and a fixed `Reason` string — never the line content.**
   The parser is structured so the offending bytes are not in scope at the point the error is
   constructed.
4. **A leak test in `cmd/envdiff/main_test.go`** runs the real binary path over fixtures seeded with
   the sentinel `hunter2-TOTALLY-SECRET`, including deliberately malformed files that force the
   error path, and asserts the sentinel appears in neither stdout nor stderr. It fails loudly the
   first time someone adds an output path that bypasses the chokepoint.

## Options rejected

**Length-preserving masks (`DATABASE_URL=************`).** Loses on two counts: it leaks value
length, which narrows a brute-force meaningfully for short secrets, and it cannot distinguish
"changed" from "same" — the reviewer sees identical asterisks on both sides and has to trust the
`~` sigil with no way to confirm. Salted fingerprints cost the same to render and answer the
question the reader actually has.

**Unsalted `sha256(value)[:8]`.** Reproducible across runs, which is genuinely nicer. Rejected
because `.env` files are full of low-entropy values — `8080`, `true`, `postgres`, `development`,
`localhost` — and an unsalted digest of those is a dictionary lookup, not a mask. Anyone holding
the ticket recovers them in seconds. Per-run salt gives up cross-run comparability; the footer line
states that limitation rather than hiding it.

**Reusing `github.com/joho/godotenv` for parsing.** It is the obvious dependency and it loses on
the thing this tool is about. It applies its own escape-sequence and inline-comment rules, so
`PASSWORD=abc#123` silently becomes `abc` and a diff reports a change that does not exist. It hands
back a `map[string]string` of live plaintext that we would then have to carefully avoid printing —
exactly the discipline-dependent shape the failure-mode section is designed to eliminate. Roughly
80 lines of stdlib parsing buys a grammar we control and a zero-dependency build.

**Wrapping `diff(1)` or a line-based text diff.** Cheapest to build and wrong at the unit of
comparison: reordering keys, editing a comment, or reflowing whitespace all report as differences,
while the one thing that matters — same key, different value — is indistinguishable from an
add plus a remove. The unit here is the key, not the line.

**Shipping `-format json` in v1.** Useful for CI consumers and it is the natural next flag, but the
stated use case is pasting into a ticket, which is text. Deferred rather than rejected outright —
decisions 1 and 2 above exist specifically so adding it later cannot leak.

**An `ENVDIFF_SALT` environment variable for reproducible fingerprints across runs.** Tempting for
comparing two tickets, but a salt that lives in the environment is a salt that ends up committed to
a CI config, at which point every fingerprint ever published becomes brute-forceable retroactively.
Not worth the convenience.
