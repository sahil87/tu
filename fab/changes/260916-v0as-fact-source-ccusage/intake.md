# Intake: Fact Type and ccusage Source (Go port row V1)

**Change**: 260916-v0as-fact-source-ccusage
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation, handed over from the Go-port plan's queue (plan row V1, the first Phase 1 row):

> Context: fab/plans/sahil/26-09-15-go-port.md, row V1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Build the fact package (the one fact type: Date, Tool, User, Machine, Totals; Totals is cost plus five token counters; ISO date labels only) and source/ccusage (exec with context timeout, JSON normalize, six-tool registry with labelKey/prefixArgs) and source/cache (on-disk JSON keyed by hash of tool+period+args, 60s TTL). Typed per-source errors collected, never thrown through; a stderr warning at the edge. Unit tests against the P3 harness fixtures.

No prior discussion in this conversation. The plan's Decisions (D1–D13) and Target architecture table were read and are the binding design; the current TypeScript `src/node/core/fetcher.ts` / `types.ts`, the spec (`docs/specs/usage.md` § Data Model, § Data Flow), the memory (`docs/memory/cli/data-pipeline.md`, `docs/memory/harness/differential-harness.md`) and the P3a harness code (`src/go/internal/harness/`, `harness/fixtures/_placeholder/`) were read to ground every value below.

Plan context that shapes this change:

- **D2** — Go lands dark in `main`; nothing here ships. `src/go/cmd/tu/main.go` keeps printing `tu: not implemented (Go port in progress)` for every non-version invocation. V1 adds packages only; the first wiring into `cmd/tu` is V2.
- **D5** — layout `src/go/internal/<pkg>/`, Go 1.25, zero dependencies so far (`go.mod` has no `require`).
- **Target architecture** — `fact` is "the one fact type: `{Date, Tool, User, Machine, Totals}` and `Totals` (cost + five token counters). ISO labels only (Principle V)"; `source` holds adapters producing `[]fact.Record` — `ccusage` (exec with context timeout, JSON normalize, per-tool `labelKey`), `cache` (on-disk JSON keyed by hash of tool+period+args, 60 s TTL); "typed per-source errors collected, never thrown through". `metrics` (the repo reader) is **B3**, not V1.
- **Goal** — external surfaces are frozen. V1 touches none of them, but two of its outputs are *future* surface bytes: the `warning: {Tool} fetch failed (...), showing zero data` stderr line, and the JSON field names on `Totals` that `render/json` (V2) will emit.
- **G1** — Sahil reviews V1+V2 as a unit (package boundaries, error flow, test shape) before Phase 2 stacks seven rows on the layering. This intake records every layering choice so G1 can see and reverse it cheaply.

## Why

The Go port is a pipeline of pure stages with I/O only at the ends. `fact` and `source` are the input end, and everything downstream (`query`, `view`, `render`, `command`, `sync`) is typed against `fact.Record`. Getting this type and the one impure adapter right first is what lets V2 validate the layering with a real end-to-end snapshot before Phase 2 builds breadth on it.

The TypeScript side these packages replace is the tangle the plan names: `fetcher.ts` mixes exec, parse, cache, and aggregation; tool/user/machine live as map keys and tuple shapes rather than as dimensions of one record; a failed fetch is logged from inside the exec wrapper (`console.warn` at `fetcher.ts:134`) and silently becomes `""`, so the output channel leaks into the data layer. V1 fixes the shape without changing behavior: one record type with the three dimensions as fields, an adapter that returns records plus a typed error instead of printing, and a cache that is a separate package keyed on what actually determines the result.

Doing this as its own row rather than folded into V2: the exec/normalize/cache code is where the ccusage v20 shape knowledge lives (the codex `costUSD` outlier, the `date` label key, the empty-agent `daily: []` case), and it is the only part of Phase 1 that can be unit-tested against the P3 fixture corpus without any rendering. A small, fully tested input layer is the right thing to hand V2.

## What Changes

