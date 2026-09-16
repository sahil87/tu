---
type: memory
description: The Go port's toolkit layer — internal/toolkit (help-dump envelope via BuildHelpDoc/Encode, no HTML escaping, BareVersion/DisplayVersion/VersionLine, the Brew driver seam with verbatim argv, SIGTERM-graceful 600s/60s bounds and unbounded HOMEBREW_NO_ASK=1 upgrade, the /Cellar/tu/ install gate, embedded completions and skill.md with drift guards) and cmd/tu sequences + exit codes for help/-h/--help, help-dump, skill, shell-init and update; standards audit record (shll v0.1.32, findings S1/V1)
---
# Toolkit Layer (Go port)

**Domain**: go-port

## Overview

`internal/toolkit` is the last leaf package of the target architecture: it owns the shll toolkit contracts the Go binary answers — the version line, the `help-dump` envelope, the shell completion scripts, the `skill` bundle, and the Homebrew self-update machinery — each byte-exact to the shipped TypeScript. `cmd/tu` sequences the steps and prints every line; the package itself never writes to stdout/stderr. Dispatch details (runCommand rows, write order) live in [command-edge](/go-port/command-edge.md); the build-side pieces (`just go-build` guard, version stamping) in [toolchain](/build/toolchain.md).

## Requirements

### Requirement: FullHelp lives in internal/command
`command.FullHelp` (in `request.go`, beside `ShortUsage`) is the byte-exact TS `FULL_HELP` constant — a Go raw string (the text contains no backticks). It is grammar documentation, the same kind of constant as `ShortUsage`, so `command` (which already owns the grammar) owns it; `toolkit` receives the text as a parameter, mirroring the TS `buildHelpDoc({helpText})` input shape. `cmd/tu` prints it with `fmt.Fprintln(stdout, command.FullHelp)` (the TS `console.log` appends exactly one `\n`). `help-dump` is hidden: it appears nowhere in `FullHelp` and in none of the completion scripts.

### Requirement: help-dump envelope
`BuildHelpDoc(version, helpText string) HelpDoc` assembles the flat document — one root node, `Commands` a non-nil empty slice — and `(HelpDoc) Encode(w io.Writer) error` writes it exactly as `JSON.stringify(doc, null, 2) + "\n"` does: `json.NewEncoder` with `SetEscapeHTML(false)` and `SetIndent("", "  ")` (`Encode` appends the trailing `\n`). Envelope field order is `tool, version, schema_version, root`; node order is `name, path, short, usage, text, commands`. Constants: `Tool = "tu"`, `HelpSchemaVersion = 1`, and `Description = "AI coding assistant cost tracking CLI"` — the `package.json` description carried as a Go constant (the long-term shape; a transitional guard test pins it against `package.json`, skipping when none is found walking up from the package directory). `version` is the BARE form (`"0.11.5"`, never `"v0.11.5"`); `usage` is the first `helpText` line starting with `Usage:` (fallback: first non-empty line); `text` is `helpText` verbatim. No `captured_at` (the capture timestamp is shll.ai's puller's), no `aliases` key (omitted entirely when there are none).

`SetEscapeHTML(false)` is load-bearing: `encoding/json` escapes `<`, `>`, `&` by default where `JSON.stringify` does not, and the help text carries `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`.

### Requirement: Version helpers
`BareVersion(v)` strips one leading `v` (`"v0.11.5"` → `"0.11.5"`; `"dev"` → `"dev"`); `DisplayVersion(v)` prefixes `v` only when the bare form starts with an ASCII digit, else returns the input unchanged; `VersionLine(v)` is `"tu version " + DisplayVersion(v)` — the toolkit version standard's canonical `<tool> version vX.Y.Z`. The `--version` path calls `toolkit.VersionLine(version)`; `var version` and the `-X main.version=` stamp live in `main` (the justfile's stamp contract). `help-dump` uses `BareVersion`; the `update` wrapper lines use `DisplayVersion`.

