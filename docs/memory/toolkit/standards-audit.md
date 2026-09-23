---
type: memory
description: The toolkit-standards conformance posture — the constitution's Toolkit Standards article, the version-pinned audit rule, the audited-standards ledger, and the shll v0.1.32 clause-by-clause audit of the five toolkit surfaces (help-dump, update, shell-init, skill, version) with frozen-surface findings S1 and V1.
---

# Standards Audit

**Domain**: toolkit

## Overview

tu is a member of the shll toolkit, and its constitution binds every change to the CLI surface, help output, `README.md`, or `docs/site/` to a check against the toolkit's published standards; conformance audits pin the shll version they ran against. The per-surface behavior lives in [version-and-help-dump](/toolkit/version-and-help-dump.md), [update](/toolkit/update.md), [shell-init-and-completions](/toolkit/shell-init-and-completions.md), and [skill-bundle](/toolkit/skill-bundle.md); the audited-standards ledger is mirrored in [toolchain](/build/toolchain.md).

## Requirements

### Requirement: Standards conformance is a precondition for surface changes
- Any change to the CLI surface, help output, `README.md`, or `docs/site/` MUST first be checked against the toolkit's published standards — enumerated at runtime via `shll standards`, read via `shll standards <name>` (constitution v2.0.0, `### Toolkit Standards` article) (rdo3).
- The standards are versioned with the shll release, so a conformance audit MUST pin the shll version it ran against (rdo3).

### Requirement: Audited-standards ledger
- Audited standards to date: `principles`, `help-dump`, `readme-extraction`, `skill` against shll v0.0.23, 2026-07-18 (rdo3); the `skill` standard adopted per 260717-uch0 — see [skill-bundle](/toolkit/skill-bundle.md); `update`, `version`, `shell-init` against shll v0.0.23+, 2026-07-20 (ba5w); `readme-extraction` rule 1 re-audited against shll v0.1.31, sahil87/shll#98 (v2cu); the five toolkit surfaces — `help-dump`, `update`, `shell-init`, `skill`, `version` — audited against **shll v0.1.32**, 2026-09-17 (vcur).
- The shll `help-dump` standard's stale "tu exception" prose (describing tu as flag-based with `root.commands: []`) is an upstream shll-repo doc issue, not a tu violation — tu's flat document passes the shape-agnostic invocation contract and checklist and is NOT flattened to match the prose (vcur).

### Requirement: shll v0.1.32 audit of the five toolkit surfaces
Each surface audits clause by clause against `shll standards <name>` as installed at shll v0.1.32 (2026-09-17):

| Standard | Result | Notes |
|---|---|---|
| `help-dump` | conformant | the flat document (`commands: []`, full `--help` in `root.text`) is the standard's "tu exception"; the version comes from `-ldflags`, bare — see [version-and-help-dump](/toolkit/version-and-help-dump.md) |
| `update` | conformant | `HOMEBREW_NO_ASK=1` satisfies the wrapped-subprocess prompt-free MUST; bounds only on the metadata calls (SIGTERM + 10 s grace); `brew upgrade` unbounded — see [update](/toolkit/update.md) |
| `shell-init` | conformant | three shells; eval-safe stdout, diagnostics stderr-only, exit 2 on missing/unsupported shell with stdout empty — see [shell-init-and-completions](/toolkit/shell-init-and-completions.md) |
| `skill` | conformant except one | **Finding S1** below |
| `version` | conformant | `toolkit.VersionLine` keeps the canonical shape; **Finding V1** below |

#### Scenario: finding S1 — `tu skill topics`
- **GIVEN** the `skill` standard's reserved `skill topics` clause, which requires `tu skill topics` to print empty stdout and exit 0
- **WHEN** `tu skill topics` runs
- **THEN** the binary prints the whole bundle (arguments are ignored — `runCommand` in `cmd/tu/main.go` dispatches `skill` to `toolkit.WriteSkill(stdout)` regardless of args) — a frozen-surface matter the audit records as finding S1

#### Scenario: finding V1 — the `-v` alias
- **GIVEN** the `version` standard, which does not require a `-v` alias and whose `--help` does not list one
- **WHEN** `tu -v` runs
- **THEN** the binary prints the version line and exits 0 — a pre-existing extra alias the audit records as finding V1 (DC-09)

## Design Decisions

### Frozen surfaces outrank a conforming fix
**Decision**: findings S1 (`tu skill topics` prints the bundle) and V1 (the `-v` alias) stand as recorded findings rather than fixes.
**Why**: both surfaces are frozen bytes the differential harness pins; changing them for standard-conformance would break harness parity, so the audit records the deviation instead of fixing it (vcur).
**Rejected**: fixing S1/V1 to satisfy the standard (breaks the frozen surface the harness pins; S1's `topics` clause is a reserved clause, not a shipped behavior).
*Introduced by*: 260916-vcur-toolkit-layer

### Standards checks gate surface changes
**Decision**: the constitution's `### Toolkit Standards` article binds any change to the CLI surface, help output, `README.md`, or `docs/site/` to a prior check against the toolkit's published standards, and each audit pins the shll version it ran against.
**Why**: tu is a member of the shll toolkit, so its user-facing surface is governed cross-repo; a versioned standard means an audit is only meaningful against the version it ran against (rdo3).
**Rejected**: auditing once and treating the result as permanent (the standards evolve with shll releases, so an unpinned audit silently rots).
*Introduced by*: 260717-rdo3-toolkit-standards-conformance