### 1. Package layout (all new, under `src/go/internal/`)

```
src/go/internal/
  fact/
    fact.go            Record, Totals, Add, IsZero
    fact_test.go
  source/
    error.go           Error, Kind, Warning(), WriteWarnings()
    error_test.go
    ccusage/
      registry.go      Tool, Tools (ordered), Lookup
      exec.go          binary resolution, run (CommandContext)
      normalize.go     Parse, normalizeLabel, totals coercion
      source.go        Source, Fetch, FetchAll
      *_test.go
    cache/
      cache.go         Store, Key, Get, Put
      cache_test.go
```

No new module dependencies (`go.mod` stays `require`-free). `just go-build` / `go-test` / `go-lint` and the `go-build-and-test` CI lane pick the packages up through their existing `./...` sweeps — no justfile or workflow edit. `src/go/cmd/tu/main.go` is not touched.

### 2. `fact` — the one record type

```go
package fact

// Record is one observed usage fact: what a tool cost a user on a machine
// on a date. Every downstream stage consumes []Record and nothing else.
type Record struct {
    Date    string // ISO label: "YYYY-MM-DD" (daily, and a week's Sunday) or "YYYY-MM" (monthly)
    Tool    string // registry key: cc, codex, oc, gemini, copilot, kimi
    User    string
    Machine string
    Totals
}

// Totals is cost plus the five token counters. Field names and JSON tags are the
// pinned external names from docs/specs/usage.md § Data Model, so render/json
// (V2) and the cache envelope can encode the struct directly.
type Totals struct {
    TotalCost           float64 `json:"totalCost"`
    InputTokens         int64   `json:"inputTokens"`
    OutputTokens        int64   `json:"outputTokens"`
    CacheCreationTokens int64   `json:"cacheCreationTokens"`
    CacheReadTokens     int64   `json:"cacheReadTokens"`
    TotalTokens         int64   `json:"totalTokens"`
}

func (t Totals) Add(o Totals) Totals   // field-wise sum; pure
func (t Totals) IsZero() bool          // all six fields zero
```

