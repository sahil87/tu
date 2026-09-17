---
type: memory
description: The Go port's input layer — internal/fact (Record/Totals, six-tool registry), internal/source (typed Error, WriteWarnings, PeriodDaily/DefaultTimeout), internal/source/ccusage (invocations map, exec, Fetch/FetchAll, edge-stamped User/Machine), internal/source/metrics (the read-only metrics-repo reader sharing the DayFile/Name/Path encoding with the sync writer — metrics-sync), internal/source/cache (hash-keyed JSON, 60 s TTL); consumed by command and cmd/tu, unshipped until cutover
---
# Fact Type and ccusage Sources (Go port)

**Domain**: go-port

## Overview

The Go port's input layer is five packages under `src/go/internal/`: `fact` is pure — the one record type every downstream stage consumes plus the tool registry; `source` holds the typed per-source `Error`, `WriteWarnings` (the single edge writer), and the fetch-contract constants `PeriodDaily`/`DefaultTimeout`; `source/ccusage` is the live adapter (an adapter-private invocation map, exec, JSON normalize, fetch); `source/metrics` is the second adapter — a read-only, silent reader of the metrics-repo clone; `source/cache` is the on-disk JSON store behind the live adapter. The command edge consumes this layer: `command.Run` fetches through the `command.Fetcher` interface (`Fetch` takes `fact.Tool`), which `*ccusage.Source` satisfies at the `cmd/tu` assignment, and reads the repo through the `command.Repo` interface, which `metrics.Source` satisfies there; the edge stamps the ccusage `Source` with `cfg.User`/`cfg.Machine` from the loaded config, applies `source.DefaultTimeout` once per invocation, builds its source with `cache.Default()`, and `cmd/tu` writes the result's warnings with `source.WriteWarnings` (see [command-edge](/go-port/command-edge.md); the multi-mode composition over these adapters is [multi-mode](/go-port/multi-mode.md)). The corpus these packages are tested against lives in [differential-harness](/harness/differential-harness.md); the shipped TypeScript data model they mirror is in [data-pipeline](/cli/data-pipeline.md); build/test wiring is in [toolchain](/build/toolchain.md).

## Requirements

