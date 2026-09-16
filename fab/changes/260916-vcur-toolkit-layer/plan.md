# Plan: Toolkit Layer — help, help-dump, update, shell-init, skill (Go port row B8)

**Change**: 260916-vcur-toolkit-layer
**Intake**: `intake.md`

## Requirements

> Every surface below is Goal-frozen: the Go binary reproduces the shipped TypeScript bytes (intake §1 table, §9 byte references). Byte references live under `bin/harness/report/cases/<id>/single/default/pipe/fixed/node.*` after `just go-diff --placeholder`; `src/node/core/cli.ts`, `help-dump.ts`, `completions.ts`, `skill.ts` are the oracle sources.

### Toolkit: help text and help-dump

#### R1: FullHelp constant and the help command
`internal/command` SHALL export `FullHelp`, a Go raw-string constant byte-identical to the TS `FULL_HELP` (`src/node/core/cli.ts`), placed beside `ShortUsage` in `request.go`. `cmd/tu` SHALL answer `Request.Command ∈ {help, -h, --help}` by writing `FullHelp` followed by exactly one `\n` to stdout, nothing to stderr, exit 0, before any `$HOME` or config access.

- **GIVEN** argv `help`, `-h`, `--help`, or `help --dry-run`
- **WHEN** `run` executes
- **THEN** stdout is `FullHelp + "\n"` (2,914 bytes at v0.11.5), stderr is empty, exit is 0, and `$HOME` unset does not change the result

#### R2: help-dump envelope
`internal/toolkit` SHALL provide `BuildHelpDoc(version, helpText string) HelpDoc` and `(HelpDoc) Encode(w io.Writer) error`. The envelope field order MUST be `tool, version, schema_version, root`; the node order MUST be `name, path, short, usage, text, commands`. `Tool` and `root.name`/`root.path` are `"tu"`; `root.short` is `Description` (`"AI coding assistant cost tracking CLI"`, the package.json description); `root.usage` is the first line of `helpText` starting with `Usage:` (fallback: first non-empty line); `root.text` is `helpText` verbatim; `root.commands` is a non-nil empty slice (`[]`); `schema_version` is `1`; `version` is the BARE version (`BareVersion`). `Encode` MUST write exactly what `JSON.stringify(doc, null, 2) + "\n"` writes: `json.Encoder` with `SetEscapeHTML(false)` and `SetIndent("", "  ")`. No `captured_at`, no `aliases` key. `cmd/tu` answers `help-dump` with `BuildHelpDoc(BareVersion(version), command.FullHelp+"\n").Encode(stdout)`, stderr empty, exit 0.

- **GIVEN** the stamped version `v0.11.5`
- **WHEN** `tu help-dump` runs
- **THEN** stdout is the 3,224-byte document from the node capture: `"version": "0.11.5"`, `<date>` appears raw (never `<date>`), `"commands": []` on one line, the document ends `}` + `\n`, stderr is empty, exit 0

#### R3: Version helpers
`internal/toolkit` SHALL provide `BareVersion(v)` (strips one leading `v`), `DisplayVersion(v)` (`"v" + bare` when `bare` starts with an ASCII digit, else `v` unchanged) and `VersionLine(v)` (`"tu version " + DisplayVersion(v)`). `cmd/tu`'s `versionLine` is REMOVED and the `--version` path calls `toolkit.VersionLine(version)`; `var version` and the `-X main.version=` stamp stay in `main`.

- **GIVEN** versions `v0.11.5`, `0.11.5`, `dev`, `""`
- **WHEN** the three helpers run
- **THEN** `BareVersion` gives `0.11.5`, `0.11.5`, `dev`, `""`; `VersionLine` gives `tu version v0.11.5`, `tu version v0.11.5`, `tu version dev`, `tu version ` — and the harness cases `version`/`version-short` stay green

### Toolkit: shell-init