- `Date` is a **string label, not `time.Time`**: Principle V fixes the label formats, weekly/monthly roll-ups (V2 `query`) produce `YYYY-MM` labels that have no single instant, and lexicographic order on ISO labels is the total order every filter in the TS code relies on.
- `TotalTokens` is **stored, not derived** — the TS reads it from ccusage JSON as-is and sums it in aggregation; the placeholder corpus keeps it equal to the four-counter sum, but the adapter must not recompute it (byte parity on the Total column depends on ccusage's own value).
- The zero `Totals{}` is the TS `EMPTY`.
- `Tool` holds the **registry key** (`cc`), not the display name — the metrics-repo file layout (`{toolKey}-*.jsonl`) and the cache are keyed on it; the display name (`Claude Code`) is a registry lookup at render time.

### 3. `source` — the typed per-source error and the edge warning

```go
package source

type Kind int

const (
    KindExec    Kind = iota // binary missing, spawn failure, non-zero exit, signal
    KindTimeout             // the caller's context deadline fired
    KindParse               // stdout was not the expected JSON document
)

// Error is one source's failure. It is returned beside the (zero) records,
// never panicked or written anywhere by the source packages.
type Error struct {
    Tool   string // registry key
    Name   string // display name, for the warning line
    Kind   Kind
    Detail string // human detail, pre-rendered (see Warning)
    Err    error  // wrapped cause
}

func (e *Error) Error() string
func (e *Error) Unwrap() error

// Warns reports whether this error produces a stderr line in the TS binary.
// Exec and Timeout do; Parse does not (the TS returns [] silently on bad JSON).
func (e *Error) Warns() bool

// Warning is the byte-exact TS line:
//   warning: {Name} fetch failed ({Detail}), showing zero data
func (e *Error) Warning() string

// WriteWarnings writes Warning()+"\n" for every error with Warns() true, in the
// order given. This is the ONLY place a source error reaches an io.Writer, and
// it is called by the command edge (V2), never by a source.
func WriteWarnings(w io.Writer, errs []*Error)
```

`Detail` mirrors Node's `error.message` shapes for the two realistic failures, so the harness has a chance at byte parity on this line:

| Failure | TS detail (Node `execFile`) | Go `Detail` |
|---------|-----------------------------|-------------|
| Non-zero exit or signal | `Command failed: {binary} {args joined by " "}\n{captured stderr}` | identical text (stderr captured, not forwarded) |
| Binary not found | `spawn {binary} ENOENT` | `spawn {binary} ENOENT` when the error is `exec.ErrNotFound` / `fs.ErrNotExist` |
| Other spawn error | `spawn {binary} {errno}` | Go's native error text |
| Context deadline | *(none — TS has no timeout)* | `timeout after {d}` |

Note the TS embeds the **absolute** ccusage path in the message, so byte parity on this line depends on P4 staging the fake ccusage at the same vendor path for both binaries (see Open Questions). ccusage's stderr is **captured and swallowed on success**, exactly as `execFile` buffers it.

### 4. `source/ccusage` — registry, exec, normalize, fetch

#### Registry (ordered)

```go
type Tool struct {
    Key        string   // tu's key: cc, codex, oc, gemini, copilot, kimi
    Name       string   // display name
    PrefixArgs []string // ccusage per-agent subcommand
    LabelKey   string   // JSON key carrying the ISO date label
}

// Tools is the registry in column order (Output Stability: insertion order is
// the all-tools column order; new tools append).
var Tools = []Tool{
    {Key: "cc",      Name: "Claude Code", PrefixArgs: []string{"claude"},   LabelKey: "date"},
    {Key: "codex",   Name: "Codex",       PrefixArgs: []string{"codex"},    LabelKey: "date"},
    {Key: "oc",      Name: "OpenCode",    PrefixArgs: []string{"opencode"}, LabelKey: "date"},
    {Key: "gemini",  Name: "Gemini",      PrefixArgs: []string{"gemini"},   LabelKey: "date"},
    {Key: "copilot", Name: "Copilot",     PrefixArgs: []string{"copilot"},  LabelKey: "date"},
    {Key: "kimi",    Name: "Kimi",        PrefixArgs: []string{"kimi"},     LabelKey: "date"},
}

func Lookup(key string) (Tool, bool)
```

- A **slice, not a map** — Go maps are unordered and the TS relies on registry insertion order for column order.
- `LabelKey` is `date` for all six at ccusage v20 (live-verified, r7dh); the field exists because the key varies by upstream serializer (`period` on the bare all-agents aggregate tu never calls).
- The TS `needsFilter` / `stripNoise` (drop stdout lines starting with `[`) is **not ported**: memory records it as a defensive no-op since v20 emits clean JSON, and the Go adapter treats non-JSON stdout as a `KindParse` error instead.
- Source aliases (`co`, `gem`, `cop`, `ki`) are command-grammar parsing and belong to V2's `command` package, not the registry.

#### Binary resolution

`ResolveBinary() (string, error)`, called once per `Source` unless an explicit path is set:

1. `filepath.Join(filepath.Dir(os.Executable()), "vendor", "ccusage", "bin", "ccusage")` if it exists — the R1 tarball layout (binary + `vendor/ccusage/bin/ccusage` beside it), matching the harness memory's "vendor-first relative to `os.Executable()`".
2. `exec.LookPath("ccusage")` — dev and harness mode (`bin/harness/ccusage` on `PATH`).
3. Neither → every fetch returns a `KindExec` error with `Detail` `spawn ccusage ENOENT`.

No walk to the repo root and no `node_modules` probing: the tu binary is not the capture tool. `Source.Binary` overrides resolution (tests point it at the built fake).

#### Exec

```go
// DefaultTimeout is the per-source deadline the command edge (V2) should put
// on the context. The source itself imposes no deadline: it honors the ctx.
const DefaultTimeout = 120 * time.Second

func run(ctx context.Context, binary string, args []string) (stdout []byte, err *source.Error)
```

- `exec.CommandContext(ctx, binary, args...)`, argv = `[PrefixArgs..., period, "--json", extraArgs...]` — the TS composition verbatim (`ccusage claude daily --json`). Inherited environment and cwd, no shell.
- stdout and stderr into separate buffers; **no output cap** (the TS `maxBuffer: 10 MB` makes a >10 MB response a *failure* there; Go reads it — divergence only where the TS binary breaks).
- Deadline → `KindTimeout`; `exec.ExitError` / signal → `KindExec` with the Node-shaped `Command failed` detail; start failure → `KindExec` with the `spawn` detail.

#### Normalize

```go
// Parse converts one ccusage per-agent daily document into records for tool
// (Date + Tool + Totals; User/Machine are stamped by Source). It never returns
// an error for an empty daily array — that is a legitimate zero result.
func Parse(raw []byte, tool Tool) ([]fact.Record, *source.Error)
```

- Document shape: `{"daily": [ {...entry...}, ... ], "totals": {...}}`. Only `daily[]` is read; `totals` is ignored (its `-0.0` cost on empty agents never reaches tu).
- Per entry, the TS `toUsageTotals` coercion rules, byte-for-byte in effect:
  - `TotalCost` ← `totalCost`, falling back to `costUSD` (codex);
  - `CacheReadTokens` ← `cacheReadTokens`, falling back to `cachedInputTokens`;
  - `InputTokens`, `OutputTokens`, `CacheCreationTokens`, `TotalTokens` ← same-named keys;
  - a missing key or a non-numeric JSON value → `0` (the TS `Number(x) || 0`). JSON numbers only; a numeric *string* is `0` in Go where `Number("12")` is `12` in TS — ccusage never emits one.
- `Date` ← `normalizeLabel(entry[tool.LabelKey])`: ISO passes through; the defensive human-readable forms are kept because the spec lists them (`Feb 14, 2026` → `2026-02-14`, `Feb 2026` → `2026-02`); an unknown month abbreviation yields `00` as in the TS; a missing label yields `""`.
- `raw` empty/whitespace, not an object, or `daily` absent/not an array → `KindParse`. `daily: []` → `([]fact.Record{}, nil)`.

#### Fetch

```go
type Source struct {
    Binary  string       // "" → ResolveBinary()
    User    string       // stamped on every record
    Machine string       // stamped on every record
    Cache   *cache.Store // nil → no caching
}

const PeriodDaily = "daily"

// Fetch returns tool's records for period. On any failure it returns
// (nil, err) — the caller decides what "zero data" means; nothing is printed.
func (s *Source) Fetch(ctx context.Context, tool Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error)

// FetchAll fetches every tool in Tools concurrently and returns the records
// concatenated in registry order plus every error, also in registry order.
// It never stops early: one failing tool does not cancel the others.
func (s *Source) FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error)
```

Fetch order of operations, mirroring `fetchHistory`:

1. `if !fresh && s.Cache != nil` → `Cache.Get(key)`; a hit returns the cached records with `User`/`Machine` stamped.
2. `run` → on error return `(nil, err)` and **do not write the cache** (the TS returns before `writeCache`).
3. `Parse` → on `KindParse` return `(nil, err)`, no cache write. On zero records return `([]fact.Record{}, nil)` and **do not write the cache** either — the TS `if (!entries || entries.length === 0) return []` exits before `writeCache`, so an empty agent is re-fetched every call. (The spec sentence "a tool with no data still gets a cache file" contradicts the shipped code; see Open Questions.)
4. Non-empty → `Cache.Put(key, records)` (write failures swallowed), then stamp `User`/`Machine` and return. `fresh` skips the read but **still writes**, as the TS does.

`FetchAll` uses a `sync.WaitGroup` with one result slot per registry index, so output order is deterministic regardless of completion order. The TS emits its warnings in completion order (nondeterministic across tools); the Go edge emits them in registry order, which is always one of the orders the TS could produce.

### 5. `source/cache` — on-disk JSON, hash-keyed, 60 s TTL

```go
package cache

const TTL = 60 * time.Second

type Key struct {
    Tool   string
    Period string
    Args   []string
}

// Filename is "{tool}-{period}-{h}.json" where h is the first 16 hex chars of
// sha256(tool + "\x00" + period + "\x00" + strings.Join(args, "\x00")). The hash
// alone is the key; the readable prefix is a courtesy for `ls ~/.tu/cache`.
func (k Key) Filename() string

type Store struct {
    Dir string                 // default filepath.Join(os.Getenv("HOME"), ".tu", "cache")
    TTL time.Duration          // default TTL
    Now func() time.Time       // default time.Now; injected by tests
}

func Default() (*Store, error)              // $HOME unset → error (the config-home standard's exit-1 case is the edge's to report)
func (s *Store) Get(k Key) ([]fact.Record, bool)   // miss on: absent, mtime older than TTL, unreadable, envelope mismatch
func (s *Store) Put(k Key, recs []fact.Record) error // mkdir -p, write envelope; caller ignores the error
```

Envelope on disk:

```json
{"v":1,"tool":"cc","period":"daily","args":[],"records":[{"Date":"2026-01-05","Tool":"cc","totalCost":0.5,...}]}
```

- **TTL by file mtime**, as the spec states ("checked via file mtime"); the envelope's `tool`/`period`/`args` are verified on read so a hash collision or a stale schema is a miss, never a wrong answer.
- Records are cached **without** `User`/`Machine` (both empty in the file); `Source.Fetch` stamps them on the way out, so a config change inside the TTL window cannot serve a stale user or hostname.
- Because the key includes `args`, an extra-args fetch is cached too. The TS caches only vanilla calls; no production call site passes extra args, so this is unobservable today and the plan's chosen generalization.
- The TS files `~/.tu/cache/{tool}-daily.json` are neither read nor removed. The two implementations will co-exist on maintainer machines during R2 with separate cache files — a deliberate non-sharing (the envelopes differ).

### 6. Tests (all `_test.go` siblings, `go test ./...`)

- **`fact_test.go`** — `Add` is field-wise and pure; `IsZero`; JSON tags round-trip to the pinned names.
- **`source/error_test.go`** — `Warning()` byte-exact for the Exec/ENOENT/Timeout shapes; `Warns()` false for Parse; `WriteWarnings` writes only warning kinds, in order.
- **`ccusage/normalize_test.go`** — table tests over the **P3 placeholder corpus**: for each of the six sources read `../../../../../harness/fixtures/_placeholder/<source>/daily.json` (the same relative-walk convention as `harness/corpus_test.go`), `Parse` it, assert 3 records dated `2026-01-05..07` with the exact placeholder totals, and that the sum of the four counters equals `TotalTokens`; codex proves the `costUSD` path; plus hand-written cases for `cachedInputTokens`, missing/non-numeric fields → 0, human-readable labels, `daily: []` → zero records no error, garbage/empty stdout → `KindParse`.
- **`ccusage/registry_test.go`** — six tools in the pinned order with the pinned `PrefixArgs`/`LabelKey`; `Lookup` miss.
- **`ccusage/source_test.go`** — `TestMain` builds `../../../cmd/fakeccusage` into a temp dir with `go build` and sets `TUDIFF_FIXTURES` to the **`_placeholder` alias only** (never a local capture — deterministic in CI and on dev machines); then: `Fetch` of each tool returns the placeholder records with `User`/`Machine` stamped; argv composition is proven through the fake's `TUDIFF_CALL_LOG` (`["claude","daily","--json"]`); an unknown period is a fixture miss → the fake exits 2 → `KindExec` with a `Command failed: … \nfakeccusage: no fixture for argv …` detail; a nonexistent binary → `spawn … ENOENT`; a `#!/bin/sh` stub that sleeps → `KindTimeout` under a 200 ms context; `FetchAll` with one tool pointed at a miss returns five tools' records in registry order and exactly one error.
- **`cache/cache_test.go`** — temp `Dir`, injected `Now`: `Put` then `Get` hits; `Get` after mtime older than TTL misses; corrupt file misses; envelope tool mismatch misses; `Filename()` is stable and differs across tool/period/args; through `Source`: a second `Fetch` inside the TTL does not invoke the binary (call log unchanged), `fresh` re-invokes and rewrites, an empty-`daily` fetch writes no file.

### 7. Explicitly not in V1

- No `query` (window filter, roll-ups, `currentLabel`/`pickCurrentEntry`), `view`, `render`, or `command` — V2.
- No `source/metrics` (repo reader) — B3. No `config` — B1 (V1 takes `User`/`Machine` as plain fields).
- No change to `src/go/cmd/tu/main.go`, the justfile, CI, `src/node/`, or the specs. No external surface changes — the Go binary still prints `not implemented` for every data command.

## Affected Memory

- `go-port/fact-and-sources`: (new) the `fact.Record`/`fact.Totals` shape and its invariants (string ISO `Date`, registry-key `Tool`, stored `TotalTokens`, pinned JSON tags); the ordered six-tool registry and argv composition; binary resolution order (vendor beside `os.Executable()` → `PATH`); the `source.Error` kinds and which ones warn; the byte-exact warning line and its Node-shaped details; the normalize coercion rules; the cache filename/hash/envelope/TTL and the no-cache-on-empty-or-failure rule; `FetchAll`'s registry-order determinism; the test shape (TestMain-built fake, `_placeholder`-only). Design Decisions: string label not `time.Time`; slice registry not map; `needsFilter` dropped; `WaitGroup` not `errgroup`; caller-owned timeout with an exported default; cache records unstamped; no stdout cap.
- `build/toolchain`: (modify) one bullet — the ccusage adapter tests build `cmd/fakeccusage` in `TestMain` (so `go test ./...` needs the Go toolchain at test time, already true) and run inside the existing `go-build-and-test` lane with no workflow change.

`cli/data-pipeline` is not modified: it describes the shipped TypeScript, which V1 does not touch. The spec correction in Open Questions is a human edit, not a hydrate.

## Impact

- **New Go code**: `src/go/internal/fact/`, `src/go/internal/source/`, `src/go/internal/source/ccusage/`, `src/go/internal/source/cache/` with sibling tests. Roughly 600–900 lines including tests.
- **Untouched**: `src/go/cmd/tu/`, `src/go/internal/harness/`, `src/node/**`, `justfile`, `.github/workflows/*`, `docs/specs/*`, `Formula/`, `package.json`.
- **Dependencies**: none added; stdlib only (`os/exec`, `context`, `encoding/json`, `crypto/sha256`, `sync`).
- **Harness**: the `_placeholder` corpus becomes a test input for a second package; the corpus test's sha256 pin now protects two consumers. Tests build `cmd/fakeccusage` once per test binary (~1 s).
- **Downstream rows**: V2 imports `fact`, `source`, `source/ccusage`, `source/cache` and wires `DefaultTimeout` + `WriteWarnings` at the command edge; B3 adds `source/metrics` beside `ccusage`; B1 supplies `User`/`Machine`.
- **Risk**: the Node-shaped warning detail is a bet on P4 staging both binaries at the same vendor path; if P4 chooses normalization instead, the shape is one function to change.

## Open Questions

- **Spec correction (human, not blocking)**: `docs/specs/usage.md` § Data Flow › Caching says "a tool with no data still gets a cache file", but `fetchHistory` returns before `writeCache` on `daily: []` — an empty agent is re-fetched on every call. V1 follows the binary. The sentence should be fixed (or a `[DECIDE]` added) at G0; the harness's call-log comparison would otherwise flag the *Go* side if it cached empties.
- **Warning-line parity strategy (for P4/G1)**: the TS detail embeds the absolute ccusage path (`Command failed: /abs/dist/vendor/ccusage/bin/ccusage claude daily --json\n…`). V1 reproduces Node's shape; byte parity then needs P4 to stage the Go binary inside the same staged `dist/` so both resolve the identical `vendor/ccusage/bin/ccusage` path — or a normalization rule for that line. Either works with this design; P4 should pick one.
- **`errgroup` mention in the plan**: the plan lists `errgroup` as a Go-native choice for parallel fetches. V1 uses `sync.WaitGroup` because `errgroup`'s cancel-on-first-error is the opposite of "collected, never thrown through" and would add the module's first dependency. Flagging for G1 in case the plan wanted `errgroup` for its `SetLimit`.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Four new packages `internal/fact`, `internal/source` (error + warning), `internal/source/ccusage`, `internal/source/cache`; zero new module deps; nothing wired into `cmd/tu` | Plan D5 layout and Target architecture table name the packages; D2 keeps Go dark; scaffold precedent is dependency-free | S:85 R:70 A:90 D:85 |
| 2 | Confident | `fact.Record{Date, Tool, User, Machine string; Totals}` with `Date` an ISO label **string** (not `time.Time`) and `Tool` the registry key | Principle V fixes the label formats incl. `YYYY-MM` which has no instant; TS filters rely on lexicographic ISO order; metrics files and cache key on the tool key. Cascades to every downstream package, hence not Certain | S:80 R:60 A:85 D:80 |
| 3 | Certain | `Totals` = `TotalCost float64` + five `int64` counters, `TotalTokens` stored not derived, JSON tags = the spec's pinned names | Spec § Data Model lists exactly these six fields and names; TS sums `totalTokens` from JSON rather than recomputing; render/json (V2) needs the tags | S:85 R:65 A:90 D:85 |
| 4 | Certain | Registry is an ordered slice of six `Tool{Key, Name, PrefixArgs, LabelKey}` in order cc, codex, oc, gemini, copilot, kimi; argv `[PrefixArgs…, period, "--json", extra…]` | Verbatim from `fetcher.ts` `TOOLS` and `runTool`; memory: insertion order is column order (Output Stability); `labelKey`/`prefixArgs` named in the row text | S:90 R:85 A:95 D:90 |
| 5 | Confident | Drop `needsFilter`/`stripNoise`; non-JSON stdout is a `KindParse` error | Row text lists `labelKey`/`prefixArgs` only; memory records `stripNoise` as a defensive no-op since v20 (clean JSON, live-verified); trivial to add back | S:60 R:90 A:80 D:70 |
| 6 | Certain | Binary resolution: `vendor/ccusage/bin/ccusage` beside `os.Executable()`, then `PATH`; `Source.Binary` overrides; no repo-root or `node_modules` walk | Harness memory states "the Go side resolves vendor-first relative to `os.Executable()` then `PATH`"; R1 fixes the tarball layout; the capture tool's dev-mode walk is not the CLI's concern | S:70 R:85 A:90 D:85 |
| 7 | Confident | Timeout is caller-owned via `context.Context`; the package exports `DefaultTimeout = 120s` for the edge to apply | Row says "exec with context timeout" but no value; TS has none, so any value is a strict addition; 120 s matches the harness's `CaptureTimeout`; a const is one line to change | S:50 R:95 A:70 D:60 |
| 8 | Confident | No stdout size cap (TS `maxBuffer` 10 MB not ported) | Go reads unbounded; the only divergence is where the TS *fails* on a >10 MB response and Go succeeds — never exercised by the corpus; capping to mimic a failure would be porting a limitation | S:40 R:90 A:75 D:70 |
| 9 | Certain | Normalize: `totalCost`→`costUSD` fallback, `cacheReadTokens`→`cachedInputTokens` fallback, missing/non-numeric → 0, label via `LabelKey` + `normalizeLabel` incl. the human-readable forms; `daily: []` → zero records, no error | Rules lifted line-for-line from `toUsageTotals`/`normalizeLabel`; spec § Fetching lists the defensive label conversion; live-verified codex shape in memory | S:80 R:85 A:90 D:85 |
| 10 | Confident | Error kinds Exec / Timeout / Parse; only Exec and Timeout produce the stderr line; Parse and empty results are silent zero data | TS warns only inside `execFileAsync`; `parseJson` failure returns `null` → `[]` with no output. Kinds are a small enum to extend | S:55 R:85 A:80 D:65 |
| 11 | Confident | `Detail` mimics Node's `error.message`: `Command failed: {binary} {args}\n{stderr}` and `spawn {binary} ENOENT`; timeout uses a Go-only `timeout after {d}` | Byte parity on the warning line is the harness bar; the absolute-path component needs a P4 staging or normalization decision either way (Open Questions). Single function to change if P4 normalizes the whole detail | S:35 R:85 A:40 D:35 |
| 12 | Confident | No cache write on exec failure, parse failure, **or empty `daily`**; the spec sentence claiming otherwise is flagged, not followed | `fetchHistory` returns before `writeCache` in all three cases; plan: "anything not in the drop list is a behavior to keep"; the harness call-log would flag Go caching empties | S:60 R:90 A:75 D:65 |
| 13 | Confident | Cache filename `{tool}-{period}-{sha256[:16]}.json` over NUL-joined tool/period/args; envelope `{v, tool, period, args, records}` verified on read; TTL by mtime; `fresh` skips read but still writes; `Dir`/`TTL`/`Now` injectable, default `$HOME/.tu/cache` | Row fixes "hash of tool+period+args, 60s TTL"; spec fixes mtime; readable prefix is a debugging courtesy that leaves the hash as the key; TS writes after a fresh fetch | S:65 R:95 A:75 D:60 |
| 14 | Confident | `FetchAll` uses `sync.WaitGroup` with per-index slots (registry-order output, all errors collected); no `golang.org/x/sync/errgroup` | Collect-all semantics contradict errgroup's cancel-on-first-error; keeps the module dependency-free; plan's `errgroup` mention flagged for G1 | S:55 R:90 A:85 D:70 |
| 15 | Confident | `Source{Binary, User, Machine, Cache}` — `User`/`Machine` are plain fields the caller supplies; cached records are stored unstamped and stamped on read | `config` is B1; the TS derives them from `config.user`/`config.machine` (`$HOSTNAME` sentinel); stamping after the cache means a config change inside the TTL is never served stale | S:50 R:80 A:80 D:70 |
| 16 | Confident | Tests: `TestMain` builds `cmd/fakeccusage` into a temp dir; `TUDIFF_FIXTURES` = `_placeholder` only; `Parse` tests read fixture bytes via the `../../../../../harness/fixtures` walk; timeout via a sleeping `sh` stub; cache tests inject `Dir`/`Now` | Row: "unit tests against the P3 fixtures"; the fake is the real replay path; excluding local captures keeps tests deterministic (they carry real spend and vary per machine); `corpus_test.go` sets the relative-walk precedent | S:60 R:90 A:85 D:75 |
| 17 | Confident | Memory: new domain `go-port` with `fact-and-sources.md`; one bullet added to `build/toolchain`; `cli/data-pipeline` untouched | P3a opened a new domain (`harness`) for new Go-side behavior; plan X3 reshapes domains to the package list at cutover, so a holding domain is the low-regret choice | S:45 R:90 A:70 D:60 |
| 18 | Certain | No external surface changes: no `cmd/tu` wiring, no spec/TS/CI/justfile edits | Row text and plan Goal forbid it; V2 is the first row that produces output | S:95 R:90 A:95 D:95 |

18 assumptions (6 certain, 12 confident, 0 tentative, 0 unresolved).