### Requirement: Brew driver seam and the update sequence
`Brew` is the injected driver interface — `Update(ctx) error`, `Info(ctx) ([]byte, error)`, `Upgrade(ctx, stdin io.Reader, stdout, stderr io.Writer) error` — the `internal/sync` `Exec` precedent: `toolkit` prints nothing, and the caller passes the process streams through. `BrewExec` drives the real brew with argv **verbatim from the TS**: `brew update --quiet`, `brew info --json=v2 tu`, `brew upgrade tu` (not the siblings' fully-qualified `sahil87/tap/tu`).

Safety posture: `Update` and `Info` are bounded at **600 s** and **60 s** via `context.WithTimeout`, with `cmd.Cancel` overridden to send **SIGTERM** (never the `exec.CommandContext` default SIGKILL — a kill landing mid-transaction corrupts the keg) and `cmd.WaitDelay = 10 s` of grace before the runtime's forced kill. `Upgrade` is deliberately **unbounded** — no deadline, no `Cancel` — with streams inherited and `HOMEBREW_NO_ASK=1` appended to `os.Environ()` (Homebrew 6 ask-mode suppression; the env var, not `--no-ask`, so Homebrew < 6 is unaffected). Ctrl-C is the user's escape hatch.

`IsBrewInstall(resolvedExe)` is the `/Cellar/tu/` gate on the symlink-resolved executable (the TS `includes("/Cellar/tu/")`, narrower than the siblings' `/Cellar/`). The pure pieces — `CheckLatest` (update-unless-skip, then info+parse of `formulae[0].versions.stable`), `UpToDate`, `Upgrade`, `NotBrewInstallLines`, and the line builders `CurrentVersionLine` / `AlreadyUpToDateLine` / `UpdatingLine` / `UpdatedLine` — plus the three brew-failure messages carried by `UpdateError` are byte-exact to the TS `runUpdate`; `cmd/tu`'s `runUpdate` sequences them and prints.

#### Scenario: update dispatch and exit codes
- **GIVEN** `update --help` or `-h` anywhere in the args, or `update` off-Homebrew (executable does not resolve under `/Cellar/tu/`), or up-to-date, or a successful upgrade
- **WHEN** `runUpdate` executes
- **THEN** the first prints `FullHelp` and runs nothing (exit 0); the second prints the two off-Homebrew lines (`tu v… was not installed via Homebrew.` / `Update manually, or reinstall with: brew install sahil87/tap/tu`), exit 0; up-to-date prints `Already up to date (…).` exit 0; success ends with `Updated to ….` exit 0. Each brew failure maps to its exact stderr line (`Error: could not check for updates (brew update failed). Check your network connection.` / `Error: could not determine latest version.` / `Error: brew upgrade failed.`), exit 1

### Requirement: Embedded shell completions
The three completion scripts live as plain files `completions/tu.{bash,zsh,fish}` under the package, `go:embed`ded verbatim — each byte-identical to the TS constant with the template-literal escapes resolved (`\${` → `${`, `\\` → `\`, `` \` `` → `` ` ``), ending with exactly one `\n` and beginning `# tu(1) <shell> completion`. `Completion(shell)` returns the script for `bash`/`zsh`/`fish` and `false` otherwise; `ShellInitUsage` is the 6-line usage block; `UnknownShellMessage(shell)` is `Unknown shell: {shell}. Supported: bash, zsh, fish`. Edge behaviour (`cmd/tu runShellInit`): known shell → script to stdout, no added newline, exit 0; missing or unknown shell → the usage block or unknown-shell line on stderr, stdout empty, exit 2 (stdout may be eval'd). Arguments after the shell are ignored. Per-shell parse subtests (`bash -n`, `zsh -n`, `fish --no-execute`) skip when the shell is not on PATH; a content test pins the spec's frozen token inventory (the non-data subcommands without `help-dump`, every long and short flag, `cost tokens`, `bash zsh fish`).

### Requirement: Embedded skill bundle with two drift guards
The Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/skill.md` — the mechanism is the committed-copy pattern (the sibling `idea`/`hop` shape, also this repo's own `tu.default.conf` precedent): a byte-identical copy `src/go/internal/toolkit/skill.md` embedded into `var Skill []byte` (`skill.go` carries the `//go:generate ../../../../scripts/sync-skill.sh` directive), `WriteSkill(w)` writing it verbatim. `scripts/sync-skill.sh` (run from the repo root regardless of caller CWD) copies `docs/site/skill.md` over it, printing `synced skill bundle: src/go/internal/toolkit/skill.md`. Two guards keep the copy byte-honest: a test drift guard (`skill_test.go` walks up to the directory containing `package.json`, asserts byte-equality with `docs/site/skill.md`, ≤ 150 lines, trailing `\n`, `WriteSkill` identity) and a build drift guard (`just go-build` runs `cmp -s` on both files before `go build`, failing with `error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh` on stderr, exit 1 — the same failure CI's `go-build-and-test` lane surfaces at its Build step). `cmd/tu` answers `skill` with any arguments — including `topics` — by writing the bundle (the TS ignores arguments), stderr empty, exit 0.

### Requirement: cmd/tu sequences and exit codes
`runCommand` answers the five toolkit commands for real **before** `config.ResolvePaths` — none of them needs `$HOME`: `help`/`-h`/`--help` → `FullHelp + "\n"` on stdout, exit 0; `help-dump` → `BuildHelpDoc(BareVersion(version), FullHelp+"\n").Encode(stdout)`, exit 0; `skill` → `WriteSkill(stdout)`, exit 0; `shell-init` → `runShellInit`; `update` → `runUpdate`. Exit codes follow the spec table: 0 success (including off-Homebrew `update`, up-to-date, and `update --help`), 1 the three brew failures, 2 the shell-init usage errors. See [command-edge](/go-port/command-edge.md) for the full dispatch order.

## Standards audit — shll v0.1.32 (2026-09-17)

Each surface was audited clause by clause against `shll standards <name>` as installed at shll v0.1.32:

| Standard | Go result | Notes |
|---|---|---|
| `help-dump` | conformant | tu keeps the flat document (`commands: []`, full `--help` in `root.text`) the standard's "tu exception" describes; the version comes from `-ldflags`, bare; the standard's prose still says "Node/TS" |
| `update` | conformant | `HOMEBREW_NO_ASK=1` satisfies the wrapped-subprocess prompt-free MUST; bounds only on the metadata calls (SIGTERM + 10 s grace); `brew upgrade` unbounded |
| `shell-init` | conformant | three shells; eval-safe stdout, diagnostics stderr-only, exit 2 on missing/unsupported shell with stdout empty |
| `skill` | conformant except one | **Finding S1**: the reserved `skill topics` clause requires `tu skill topics` to print empty stdout, exit 0; the shipped TS prints the whole bundle (arguments are ignored) and the Go port reproduces it — a frozen-surface matter, not fixable within the port |
| `version` | conformant | `VersionLine` keeps the canonical shape. **Finding V1** (pre-existing, DC-09): `-v` is an extra alias the standard does not require and `--help` does not list |

The audit is also recorded in the audited-standards ledger of [toolchain](/build/toolchain.md).

## Design Decisions

### update through an injected Brew driver
**Decision**: `toolkit` exposes a `Brew` interface with `BrewExec` as the real driver; `toolkit` itself never writes to stdout/stderr, `cmd/tu` sequences the steps and prints the wrapper lines; the interactive `brew upgrade` streams are passed through the driver call.
**Why**: Nothing below `cmd/tu` prints; the `internal/sync` `Exec` + `CloneStep` precedent already execs a subprocess below the edge with streams passed through; a fake driver makes the whole Homebrew sequence unit-testable without brew.
**Rejected**: Running the exec directly in `cmd/tu` (untestable, bloats the edge); a `toolkit.Update(...)` that prints (breaks the only-writer rule).
*Introduced by*: 260916-vcur-toolkit-layer

### Completion scripts as embedded files, not Go strings
**Decision**: The three scripts live as plain files under `internal/toolkit/completions/` and are `go:embed`ded.
**Why**: The zsh script contains backticks and all three contain `${…}` and backslash continuations that the TS template literals escape; a Go raw string cannot hold a backtick, and hand-un-escaping into interpreted strings is the transcription error the harness exists to catch. Files `diff` cleanly against the node captures.
**Rejected**: Go raw strings with `+ "`" +` concatenation (unreadable, error-prone); generating the scripts from the grammar (a redesign of a frozen surface).
*Introduced by*: 260916-vcur-toolkit-layer

### Skill bundle: committed copy, sync script, two guards
**Decision**: A committed byte-identical copy of `docs/site/skill.md` under the package, refreshed by `scripts/sync-skill.sh`, pinned by a `go test` drift guard AND a `cmp` in `just go-build`.
**Why**: The Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/`; the siblings (`idea`, `hop`) and this repo's own `tu.default.conf` embed use the committed-copy pattern; the build-time `cmp` gives the earliest, clearest failure in CI's `Build` step and mirrors `scripts/build.sh`'s post-build guard for the Node bundle.
**Rejected**: A symlink (fragile across platforms/CI checkouts, and `go:embed` refuses symlinks outside the module); reading the file at runtime (a runtime file read for a static bundle, and it would not exist on an installed machine).
*Introduced by*: 260916-vcur-toolkit-layer

### JSON encoding without HTML escaping
**Decision**: `HelpDoc.Encode` uses `json.Encoder` with `SetEscapeHTML(false)` and two-space indent.
**Why**: `encoding/json` escapes `<`, `>`, `&` as `<` etc. by default; `JSON.stringify` does not, and the help text carries `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`. `Encoder.Encode` also appends the trailing `\n` the TS adds explicitly.
**Rejected**: `json.MarshalIndent` + manual post-replacement of the escapes (fragile); a hand-rolled writer (needless).
*Introduced by*: 260916-vcur-toolkit-layer
