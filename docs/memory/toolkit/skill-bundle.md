---
type: memory
description: The `tu skill` agent usage bundle — a committed byte-identical copy of docs/site/skill.md go:embedded as toolkit.Skill, refreshed by scripts/sync-skill.sh, pinned by a go test drift guard and the cmp guard in `just go-build`, printed verbatim with a 150-line budget.
---

# Skill Bundle

**Domain**: toolkit

## Overview

`tu skill` prints the agent usage bundle — a committed, byte-identical copy of the canonical `docs/site/skill.md` `go:embed`ded as `toolkit.Skill` in `internal/toolkit/skill.go` — verbatim, with no rendering, no pager, and no framing. The same embedded-asset pattern backs `tu.default.conf` in the config domain — see [cascade](/config/cascade.md); the build-side guard lives in [toolchain](/build/toolchain.md).

## Requirements

### Requirement: Embedded committed copy
- `Skill` in `internal/toolkit/skill.go` MUST be a `//go:embed skill.md` byte string — a committed, byte-identical copy of the canonical `docs/site/skill.md`. The Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/` directly.
- `skill.go` MUST carry the `//go:generate ../../../../scripts/sync-skill.sh` directive; `scripts/sync-skill.sh` MUST copy `docs/site/skill.md` over `src/go/internal/toolkit/skill.md` from the repo root regardless of caller CWD (`cd "$(dirname "$0")/.."`) and print `synced skill bundle: src/go/internal/toolkit/skill.md`.
- `WriteSkill(w io.Writer) error` MUST write the bundle verbatim to `w` — raw markdown, no rendering, no pager, no framing (the toolkit skill standard).

### Requirement: Drift guards
- The test drift guard (`TestSkillDriftGuard` in `internal/toolkit/skill_test.go`) MUST walk up from the package directory to the directory containing `justfile` and assert byte-equality between `Skill` and `docs/site/skill.md` — editing one byte of either copy fails `go test`.
- The build drift guard (the private `_go-skill-guard` recipe in the justfile, run by `go-build` and `go-build-target` before `go build`) MUST run `cmp -s docs/site/skill.md src/go/internal/toolkit/skill.md` and fail the build with `error: src/go/internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh` on stderr, exit 1 (0118).
- The bundle MUST stay within the 150-line hard budget and end with exactly a trailing newline (`TestSkillShape` in `skill_test.go` pins both); `docs/site/skill.md` is 117 lines.

#### Scenario: embedded copy drifts from the canonical file
- **GIVEN** `src/go/internal/toolkit/skill.md` edited without re-running `scripts/sync-skill.sh`
- **WHEN** `go test` or `just go-build` runs
- **THEN** `go test` fails `TestSkillDriftGuard` (`…drifted from docs/site/skill.md — run scripts/sync-skill.sh`), and the build fails before `go build` with the `cmp` error on stderr, exit 1 — the same failure CI's `go-build-and-test` lane surfaces at its Build step

### Requirement: tu skill dispatch
- `cmd/tu` MUST answer `skill` by writing the bundle to stdout via `toolkit.WriteSkill(stdout)` (`runCommand` in `cmd/tu/main.go`), stderr empty, exit 0.
- Any arguments — including `topics` — MUST be ignored: the bundle prints whole.

#### Scenario: tu skill with arguments
- **GIVEN** `tu skill topics` (or any other arguments)
- **WHEN** `runCommand` dispatches `skill`
- **THEN** stdout is the whole bundle verbatim, stderr is empty, exit 0 — arguments are ignored

## Design Decisions

### Skill bundle: committed copy, sync script, two guards
**Decision**: a committed byte-identical copy of `docs/site/skill.md` under the package, refreshed by `scripts/sync-skill.sh`, pinned by a `go test` drift guard AND a `cmp` guard in `just go-build`.
**Why**: the Go module root is `src/go/`, so `//go:embed` cannot reach `docs/site/`; the sibling tools (`idea`, `hop`) and this repo's own `tu.default.conf` embed use the same committed-copy pattern; the build-time `cmp` gives the earliest, clearest failure in CI's Build step (0118).
**Rejected**: a symlink (fragile across platforms and CI checkouts, and `go:embed` refuses symlinks outside the module); reading the canonical file at runtime (a runtime file read for a static bundle, and the file does not exist on an installed machine).
*Introduced by*: 260916-vcur-toolkit-layer

### Verbatim bundle, arguments ignored
**Decision**: `tu skill` prints the raw markdown bundle verbatim — no rendering, no pager, no framing — and ignores every argument, including the `skill` standard's reserved `topics`.
**Why**: the skill standard's byte-identical-and-guarded intent is satisfied by the embed plus the two drift guards; printing the whole bundle for any argument keeps the surface frozen and matches how the binary's other non-data commands behave (uch0, vcur).
**Rejected**: rendering or framing the markdown (breaks byte-identity against the canonical file); an argument-sensitive dispatch (an unfrozen surface the standards audit records as finding S1 — see [standards-audit](/toolkit/standards-audit.md)).
*Introduced by*: 260717-uch0-adopt-skill-standard (verbatim embed: 260916-vcur-toolkit-layer)
