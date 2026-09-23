---
type: memory
description: The version helpers (BareVersion/DisplayVersion/VersionLine) and the help-dump envelope (BuildHelpDoc/HelpDoc.Encode) — the flat {tool, version, schema_version, root} document with "commands": [], encoded with SetEscapeHTML(false), consumed by shll.ai's pull cron.
---

# Version and Help Dump

**Domain**: toolkit

## Overview

`internal/toolkit` owns two byte-level toolkit contracts: the canonical version line and the `help-dump` JSON envelope that shll.ai's pull cron invokes against the installed binary. Dispatch and exit codes live in [entry-point](/command/entry-point.md); version stamping and the skill drift guard live in [toolchain](/build/toolchain.md); the update flow is in [update](/toolkit/update.md).

## Requirements

### Requirement: Version helpers
- `BareVersion(v)` in `internal/toolkit/version.go` MUST strip one leading `v` (`"v0.11.5"` → `"0.11.5"`; `"dev"` → `"dev"`).
- `DisplayVersion(v)` MUST prefix `v` only when the bare form starts with an ASCII digit; otherwise it returns the input unchanged, so an unstamped `"dev"` stays `"dev"`. Both `-X main.version=0.11.5` and `-X main.version=v0.11.5` stamps MUST print identically.
- `VersionLine(v)` MUST render `"tu version " + DisplayVersion(v)` — the toolkit version standard's canonical `<tool> version vX.Y.Z` shape. `cmd/tu` prints it for `--version`/`-V`/`-v` after grammar validation, exit 0 (dispatch in [entry-point](/command/entry-point.md)).
- The stamp is `var version = "dev"` in `src/go/cmd/tu/main.go`, set at build time via `-ldflags "-X main.version=…"` (stamping contract in [toolchain](/build/toolchain.md)).

#### Scenario: unstamped dev build
- **GIVEN** a build with no `-X main.version=` stamp (`version` stays `"dev"`)
- **WHEN** `tu --version` runs
- **THEN** stdout is exactly `tu version dev` (no `v` prefix) and the exit code is 0