#### R4: Embedded completion scripts
`internal/toolkit` SHALL embed `completions/tu.bash`, `completions/tu.zsh`, `completions/tu.fish` via `//go:embed` and expose `Completion(shell string) ([]byte, bool)` returning the script for `bash`/`zsh`/`fish` and `false` otherwise. Each file MUST be byte-identical to the value of the TS constant (`BASH_COMPLETION`/`ZSH_COMPLETION`/`FISH_COMPLETION` in `completions.ts`) — the template-literal escapes resolved (`\${` → `${`, `\\` → `\`, `` \` `` → `` ` ``), ending with exactly one `\n`. Each script begins with `# tu(1) {shell} completion` and covers the spec's token inventory: non-data subcommands `help init-conf init-metrics sync status update shell-init skill` (never `help-dump`), all source/period/display tokens (incl. `w weekly wh lb lbh kimi ki gem cop`), every long flag (incl. `--skip-brew-update`, `--version`, `--help`), every short flag (`-f -w -i -u -s -j -t -v -V -h`), `cost tokens`, `bash zsh fish`. `cmd/tu` writes the script to stdout with no added newline, stderr empty, exit 0.

- **GIVEN** argv `shell-init zsh`
- **WHEN** `run` executes
- **THEN** stdout equals the node capture `shell-init-zsh/…/node.stdout` byte for byte, stderr is empty, exit 0; `zsh -n` accepts the bytes

#### R5: shell-init usage errors
With no shell argument, `cmd/tu` SHALL write `toolkit.ShellInitUsage` (the 6-line block: `Usage: tu shell-init <bash|zsh|fish>`, blank, `Install:`, three indented install lines) plus `\n` to stderr, leave stdout empty, exit 2. With an unknown shell it SHALL write `Unknown shell: {shell}. Supported: bash, zsh, fish` + `\n` to stderr, stdout empty, exit 2. Arguments after the shell are ignored.

- **GIVEN** argv `shell-init` and `shell-init tcsh`
- **WHEN** `run` executes
- **THEN** stderr is 223 bytes (the usage block) and `Unknown shell: tcsh. Supported: bash, zsh, fish\n` respectively, stdout is empty, exit 2 in both cases

### Toolkit: skill

#### R6: Embedded skill bundle with two drift guards
`internal/toolkit` SHALL embed a committed copy `src/go/internal/toolkit/skill.md` (`//go:embed skill.md` → `Skill []byte`) and expose `WriteSkill(w io.Writer) error` writing it verbatim. `scripts/sync-skill.sh` copies `docs/site/skill.md` to that path (run from the repo root; prints `synced skill bundle: src/go/internal/toolkit/skill.md`); `skill.go` carries a `//go:generate` directive pointing at it. A test drift guard MUST assert `Skill` equals `docs/site/skill.md` (found by walking up to the directory containing `package.json`), that the bundle is ≤ 150 lines, and that it ends with `\n`. `just go-build` MUST run `cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md` before `go build` and fail with `error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh` on stderr, exit 1, when they differ. `cmd/tu` answers `skill` (any args) with `WriteSkill(stdout)`, stderr empty, exit 0.

- **GIVEN** the committed copy equals `docs/site/skill.md`
- **WHEN** `tu skill` and `tu skill topics` run
- **THEN** stdout is the 6,285-byte bundle in both cases (`cmp` clean against `docs/site/skill.md`), stderr empty, exit 0; and when one byte of the copy is changed, `go test ./internal/toolkit` fails the drift guard and `just go-build` exits 1 with the message above

### Toolkit: update

#### R7: update sequence, gate and messages
`cmd/tu` SHALL implement `update` in the TS `runUpdate` order using pure/driver pieces from `internal/toolkit`: (1) `--help` or `-h` anywhere in `req.Args` → `FullHelp + "\n"` on stdout, exit 0, nothing else runs; (2) `toolkit.IsBrewInstall(resolved)` where `resolved` is `filepath.EvalSymlinks(os.Executable())` and the test is `strings.Contains(resolved, "/Cellar/tu/")` — false → stdout `tu v0.11.5 was not installed via Homebrew.` then `Update manually, or reinstall with: brew install sahil87/tap/tu`, exit 0; (3) `Current version: v0.11.5`; (4) unless `req.Flags.SkipBrewUpdate`, `brew.Update` — any error → stderr `Error: could not check for updates (brew update failed). Check your network connection.`, exit 1; (5) `brew.Info` parsed for `formulae[0].versions.stable` (a non-string or empty value or any exec/parse error) → stderr `Error: could not determine latest version.`, exit 1; (6) `BareVersion(version) == latest` → `Already up to date (v0.11.5).`, exit 0; (7) `Updating v0.11.5 → v0.11.6...`, `brew.Upgrade` with `os.Stdin`/stdout/stderr passed through — error → stderr `Error: brew upgrade failed.`, exit 1; success → `Updated to v0.11.6.`, exit 0. Version strings in these lines use `DisplayVersion`. `toolkit` writes nothing to stdout/stderr itself (the driver passes the upgrade streams through).

