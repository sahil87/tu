---
type: memory
description: internal/fact's tool registry — fact.Tool{Key, Name}, the ordered fact.Tools slice (cc, codex, oc, gemini, copilot, kimi in column order), fact.Lookup, aliases living in command grammar, and the per-tool ccusage subcommand table held by the adapter
---

# Tool Registry

**Domain**: fact

## Overview

`package fact` (`src/go/internal/fact/tool.go`) owns the tool registry: `fact.Tool{Key, Name}`, the ordered `fact.Tools` slice, and `fact.Lookup`. Key, display name, and column order are properties of the fact model; adapter-specific data (how ccusage is driven per tool) lives in the adapter, keyed by `Tool.Key` — see [ccusage-adapter](/source/ccusage-adapter.md).

## Requirements

### Requirement: fact.Tool shape
`package fact` SHALL define `Tool{Key, Name string}` (`src/go/internal/fact/tool.go`): `Key` is the registry key carried by `fact.Record.Tool`; `Name` is the display name. Adapter-specific data (how a source is driven for the tool) MUST live in the adapter, keyed by `Key`, not on `fact.Tool`.

### Requirement: fact.Tools is the ordered registry
`fact.Tools` MUST be a slice — never a map, Go maps are unordered — holding exactly these six entries in this order (`src/go/internal/fact/tool.go`, asserted by `tool_test.go` `TestToolsOrderAndShape`):

| # | Key | Name | ccusage subcommand |
|---|--------|-------------|-------------------|
| 1 | `cc` | Claude Code | `claude` |
| 2 | `codex` | Codex | `codex` |
| 3 | `oc` | OpenCode | `opencode` |
| 4 | `gemini` | Gemini | `gemini` |
| 5 | `copilot` | Copilot | `copilot` |
| 6 | `kimi` | Kimi | `kimi` |

The slice order IS the all-tools column order (see the column-order requirement below). The per-tool ccusage subcommand lives in the adapter's `invocations` map keyed by `fact.Tool.Key` — see [ccusage-adapter](/source/ccusage-adapter.md).

#### Scenario: Registry order and content
- **GIVEN** `fact.Tools` iterated
- **WHEN** keys are read in slice order
- **THEN** they are exactly `cc, codex, oc, gemini, copilot, kimi` with names `Claude Code, Codex, OpenCode, Gemini, Copilot, Kimi` (`tool_test.go` `TestToolsOrderAndShape`)

### Requirement: fact.Lookup scans the registry
`func Lookup(key string) (Tool, bool)` MUST scan `fact.Tools` and return the tool whose `Key` matches, or `(Tool{}, false)`. It matches registry keys only.

#### Scenario: Lookup hits keys, misses aliases
- **GIVEN** the registry above
- **WHEN** `fact.Lookup("gemini")` runs
- **THEN** it returns `Tool{Key: "gemini", Name: "Gemini"}, true`; `fact.Lookup("gem")`, `fact.Lookup("co")`, `fact.Lookup("cop")`, `fact.Lookup("ki")`, and `fact.Lookup("")` all miss (`tool_test.go` `TestLookup`)

### Requirement: Aliases are command grammar, not registry entries
Source aliases MUST NOT appear in the registry: `co` → `codex`, `gem` → `gemini`, `cop` → `copilot`, `ki` → `kimi` (plus the `all` source) live in command grammar as `sourceAliases` in `internal/command/parse.go` (see [request-and-parse](/command/request-and-parse.md)); the scenario above is the registry-side assertion (`tool_test.go` `TestLookup`).

### Requirement: Registry order is the column order
`fact.Tools` insertion order IS the all-tools column order (Output Stability): new tools MUST append so existing columns keep their positions. The per-tool ccusage invocation (the subcommand column above, plus argv composition `claude daily --json`) lives in the adapter's `invocations` map keyed by `fact.Tool.Key` (`internal/source/ccusage/registry.go`), whose key set is asserted equal to the keys of `fact.Tools` (`registry_test.go`); every entry's JSON label key is `date`. Consumers: [ccusage-adapter](/source/ccusage-adapter.md) drives the binary per tool; records flow on to [aggregation](/query/aggregation.md).

## Design Decisions

### Registry is an ordered slice
**Decision**: `fact.Tools` is `[]Tool`; `fact.Lookup` scans it.
**Why**: Go maps are unordered and insertion order is the all-tools column order (Output Stability).
**Rejected**: `map[string]Tool` plus a separate order slice (two sources of truth).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Registry lives in `fact`, not a new package
**Decision**: `fact.Tool{Key, Name}`, `fact.Tools`, `fact.Lookup` own the registry; adapters hold their own per-key data.
**Why**: the registry (key, display name, column order) is a property of the fact model — `fact.Record.Tool` already carries the key — and `fact` is the package every stage imports; `source/metrics` and `command` need it without taking an adapter dependency.
**Rejected**: a tiny `internal/tools` package (a fourth import for ~30 lines, no isolation gain); the registry keyed inside the ccusage adapter (keeps a weld between the fact model and one adapter).
*Introduced by*: 260916-m9of-g1-rework-1

### Per-tool ccusage subcommands on a single binary
**Decision**: each registry tool drives the single ccusage binary through a per-agent subcommand — `cc` → `claude`, `codex` → `codex`, `oc` → `opencode`, `gemini` → `gemini`, `copilot` → `copilot`, `kimi` → `kimi` — held in the adapter's invocation map keyed by `fact.Tool.Key`, so argv is e.g. `claude daily --json`.
**Why**: bare `ccusage daily` is an all-agents aggregate, so mapping `cc` to it over-counts other agents' usage and double-counts the all-tools total; one binary serves every tool, so per-tool packages add nothing (260703-bxuh). The latent risk was real: on a claude-only machine the aggregate equals claude, so the bug ships silently until a second agent's transcripts appear — and cost accuracy is the tool's purpose (260703-ccfx). Gemini and Copilot joined the registry in 260703-gmcp, Kimi in 260810.
**Rejected**: client-side filtering of the bare all-agents aggregate via its metadata (adds a parsing path for data ccusage already serves cleanly per-agent); per-tool packages (deprecated upstream, final release 19.0.0).
*Introduced by*: 260703-ccfx

### Column order is registry insertion order
**Decision**: registry insertion order is the all-tools column order; new tools always append.
**Why**: Output Stability — existing columns keep their positions across releases, so downstream consumers (scripts parsing columns) never shift.
**Rejected**: alphabetical or curated ordering (reshuffles columns on every addition, breaking Output Stability).
*Introduced by*: 260703-gmcp
