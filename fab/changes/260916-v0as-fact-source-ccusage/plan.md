# Plan: Fact Type and ccusage Source (Go port row V1)

**Change**: 260916-v0as-fact-source-ccusage
**Intake**: `intake.md`

## Requirements

> All packages live under `src/go/internal/`. Go 1.25, module `github.com/sahil87/tu`, **no new `require` entries in `go.mod`**. Nothing in this change touches `src/go/cmd/tu/`, `src/node/`, the justfile, CI, or `docs/specs/`. Tests are `_test.go` siblings (constitution § Go Transition).

### fact: the one record type

#### R1: Record and Totals shape
`package fact` SHALL define exactly:

```go
type Record struct {
    Date    string // ISO label: "YYYY-MM-DD" or "YYYY-MM"
    Tool    string // registry key: cc, codex, oc, gemini, copilot, kimi
    User    string
    Machine string
    Totals
}

type Totals struct {
    TotalCost           float64 `json:"totalCost"`
    InputTokens         int64   `json:"inputTokens"`
    OutputTokens        int64   `json:"outputTokens"`
    CacheCreationTokens int64   `json:"cacheCreationTokens"`
    CacheReadTokens     int64   `json:"cacheReadTokens"`
    TotalTokens         int64   `json:"totalTokens"`
}
```

`Date` MUST be a string, never `time.Time`. `TotalTokens` is a stored field — nothing in this change recomputes it from the other four. `Record`'s four string fields carry no JSON tags (Go's default names) — the pinned external names apply to `Totals` only.

- **GIVEN** a `Totals{TotalCost: 0.5, InputTokens: 3000}`
- **WHEN** encoded with `encoding/json`
- **THEN** the output contains `"totalCost":0.5` and `"inputTokens":3000` and the four other pinned keys

#### R2: Totals arithmetic is pure
`func (t Totals) Add(o Totals) Totals` MUST return the field-wise sum without mutating either receiver or argument. `func (t Totals) IsZero() bool` MUST report true only when all six fields are zero. The zero value `Totals{}` is the "no data" value (the TS `EMPTY`).

- **GIVEN** `a := Totals{TotalCost: 1, InputTokens: 2}` and `b := Totals{TotalCost: 0.5, OutputTokens: 3}`
- **WHEN** `c := a.Add(b)`
- **THEN** `c == Totals{TotalCost: 1.5, InputTokens: 2, OutputTokens: 3}` and `a`, `b` are unchanged and `Totals{}.IsZero()` is true and `c.IsZero()` is false

### source: typed per-source error and the edge warning

#### R3: Error kinds
`package source` SHALL define `type Kind int` with constants `KindExec`, `KindTimeout`, `KindParse` (in that order, `iota`), and

```go
type Error struct {
    Tool   string // registry key
    Name   string // display name
    Kind   Kind
    Detail string // pre-rendered human detail (see R4)
    Err    error  // wrapped cause, may be nil
}
```

implementing `error` (`Error() string` — any readable form that includes Name and Detail) and `Unwrap() error`. `func (e *Error) Warns() bool` MUST return true for `KindExec` and `KindTimeout` and false for `KindParse`. Source packages MUST never write to stdout/stderr and MUST never panic on a source failure: failures are returned as `*source.Error` values.

- **GIVEN** an `Error{Kind: KindParse}`
- **WHEN** `Warns()` is called
- **THEN** it returns false; for `KindExec` and `KindTimeout` it returns true

#### R4: Warning line and WriteWarnings
`func (e *Error) Warning() string` MUST return exactly `warning: {Name} fetch failed ({Detail}), showing zero data` (no trailing newline). `func WriteWarnings(w io.Writer, errs []*Error)` MUST write `Warning() + "\n"` for each error whose `Warns()` is true, in the order given, and nothing for the others. This is the only function in the `source` tree that touches an `io.Writer`.

`Detail` is composed by the ccusage adapter (R8) in these shapes:

| Failure | `Detail` |
|---------|----------|
| non-zero exit or signal | `Command failed: {binary} {args joined by single spaces}` + `"\n"` + captured stderr bytes verbatim (so an empty stderr leaves a trailing newline before the closing paren) |
| binary not found (`exec.ErrNotFound` or `fs.ErrNotExist` in the chain) | `spawn {binary} ENOENT` |
| other start failure | `spawn {binary} ` + Go's error text |
| context deadline | `timeout after {d}` where `d` is `time.Duration.String()` of the elapsed run time rounded to milliseconds |

- **GIVEN** `Error{Name: "Kimi", Kind: KindExec, Detail: "Command failed: /x/ccusage kimi daily --json\nboom\n"}`
- **WHEN** `Warning()` is called
- **THEN** it returns `"warning: Kimi fetch failed (Command failed: /x/ccusage kimi daily --json\nboom\n), showing zero data"`
- **GIVEN** `errs := []*Error{execErr, parseErr, timeoutErr}`
- **WHEN** `WriteWarnings(&buf, errs)`
- **THEN** `buf` holds exactly two lines, the exec warning then the timeout warning

### source/ccusage: registry, exec, normalize, fetch

#### R5: Ordered six-tool registry
`package ccusage` SHALL define

```go
type Tool struct {
    Key        string
    Name       string
    PrefixArgs []string
    LabelKey   string
}
var Tools = []Tool{ /* exactly this order */
    {"cc", "Claude Code", []string{"claude"}, "date"},
    {"codex", "Codex", []string{"codex"}, "date"},
    {"oc", "OpenCode", []string{"opencode"}, "date"},
    {"gemini", "Gemini", []string{"gemini"}, "date"},
    {"copilot", "Copilot", []string{"copilot"}, "date"},
    {"kimi", "Kimi", []string{"kimi"}, "date"},
}
func Lookup(key string) (Tool, bool)
const PeriodDaily = "daily"
```

`Tools` MUST be a slice (ordered), never a map. Source aliases (`co`, `gem`, `cop`, `ki`) are NOT part of this package.

- **GIVEN** the registry
- **WHEN** iterated
- **THEN** keys are `cc, codex, oc, gemini, copilot, kimi` in that order and every `LabelKey` is `date`; `Lookup("gemini")` returns the Gemini tool; `Lookup("gem")` returns `ok == false`

#### R6: argv composition
The argv handed to the binary for a fetch MUST be `append(append(append([]string{}, tool.PrefixArgs...), period, "--json"), extraArgs...)` — e.g. `claude daily --json`. No shell is involved (`os/exec` with an argv slice).

- **GIVEN** the `cc` tool, period `daily`, no extra args
- **WHEN** a fetch runs against the fake ccusage with `TUDIFF_CALL_LOG` set
- **THEN** the logged `argv` is exactly `["claude","daily","--json"]`

#### R7: Binary resolution
`func ResolveBinary() (string, error)` SHALL return, in order: (1) `filepath.Join(filepath.Dir(exe), "vendor", "ccusage", "bin", "ccusage")` where `exe` is `os.Executable()` (symlinks resolved via `filepath.EvalSymlinks` when possible), if that path exists; (2) `exec.LookPath("ccusage")`; (3) otherwise an error wrapping `exec.ErrNotFound`. It MUST NOT walk to a repo root or probe `node_modules`. `Source.Binary` non-empty bypasses resolution entirely.

- **GIVEN** no vendor sibling and no `ccusage` on `PATH`
- **WHEN** `Fetch` runs with `Binary == ""`
- **THEN** it returns `(nil, err)` with `err.Kind == KindExec` and `err.Detail == "spawn ccusage ENOENT"`

#### R8: Exec semantics
The adapter SHALL run the binary with `exec.CommandContext(ctx, binary, argv...)`, inherited environment and cwd, stdout and stderr captured into separate buffers with **no size cap**, and the source MUST impose no deadline of its own — `const DefaultTimeout = 120 * time.Second` is exported for the caller to use. Classification of a failed run: `ctx.Err() == context.DeadlineExceeded` → `KindTimeout`; a start failure → `KindExec` with the `spawn` detail; an `*exec.ExitError` (non-zero exit or signal) → `KindExec` with the `Command failed` detail (R4 table). ccusage's stderr on a **successful** run is discarded, never forwarded.