- **GIVEN** the `go test` binary (never under `/Cellar/tu/`)
- **WHEN** `run([]string{"update"})` executes
- **THEN** stdout is the two off-Homebrew lines, stderr empty, exit 0; and `run([]string{"update", "--help"})` prints `FullHelp + "\n"`, exit 0

- **GIVEN** a fake `Brew` whose `Info` returns `{"formulae":[{"versions":{"stable":"0.11.5"}}]}` and version `v0.11.5`
- **WHEN** the Homebrew sequence runs with `SkipBrewUpdate` true
- **THEN** `Update` is never called, `Upgrade` is never called, and the lines are `Current version: v0.11.5` then `Already up to date (v0.11.5).`

#### R8: Brew driver safety
`toolkit.Brew` is the driver interface (`Update(ctx) error`, `Info(ctx) ([]byte, error)`, `Upgrade(ctx, stdin io.Reader, stdout, stderr io.Writer) error`); `toolkit.BrewExec` is the real driver with argv verbatim from the TS: `brew update --quiet`, `brew info --json=v2 tu`, `brew upgrade tu`. `Update` and `Info` MUST be bounded — `context.WithTimeout` of 600 s and 60 s — through a helper that sets `cmd.Cancel` to send `SIGTERM` and `cmd.WaitDelay = 10 * time.Second` (never the default `SIGKILL`). `Upgrade` MUST have no deadline and no `Cancel` (`context.Background()` semantics), MUST set `cmd.Env = append(os.Environ(), "HOMEBREW_NO_ASK=1")`, and MUST pass the given streams through. No code path reads stdin for a confirmation.

- **GIVEN** the exported/testable command constructors
- **WHEN** the upgrade command is built
- **THEN** `Args == ["brew", "upgrade", "tu"]`, `Env` contains `HOMEBREW_NO_ASK=1`, `Cancel == nil`, `WaitDelay == 0`; and a bounded command has `Cancel != nil` and `WaitDelay == 10s`

### Edge: dispatch

#### R9: runCommand rows and exit codes
`cmd/tu` `runCommand` SHALL answer `help`/`-h`/`--help`, `help-dump`, `skill`, `shell-init`, `update` for real BEFORE `config.ResolvePaths` (none needs `$HOME`), keep `init-conf`/`init-metrics`/`status` as today, and leave `sync` on the placeholder. Exit codes follow the spec table: 0 success (incl. off-Homebrew `update`, up-to-date, `update --help`), 1 brew failures, 2 shell-init usage errors. `TestRunNotImplemented` MUST stop using `--help` as its unported example and use `sync` instead.

- **GIVEN** `HOME` unset
- **WHEN** `help`, `help-dump`, `skill`, `shell-init bash`, `update --help` run
- **THEN** each succeeds exactly as with `HOME` set (no `tu: $HOME is not set` error)

### Verification

#### R10: Harness gate and tests
`just go-diff --placeholder` MUST report **144 green** of 368 with these ten cases newly green: `help` (pipe + tty), `help-cmd`, `help-dump`, `skill`, `shell-init-bash`, `shell-init-zsh`, `shell-init-fish`, `shell-init-missing`, `shell-init-unknown`; no other case changes colour. `just go-lint`, `just go-test`, `just go-build` pass. `cmd/tu/main_test.go` covers every brew-free row of intake §1's table; `internal/toolkit` tests cover the helpers, the fake-`Brew` sequence, the command shapes, the script inventory and the parse checks (`bash -n`, `zsh -n`, `fish --no-execute`, each `t.Skip`ped when the shell is not on PATH).

- **GIVEN** the build after T011
- **WHEN** `bin/harness/tudiff run --placeholder` runs
- **THEN** the summary line reads `368 cases — 144 green, 224 red, 0 timeout`

