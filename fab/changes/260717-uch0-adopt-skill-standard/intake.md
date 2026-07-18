# Intake: Adopt Toolkit `skill` Standard

**Change**: 260717-uch0-adopt-skill-standard
**Created**: 2026-07-18

## Origin

One-shot invocation: `/fab-new uch0` (backlog ID). No prior conversation context. Backlog entry (`fab/backlog.md`, verbatim):

> Adopt the toolkit `skill` standard for tu — add a `tu skill` subcommand that prints a stable one-page agent usage bundle to stdout (raw markdown, exit 0, empty stderr), byte-identical to a new canonical `docs/site/skill.md`. Per `shll standards skill` (audited at shll v0.0.23): the bundle is a USAGE briefing (not a README clone, not a flag table) — when-to-use, capabilities map (one line per subcommand/grammar), composition patterns (shells out to ccusage/git/brew; renders on shll.ai; shll agent-setup will aggregate it), output/exit-code contracts (stdout=data/stderr=diagnostics, `--json`/`--csv`/`--md`, exit convention), and gotchas. HARD RULES: static-only (no timestamps/env lookups — contrast run-kit context), ≤150 lines, byte-identical to `docs/site/skill.md` (embed at build time via a sync + drift-guard test, reusing the exact mechanism `shll standards` uses for its standards docs). tu is a bundled Node/ESM binary (esbuild → dist/tu.mjs, Constitution III no runtime node_modules), so embed the markdown as a build-time `--define` string constant (like `__PKG_*__` in scripts/build.sh) or an esbuild text loader, NOT a runtime file read. Wire `skill` into the non-data command dispatch in src/node/core/cli.ts (alongside shell-init/help-dump). `docs/site/skill.md` renders at /tu/skill on shll.ai automatically (part of the pulled docs/site/** tree — readme-extraction). Deferred (not a violation): the standard is phased per-repo (no seven-repo flag-day), P10 is a SHOULD, and "No tool ships `skill` today." Deferred from change 260717-rdo3 (toolkit standards conformance audit).

The governing standard was re-read at intake time via `shll standards skill` — its rules are reproduced in What Changes below so downstream stages need no live `shll` access. Deferral provenance: change `260717-rdo3-toolkit-standards-conformance` dispositioned the skill standard as "deferred, not yet adopted" and created this backlog entry as the deferral target.

## Why

1. **The pain point**: an agent operating an installed `tu` binary has no offline usage briefing. The three existing surfaces each fall short (per the standard): `-h`/`help-dump` is flag/structure reference, not when-to-reach-for-what; README/`docs/site` needs the repo checked out or a network round-trip; `fab/project` context is repo-*development*-scoped (orients a contributor, not a caller). A `<tool> skill` bundle is embedded, offline, present wherever the tool is, and version-locked by construction — the prose ships in the same binary as the flags it describes.
2. **If we don't do it**: tu stays out of the toolkit's `skill` contract, and the planned `shll agent-setup` aggregator (which concatenates every installed tool's `<tool> skill` output into agent context) will have nothing to aggregate for tu. The constitution's Toolkit Standards section binds tu to published standards; this one is currently a known deferred gap (recorded by rdo3).
3. **Why this approach**: the standard prescribes the shape (`skill` subcommand, byte-identical to `docs/site/skill.md`, embedded). The only tu-specific latitude is the embed mechanism — the backlog entry already resolves it: build-time `--define` string constant following the existing `__PKG_*__` pattern in `scripts/build.sh`, because Constitution III (single-bundle distribution, no runtime `node_modules`/files) forbids a runtime file read in the shipped binary.

## What Changes

### 1. New canonical `docs/site/skill.md` (the bundle content)

A new ≤150-line, static-only markdown usage briefing. It renders at `/tu/skill` on shll.ai automatically (the `docs/site/**` tree is already pulled via readme-extraction — no site wiring needed). Genre per the standard — usage briefing, NOT a README clone or flag table. Required sections and tu-specific content:

- **When to use**: cost/usage questions about AI coding assistants on this machine (Claude Code, Codex, OpenCode); when NOT to reach for it (billing management, non-supported tools, anything needing per-request granularity beyond what ccusage emits).
- **Capabilities map** — one line per subcommand/grammar, keyed to the command:
  - Data grammar: `tu [source] [period] [display]` — sources `cc`/`codex`(`co`)/`oc`/`gemini`(`gem`)/`copilot`(`cop`) (omit = all), periods `d`/`w`/`m` plus `dh`/`wh`/`mh` (today/week/month, `*h` = history).
  - Setup/non-data: `init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init <sh>`, `help-dump`, `skill` (self-reference).
- **Composition patterns**: shells out to a single vendored `ccusage` binary (v20 all-agent CLI — per-source subcommands via prefixArgs, no separate `ccusage-*` binaries), `git` (multi-machine metrics sync), `brew` (`tu update`); is shelled-out-to by shll.ai's pull cron (`tu help-dump`) and, forward, `shll agent-setup` (aggregates `tu skill`).
- **Output & exit-code contracts**: stdout = data, stderr = diagnostics/warnings (graceful degradation per Constitution II — a missing data source warns and falls back, never crashes); `--json`/`-j`, `--csv`, `--md` on data commands only; document tu's *actual* exit-code behavior (verify at apply — do not fabricate a `0/1/2` convention the binary doesn't implement).
- **Gotchas**: cached data with TTL (`--fresh`/`-f` to bypass); single vs multi mode depends on `~/.tu.conf` (`tu status` shows which); `--watch`/`-w` is an interactive TUI — agents should not invoke it; `--no-color` for clean parseable output.

Exact prose is authored at apply against the real CLI surface (`src/node/core/cli.ts` FULL_HELP and actual flag behavior), then frozen as the canonical file.

### 2. New `tu skill` subcommand (dispatch wiring)

Wire into the non-data command dispatch in `src/node/core/cli.ts` (currently `cli.ts:1323-1324`), alongside the existing rows:

```ts
if (cmd === "shell-init") { runShellInit(filteredArgs[1]); return; }
if (cmd === "help-dump") { runHelpDump(); return; }
if (cmd === "skill") { runSkill(); return; }   // new
```

Invocation contract (uniform across the toolkit, per the standard):
- Command name exactly `skill` (not `agent`, not `context` — name rationale is settled by the standard).
- Prints the bundle as **raw markdown to stdout**, byte-identical to `docs/site/skill.md`.
- **stderr empty on success, exit code 0.** No rendering, no pager, no added framing.

### 3. Build-time embed in `scripts/build.sh`

Follow the existing `__PKG_*__` define pattern exactly:

```bash
SKILL_DEF=$(node -p 'JSON.stringify(require("fs").readFileSync("docs/site/skill.md", "utf8"))')
# ... add to the esbuild invocation:
  --define:__SKILL_MD__="$SKILL_DEF"
```

Because the define reads the canonical `docs/site/skill.md` directly at bundle time, byte-identity of the shipped binary is **by construction** — there is no committed embedded copy and therefore no sync script to run or drift between copies to guard (the shll mechanism's sync step exists for Go's `go:embed`, which needs a file inside the package; esbuild `--define` does not). The drift-guard obligation is met by tests + a post-build check (§5).

### 4. New `src/node/core/skill.ts` module (resolution + dev fallback)

A small module exporting the resolved bundle string, mirroring the `__PKG_*__` typeof-guard pattern in `cli.ts:31-58`:

```ts
declare const __SKILL_MD__: string | undefined;
// Built bundle: esbuild --define supplies the string (static, no I/O).
// Dev/tsx (tests, npx tsx): define is absent — read the canonical file from
// the repo. Dev-only path; the shipped bundle never touches the filesystem.
export const SKILL_MD: string =
  typeof __SKILL_MD__ !== "undefined"
    ? __SKILL_MD__
    : readFileSync(new URL("../../../docs/site/skill.md", import.meta.url), "utf8");
```

(Exact path resolution to repo root decided at apply; the constraint is: shipped bundle = define only, dev = canonical file, no third copy of the content anywhere.) `runSkill()` in `cli.ts` is then `process.stdout.write(SKILL_MD)`.

### 5. Drift guard + tests

No existing test exercises `dist/tu.mjs` (verified at intake — the suite is all tsx unit tests), so the guard has two layers:

- **`src/node/core/__tests__/skill.test.ts`** (co-located per constitution, tsx runner):
  - `SKILL_MD` is byte-identical to `docs/site/skill.md` (pins the dev-fallback path and any future refactor that introduces a second copy).
  - Line budget: ≤150 lines (hard rule from the standard).
  - Static-only sanity: content contains no obvious dynamic markers (this is a genre check, best-effort).
  - CLI contract: `skill` dispatch writes the bundle to stdout, nothing to stderr, exits 0 (follow existing `cli-*.test.ts` patterns).
- **Post-build check in `scripts/build.sh`** (the layer that actually guards the shipped artifact, since tsx tests never see the bundle):

  ```bash
  node dist/tu.mjs skill | cmp -s - docs/site/skill.md || { echo "error: tu skill output drifted from docs/site/skill.md" >&2; exit 1; }
  ```

### 6. Help text + shell completions

- Add a `tu skill` line to `FULL_HELP` in `src/node/core/cli.ts` (the Setup/non-data block around line 95, e.g. `tu skill             Print agent usage bundle (markdown)`). This is additive help-output change; `help-dump` captures help text dynamically, so no help-dump changes needed.
- Add `skill` to the non-data subcommand lists in `src/node/core/completions.ts` for all shells that enumerate subcommands (bash list at `completions.ts:19`, plus the zsh/fish equivalents in the same file).

### Out of scope

- No `shll`/shll.ai side changes — the site renders `docs/site/skill.md` automatically from the already-pulled tree.
- No changes to `run-kit context` or any other tool.
- No `shll agent-setup` work (forward design, explicitly "planned, not yet built" in the standard).
- Backlog entry `[uch0]` is ticked at archive time per the normal lifecycle, not in this change's diff... (standard fab flow handles it).

## Affected Memory

- `cli/data-pipeline`: (modify) new non-data subcommand `skill` in the CLI dispatch; agent-facing usage-bundle contract (stdout/exit semantics)
- `build/toolchain`: (modify) new build-time `__SKILL_MD__` define + post-build byte-identity check in `scripts/build.sh`; drift-guard test; toolkit-standards conformance posture updates from "skill: deferred, not yet adopted" (recorded by rdo3) to "adopted"

## Impact

- **New files**: `docs/site/skill.md` (canonical bundle), `src/node/core/skill.ts`, `src/node/core/__tests__/skill.test.ts`
- **Modified files**: `src/node/core/cli.ts` (dispatch row + FULL_HELP line), `src/node/core/completions.ts` (subcommand lists, all shells), `scripts/build.sh` (define + post-build check)
- **Dependencies**: none added; no runtime behavior change for existing commands
- **CLI surface**: additive (new subcommand + help line) → **minor version bump** at release per the constitution's Output Stability rule
- **Standards conformance**: checked against `shll standards skill` (this change's whole purpose); `help-dump` unaffected (dynamic capture); `readme-extraction` unaffected (new file rides the existing pulled tree)

## Open Questions

None — the backlog entry is fully specified and every file/pattern it references was verified at intake.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Embed via build-time `--define:__SKILL_MD__` (JSON.stringify of file contents), not an esbuild text loader | Backlog offers both; `--define` is the established `__PKG_*__` pattern in `scripts/build.sh` — zero new build machinery | S:75 R:85 A:90 D:80 |
| 2 | Confident | New `src/node/core/skill.ts` module with typeof-guard + dev/tsx fallback reading `docs/site/skill.md` from disk | Mirrors the existing `__PKG_*__`/`_devPkg` fallback in `cli.ts:31-58`; "NOT a runtime file read" binds the shipped bundle, not dev mode | S:60 R:90 A:85 D:75 |
| 3 | Confident | No sync script / committed embedded copy — byte-identity by construction (build reads the canonical file); drift guard = tsx test + post-build `cmp` in `build.sh` | The shll sync mechanism exists for `go:embed`'s in-package-copy requirement; esbuild `--define` reads the canonical file directly, satisfying the standard's intent (byte-identical, guarded) without the copy | S:70 R:80 A:75 D:65 |
| 4 | Certain | Help text lists `tu skill`; completions add `skill` to non-data subcommand lists in all shells | Every other non-data subcommand is listed in both; toolkit consistency; purely additive | S:65 R:95 A:95 D:85 |
| 5 | Confident | Bundle content follows the standard's five-section genre with the tu-specific outline in §1; exact prose authored at apply against the real CLI surface (incl. verifying actual exit-code behavior rather than asserting `0/1/2`) | Standard prescribes sections; tu's surface is known; prose is cheap to revise; fabricating an unverified exit convention would violate the genre's accuracy point | S:75 R:85 A:80 D:70 |
| 6 | Certain | Release is a minor version bump (additive CLI surface = feature) | Constitution Output Stability rule + semver; handled at ship, noted for the release step | S:70 R:95 A:100 D:95 |

6 assumptions (3 certain, 3 confident, 0 tentative, 0 unresolved).
