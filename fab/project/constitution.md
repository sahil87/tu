# tu Constitution

## Core Principles

### I. Single-Purpose CLI
The tool SHALL remain a focused cost-tracking CLI for AI coding assistants. Feature additions MUST serve the core use case of viewing, aggregating, and syncing usage/cost data. Unrelated functionality (e.g., billing management, AI orchestration) MUST NOT be added.

### II. Graceful Degradation
External dependencies (ccusage binaries, metrics repos, network) MUST NOT crash the CLI. When a data source is unavailable, the tool SHALL warn on stderr and fall back to the best available data (cached, local-only, or zero). The user SHOULD always get *some* output.

### III. Single Static Binary
The CLI MUST build to a single statically linked Go binary (`CGO_ENABLED=0`, version stamped via `-ldflags "-X main.version=…"`) from `src/go/cmd/tu`. It is distributed as a prebuilt per-platform tarball containing exactly `tu`, `vendor/ccusage/bin/ccusage`, and `tu.default.conf`, installed through a generated Homebrew formula that MUST NOT declare `depends_on "node"` or any other runtime dependency. The ccusage binary the CLI runs is the one vendored beside it: resolution MUST prefer `vendor/ccusage/bin/ccusage` relative to the resolved `os.Executable()` and fall back to `PATH` only when the vendored copy is absent. The pinned ccusage version lives in `CCUSAGE_VERSION`. Data files the binary needs at runtime (`tu.default.conf`, the `skill` bundle, shell completions) MUST be embedded with `go:embed`, never read from the repo or a package directory.

### IV. Fast Startup
The CLI SHOULD minimize startup latency. Package initialization (`init()` functions and package-level `var` initializers) MUST NOT perform I/O, process execution, or network access — it is limited to compile-time tables, compiled regexes, sentinel errors, and `go:embed` payloads. All fetching happens inside the command that needs it, under a context deadline, and heavy operations (ccusage execution, metrics-repo reads) MUST be cached on disk with a reasonable TTL. Dependencies SHOULD stay minimal (`golang.org/x/term` and `golang.org/x/sys`, both for the `watch` terminal seam, are the only non-stdlib modules today); adding a module requires a stated reason.

### V. Consistent Data Model
All data flows through the one fact type: `fact.Record` (`{Date, Tool, User, Machine, Totals}`) and `fact.Totals`. New data sources MUST produce `[]fact.Record`. Aggregation (daily-to-weekly/monthly roll-up, merge, group-by) MUST be pure functions operating on these types. The `Date` label MUST use ISO date format (`YYYY-MM-DD` or `YYYY-MM`).

## Go Conventions

- The module is `src/go/` (`module github.com/sahil87/tu`). Code MUST be `gofmt`-clean and `go vet`-clean; `just go-lint` is the gate CI enforces and a PR MUST NOT merge with either failing.
- Layout follows the sibling toolkit repos: `src/go/cmd/tu/` is the only shipped `main`; all library code lives under `src/go/internal/`, one package per pipeline stage — `fact`, `source` (adapters `ccusage`, `metrics`, `cache`), `query`, `view`, `render` (encoders `ansi`, `json`, `csv`, `markdown`), `command`, `sync`, `config`, `watch`, `toolkit`. Other `cmd/` entries (`turepair`, `tudiff`, `fakeccusage`, `fakegit`) are maintainer and harness tools and are never packaged.
- The pipeline is pure between its two I/O ends. `source` (exec, files, git) and `cmd/tu` + `watch` (stdout, terminal) are the only packages that touch the outside world. `query`, `view`, and `render` MUST NOT import `os`, exec anything, or write to a stream — they take values and return values; `render` returns `[]string` or writes to an `io.Writer` handed to it. **Nothing under `internal/` prints**: `fmt.Print*`, `os.Stdout`, and `os.Stderr` appear only in `cmd/tu` and in `watch`'s terminal seam.
- No globals for results or render state. A command's outcome is the returned `command.Result` (lines, notices, warnings, totals); package-level `var`s are limited to compile-time tables, compiled regexes, sentinel errors, and `go:embed` payloads.
- Errors are returned, not printed. Data sources return a typed `*source.Error` beside their (zero) records and never panic; the command edge is the only place that turns errors into stderr lines (`source.WriteWarnings`) and exit codes. Library packages MUST NOT call `os.Exit` or `log.Fatal`.
- Dependency direction follows the pipeline: `fact` depends on nothing; `sync` depends on `fact` and `source/metrics`, never on the ccusage adapter; `command` composes `source → query → view → render`; `cmd/tu` wires adapters to `command`'s interfaces (`Fetcher`, `Repo`, `Writer`). A lower stage MUST NOT import a higher one.
- Prefer functions and plain structs; interfaces are defined by the consumer (`command`) and satisfied at the edge (`cmd/tu`), which is where the compile-time `var _ command.Fetcher = (*ccusage.Source)(nil)` assertions live.

## Additional Constraints

### Test Integrity
Tests MUST conform to the implementation spec — never the other way around. When tests fail, the fix SHALL either (a) update the tests to match the spec, or (b) update the implementation to match the spec. Modifying implementation code solely to accommodate test fixtures or test infrastructure is prohibited. Specs are the source of truth; tests verify conformance to specs.

### Test Runner
Tests use the standard Go toolchain: `go test ./...` from `src/go/` (`just go-test` runs it with `-count=1`). No assertion or mocking framework SHOULD be introduced; table-driven tests with the standard `testing` package are the norm for `query`, `command` parsing, and `config`. Output encoders (`render/*`) and the watch compositor are pinned by **golden files**: expected output lives in `testdata/*.golden` beside the test, compared byte-for-byte, and regenerated only by an explicit `go test ./... -update` (each golden-bearing package declares one package-level `var update = flag.Bool("update", false, …)`). A golden change is an output change and falls under Output Stability. The differential harness (`cmd/tudiff`, `harness/`) is the release gate for the frozen external surfaces and runs in CI as the `tudiff` lane; it complements, not replaces, unit tests.

### Test Location
Test files MUST be `_test.go` siblings of the code they test, in the same package directory (e.g., `src/go/internal/source/ccusage/exec_test.go` for `exec.go`). Golden files and fixtures live in that package's `testdata/` directory. `__tests__/` folders MUST NOT exist anywhere under `src/go/`. Binary-level (end-to-end) tests live beside `main.go` in `src/go/cmd/tu/`.

### Output Stability
CLI output format (table layouts, color usage, JSON structure) SHOULD remain stable across patch versions. Breaking output changes MUST be accompanied by a minor version bump since downstream scripts may parse the output.

### Toolkit Standards

This tool is part of the shll toolkit and MUST conform to the toolkit's published standards. The standards are enumerated by running `shll standards` — each entry names what it governs; read one with `shll standards <name>`. Before changing the CLI surface, help output, README.md, or docs/site/, the change MUST be checked against the standards governing that surface. If shll is unavailable, the canonical sources are the sahil87/shll repository's docs/site/standards/ tree (rendered on https://shll.ai). Standards added or revised there bind this repo without further amendment to this constitution.

## Governance

**Version**: 2.0.0 | **Ratified**: 2026-03-06 | **Last Amended**: 2026-09-23