### Non-Goals
- No change to `src/node/`, `docs/specs/`, `docs/site/`, `README.md`, `harness/matrix.json`, `.github/workflows/`, `Formula/`, `scripts/build.sh`, or the plan document (D4 freeze; the operator owns row status).
- No `update` harness group (it would spawn brew); no fix for the `skill topics` standards finding S1 (frozen surface — Open Question).
- No fully-qualified `sahil87/tap/tu` brew argv (parity with the TS).

### Design Decisions

#### update through an injected Brew driver
**Decision**: `toolkit` exposes a `Brew` interface with `BrewExec` as the real driver; `toolkit` itself never writes to stdout/stderr, `cmd/tu` sequences the steps and prints the wrapper lines; the interactive `brew upgrade` streams are passed through the driver call.
**Why**: G1 items 1/4 — nothing below `cmd/tu` prints; the `internal/sync` `Exec` + `CloneStep` precedent already execs a subprocess below the edge with streams passed through; a fake driver makes the whole Homebrew sequence unit-testable without brew.
**Rejected**: Running the exec directly in `cmd/tu` (untestable, bloats the edge); a `toolkit.Update(...)` that prints (breaks the only-writer rule).
*Introduced by*: 260916-vcur-toolkit-layer

#### Completion scripts as embedded files, not Go strings
**Decision**: The three scripts live as plain files under `internal/toolkit/completions/` and are `go:embed`ded.
**Why**: The zsh script contains backticks and all three contain `${…}` and backslash continuations that the TS template literals escape; a Go raw string cannot hold a backtick, and hand-un-escaping into interpreted strings is the transcription error the harness exists to catch. Files `diff` cleanly against the node captures.
**Rejected**: Go raw strings with `+ "`" +` concatenation (unreadable, error-prone); generating the scripts from the grammar (a redesign of a frozen surface).
*Introduced by*: 260916-vcur-toolkit-layer

