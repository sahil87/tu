---
type: memory
description: The ccusage source adapter — vendor-first binary resolution beside the running executable, the per-tool invocations map and argv composition, context-deadline exec with temp-file capture, Fetch/FetchAll with cache integration and identity stamping, and the command.Fetcher edge assertion.
---

# ccusage Adapter

**Domain**: source

## Overview

`internal/source/ccusage` is the adapter for the single ccusage binary that serves all six tools: it owns the adapter-private invocations map, vendor-first binary resolution, the `exec.CommandContext` run with classified failures, JSON normalization (see [ccusage-json-shapes](/source/ccusage-json-shapes.md)), and `Fetch`/`FetchAll` returning `fact.Record` values (see [records](/fact/records.md)). `*ccusage.Source` satisfies `command.Fetcher` (see [run-and-result](/command/run-and-result.md)).

## Requirements

### Requirement: Invocation map and argv composition
The unexported `invocations map[string]invocation` SHALL be keyed by `fact.Tool.Key` and carry, per tool, the per-agent subcommand `prefixArgs` (`cc`→`claude`, `codex`→`codex`, `oc`→`opencode`, `gemini`→`gemini`, `copilot`→`copilot`, `kimi`→`kimi`) and the JSON `labelKey` (`"date"` for every registry tool at ccusage v20); `registry_test.go` asserts the key set equals `fact.Tools` (see [tool-registry](/fact/tool-registry.md)). `argv(tool, period, extraArgs)` MUST compose `prefixArgs…, period, "--json", extraArgs…` (e.g. `claude daily --json`) as a slice handed to `os/exec` verbatim — no shell.

#### Scenario: argv composition
- GIVEN the `cc` tool, period `daily`, no extra args
- WHEN a fetch runs against the fake ccusage with `TUDIFF_CALL_LOG` set
- THEN the logged argv is exactly `["claude", "daily", "--json"]`

### Requirement: Vendor-first binary resolution
`ResolveBinary()` SHALL return, in order: (1) `vendor/ccusage/bin/ccusage` beside the running executable — `os.Executable()` with symlinks resolved via `filepath.EvalSymlinks`, so a Homebrew `bin/` symlink finds the vendor tree beside the real file; (2) `ccusage` on `PATH`; (3) an error wrapping `exec.ErrNotFound`. It MUST NOT walk to a repo root or probe node_modules. A non-empty `Source.Binary` bypasses resolution entirely.

#### Scenario: No binary anywhere
- GIVEN no vendor sibling and no `ccusage` on `PATH`
- WHEN `Fetch` runs with `Binary == ""`
- THEN it returns a `KindExec` error with detail `spawn ccusage ENOENT`

### Requirement: Exec semantics
`run(ctx, tool, binary, args)` SHALL execute with `exec.CommandContext`, inheriting environment and cwd, capturing stdout and stderr into separate temp files with no size cap. Failure classification: context deadline → `KindTimeout` with detail `timeout after {elapsed}` rounded to milliseconds; non-zero exit or signal → `KindExec` with detail `Command failed: {binary} {args}\n{stderr}` (stderr verbatim — an empty stderr leaves the trailing newline); start failure → `KindExec` with the spawn detail (`spawn {binary} ENOENT` when the binary is missing). A successful run's stderr MUST be discarded, never forwarded.

#### Scenario: Deadline fires
- GIVEN an `sh` stub that sleeps 5 s and a context with a 200 ms timeout
- WHEN `Fetch` runs
- THEN it returns promptly with `KindTimeout` and a detail starting `timeout after `

### Requirement: Fetch order of operations
`Source.Fetch(ctx, tool, period, extraArgs, fresh)` SHALL run, in order: (1) cache read with key `cache.Key{Tool, Period, Args}` — skipped when `fresh` or when no cache is set; a hit returns stamped records; (2) resolve the binary and run — any failure returns with no cache write; (3) `Parse` — a parse failure returns with no cache write, and zero records return with no cache write; (4) `Cache.Put` with errors ignored, then stamp and return. `fresh` skips the read but still writes (see [cache](/source/cache.md)).

#### Scenario: Cache hit skips the binary
- GIVEN a `Source` with a temp-dir cache
- WHEN the same `Fetch` runs twice within the TTL
- THEN the binary is invoked exactly once, both results are equal, and every record carries the configured `User`/`Machine`

