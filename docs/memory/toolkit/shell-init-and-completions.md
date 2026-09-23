---
type: memory
description: `tu shell-init` and the embedded completion scripts — completions/tu.{bash,zsh,fish} go:embedded verbatim, eval-safe stdout with diagnostics on stderr only, exit 2 with stdout empty on a missing or unknown shell, and the per-shell frozen token inventory pinned by tests.
---

# Shell Init and Completions

**Domain**: toolkit

## Overview

`tu shell-init <shell>` emits a static completion script for bash, zsh, or fish from three `go:embed`ded files under `internal/toolkit/completions/`, keeping stdout eval-safe by sending every diagnostic to stderr. Dispatch and exit codes live in [entry-point](/command/entry-point.md); the version surface is in [version-and-help-dump](/toolkit/version-and-help-dump.md).

## Requirements

### Requirement: Embedded completion scripts
- The three scripts MUST live as plain files `completions/tu.bash`, `completions/tu.zsh`, `completions/tu.fish` under `src/go/internal/toolkit/`, embedded verbatim via `//go:embed` into the `completions embed.FS` in `shellinit.go`.
- Each script MUST begin with `# tu(1) <shell> completion` and end with exactly one newline (`shellinit_test.go` pins both).
- Each script MUST cover the frozen token inventory pinned per shell in `shellinit_test.go`'s `completionInventory`: the non-data subcommands (`help init-conf init-metrics sync status update shell-init skill`), the sources (`cc codex co oc gemini gem copilot cop kimi ki all`), periods (`d w m daily weekly monthly`), displays (`h history dh wh mh lb lbh`), every long flag (`--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help`), every short flag (`-f -w -i -u -s -j -t -v -V -h`), `cost tokens`, and `bash zsh fish`. Fish spells long flags `-l json`, not `--json`.
- No script MUST mention `help-dump` — the command is hidden (pinned by `shellinit_test.go`).

### Requirement: Completion lookup and messages
- `Completion(shell)` in `internal/toolkit/shellinit.go` MUST return the embedded script for `bash`/`zsh`/`fish` and `(nil, false)` for any other shell name.
- `UnknownShellMessage(shell)` MUST be `Unknown shell: {shell}. Supported: bash, zsh, fish` — the `Shells` constant holds the supported list as it appears in the message.
- `ShellInitUsage` MUST be the 6-line usage block (`Usage: tu shell-init <bash|zsh|fish>` plus a blank line, `Install:`, and the bash/zsh/fish install lines); `cmd/tu` prints it with `Fprintln`, adding the one trailing newline.

#### Scenario: unknown shell
- **GIVEN** `tu shell-init tcsh`
- **WHEN** `runShellInit` in `cmd/tu/main.go` runs
- **THEN** stderr is `Unknown shell: tcsh. Supported: bash, zsh, fish`, stdout is empty, and the exit code is 2 (`command.ExitUsage`)

### Requirement: cmd/tu shell-init behavior
- `runShellInit` in `cmd/tu/main.go` MUST answer three ways: known shell → the script on stdout with no added newline, exit 0; missing shell → `ShellInitUsage` on stderr, exit 2; unknown shell → `UnknownShellMessage` on stderr, exit 2.
- For both error paths stdout MUST be empty — stdout may be eval'd (`eval "$(tu shell-init …)"`), so usage text must never reach it (ba5w).
- Arguments after the shell MUST be ignored.

#### Scenario: missing shell argument
- **GIVEN** `tu shell-init` with no argument
- **WHEN** `runShellInit` runs
- **THEN** the 6-line usage block goes to stderr (with the one trailing newline added by `Fprintln`), stdout is empty, and the exit code is 2

### Requirement: Script validity guards
- `shellinit_test.go` MUST pin the frozen token inventory per shell (each script's own syntax) and the `# tu(1) <shell> completion` first line / single trailing newline shape.
- Per-shell parse subtests (`bash -n`, `zsh -n`, `fish --no-execute` on the script written to a temp file) SHOULD run when the shell is on PATH and MUST skip when it is not — the script is passed as a file argument, so nothing executes.

## Design Decisions

### Completion scripts as embedded files, not Go strings
**Decision**: the three scripts live as plain files under `internal/toolkit/completions/` and are `go:embed`ded verbatim.
**Why**: the zsh script contains backticks a Go raw string cannot hold, and all three carry `${…}` expansions and backslash continuations; hand-un-escaping them into Go strings is the transcription error the differential harness exists to catch. Plain files `diff` cleanly against the oracle's captures ([differential-harness](/harness/differential-harness.md)).
**Rejected**: Go raw strings with `+ "`" +` concatenation (unreadable, error-prone); generating the scripts from the grammar (a redesign of a frozen surface).
*Introduced by*: 260916-vcur-toolkit-layer

### Eval-safe stdout: errors are stderr-only, exit 2
**Decision**: a missing or unknown shell writes usage or the unknown-shell line to stderr with stdout empty and exit 2 (`command.ExitUsage`).
**Why**: stdout may be eval'd by the user's shell (`eval "$(tu shell-init …)"`), so usage text must never reach it; the shell-init standard requires a missing/unsupported shell arg to exit non-zero (convention 2) with usage on stderr and stdout empty.
**Rejected**: printing usage to stdout with exit 0 (a user's `eval` would try to execute the usage text, and a zero exit hides the failure from scripts).
*Introduced by*: 260719-ba5w-update-version-shellinit-conformance