#### Skill bundle: committed copy, sync script, two guards
**Decision**: A committed byte-identical copy of `docs/site/skill.md` under the package, refreshed by `scripts/sync-skill.sh`, pinned by a `go test` drift guard AND a `cmp` in `just go-build`.
**Why**: The Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/`; the siblings (`idea`, `hop`) and this repo's own `tu.default.conf` embed use the committed-copy pattern; the build-time `cmp` gives the earliest, clearest failure in CI's `Build` step and mirrors `scripts/build.sh`'s post-build guard for the Node bundle.
**Rejected**: A symlink (fragile across platforms/CI checkouts, and `go:embed` refuses symlinks outside the module); reading the file at runtime (a runtime file read for a static bundle, and it would not exist on an installed machine).
*Introduced by*: 260916-vcur-toolkit-layer

#### JSON encoding without HTML escaping
**Decision**: `HelpDoc.Encode` uses `json.Encoder` with `SetEscapeHTML(false)` and two-space indent.
**Why**: `encoding/json` escapes `<`, `>`, `&` as `<` etc. by default; `JSON.stringify` does not, and the help text carries `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`. `Encoder.Encode` also appends the trailing `\n` the TS adds explicitly.
**Rejected**: `json.MarshalIndent` + manual post-replacement of the escapes (fragile); a hand-rolled writer (needless).
*Introduced by*: 260916-vcur-toolkit-layer

## Tasks

### Phase 1: Setup

- [x] T001 Create `src/go/internal/toolkit/version.go` with `BareVersion`, `DisplayVersion`, `VersionLine` (package doc names the toolkit standards audited: help-dump, update, shell-init, skill, version — shll v0.1.32, 2026-09-17) and `version_test.go` (table from R3); delete `versionLine` from `src/go/cmd/tu/main.go`, call `toolkit.VersionLine(version)` on the `--version` path, and move `TestVersionLine` from `main_test.go` into the toolkit test. <!-- R3 -->
- [x] T002 [P] Add the `FullHelp` constant to `src/go/internal/command/request.go` beside `ShortUsage`, byte-exact to `FULL_HELP` in `src/node/core/cli.ts` (verify: `printf '%s\n' "$FullHelp"` equals `bin/harness/report/cases/help-cmd/single/default/pipe/fixed/node.stdout`); add a `request_test.go`/`parse_test.go` assertion that `FullHelp` starts with `Usage: tu [source] [period] [display]`, contains `--skip-brew-update`, and does not contain `help-dump`. <!-- R1 -->
- [x] T003 [P] Create `src/go/internal/toolkit/completions/tu.bash`, `tu.zsh`, `tu.fish` from the TS constants in `src/node/core/completions.ts` with escapes resolved (the simplest exact method: `node dist/tu.mjs shell-init <shell> > file` after `just build`, then `cmp` each against `bin/harness/report/cases/shell-init-<shell>/single/default/pipe/fixed/node.stdout`). <!-- R4 -->
- [x] T004 [P] Add `scripts/sync-skill.sh` (executable; `cd "$(dirname "$0")/.."`, `cp -f docs/site/skill.md src/go/internal/toolkit/skill.md`, echo `synced skill bundle: …`), run it once to create the committed copy, and add the `cmp -s … || { echo "error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh" >&2; exit 1; }` guard at the top of the `go-build` recipe in `justfile` (before `go build`). <!-- R6 -->

### Phase 2: Core Implementation

- [x] T005 Write `src/go/internal/toolkit/helpdump.go` (`Tool`, `Description`, `HelpSchemaVersion` constants; `HelpNode`/`HelpDoc` structs with the ordered JSON tags; `BuildHelpDoc`; `Encode` via `json.Encoder` + `SetEscapeHTML(false)` + `SetIndent("", "  ")`) and `helpdump_test.go`: encode a fixed doc and compare to an inline expected string; assert `<date>` raw, bare version, `"commands": []`, trailing `\n`, usage-line extraction and fallback; a `Description`-vs-`package.json` guard that walks up to `package.json` (skip when absent). <!-- R2 -->
- [x] T006 Write `src/go/internal/toolkit/shellinit.go` (`//go:embed completions/tu.bash completions/tu.zsh completions/tu.fish`; `Shells = "bash, zsh, fish"`; `Completion`; `ShellInitUsage`; `UnknownShellMessage`) and `shellinit_test.go`: each script non-empty, first line `# tu(1) <shell> completion`, ends with one `\n`, contains the R4 token inventory and never `help-dump`; `Completion("tcsh")` false; `UnknownShellMessage("tcsh")` byte-exact; parse subtests `bash -n`, `zsh -n`, `fish --no-execute` (stdin-fed, `t.Skip` when `exec.LookPath` fails). <!-- R4, R5 -->
- [x] T007 Write `src/go/internal/toolkit/skill.go` (`//go:generate ../../../../scripts/sync-skill.sh`; `//go:embed skill.md` → `Skill []byte`; `WriteSkill`) and `skill_test.go`: drift guard against `docs/site/skill.md` (walk up to `package.json`, mirroring `internal/config/defaults_test.go`), `≤ 150` lines, trailing `\n`, `WriteSkill` output equals `Skill`. <!-- R6 -->
- [x] T008 Write `src/go/internal/toolkit/update.go`: `Brew` interface, `BrewExec` (bounded `Update`/`Info` via a `boundedBrewCmd(ctx, args...)` helper — `Cancel` → `SIGTERM`, `WaitDelay` 10 s, timeouts 600 s / 60 s; `Upgrade` unbounded with `HOMEBREW_NO_ASK=1` appended to `os.Environ()`, streams passed through), `IsBrewInstall`, `NotBrewInstallLines`, `CheckLatest` (update-unless-skip then info+parse; returns `*UpdateError` with the exact message), `UpToDate`, `Upgrade`, `UpdateError`, `CurrentVersionLine`, `AlreadyUpToDateLine`, `UpdatingLine`, `UpdatedLine`; keep the exec constructors testable (unexported helpers returning `*exec.Cmd`). `update_test.go`: fake `Brew` recording calls — skip flag skips `Update`; `Update` error → its message; `Info` error / bad JSON / empty stable → `could not determine latest version`; equal → `UpToDate`; `Upgrade` error → its message and the passed writers receive the fake's output; `IsBrewInstall` true/false; the R8 command-shape assertions. <!-- R7, R8 -->

### Phase 3: Integration & Edge Cases