- **GIVEN** a `#!/bin/sh` stub that runs `sleep 5` and a context with a 200 ms timeout
- **WHEN** `Fetch` runs
- **THEN** it returns within ~1 s with `err.Kind == KindTimeout` and `err.Detail` starting `timeout after `
- **GIVEN** the fake ccusage and an argv it has no fixture for
- **WHEN** `Fetch` runs
- **THEN** `err.Kind == KindExec` and `err.Detail` starts with `Command failed: {binary} kimi weekly --json\nfakeccusage: no fixture for argv`

#### R9: Parse coercion and label normalization
`func Parse(raw []byte, tool Tool) ([]fact.Record, *source.Error)` SHALL decode `{"daily": [ ... ]}` and, for each entry, build a `fact.Record` with `Tool = tool.Key`, empty `User`/`Machine`, and:

- `TotalCost` ← `totalCost`, else `costUSD`, else 0
- `CacheReadTokens` ← `cacheReadTokens`, else `cachedInputTokens`, else 0
- `InputTokens`, `OutputTokens`, `CacheCreationTokens`, `TotalTokens` ← same-named keys, else 0
- a present key whose value is not a JSON number → 0 (never an error)
- `Date` ← `normalizeLabel(entry[tool.LabelKey] as string)` where `normalizeLabel` passes ISO through unchanged, maps `Feb 14, 2026` → `2026-02-14`, `Feb 2026` → `2026-02`, an unknown 3-letter month → `00`, a day is zero-padded, and a missing/non-string label → `""`

The `totals` object is ignored. Decoding MUST tolerate unknown keys (`modelBreakdowns`, `models`, `reasoningOutputTokens`, …).

- **GIVEN** `harness/fixtures/_placeholder/codex/daily.json`
- **WHEN** parsed with the codex tool
- **THEN** 3 records dated `2026-01-05`, `2026-01-06`, `2026-01-07`, each `Totals{0.5, 3000, 400, 1000, 20000, 24400}` with `Tool == "codex"` (the `costUSD` path)
- **GIVEN** each of the other five placeholder fixtures with its own tool
- **WHEN** parsed
- **THEN** the same three dates and the same totals, and `InputTokens+OutputTokens+CacheCreationTokens+CacheReadTokens == TotalTokens` for every record
- **GIVEN** `{"daily":[{"date":"Feb 14, 2026","inputTokens":"12","cachedInputTokens":7}]}`
- **WHEN** parsed
- **THEN** one record with `Date == "2026-02-14"`, `InputTokens == 0`, `CacheReadTokens == 7`

#### R10: Parse edge cases
`Parse` MUST return `([]fact.Record{}, nil)` (non-nil empty slice, no error) for `{"daily":[]}` and for a document whose `daily` is absent-but-object-valid is treated as `KindParse`. It MUST return `(nil, &source.Error{Kind: KindParse})` when `raw` is empty/whitespace, is not a JSON object, or has a `daily` that is missing or not an array.

- **GIVEN** `{"daily":[],"totals":{"totalCost":-0.0}}`
- **WHEN** parsed
- **THEN** zero records and a nil error
- **GIVEN** `""`, `"not json"`, `"[]"`, `{"totals":{}}`
- **WHEN** parsed
- **THEN** each returns `err.Kind == KindParse`

#### R11: Fetch order of operations
```go
type Source struct {
    Binary  string
    User    string
    Machine string
    Cache   *cache.Store // nil → no caching
}
func (s *Source) Fetch(ctx context.Context, tool Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)
```

`Fetch` SHALL, in order: (1) when `!fresh && s.Cache != nil`, `Cache.Get(key)` and on a hit return the records with `User`/`Machine` stamped; (2) resolve the binary (R7) and run (R8); on error return `(nil, err)` with **no cache write**; (3) `Parse` (R9/R10); on `KindParse` return `(nil, err)` with no cache write; on zero records return `([]fact.Record{}, nil)` with **no cache write**; (4) on one or more records, `Cache.Put(key, records)` **before** stamping (cached records carry empty `User`/`Machine`; `Put` errors are ignored), then stamp `User`/`Machine` on every record and return them. `fresh` skips step 1 but never step 4. The cache key is `cache.Key{Tool: tool.Key, Period: period, Args: extraArgs}`.

