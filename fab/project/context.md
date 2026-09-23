# Project Context

## Stack

- **Language**: Go (`src/go/go.mod`, `module github.com/sahil87/tu`, go 1.26; non-stdlib dependencies `golang.org/x/term` and `golang.org/x/sys` only)
- **Build**: `just go-build` (dev binary at `bin/tu`), `just go-build-all` (four static cross-compiled targets, `CGO_ENABLED=0`), version via `-ldflags`
- **Lint / test**: `just go-lint` (`gofmt -l` + `go vet ./...`), `just go-test` (`go test ./... -count=1`); golden files regenerate with `go test ./... -update`
- **Task runner**: justfile
- **Distribution**: Homebrew tap (`sahil87/tap`), binary name `tu`; prebuilt `tu-go-<os>-<arch>.tar.gz` release assets (binary + vendored ccusage + `tu.default.conf`) behind a generated formula with no runtime dependencies
- **License**: MIT

## Retired TypeScript tree

`src/node/` is the pre-cutover TypeScript implementation. It no longer ships (the formula flipped at plan row X1, plan: `fab/plans/sahil/26-09-15-go-port.md`) and is frozen: bug fixes only, each paired with a harness fixture and a Go port. It remains in the repo until plan row Z1 (two releases after cutover) as the differential harness's oracle (`node dist/tu.mjs` vs the Go binary via `cmd/tudiff`) and the D10 rollback build. Its toolchain (`npm ci && npm run build && npm test`, esbuild, `__tests__/` co-location) is recorded in `docs/memory/build/toolchain.md`; the constitution no longer governs it.

## Architecture

CLI tool that aggregates cost/usage data from multiple AI coding assistant tools:
- **Claude Code** via `ccusage`
- **Codex** via `ccusage-codex`
- **OpenCode** via `ccusage-opencode`

### Package layout (`src/go/internal/`)

| Package | Responsibility |
|---------|---------------|
| `fact` | `Record` / `Totals`, the six-tool registry |
| `source` | Typed `Error`, `WriteWarnings`; adapters `ccusage` (exec + normalize, vendor-first binary resolution), `metrics` (metrics-repo reader), `cache` (hash-keyed JSON, 60 s TTL) |
| `query` | Pure: window filter, roll-up, `MaxMerge`, `Collapse` over one `GroupBy` |
| `view` | Pure: query result → ANSI-free table models, bars, deltas, leaderboard |
| `render` | Encoders to `[]string` / `io.Writer`: `ansi`, `json`, `csv`, `markdown` — nothing prints |
| `command` | `Request` parsing, `Run` composing source→query→view→render into `Result`; the `Fetcher`/`Repo`/`Writer` seams |
| `sync` | Metrics-repo writer (never-shrink guard), git driver, dry-run report, repair |
| `config` | `tu.conf` / `org.conf` cascade, config-home, `init-conf`, `status`; embeds `tu.default.conf` |
| `watch` | `x/term` TUI loop, compositor, rain, panel |
| `toolkit` | `--version`, `help-dump`, `update`, `shell-init`, `skill` (embedded), completions |
| `harness` | `tudiff` matrix, fixtures, expected-diff ledger (maintainer tooling) |

`cmd/tu` is the only writer to stdout/stderr and the only shipped binary; `cmd/turepair`, `cmd/tudiff`, `cmd/fakeccusage`, `cmd/fakegit` are maintainer/harness tools. Tests are `_test.go` siblings; golden files sit in each package's `testdata/`.

### Modes

- **Single mode** (default): reads from local ccusage output only
- **Multi mode**: syncs metrics to a shared git repo for cross-machine aggregation