- [x] T009 Wire `src/go/cmd/tu/main.go` `runCommand`: rows for `help`/`-h`/`--help` (`Fprintln(stdout, command.FullHelp)`), `help-dump`, `skill`, `shell-init` (`runShellInit(args, stdout, stderr) int`), `update` (`runUpdate(req, stdout, stderr) int` implementing R7 with `toolkit.BrewExec{}`, `os.Stdin`, `os.Executable` + `filepath.EvalSymlinks`), all before `config.ResolvePaths`; `sync` stays on the placeholder; update the file-header comment listing the answered surfaces. <!-- R9, R7, R5, R1, R2, R6 -->
- [x] T010 Update `src/go/cmd/tu/main_test.go`: `TestRunNotImplemented` uses `{"sync"}` and `{"h", "--by-machine"}`; add table-driven tests (with `HOME` unset where R9 says it must not matter) for `help`/`-h`/`--help`/`help --dry-run` (stdout `command.FullHelp+"\n"`), `help-dump` (parses; `version` bare from a stubbed `version = "v1.2.3"` → `"1.2.3"`; contains `<date>` raw; stderr empty), `skill` (stdout equals `toolkit.Skill`), the five `shell-init` cases (bytes equal `toolkit.Completion`; the two exit-2 stderr strings), `update --help` / `update -h` / `update --skip-brew-update -h`, and `update` off-Homebrew (the two lines, exit 0). <!-- R9, R10 -->
- [x] T011 Run `just go-lint`, `just go-test`, `just go-build`, then `just go-diff --placeholder --filter help`, `--filter skill`, `--filter shell-init` and the full `just go-diff --placeholder`; fix any byte divergence in the embedded files or constants until the summary reads `144 green, 224 red, 0 timeout` (exit 1 is expected while any case is red — read the summary line). <!-- R10 -->

### Phase 4: Polish

- [x] T012 Prove the build drift guard: append a byte to `src/go/internal/toolkit/skill.md`, confirm `just go-build` exits 1 with the R6 message and `go test ./internal/toolkit` fails, then run `scripts/sync-skill.sh` to restore and confirm both pass again; `gofmt -l src/go` is empty. <!-- R6 -->

## Execution Order

- T001–T004 are independent; T001 must land before T009 (the `versionLine` removal).
- T005–T008 depend on T001 (package exists) and T003/T004 (embedded files exist); they are otherwise independent.
- T009 depends on T002 and T005–T008; T010 on T009; T011 on T010; T012 on T011.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `command.FullHelp` is byte-identical to the TS `FULL_HELP`; `tu help`, `tu -h`, `tu --help` print it plus one newline, exit 0, stderr empty
- [x] A-002 R2: `tu help-dump` output equals the node capture byte for byte (3,224 bytes at v0.11.5); the envelope has no `captured_at` and no `aliases`
- [x] A-003 R3: `toolkit.BareVersion`/`DisplayVersion`/`VersionLine` exist with the R3 table behaviour; `cmd/tu` no longer defines `versionLine`
- [x] A-004 R4: `tu shell-init bash|zsh|fish` output equals the respective node captures byte for byte; the files carry the full token inventory and never mention `help-dump`
- [x] A-005 R5: `tu shell-init` and `tu shell-init tcsh` write the exact usage block / unknown-shell line to stderr, leave stdout empty, exit 2
- [x] A-006 R6: `tu skill` (and `tu skill topics`) print `docs/site/skill.md` byte for byte, stderr empty, exit 0; `scripts/sync-skill.sh` exists, is executable, and the committed copy equals the canonical file
- [x] A-007 R7: `update --help`/`-h` print `FullHelp`, exit 0, without touching brew; the off-Homebrew path prints the two lines and exits 0; the Homebrew sequence and its five wrapper lines and three error lines are byte-exact to `runUpdate`
- [x] A-008 R8: `BrewExec` argv is `brew update --quiet`, `brew info --json=v2 tu`, `brew upgrade tu`; the upgrade command carries `HOMEBREW_NO_ASK=1`, no deadline, no `Cancel`; the bounded commands use `SIGTERM` + `WaitDelay` 10 s
- [x] A-009 R9: the five toolkit commands are answered before `config.ResolvePaths` and succeed with `HOME` unset; `sync` remains on the placeholder

### Behavioral Correctness