### Requirement: FetchAll collects in registry order
`FetchAll(ctx, period, extraArgs, fresh)` SHALL fetch every tool in `fact.Tools` concurrently (one goroutine each, `sync.WaitGroup`, one result slot per registry index), never cancelling siblings on a failure, and return the records concatenated in registry order followed by the non-nil errors, also in registry order. A failed tool contributes no records.

#### Scenario: Deterministic order regardless of completion
- GIVEN the fake ccusage serving all six placeholder fixtures
- WHEN `FetchAll` runs for `daily`
- THEN the records are ordered cc×3, codex×3, oc×3, gemini×3, copilot×3, kimi×3 with zero errors; for a period with no fixtures, six `KindExec` errors in registry order and zero records

### Requirement: Identity stamping
`Source{User, Machine}` SHALL be stamped on every record on every return path, after the cache write. Cached records are stored unstamped: the cache key excludes identity, so a config change inside the TTL window is never served a stale user or hostname.

### Requirement: Interface satisfaction at the edge
`*ccusage.Source` SHALL satisfy `command.Fetcher` and the sync `Fetcher`, asserted at compile time where `cmd/tu` assigns it (`src/go/cmd/tu/main.go`). `command.Fetcher.Fetch` takes the `fact.Tool` struct, not the key string.

## Design Decisions

### Vendor-first binary resolution
**Decision**: `ResolveBinary` checks the vendor tree beside the symlink-resolved executable before `PATH`.
**Why**: the shipped tarball and Homebrew layouts install a vendored ccusage beside the real binary; resolving it first guarantees the vendored version wins over any stray `PATH` entry, and symlink resolution makes a `bin/` symlink find the vendor tree beside the real file (vendoring rationale: eu2i).
**Rejected**: a repo-root walk or node_modules probe (there is no repo at runtime; the install layout is the contract).
*Introduced by*: 260917-0118-go-release-pipeline

### Per-agent subcommands for every tool
**Decision**: every registry tool, including `cc`, invokes its per-agent subcommand (`claude`, `codex`, `opencode`, `gemini`, `copilot`, `kimi`).
**Why**: ccusage v20's bare `ccusage daily` is an all-agents aggregate; mapping `cc` to it would over-count `tu cc` and double-count the all-tools total the moment any other agent's transcripts exist locally.
**Rejected**: client-side filtering of the bare aggregate (adds a parsing path for data ccusage already serves cleanly per agent).
*Introduced by*: 260703-ccfx-fix-cc-source-mapping

### No shell in argv composition
**Decision**: argv is a slice passed to `os/exec` verbatim; the registry entry splits the compound command into binary plus prefix args.
**Why**: removes the shell fork per call and any injection surface on interpolated values.
**Rejected**: a shell command string (injection surface, extra fork).
*Introduced by*: 260423-lx0g-exec-csv-completions

### Temp-file capture instead of pipes
**Decision**: `run` captures stdout and stderr into real temp files, removed after the run.
**Why**: a context-killed child can leave grandchildren holding a pipe open; `exec.Wait` would then block past the deadline waiting for pipe EOF. Files make the deadline effective.
**Rejected**: in-memory pipes or `CombinedOutput` (deadline violation on killed children); a capped buffer (a large daily dump must not truncate).
*Introduced by*: 260916-v0as-fact-source-ccusage

### No cache write on failure or empty results
**Decision**: only a non-empty parse result is written to the cache.
**Why**: the fetch returns before the cache write in all three no-write cases — parity with the frozen golden corpus (`/harness/golden-corpus.md`, the retired TypeScript implementation's bytes), whose call-log comparison would flag a skipped binary invocation.
**Rejected**: caching empties (diverges from the compared binary behavior).
*Introduced by*: 260916-v0as-fact-source-ccusage

### WaitGroup, not errgroup
**Decision**: `FetchAll` uses `sync.WaitGroup` with per-index result slots.
**Why**: collect-all semantics are the opposite of errgroup's cancel-on-first-error, and the choice keeps the module dependency-free.
**Rejected**: `golang.org/x/sync/errgroup` (a first dependency with the wrong semantics).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Fetcher.Fetch takes fact.Tool
**Decision**: the interface parameter is the struct, not the key string.
**Why**: the command edge has already resolved the tool after grammar validation; passing the struct spares every adapter a re-lookup and a miss branch, and `source.Error{Tool, Name}` needs the display name anyway.
**Rejected**: `Fetch(ctx, key string, …)` (a smaller interface that pushes a lookup-and-miss path into each adapter).
*Introduced by*: 260916-m9of-g1-rework-1