- **GIVEN** a `Source` with a temp-dir `Cache` and the fake ccusage
- **WHEN** `Fetch(cc)` runs twice within the TTL
- **THEN** the call log has exactly one ccusage line, both results are equal, and every record has the configured `User` and `Machine`
- **GIVEN** the same source
- **WHEN** `Fetch(cc, fresh=true)` runs
- **THEN** the call log gains a line and the cache file's mtime is refreshed
- **GIVEN** a fixture whose `daily` is empty (a stub binary printing `{"daily":[]}`)
- **WHEN** `Fetch` runs with a cache
- **THEN** zero records, nil error, and no file under the cache dir

#### R12: FetchAll collects everything in registry order
`func (s *Source) FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)` SHALL call `Fetch` for every entry of `Tools` concurrently (one goroutine each, `sync.WaitGroup`, one result slot per index — **no** `golang.org/x/sync`), never cancelling siblings on a failure, and return the records concatenated in registry order followed by the non-nil errors in registry order. A tool that failed contributes no records.

- **GIVEN** the fake ccusage serving all six placeholder fixtures and an extra tool-miss induced for one tool (e.g. an unknown period)
- **WHEN** `FetchAll` runs with the placeholder corpus for period `daily`
- **THEN** 18 records ordered cc×3, codex×3, oc×3, gemini×3, copilot×3, kimi×3 and zero errors; with period `weekly` (no fixtures) six `KindExec` errors in registry order and zero records

### source/cache: on-disk JSON, hash-keyed, 60 s TTL

#### R13: Key, filename, envelope, TTL
```go
const TTL = 60 * time.Second
type Key struct { Tool, Period string; Args []string }
func (k Key) Filename() string
type Store struct { Dir string; TTL time.Duration; Now func() time.Time }
func Default() (*Store, error)
func (s *Store) Get(k Key) ([]fact.Record, bool)
func (s *Store) Put(k Key, recs []fact.Record) error
```

`Filename()` MUST be `{Tool}-{Period}-{h}.json` where `h` is the first 16 hex characters of `sha256(Tool + "\x00" + Period + "\x00" + strings.Join(Args, "\x00"))`. `Default()` MUST return `Dir = $HOME/.tu/cache`, `TTL = TTL`, `Now = time.Now`, or an error when `$HOME` is empty. `Put` MUST `MkdirAll(Dir, 0o755)` and write the envelope `{"v":1,"tool":…,"period":…,"args":[…],"records":[…]}` (encode `Args` as `[]` not `null` when empty). `Get` MUST return `(nil, false)` when the file is absent, when `Now() - mtime > TTL` (a zero `TTL` field means use the package `TTL`), when the file does not decode, or when the envelope's `v`/`tool`/`period`/`args` differ from the key; otherwise `(records, true)`.

- **GIVEN** a `Store` with a temp `Dir` and a controllable `Now`
- **WHEN** `Put(k, recs)` then `Get(k)`
- **THEN** the records round-trip; after advancing `Now` by 61 s `Get` misses; a file whose envelope says `tool: codex` under key `cc` misses; a corrupt file misses; `Key{"cc","daily",nil}.Filename()` is stable across calls and differs from `Key{"cc","daily",[]string{"--x"}}.Filename()` and from `Key{"codex","daily",nil}.Filename()`

### Tests

#### R14: Tests against the P3 placeholder corpus, deterministic
Adapter tests SHALL read fixtures via the relative walk `../../../../../harness/fixtures/_placeholder` (the `harness/corpus_test.go` convention shifted one level), and `source_test.go` SHALL have a `TestMain` that builds `../../../cmd/fakeccusage` with `go build -o <tmpdir>/ccusage` and sets `TUDIFF_FIXTURES` to the `_placeholder` directory **only** (never a local capture). Tests MUST fail, not skip, when the fixtures directory is absent. `go test ./...`, `gofmt -l`, and `go vet ./...` under `src/go/` MUST be clean.

- **GIVEN** a clean checkout with `go` on `PATH`
- **WHEN** `just go-lint && just go-test` run
- **THEN** both exit 0 and the new packages' tests appear in the output

### Non-Goals