- [x] A-010 R2: `Encode` never HTML-escapes — a unit test asserts `<date>` appears raw in the encoded bytes and `<` does not
- [x] A-011 R2: `"version"` in help-dump is bare (`0.11.5`) while `--version` still prints `tu version v0.11.5`
- [x] A-012 R7: with `--skip-brew-update`, the fake driver records no `Update` call; without it, `Update` precedes `Info`
- [x] A-013 R9: `TestRunNotImplemented` no longer lists `--help`; `sync` is its unported example

### Scenario Coverage

- [x] A-014 R10: `just go-diff --placeholder` summary reads `144 green, 224 red, 0 timeout`; the ten named cases are green; `version`/`version-short` remain green
- [x] A-015 R4: parse subtests (`bash -n`, `zsh -n`, `fish --no-execute`) exist and pass where the shell is installed, skip otherwise
- [x] A-016 R7: fake-`Brew` tests cover up-to-date, upgrade success, and each of the three error paths with their exact stderr lines

### Edge Cases & Error Handling

- [x] A-017 R6: mutating the committed `skill.md` copy fails both `go test ./internal/toolkit` and `just go-build` (with the R6 message); `scripts/sync-skill.sh` restores it
- [x] A-018 R3: an unstamped `dev` version yields `tu version dev`, help-dump `"version": "dev"`, and `tu dev was not installed via Homebrew.`
- [x] A-019 R5: `tu shell-init bash extra` still prints the bash script and exits 0 (extra args ignored)
- [x] A-020 R7: `brew info` returning JSON without a string `stable` maps to `Error: could not determine latest version.`, exit 1

### Code Quality

- [x] A-021 Pattern consistency: `toolkit` follows the sibling package shapes (`defaults_test.go` walk-up helper, `sync.Exec` driver style, `run(args, stdout, stderr) int` seam); `gofmt`/`go vet` clean
- [x] A-022 No unnecessary duplication: the walk-up-to-`package.json` helper and the version normalization are single-sourced within their packages; no second copy of `ShortUsage`/`FullHelp`
- [x] A-023 Readability over cleverness: message strings live in named constants/functions, not scattered literals; no god functions (>50 lines) in `update.go` or `main.go` beyond the sequential `runUpdate`
- [x] A-024 Minimum pathways: one `Completion` lookup for all three shells; one `Encode` path; no parallel dev/prod code paths for the skill bundle
- [x] A-025 Errors never swallowed silently: every brew failure maps to a specific stderr line and exit 1; embed read failures surface as errors

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`
- Byte references are regenerated by `just go-diff --placeholder` into `bin/harness/report/cases/` (gitignored); `node dist/tu.mjs <cmd>` after `just build` is the same oracle.

## Deletion Candidates

- None — this change adds new functionality without making existing code redundant. The two symbols it obsoleted (`versionLine` and the `toolName` constant in `src/go/cmd/tu/main.go`, plus `TestVersionLine` in `src/go/cmd/tu/main_test.go`) were already removed/moved to `internal/toolkit` by the change itself.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | Exported helper names as written (`BuildHelpDoc`, `Completion`, `WriteSkill`, `CheckLatest`, …); apply MAY adjust names but not the split of responsibilities | Intake §3–§7 sketches; names are internal, the harness pins bytes | S:65 R:95 A:85 D:75 |
| 2 | Confident | Completion files are produced by capturing `node dist/tu.mjs shell-init <shell>` rather than hand-editing the TS literals | Identical bytes by construction; `just build` already exists in the worktree flow | S:60 R:95 A:90 D:80 |
| 3 | Confident | `fish --no-execute` is the fish parse-check flag; all three parse subtests skip when the shell is absent | fish documents `-n/--no-execute`; CI runners may lack fish/zsh | S:55 R:95 A:80 D:75 |
| 4 | Certain | The `Description` guard skips (not fails) when no `package.json` is found walking up | Post-Z1 the file disappears; the guard is transitional | S:70 R:95 A:90 D:85 |
| 5 | Certain | `TestRunNotImplemented` keeps `{"h","--by-machine"}` and swaps `{"--help"}` for `{"sync"}` | `sync` is B6's and stays unported after B8 | S:80 R:95 A:95 D:90 |

5 assumptions (2 certain, 3 confident, 0 tentative).