### Requirement: help-dump envelope
- `BuildHelpDoc(version, helpText string) HelpDoc` in `internal/toolkit/helpdump.go` MUST assemble a flat document — one root node whose `Commands` is a non-nil empty slice, serialized as the one-line `"commands": []`.
- Field order is frozen: envelope `tool, version, schema_version, root`; node `name, path, short, usage, text, commands`. The JSON is a frozen cross-repo contract — fields MUST NOT be reordered or renamed.
- Constants: `Tool = "tu"`, `HelpSchemaVersion = 1`, `Description = "AI coding assistant cost tracking CLI"`. The description is a Go constant; `TestDescriptionMatchesPackageJSON` in `helpdump_test.go` pins it against `package.json`'s `description` while that file exists, skipping when no `package.json` is found walking up from the package directory.
- The envelope's `version` MUST be the BARE form (`"0.11.5"`, never `"v0.11.5"` — the caller passes `BareVersion(version)`). `usage` MUST be the first `helpText` line starting with `Usage:` (fallback: first non-empty line, then `""`) via `extractUsage`. `text` MUST be `helpText` verbatim — the caller passes `command.FullHelp + "\n"`, byte-identical to `tu --help`.
- The envelope MUST NOT carry `captured_at` (the capture timestamp is owned by shll.ai's puller — a tool cannot know its own capture time) nor an `aliases` key (omitted entirely when there are none) (rdo3).

#### Scenario: serialization is the frozen wire shape
- **GIVEN** `BuildHelpDoc("1.2.3", "Usage: tu [source] [period] [display]\n\nbody\n")`
- **WHEN** `HelpDoc.Encode(w)` runs
- **THEN** the output is exactly the two-space-indented document with envelope order `tool, version, schema_version, root`, the one-line `"commands": []`, and one trailing newline — pinned byte-for-byte by `TestHelpDocEncodeGolden` in `helpdump_test.go`

### Requirement: Encoding without HTML escaping
- `HelpDoc.Encode(w io.Writer) error` in `internal/toolkit/helpdump.go` MUST use `json.NewEncoder` with `SetEscapeHTML(false)` and `SetIndent("", "  ")`; `Encoder.Encode` appends the one trailing newline of the frozen `JSON.stringify(doc, null, 2) + "\n"` shape.

#### Scenario: help text carries angle brackets
- **GIVEN** help text carrying `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`
- **WHEN** `HelpDoc.Encode` serializes the document
- **THEN** the angle brackets appear raw — `encoding/json`'s default would escape them as `<`, `>`, `&` etc., bytes the frozen wire shape does not contain (`TestHelpDocEncodeNoHTMLEscape` in `helpdump_test.go` pins both the raw `<date>` and the absence of any escaped form)

### Requirement: shll.ai pull contract
- `tu help-dump` MUST be a real in-binary subcommand: stdout only, exit 0, nothing on stderr — shll.ai treats any stderr on success as contract drift (rdo3).
- shll.ai MUST pull: its own scheduled cron installs tu via Homebrew, runs `tu help-dump`, validates, and direct-commits `help/tu.json` to the shll.ai repo with the default `GITHUB_TOKEN`. tu pushes nothing; the puller stamps `captured_at` after capture (dmhw, rdo3).
- `help-dump` is hidden: it MUST appear nowhere in `command.FullHelp` and in none of the completion scripts (`shellinit_test.go` pins the absence from all three scripts).

## Design Decisions

### JSON encoding without HTML escaping
**Decision**: `HelpDoc.Encode` uses `json.Encoder` with `SetEscapeHTML(false)` and a two-space indent.
**Why**: parity with the frozen `src/node/` oracle (until plan row Z1) — `JSON.stringify` does not escape `<`, `>`, `&` where `encoding/json` does by default, and the help text carries `<date>`, `<m>`, `<n>`, `<s>`, `<user>`, `<sh>`; `Encoder.Encode` also appends the one trailing newline of the frozen shape.
**Rejected**: `json.MarshalIndent` plus manual post-replacement of the escapes (fragile); a hand-rolled writer (needless).
*Introduced by*: 260916-vcur-toolkit-layer

### Flat document, not a walked subcommand tree
**Decision**: tu emits a structurally valid flat contract document — one root node, `commands: []`, the full `--help` text in `root.text`.
**Why**: tu is a flag-based CLI with no walkable subcommand tree, so it cannot reuse the sibling Go tools' producer that recurses a Cobra tree; the help-dump standard's "tu exception" describes exactly this flat shape (v76l).
**Rejected**: Synthesizing a fake command tree to match the siblings (invents per-subcommand help pages tu does not print).
*Introduced by*: 260602-v76l-help-dump-shll-ai

### help-dump is an in-binary subcommand, not a repo script
**Decision**: the frozen-contract assembly lives in the shipped binary (`BuildHelpDoc`, wired into `runCommand` in `cmd/tu/main.go`); `scripts/help-dump.mjs` is a thin local wrapper that shells out to the binary and holds no contract logic.
**Why**: shll.ai's pull cron runs `<binary> help-dump` against the brew-installed binary, uniform across all seven tools — a same-named repo script is not the same surface; while tu shipped the command only as a repo script, the puller fell back to last-good every run (260604).
**Rejected**: Repo-script-only production (the puller fell back to last-good on every run).
*Introduced by*: 260602-v76l-help-dump-shll-ai (in-binary fix: 260604)

### shll.ai pulls; tu pushes nothing
**Decision**: shll.ai's own scheduled cron installs tu via Homebrew, runs `tu help-dump`, validates, and direct-commits `help/tu.json` to its own `main` with the default `GITHUB_TOKEN`; tu writes nothing to the shll.ai repo and no workflow carries a shll.ai token.
**Why**: a single trusted cron is the only writer of `help/tu.json`, which removes the multi-repo push race entirely.
**Rejected**: tu-side push via PR + auto-merge (serializes the seven tools' concurrent writes through GitHub; moot with a single cron writer).
*Introduced by*: 260603-dmhw-remove-shll-ai-push-wiring