- `query` (filters, roll-ups, current-label pick), `view`, `render`, `command` — V2.
- `source/metrics` (repo reader) — B3. `config` — B1.
- Wiring anything into `src/go/cmd/tu/main.go`; changing `src/node/`, specs, CI, or the justfile.
- Porting `needsFilter`/`stripNoise` or the 10 MB `maxBuffer`.

### Design Decisions

#### Date is an ISO label string, not time.Time
**Decision**: `fact.Record.Date` is `string` in the two Principle-V forms.
**Why**: `YYYY-MM` roll-up labels have no instant; every TS filter relies on lexicographic ISO order; it keeps the fact type free of time-zone semantics the render layer never needs.
**Rejected**: `time.Time` (forces a fake instant for months and re-introduces the 31 `new Date()` sites' local/UTC ambiguity into the data layer).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### Registry is an ordered slice
**Decision**: `ccusage.Tools` is `[]Tool`; `Lookup` scans it.
**Why**: Go maps are unordered and insertion order is the all-tools column order (Output Stability).
**Rejected**: `map[string]Tool` plus a separate order slice (two sources of truth).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### needsFilter / stripNoise not ported
**Decision**: Non-JSON stdout is a `KindParse` error; no `[`-line stripping.
**Why**: Memory records it as a defensive no-op since ccusage v20 emits clean JSON (live-verified); the mechanism is a five-line add-back if a future ccusage regresses.
**Rejected**: Porting it for every tool (would silently mask a real upstream breakage).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### Caller-owned timeout with an exported default
**Decision**: `Fetch` honors the given `context.Context` only; `DefaultTimeout = 120s` is exported for the command edge.
**Why**: The TS has no timeout, so any value is an addition; the edge is where a deadline is policy; 120 s matches the harness capture bound.
**Rejected**: A hard-coded internal deadline (untestable without sleeping 120 s; hides policy in the adapter).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### Warning detail mimics Node's error.message
**Decision**: `Detail` reproduces `Command failed: {cmd}\n{stderr}` and `spawn {binary} ENOENT`.
**Why**: The stderr line is a harness-compared byte surface; the only non-reproducible component is the absolute binary path, which P4 can equalize by staging both binaries under one `dist/`.
**Rejected**: Go-native error text (guarantees a diff on every failure fixture).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### No cache write on failure or empty results
**Decision**: Only a non-empty parse result is written to the cache.
**Why**: `fetchHistory` returns before `writeCache` in all three cases; the harness call-log comparison would flag Go for skipping a ccusage call the TS makes. The spec sentence saying otherwise is flagged in the intake for a human correction.
**Rejected**: Caching empties (spec-literal but binary-divergent).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### Cache records are stored unstamped
**Decision**: `Put` receives records with empty `User`/`Machine`; `Fetch` stamps after read/write.
**Why**: The key excludes user and machine, so a config change inside the TTL must not serve a stale identity.
**Rejected**: Including user/machine in the key (invalidates the cache on every identity change for data that did not change).
*Introduced by*: 260916-v0as-fact-source-ccusage

#### WaitGroup, not errgroup
**Decision**: `FetchAll` uses `sync.WaitGroup` with per-index result slots.
**Why**: "Collected, never thrown through" is the opposite of errgroup's cancel-on-first-error; keeps the module dependency-free.
**Rejected**: `golang.org/x/sync/errgroup` (first dependency, wrong semantics for collect-all).
*Introduced by*: 260916-v0as-fact-source-ccusage

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/fact/fact.go` (package doc, `Record`, `Totals` with the pinned JSON tags, `Add`, `IsZero`) and `src/go/internal/fact/fact_test.go` (Add purity, IsZero, JSON tag round-trip) <!-- R1, R2 -->
- [x] T002 [P] Create `src/go/internal/source/error.go` (`Kind`, `Error`, `Error()`, `Unwrap()`, `Warns()`, `Warning()`, `WriteWarnings`) and `src/go/internal/source/error_test.go` (byte-exact warning for the Exec/ENOENT/Timeout shapes, `Warns` per kind, `WriteWarnings` order and filtering) <!-- R3, R4 -->

### Phase 2: Core Implementation

- [x] T003 [P] Create `src/go/internal/source/ccusage/registry.go` (`Tool`, `Tools` in the pinned order, `Lookup`, `PeriodDaily`, `argv(tool, period, extra)` helper) and `registry_test.go` (order, names, prefix args, label keys, `Lookup` miss, argv composition) <!-- R5, R6 -->
- [x] T004 [P] Create `src/go/internal/source/ccusage/normalize.go` (`Parse`, `normalizeLabel`, numeric coercion with the `costUSD`/`cachedInputTokens` fallbacks) and `normalize_test.go` reading all six `../../../../../harness/fixtures/_placeholder/<source>/daily.json` files plus the hand-written coercion, label, empty-daily, and garbage cases; the test fails (not skips) if the fixtures dir is absent <!-- R9, R10 -->
- [x] T005 [P] Create `src/go/internal/source/cache/cache.go` (`TTL`, `Key`, `Filename`, `Store`, `Default`, `Get`, `Put`, envelope struct) and `cache_test.go` (round-trip, TTL expiry via injected `Now`, envelope mismatch, corrupt file, filename stability/distinctness, `Default` with `HOME` unset) <!-- R13 -->
- [x] T006 Create `src/go/internal/source/ccusage/exec.go` (`DefaultTimeout`, `ResolveBinary`, `run(ctx, binary, argv) ([]byte, *source.Error)` with the R4/R8 classification and Node-shaped details) <!-- R7, R8 -->
- [x] T007 Create `src/go/internal/source/ccusage/source.go` (`Source`, `Fetch` per R11, `FetchAll` per R12 with `sync.WaitGroup`) <!-- R11, R12 -->

### Phase 3: Integration & Edge Cases

- [x] T008 Create `src/go/internal/source/ccusage/source_test.go` with a `TestMain` that builds `../../../cmd/fakeccusage` into a temp dir and sets `TUDIFF_FIXTURES` to the `_placeholder` dir only; tests: `Fetch` of each tool returns the placeholder records stamped with `User`/`Machine`; argv via `TUDIFF_CALL_LOG`; fixture miss → `KindExec` with the `Command failed … fakeccusage: no fixture for argv` detail; nonexistent binary → `spawn … ENOENT`; sleeping `sh` stub under a 200 ms context → `KindTimeout`; `FetchAll` daily → 18 records in registry order and no errors; `FetchAll` weekly → 6 errors in registry order and no records <!-- R6, R7, R8, R11, R12 -->
- [x] T009 Add the cache-through-`Source` tests to `source_test.go`: second `Fetch` inside the TTL makes no ccusage call and stamps `User`/`Machine`; `fresh` re-invokes and rewrites; a stub printing `{"daily":[]}` writes no cache file; a failing fetch writes no cache file <!-- R11 -->
- [x] T010 Run `just go-lint` and `just go-test` from the repo root until both are clean; fix any gofmt/vet/test failures in the new packages only <!-- R14 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `fact.Record` and `fact.Totals` exist with exactly the specified fields, types, and JSON tags; `Date` is a string
- [x] A-002 R2: `Totals.Add` is pure and field-wise; `IsZero` is true only for the zero value
- [x] A-003 R3: `source.Kind` has `KindExec`, `KindTimeout`, `KindParse`; `source.Error` implements `error` and `Unwrap`; `Warns()` is true for Exec/Timeout and false for Parse
- [x] A-004 R4: `Warning()` is byte-exact `warning: {Name} fetch failed ({Detail}), showing zero data`; `WriteWarnings` writes only warning kinds, in order, one per line
- [x] A-005 R5: `ccusage.Tools` is a slice of six tools in the order cc, codex, oc, gemini, copilot, kimi with the pinned names, prefix args, and `date` label keys; `Lookup` works and misses cleanly
- [x] A-006 R6: argv is `PrefixArgs… period --json extra…`, proven through the fake's call log
- [x] A-007 R7: `ResolveBinary` checks the vendor sibling of the executable, then `PATH`, then errors; `Source.Binary` bypasses it
- [x] A-008 R8: exec uses `CommandContext`, captures stdout/stderr without a cap, classifies deadline/start/exit failures into the three kinds with the Node-shaped details; `DefaultTimeout` is exported and unused inside the package
- [x] A-009 R9: `Parse` applies the `costUSD` and `cachedInputTokens` fallbacks, zeroes missing/non-numeric fields, normalizes labels, ignores `totals` and unknown keys
- [x] A-010 R11: `Fetch` follows the cache-read → exec → parse → cache-write → stamp order; no cache write on failure or empty; `fresh` skips the read only
- [x] A-011 R12: `FetchAll` runs concurrently with `sync.WaitGroup`, never cancels siblings, and returns records and errors in registry order
- [x] A-012 R13: `cache.Key.Filename` is `{tool}-{period}-{sha256[:16]}.json`; the envelope carries `v`, `tool`, `period`, `args`, `records` and is verified on `Get`; TTL is by mtime; `Default` errors on empty `$HOME`

### Behavioral Correctness

- [x] A-013 R10: `{"daily":[]}` yields an empty non-nil slice and nil error; empty/garbage/array/`daily`-missing documents yield `KindParse`
- [x] A-014 R4: an empty captured stderr still produces the `\n` before the closing paren in the warning (Node parity)

### Scenario Coverage

- [x] A-015 R9: all six placeholder fixtures parse to three records dated 2026-01-05..07 with `Totals{0.5, 3000, 400, 1000, 20000, 24400}` and the four-counter sum equals `TotalTokens`
- [x] A-016 R12: `FetchAll` over the placeholder corpus returns 18 records in registry order and zero errors; `FetchAll` for `weekly` returns six `KindExec` errors in registry order
- [x] A-017 R11: a second `Fetch` within the TTL makes no ccusage call and stamps `User`/`Machine` from the `Source`

### Edge Cases & Error Handling

- [x] A-018 R8: a 200 ms context against a sleeping stub yields `KindTimeout` promptly with a `timeout after ` detail
- [x] A-019 R7: a missing binary yields `KindExec` with `spawn {binary} ENOENT`
- [x] A-020 R13: expired, corrupt, and envelope-mismatched cache files all miss; `Put` errors are non-fatal to `Fetch`

### Code Quality

- [x] A-021 Pattern consistency: new code follows the `internal/harness` package style (package doc comment, exported-identifier comments, table-driven tests, relative fixture walk, testable seams)
- [x] A-022 No unnecessary duplication: one `argv` helper, one label normalizer, one envelope type; nothing re-implements `encoding/json` or `filepath` helpers
- [x] A-023 No magic values: `TTL`, `DefaultTimeout`, `PeriodDaily`, the envelope version, and the hash length are named constants
- [x] A-024 Errors are never swallowed silently: every failure path returns a `*source.Error` or a documented fallback (`Put` errors ignored by design, stated in a comment)
- [x] A-025 Minimum pathways: cache read/write lives only in `Fetch`; no second fetch path exists
- [x] A-026 R14: `go.mod` unchanged; no files outside `src/go/internal/{fact,source}` changed; `just go-lint` and `just go-test` pass

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- The working tree IS the deliverable until ship commits it: reviewers revert their own experiments by editing back, never with `git checkout`, `git restore`, `git stash`, or `git reset`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `Error()` string form is `{Name}: {Detail}` (free-form; only `Warning()` is byte-pinned) | Nothing external reads `Error()`; the pinned surface is the warning line | S:60 R:95 A:85 D:80 |
| 2 | Confident | Timeout detail elapsed time is rounded to milliseconds via `Duration.String()` | Intake left the exact rendering open; tests assert only the prefix | S:50 R:95 A:80 D:70 |
| 3 | Confident | `ResolveBinary` resolves `os.Executable()` symlinks before taking its directory | Homebrew installs the binary behind a `bin/` symlink; the vendor sibling lives beside the real file | S:55 R:90 A:80 D:75 |
| 4 | Confident | A zero `Store.TTL` means the package `TTL` (so a literal `&Store{Dir: d}` works) | Ergonomic default; tests set it explicitly when they need to | S:50 R:95 A:85 D:75 |

4 assumptions (0 certain, 4 confident, 0 tentative).


## Deletion Candidates

None — this change adds new functionality without making existing code redundant.