### Requirement: fact.Record and fact.Totals shape
`package fact` SHALL define `Record{Date, Tool, User, Machine string; Totals}` and `Totals{TotalCost float64; InputTokens, OutputTokens, CacheCreationTokens, CacheReadTokens, TotalTokens int64}`. `Date` is an ISO label **string** (`YYYY-MM-DD`, or `YYYY-MM` for monthly roll-ups), never `time.Time`. `Tool` holds the registry key (`cc`), not the display name. `Record`'s four string fields carry no JSON tags; `Totals`' six JSON tags are the pinned external names from `docs/specs/usage.md` § Data Model (`totalCost`, `inputTokens`, …), so render/json and the cache envelope encode the struct directly. `TotalTokens` is stored, never recomputed from the other four counters (byte parity with ccusage's own value). The zero `Totals{}` is the "no data" value (the TS `EMPTY`).

#### Scenario: JSON encoding uses the pinned names
- **GIVEN** a `Totals{TotalCost: 0.5, InputTokens: 3000}`
- **WHEN** encoded with `encoding/json`
- **THEN** the output contains `"totalCost":0.5` and `"inputTokens":3000` and the four other pinned keys

### Requirement: Totals arithmetic is pure
`func (t Totals) Add(o Totals) Totals` MUST return the field-wise sum without mutating either operand. `func (t Totals) IsZero() bool` MUST report true only when all six fields are zero.

#### Scenario: Add is field-wise and non-mutating
- **GIVEN** `a := Totals{TotalCost: 1, InputTokens: 2}` and `b := Totals{TotalCost: 0.5, OutputTokens: 3}`
- **WHEN** `c := a.Add(b)`
- **THEN** `c == Totals{TotalCost: 1.5, InputTokens: 2, OutputTokens: 3}`, `a` and `b` are unchanged, `Totals{}.IsZero()` is true, and `c.IsZero()` is false

### Requirement: source.Error kinds and which warn
`package source` SHALL define `type Kind int` with `KindExec` (binary missing, spawn failure, non-zero exit, signal), `KindTimeout` (the caller's context deadline fired), and `KindParse` (stdout was not the expected JSON document), in that order. `Error{Tool, Name string; Kind; Detail string; Err error}` implements `error` (`Error()` is a free readable form, `{Name}: {Detail}`) and `Unwrap()`. `Warns()` MUST return true for Exec and Timeout and false for Parse (the TS returns `[]` silently on bad JSON). Source packages MUST never panic on a source failure and MUST never write to stdout/stderr: failures are returned as `*source.Error` values beside the (zero) records.

#### Scenario: Warns per kind
- **GIVEN** an `Error{Kind: KindParse}`
- **WHEN** `Warns()` is called
- **THEN** it returns false; for `KindExec` and `KindTimeout` it returns true

### Requirement: The byte-exact warning line
`func (e *Error) Warning() string` MUST return exactly `warning: {Name} fetch failed ({Detail}), showing zero data` (no trailing newline) — a future external surface byte the harness byte-diffs. `Detail` is composed by the ccusage adapter in these Node-`error.message` shapes:

| Failure | `Detail` |
|---------|----------|
| non-zero exit or signal | `Command failed: {binary} {args joined by single spaces}` + `"\n"` + captured stderr verbatim (an empty stderr leaves the trailing newline before the closing paren) |
| binary not found (`exec.ErrNotFound` or `fs.ErrNotExist` in the chain) | `spawn {binary} ENOENT` |
| other start failure | `spawn {binary} ` + Go's error text |
| context deadline | `timeout after {d}` where `d` is the elapsed run time as `time.Duration.String()`, rounded to milliseconds |

#### Scenario: Byte-exact warning
- **GIVEN** `Error{Name: "Kimi", Kind: KindExec, Detail: "Command failed: /x/ccusage kimi daily --json\nboom\n"}`
- **WHEN** `Warning()` is called
- **THEN** it returns `"warning: Kimi fetch failed (Command failed: /x/ccusage kimi daily --json\nboom\n), showing zero data"`

### Requirement: WriteWarnings is the only writer
`func WriteWarnings(w io.Writer, errs []*Error)` MUST write `Warning() + "\n"` for each error whose `Warns()` is true, in the order given, and nothing for the others. It is the only function in the `source` tree that touches an `io.Writer`, and it is called by the command edge (V2), never by a source.

#### Scenario: Only warning kinds, in order
- **GIVEN** `errs := []*Error{execErr, parseErr, timeoutErr}`
- **WHEN** `WriteWarnings(&buf, errs)` runs
- **THEN** `buf` holds exactly two lines, the exec warning then the timeout warning

### Requirement: Ordered six-tool registry and argv composition
`package fact` SHALL define `Tool{Key, Name string}` and own the registry: `Tools`, an **ordered slice** (never a map — Go maps are unordered and insertion order is the all-tools column order) of exactly `cc`/Claude Code, `codex`/Codex, `oc`/OpenCode, `gemini`/Gemini, `copilot`/Copilot, `kimi`/Kimi, and `Lookup(key) (Tool, bool)` scanning the slice. Source aliases (`co`, `gem`, `cop`, `ki`) are command grammar and are not part of the registry. `package ccusage` holds only the unexported `invocations map[string]invocation{prefixArgs []string; labelKey string}` keyed by `fact.Tool.Key` — the per-agent subcommand (`claude`, `codex`, `opencode`, `gemini`, `copilot`, `kimi`) and the JSON label key (every `labelKey` is `date` at ccusage v20); a registry test asserts the map's key set equals the keys of `fact.Tools`. The argv handed to the binary MUST be `invocations[tool.Key].prefixArgs…, period, "--json", extraArgs…` (e.g. `claude daily --json`), passed to `os/exec` as a slice with no shell.

#### Scenario: Registry order and argv
- **GIVEN** `fact.Tools` iterated
- **WHEN** keys are read
- **THEN** they are `cc, codex, oc, gemini, copilot, kimi` in that order, `fact.Lookup("gemini")` returns the Gemini tool, and `fact.Lookup("gem")` misses
- **GIVEN** the `cc` tool, period `daily`, no extra args
- **WHEN** a fetch runs against the fake ccusage with `TUDIFF_CALL_LOG` set
- **THEN** the logged argv is exactly `["claude","daily","--json"]`

### Requirement: Binary resolution
`func ResolveBinary() (string, error)` SHALL return, in order: (1) `vendor/ccusage/bin/ccusage` beside the running executable (`os.Executable()`, symlinks resolved via `filepath.EvalSymlinks` so a Homebrew `bin/` symlink finds the vendor tree beside the real file) if it exists; (2) `ccusage` on `PATH`; (3) an error wrapping `exec.ErrNotFound`. The vendor lookup is the unexported `resolveVendor(exe string) (string, bool)` helper that `ResolveBinary` calls, pinned by the symlink-chain tests `TestResolveVendorThroughSymlink` / `TestResolveVendorAbsent` in `source_test.go` (0118). It MUST NOT walk to a repo root or probe `node_modules`. `Source.Binary` non-empty bypasses resolution entirely.

#### Scenario: No binary anywhere
- **GIVEN** no vendor sibling and no `ccusage` on `PATH`
- **WHEN** `Fetch` runs with `Binary == ""`
- **THEN** it returns `(nil, err)` with `err.Kind == KindExec` and `err.Detail == "spawn ccusage ENOENT"`

### Requirement: Exec semantics
The adapter SHALL run the binary with `exec.CommandContext(ctx, binary, argv...)`, inherited environment and cwd, stdout and stderr captured into separate temp files with **no size cap** (real files, not pipes: a killed child can leave grandchildren holding a pipe open, and `exec.Wait` would then block past the deadline). The adapter imposes no deadline of its own — it honors the given context; the per-invocation deadline `source.DefaultTimeout = 120 * time.Second` lives in `internal/source` for the command edge to apply. Failure classification: context deadline → `KindTimeout`; start failure → `KindExec` with the `spawn` detail; non-zero exit or signal → `KindExec` with the `Command failed` detail. ccusage's stderr on a successful run is captured and discarded, never forwarded.

#### Scenario: Deadline fires
- **GIVEN** a `#!/bin/sh` stub that runs `sleep 5` and a context with a 200 ms timeout
- **WHEN** `Fetch` runs
- **THEN** it returns promptly with `err.Kind == KindTimeout` and `err.Detail` starting `timeout after `

### Requirement: Parse coercion and label normalization
`func Parse(raw []byte, tool fact.Tool) ([]fact.Record, *source.Error)` SHALL decode `{"daily": [ … ]}` and, for each entry, build a `fact.Record` with `Tool = tool.Key` and empty `User`/`Machine`, applying the TS `toUsageTotals` coercion: `TotalCost` ← `totalCost` else `costUSD` (the codex outlier) else 0; `CacheReadTokens` ← `cacheReadTokens` else `cachedInputTokens` else 0; the other four counters ← same-named keys else 0; a present key whose value is not a JSON number → 0, never an error. `Date` ← `normalizeLabel(entry[invocations[tool.Key].labelKey])`: ISO passes through unchanged, `Feb 14, 2026` → `2026-02-14` (day zero-padded), `Feb 2026` → `2026-02`, an unknown 3-letter month maps to `00`, a missing or non-string label yields `""`. The `totals` object and unknown keys (`modelBreakdowns`, `models`, `reasoningOutputTokens`, …) are ignored.

#### Scenario: Placeholder corpus parses uniformly
- **GIVEN** `harness/fixtures/_placeholder/codex/daily.json`
- **WHEN** parsed with the codex tool
- **THEN** 3 records dated `2026-01-05`, `2026-01-06`, `2026-01-07`, each `Totals{0.5, 3000, 400, 1000, 20000, 24400}` with `Tool == "codex"` (the `costUSD` path); the other five fixtures yield the same dates and totals, with the four-counter sum equal to `TotalTokens` on every record

### Requirement: Parse edge cases
`Parse` MUST return `([]fact.Record{}, nil)` (non-nil empty slice, no error) for `{"daily":[]}` — an empty agent is a legitimate zero result. It MUST return `(nil, &source.Error{Kind: KindParse})` when `raw` is empty/whitespace, is not a JSON object, or has a `daily` that is missing or not an array.

#### Scenario: Empty versus malformed
- **GIVEN** `{"daily":[],"totals":{"totalCost":-0.0}}`
- **WHEN** parsed
- **THEN** zero records and a nil error
- **GIVEN** `""`, `"not json"`, `"[]"`, `{"totals":{}}`
- **WHEN** parsed
- **THEN** each returns `err.Kind == KindParse`

### Requirement: Fetch order of operations
`Source{Binary, User, Machine string; Cache *cache.Store}` (nil cache → no caching). `Fetch(ctx, tool fact.Tool, period, extraArgs, fresh)` SHALL, in order: (1) when `!fresh` and a cache is set, `Cache.Get(key)` with `key = cache.Key{Tool: tool.Key, Period: period, Args: extraArgs}` — a hit returns the records with `User`/`Machine` stamped; (2) resolve the binary and run — on error return `(nil, err)` with **no cache write**; (3) `Parse` — on `KindParse` return `(nil, err)` with no cache write, on zero records return `([]fact.Record{}, nil)` with **no cache write**; (4) on one or more records, `Cache.Put(key, records)` (records stored unstamped; `Put` errors ignored by design — a cache write failure must not fail a successful fetch), then stamp `User`/`Machine` on every record and return. `fresh` skips the read but still writes. The Go adapter writes no cache file for an empty `daily`, matching the shipped TypeScript; `docs/specs/usage.md` § Caching states otherwise and is flagged for a human correction at gate G0.

#### Scenario: Cache hit skips the binary
- **GIVEN** a `Source` with a temp-dir cache and the fake ccusage
- **WHEN** `Fetch(cc)` runs twice within the TTL
- **THEN** the call log has exactly one ccusage line, both results are equal, and every record has the configured `User` and `Machine`

### Requirement: FetchAll collects everything in registry order
`FetchAll(ctx, period, extraArgs, fresh)` SHALL call `Fetch` for every entry of `fact.Tools` concurrently (one goroutine each, `sync.WaitGroup`, one result slot per registry index), never cancelling siblings on a failure, and return the records concatenated in registry order followed by the non-nil errors, also in registry order. A failed tool contributes no records.

#### Scenario: Deterministic output regardless of completion order
- **GIVEN** the fake ccusage serving all six placeholder fixtures
- **WHEN** `FetchAll` runs for period `daily`
- **THEN** 18 records ordered cc×3, codex×3, oc×3, gemini×3, copilot×3, kimi×3 and zero errors; for period `weekly` (no fixtures), six `KindExec` errors in registry order and zero records

### Requirement: metrics.Source reads the metrics repo clone
`package metrics` defines `Source{Dir string}` — the second adapter under `source`, the only other package that touches the filesystem. `Users()` returns the profile directories: direct children of `Dir` that are directories, excluding dot-prefixed names (`.git`) and the `nonUserDirs` set (`{"docs"}`, the TS NON_USER_DIRS), sorted ascending in byte order (the TS `Array.sort` on ASCII names); a missing or unreadable `Dir` yields nil. `Read(user string, tool fact.Tool)` returns the user's records for the tool across every machine in walk order — year directories ascending, machine directories ascending, files ascending — reading only files whose name has prefix `{tool.Key}-` and suffix `.jsonl` (so `ccx-…` never matches `cc`). Per file: read, trim, skip when empty, decode the single JSON object into the exported `DayFile{Label string \`json:"label"\`; fact.Totals}` — the one day-file encoding, in the exact key order the TS `toUsageEntry` spread produces — skip silently on any read or decode error. `DayFile` is shared with the writer by construction: `sync.Write` marshals it, `metrics.Name(tool, date)` is the basename `{tool.Key}-{date}.jsonl`, and `metrics.Path(dir, user, machine, tool, date)` is `{dir}/{user}/{year}/{machine}/{Name}` with `year` the label's first four characters (or the whole label when shorter); `json.Marshal(DayFile)` reproduces the TS `JSON.stringify(entry)` bytes (lsml — [metrics-sync](/go-port/metrics-sync.md)). The record's `Date` is the JSON `label` (never the filename date); `Tool` is `tool.Key`; `User`/`Machine` are the directory names; `Totals` are the six pinned keys, a missing key decoding as `0` (the TS would propagate NaN through `+= undefined`, but only a hand-corrupted file can lack a key — the never-shrink writer always emits all six). A missing user directory, an unreadable level, or a non-directory at the year or machine level is skipped silently. The package never writes (the writer is `internal/sync`), never prints, never execs, and never returns an error — a missing or unreadable directory or file is simply absent data, exactly as the TS readers swallow every fs error (the repo's absence is reported once, by the metrics-dir guard). The TS `excludeMachine` parameter is not ported — every production call site passed `null`, and 260610-srmi flagged it for deletion. Its tests build temp-dir trees, including a copy of the committed seed `harness/metrics-repo/` located by the walk-up-to-`package.json` helper (the `internal/config/defaults_test.go` convention).

#### Scenario: Walk order over the committed seed
- **GIVEN** the seed `harness/metrics-repo/` copied to `Dir`
- **WHEN** `Read("harness-user", cc)` runs
- **THEN** it returns, in order, `harness-machine/cc-2026-01-05` (`$0.25`), `harness-machine/cc-2026-01-06` (`$0.75`), `other-box/cc-2026-01-06` (`$0.40`), each stamped `User: harness-user` and `Machine:` the directory name, with `Date` from the JSON label; `Read("nobody", cc)` returns nil

### Requirement: Cache filename, hash, envelope, TTL
`package cache`: `const TTL = 60 * time.Second`. `Key{Tool, Period string; Args []string}`. `Filename()` MUST be `{tool}-{period}-{h}.json` where `h` is the first 16 hex characters of `sha256(tool + "\x00" + period + "\x00" + strings.Join(args, "\x00"))`; the hash alone is the key. `Store{Dir string; TTL time.Duration; Now func() time.Time}` — a zero `TTL` means the package `TTL`, a nil `Now` means `time.Now` (tests inject both). `Default()` MUST return `Dir = $HOME/.tu/cache` with the defaults, or an error when `$HOME` is empty. `Put` MUST `MkdirAll(Dir, 0o755)` and write the envelope `{"v":1,"tool":…,"period":…,"args":[…],"records":[…]}` (empty `args` encodes as `[]`, not `null`); records are stored unstamped (empty `User`/`Machine`). `Get` MUST miss when the file is absent, when `Now() − mtime > TTL` (TTL by file mtime), when the file does not decode, or when the envelope's `v`/`tool`/`period`/`args` differ from the key — a hash collision or stale schema is a miss, never a wrong answer.

#### Scenario: Round-trip and miss conditions
- **GIVEN** a `Store` with a temp `Dir` and a controllable `Now`
- **WHEN** `Put(k, recs)` then `Get(k)`
- **THEN** the records round-trip; after advancing `Now` by 61 s `Get` misses; a file whose envelope says `tool: codex` under key `cc` misses; a corrupt file misses; `Key{"cc","daily",nil}.Filename()` is stable across calls and differs from the same key with extra args and from another tool's key

### Requirement: Tests against the placeholder corpus
Adapter tests SHALL read fixtures via the relative walk `../../../../../harness/fixtures/_placeholder` (the `internal/harness/corpus_test.go` convention shifted one level — Go tests run with cwd = package dir), and `source_test.go` SHALL have a `TestMain` that builds `../../../cmd/fakeccusage` into a temp dir with `go build` and sets `TUDIFF_FIXTURES` to the `_placeholder` directory **only** — never a local capture, so the tests are deterministic in CI and on dev machines. Tests MUST fail, not skip, when the fixtures directory is absent. The suite proves: per-tool `Fetch` returns the placeholder records stamped with `User`/`Machine`; argv composition through the fake's `TUDIFF_CALL_LOG`; fixture miss → `KindExec` with the `Command failed: … fakeccusage: no fixture for argv` detail; nonexistent binary → `spawn … ENOENT`; a sleeping `sh` stub under a 200 ms context → `KindTimeout`; `FetchAll` daily → 18 records in registry order; `FetchAll` weekly → 6 errors in registry order; cache-through-`Source` behavior (hit skips the binary, `fresh` re-invokes and rewrites, empty `daily` and failures write no file). Cache tests use a temp `Dir` and injected `Now`. `go test ./...`, `gofmt -l`, and `go vet ./...` under `src/go/` are clean.

#### Scenario: Clean checkout runs the suite
- **GIVEN** a clean checkout with `go` on `PATH`
- **WHEN** `just go-lint && just go-test` run
- **THEN** both exit 0 and the new packages' tests appear in the output

## Design Decisions

### Date is an ISO label string, not time.Time
**Decision**: `fact.Record.Date` is `string` in the two Principle-V forms.
**Why**: `YYYY-MM` roll-up labels have no instant; every TS filter relies on lexicographic ISO order; it keeps the fact type free of time-zone semantics the render layer never needs.
**Rejected**: `time.Time` (forces a fake instant for months and re-introduces the 31 `new Date()` sites' local/UTC ambiguity into the data layer).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Registry is an ordered slice
**Decision**: `fact.Tools` is `[]Tool`; `fact.Lookup` scans it.
**Why**: Go maps are unordered and insertion order is the all-tools column order (Output Stability).
**Rejected**: `map[string]Tool` plus a separate order slice (two sources of truth).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Registry lives in `fact`, not a new package
**Decision**: `fact.Tool{Key, Name}`, `fact.Tools`, `fact.Lookup` own the registry; adapters hold their own per-key data.
**Why**: the registry (key, display name, column order) is a property of the fact model — `fact.Record.Tool` already carries the key — and `fact` is the package every stage imports; `source/metrics` and `command` need it without touching an adapter.
**Rejected**: a tiny `internal/tools` package (a fourth import for ~30 lines, no isolation gain); a type alias in `ccusage` (keeps the weld).
*Introduced by*: 260916-m9of-g1-rework-1

### `Fetcher.Fetch` takes `fact.Tool`
**Decision**: the parameter is the struct, not the key string.
**Why**: the command edge already resolved `fact.Lookup(req.Source)` after grammar validation; passing the struct spares every adapter a re-lookup and a miss branch, and `source.Error{Tool, Name}` needs the name anyway.
**Rejected**: `Fetch(ctx, key string, …)` — a smaller interface that pushes a lookup-and-miss path into each adapter.
*Introduced by*: 260916-m9of-g1-rework-1

### Fetch contract constants live in `source`
**Decision**: `source.PeriodDaily` and `source.DefaultTimeout`.
**Why**: both describe the contract every adapter honors (the one period tu fetches; the edge-applied deadline); `source` is already the adapter-neutral package (`Error`, `WriteWarnings`), and B3/B6 fetch under the same rules.
**Rejected**: unexported constants in `command` (correct today, but B6's `sync` would re-declare the deadline).
*Introduced by*: 260916-m9of-g1-rework-1

### needsFilter / stripNoise not ported
**Decision**: Non-JSON stdout is a `KindParse` error; no `[`-line stripping.
**Why**: Memory records it as a defensive no-op since ccusage v20 emits clean JSON (live-verified); the mechanism is a five-line add-back if a future ccusage regresses.
**Rejected**: Porting it for every tool (would silently mask a real upstream breakage).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Caller-owned timeout with an exported default
**Decision**: `Fetch` honors the given `context.Context` only; `source.DefaultTimeout = 120s` is exported for the command edge.
**Why**: The TS has no timeout, so any value is an addition; the edge is where a deadline is policy; 120 s matches the harness capture bound.
**Rejected**: A hard-coded internal deadline (untestable without sleeping 120 s; hides policy in the adapter).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Warning detail mimics Node's error.message
**Decision**: `Detail` reproduces `Command failed: {cmd}\n{stderr}` and `spawn {binary} ENOENT`.
**Why**: The stderr line is a harness-compared byte surface; the only non-reproducible component is the absolute binary path, which P4 can equalize by staging both binaries under one `dist/`.
**Rejected**: Go-native error text (guarantees a diff on every failure fixture).
*Introduced by*: 260916-v0as-fact-source-ccusage

### No cache write on failure or empty results
**Decision**: Only a non-empty parse result is written to the cache.
**Why**: `fetchHistory` returns before `writeCache` in all three cases; the harness call-log comparison would flag Go for skipping a ccusage call the TS makes. The spec sentence saying otherwise is flagged in the intake for a human correction.
**Rejected**: Caching empties (spec-literal but binary-divergent).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Cache records are stored unstamped
**Decision**: `Put` receives records with empty `User`/`Machine`; `Fetch` stamps after read/write.
**Why**: The key excludes user and machine, so a config change inside the TTL must not serve a stale identity.
**Rejected**: Including user/machine in the key (invalidates the cache on every identity change for data that did not change).
*Introduced by*: 260916-v0as-fact-source-ccusage

### WaitGroup, not errgroup
**Decision**: `FetchAll` uses `sync.WaitGroup` with per-index result slots.
**Why**: "Collected, never thrown through" is the opposite of errgroup's cancel-on-first-error; keeps the module dependency-free.
**Rejected**: `golang.org/x/sync/errgroup` (first dependency, wrong semantics for collect-all).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Cache filename carries a readable prefix
**Decision**: The cache filename is `{tool}-{period}-{sha256[:16]}.json` over the NUL-joined key; the hash alone is the key and the `{tool}-{period}` prefix is a courtesy for `ls ~/.tu/cache`.
**Why**: A readable prefix makes the cache directory inspectable while the truncated sha256 over tool, period, and args guarantees distinct keys; the envelope is verified on read, so the prefix is never load-bearing.
**Rejected**: A hash-only filename (opaque to a human debugging the cache); a fully readable filename without a hash (args would need escaping into the path and could collide).
*Introduced by*: 260916-v0as-fact-source-ccusage

### User/Machine are caller-supplied fields, stamped after the cache
**Decision**: `Source{User, Machine}` are plain fields the caller supplies — the edge stamps them from the loaded config (`cfg.User`/`cfg.Machine`); `Fetch` stamps them on every record on every return path, after the cache write.
**Why**: Identity arrives as plain fields, keeping the adapter config-free; stamping after the cache means a config change inside the TTL window is never served a stale user or hostname, and the stamp is what lets `MaxMerge` and the later machine columns key on identity.
**Rejected**: Deriving identity inside the adapter (pulls a config dependency into the input layer); stamping before the cache write (a stale identity would be served from disk).
*Introduced by*: 260916-v0as-fact-source-ccusage
