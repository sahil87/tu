# Intake: Build-time help-dump → shll.ai Command reference

**Change**: 260602-v76l-help-dump-shll-ai
**Created**: 2026-06-02
**Status**: Draft

## Origin

> Add a build-time 'help-dump' step that emits tu's CLI help as `help/tu.json` and PRs it into `sahil87/shll.ai` (the shll.ai landing site renders it as an expandable 'Command reference' on the tu tool page). CONTRACT (frozen — copy the reference sample at `sahil87/shll.ai` path `help/wt.json`): JSON shape is `{tool, version, captured_at (ISO-8601 UTC), schema_version: 1, root: Node}` where `Node = {name, path, short (one-line desc), usage, text (the RAW --help output byte-for-byte, newlines preserved), commands: Node[] (recursive; empty array = leaf)}`. IMPORTANT — tu is the ODD ONE OUT: it is a Node/TypeScript CLI (bin 'tu' -> dist/tu.mjs, entry src/node/core/cli.ts), NOT Cobra, and it is FLAG-based with NO subcommands (flags like --json --csv --md --sync --watch --user; see src/node/core/completions.ts). So there is no cobra tree to walk and no shared Go producer applies — write a small bespoke Node producer. The result will be effectively flat: root Node carries the full 'tu --help' text and commands is an empty array []. Capture the literal output of running the built CLI with --help (and read version from package.json: currently 0.4.13). If tu ever grows real subcommands, recurse the same way. PUSH: in CI (build via scripts/build.sh / esbuild) after build, run the help capture, write help/tu.json, validate it parses, then open a PR into sahil87/shll.ai using the existing repo secret SHLLAI_TOKEN (contents + pull-request write) with auto-merge enabled (PR, not direct push to main, to avoid the multi-repo push race). This is tu's slice of a 7-tool rollout; the shll.ai site-side consumer (Astro loader + reference UI) is tracked separately in the shll.ai repo.

**Mode**: One-shot, backlog-driven (`[v76l]` in `fab/backlog.md`, dated 2026-06-02). No Linear ticket. No prior `/fab-discuss` — the backlog entry is the frozen spec.

**Codebase reconnaissance performed during intake** (decisions below are grounded in these findings, not assumptions):

- `package.json`: `bin: { tu: "dist/tu.mjs" }`, `version: 0.4.14` (the backlog says 0.4.13 — stale; the producer reads version at runtime so this self-corrects), `type: module`.
- `tu --help`, `tu -h`, and `tu help` **all print the identical `FULL_HELP` constant** — `src/node/core/cli.ts:1129-1131`. `--help`/`-h` survive `parseGlobalFlags` (not in the strip set), so `tu --help` resolves to the help branch as required. `--version`/`-v`/`-V` print `tu version v{X}` on a separate branch (cli.ts:1122-1126) — **not** part of the help text.
- Build is `scripts/build.sh` → `npx esbuild src/node/core/cli.ts ... --outfile=dist/tu.mjs --define:__PKG_VERSION__="\"$VERSION\""`. esbuild injects the version at build time; the built binary's help is fully self-contained (no `node_modules` needed at runtime).
- CI: **only `.github/workflows/release.yml` exists** — triggered on `push: tags: ['v*']` and manual `workflow_dispatch`. It already builds release notes, creates a GitHub Release, and pushes to the Homebrew tap using `secrets.HOMEBREW_TAP_TOKEN`. There is **no** standalone `build.yml` / `ci.yml`. This is the central design decision (see Open Questions Q1).
- The reference sample `sahil87/shll.ai:help/wt.json` is **not present** in the local checkout (`/home/sahil/code/sahil87/shll.ai` has `docs/ fab/ sites/`, no `help/`). The shll.ai consumer (Astro loader + reference UI) is genuinely tracked separately, as the backlog states. The contract is therefore taken **verbatim from the backlog text** (it is fully specified inline); `wt.json` is the cross-repo cross-check when it lands.

## Why

1. **Problem**: shll.ai (the public toolkit landing site) wants to render a live "Command reference" for each of 7 tools. The reference must be machine-generated from each tool's real `--help` so it never drifts from the shipped CLI. tu currently exposes its help only at runtime; the site has no structured artifact to consume.
2. **Consequence if not done**: tu's tool page on shll.ai either shows no command reference or a hand-maintained copy that silently rots as flags change (and tu's flag set *does* change — recent commits added `--skip-brew-update`, `-v` version shorthand, CSV completions). Hand-maintenance across 7 tools is exactly the drift the rollout exists to eliminate.
3. **Why this approach**: A **single frozen JSON contract** shared by all 7 tools lets one site-side consumer render every tool uniformly. Each tool owns producing its own slice and **pushing it via PR** into shll.ai (rather than the site pulling), so the artifact is regenerated exactly when the tool is built/released and is always version-stamped. PR-with-auto-merge (not direct push) is mandated to avoid a **multi-repo push race**: 7 tools pushing to `main` of one repo concurrently would clobber each other; PRs serialize through GitHub's merge queue.
4. **Why bespoke (not the shared Go producer)**: 6 of the 7 tools are Cobra-based Go CLIs with a walkable command tree; a shared Go producer recurses that tree. **tu is the odd one out** — a Node/TypeScript CLI whose help is a single static string with no subcommand tree to walk. Reusing the Go producer is impossible; a ~50-line bespoke Node producer is the right tool. It emits a structurally-valid but **flat** document (one root Node, `commands: []`).

## What Changes

Three additive pieces. **No existing tu behavior changes** — this is purely a new build-time artifact + a new CI step. The runtime CLI, its output, and its grammar are untouched.

### 1. Bespoke Node help-dump producer

A new standalone Node script (no new runtime dependency — uses only `node:` built-ins, consistent with the constitution) that:

1. Reads `version` from `package.json` (runtime read — self-corrects the stale 0.4.13 → current 0.4.14 and every future bump).
2. Executes the **built** CLI to capture help **byte-for-byte**: `node dist/tu.mjs --help` (capture stdout exactly — newlines preserved, no trimming, no re-wrapping, no ANSI injection). Capturing the *built* binary (not the `FULL_HELP` source constant) is what the contract means by "the literal output of running the built CLI with --help" and guarantees the artifact matches what users actually see. Exit non-zero (fail the build) if the CLI errors or emits empty output.
3. Builds the contract object and writes it to `help/tu.json` (repo-relative, pretty-printed).
4. **Self-validates**: re-reads the written file and `JSON.parse`s it; asserts the required keys (`tool`, `version`, `captured_at`, `schema_version`, `root` and the `root` Node's keys) are present and well-typed. Non-zero exit on any failure.

**Contract object emitted** (frozen — matches the backlog spec exactly):

```json
{
  "tool": "tu",
  "version": "0.4.14",
  "captured_at": "2026-06-02T12:34:56Z",
  "schema_version": 1,
  "root": {
    "name": "tu",
    "path": "tu",
    "short": "AI coding assistant cost tracking CLI",
    "usage": "Usage: tu [source] [period] [display]",
    "text": "<RAW byte-for-byte output of `node dist/tu.mjs --help`>",
    "commands": []
  }
}
```

Field derivations (each Node field gets a deterministic source so output is reproducible):
- `tool` → `package.json.name` (`"tu"`), or literal `"tu"`.
- `version` → `package.json.version`.
- `captured_at` → `new Date().toISOString()`, normalized to a `Z`-suffixed UTC ISO-8601 instant (e.g. `2026-06-02T12:34:56.000Z` or second-precision `...Z` — see Q3).
- `schema_version` → literal `1`.
- `root.name` → `"tu"`; `root.path` → `"tu"` (single-segment path for the root; subcommand paths would be space- or slash-joined if tu ever grows them — see Q2 default).
- `root.short` → tu's one-line description. Source: `package.json.description` if present, else `fab/project/config.yaml` `project.description` (`"AI coding assistant cost tracking CLI"`), else the first non-empty line of the help text.
- `root.usage` → the `Usage:` line extracted from the help text (`"Usage: tu [source] [period] [display]"`, the first line of `FULL_HELP`).
- `root.text` → the **raw** captured `--help` output, unmodified.
- `root.commands` → `[]` (empty — tu's help is a single flat string; no subcommand tree).

> **Note on tu's actual grammar**: The backlog says tu "has NO subcommands," but `src/node/core/completions.ts` and `cli.ts` show tu *does* dispatch subcommands (`help`, `init-conf`, `init-metrics`, `sync`, `status`, `update`, `shell-init`) and positional tokens (`cc/codex/co/oc/all`, periods, display). This does **not** change the design: the contract captures the **single `tu --help` text** as the root Node's `text`, and `commands` stays `[]`. tu does not print *per-subcommand* `--help` pages (e.g. `tu sync --help` is not a distinct help screen — it would fall through grammar parsing), so there is no recursion to do. The flat document is correct. The "recurse the same way if tu grows real subcommands" guidance (Q2) is forward-looking and applies only if tu later adds per-subcommand help screens.

**Producer location & invocation** (see Q4): a script under `scripts/` (e.g. `scripts/help-dump.mjs` or a `.ts` run via `tsx`), invokable as a package script (e.g. `npm run help-dump`) and from CI. It runs **after** `scripts/build.sh` (it depends on `dist/tu.mjs` existing). Whether it also runs automatically as the tail of `build.sh` vs. only in CI is Q4.

### 2. The `help/tu.json` artifact (transient — resolved Q5)

The producer writes `help/tu.json` into the **CI workspace** (a `help/` path), but the file is **not committed to the tu repo** — it is generated transiently in the release job and PR'd into `sahil87/shll.ai`, where it is auto-merged. **shll.ai is the single source of truth.** A `.gitignore` entry for `help/` (or simply never committing it) keeps the tu repo clean and eliminates regen-on-bump staleness. The destination path in shll.ai is `help/tu.json`, mirroring the reference `help/wt.json`.

### 3. CI step: build → capture → validate → PR into shll.ai

In CI, **after the build**, run the producer, then open a PR into `sahil87/shll.ai` placing the file at `help/tu.json` (mirroring the reference `help/wt.json`), using the existing repo secret `SHLLAI_TOKEN` (scopes: contents + pull-request write) with **auto-merge enabled**.

Concrete CI behavior (runs inside `release.yml`, on the tag-push path — see topology below):
1. Build (`scripts/build.sh`) so `dist/tu.mjs` exists.
2. Run the producer → `help/tu.json` (in the CI workspace, transient); **fail the job** if validation fails (never PR a malformed artifact).
3. Clone/checkout `sahil87/shll.ai` (token-authed via `SHLLAI_TOKEN`), write the file to `help/tu.json` on a fresh branch (e.g. `tu-help-dump-v{version}`), commit, push.
4. **Open a PR and auto-merge it** (`gh pr create` then `gh pr merge --auto --squash`). User direction (Q5): the job must *both generate the PR and merge it* — not leave a dangling PR. Default merge method `--squash` (#15, governed by shll.ai branch settings).
5. **No direct push to `main`** — PR + auto-merge only, to serialize the 7-tool concurrent writes through GitHub.

**Workflow placement & trigger — resolved (Q1)**: the help-dump→shll.ai step lives in **`release.yml`**, on the **existing `v*` tag-push path**. The release pipeline is now entered via a **release-PR merge to `main`** that triggers a tagging step (which pushes the `v*` tag and thus runs the pipeline). Separately, a **new `ci.yml`** (build verification + `npm test`, triggered on **push to `main`**) is introduced but does **not** carry the help-dump step. See Open Questions Q1 and Assumptions #16/#17.
<!-- clarified: CI structure — split a new ci.yml (build+test on main push) from release.yml; help-dump lives in release.yml on the tag-push path; release entered via release-PR merge → tag → existing pipeline -->

### CI topology after this change

```
.github/workflows/
  ci.yml       (NEW)  on: push branches[main]  → build verify + `npm test`        (NO help-dump)
  release.yml  (MOD)  on: push branches[main] + release condition → tag vX.Y.Z     (new entry point)
               (MOD)  on: push tags[v*]  (EXISTING)  → release notes, GH release,
                                                        Homebrew tap, HELP-DUMP→shll.ai PR+auto-merge
```

Flow: release PR (bumps version) merges to `main` → tagging step pushes `v*` → existing tag-push job runs the full release pipeline including the help-dump step. Releases stay tag-anchored (Homebrew tap + version stamping depend on tags); the PR merge is the new entry point.

## Affected Memory

- `build/toolchain`: (modify) Document the new help-dump build step, the producer script, the `help/tu.json` contract, and the CI PR-into-shll.ai flow. This is a real build-pipeline behavior change, so the `build` domain memory should record it.

No other memory domains are affected — the CLI runtime, data pipeline, display, watch, sync, and config domains are untouched (this change adds no flags, no output changes, no new runtime code paths).

## Impact

**New files:**
- `scripts/help-dump.mjs` (or `.ts`) — the bespoke Node producer (~50 lines, `node:` built-ins only).
- `.github/workflows/ci.yml` — **new** (Q1 resolved): build verification + `npm test` on push to `main`. Does **not** run the help-dump.

**Modified files:**
- `package.json` — add a `help-dump` script entry (Step 1/2 of producer invocation).
- `.github/workflows/release.yml` — **yes** (Q1/Q5 resolved): carries the help-dump→shll.ai PR-and-auto-merge step on the existing `v*` tag-push path; adds a release-PR-merge→tag entry point on `main`.
- `.gitignore` — add `help/` (Q5 resolved: `tu.json` is generated transiently in CI, never committed to the tu repo).
- `docs/memory/build/toolchain.md` — at hydrate time.

**Not committed (transient):**
- `help/tu.json` — generated in the CI workspace, PR'd into shll.ai, auto-merged. Never tracked in the tu repo (Q5 resolved).

**External / cross-repo:**
- `sahil87/shll.ai` receives PRs at `help/tu.json`. Requires the `SHLLAI_TOKEN` secret to already exist in the tu repo's Actions secrets with **contents: write + pull-requests: write** on shll.ai, and shll.ai must permit auto-merge (branch protection / merge-queue config). Both are **out of scope for this change** (assumed pre-provisioned per the backlog — see Assumptions) and live in the shll.ai repo / GitHub settings.

**Dependencies:** None added at runtime (constitution III: single bundle, no `node_modules` at install). CI uses `gh` (already used by `release.yml`) and standard `actions/setup-node`.

**Constitutional alignment:**
- I. Single-Purpose CLI — the *runtime* CLI is unchanged; this is build/release tooling, not a CLI feature. ✓
- II. Graceful Degradation — N/A to runtime; the producer instead **fails loudly** in CI (correct for a build artifact — a malformed reference must block, not degrade). ✓
- III. Single-Bundle Distribution — no runtime deps added; producer uses `node:` built-ins only. ✓
- TypeScript Conventions — if the producer is `.ts`, it follows strict mode, `node:` imports, `type` imports, functions-not-classes.

**Risks / edge cases:**
- **Multi-repo push race** — the explicit reason for PR-not-push; auto-merge serializes 7 concurrent tool writes.
- **Stale version in backlog (0.4.13 vs 0.4.14)** — neutralized by reading version at runtime.
- **ANSI / color in captured help** — must capture help with color **disabled** (tu honors `--no-color` and `NO_COLOR`); the `text` field must be plain (the FULL_HELP constant contains no ANSI, but capturing via the built binary should still force `NO_COLOR=1` to be safe). Flag for the plan.
- **Byte-for-byte fidelity** — no trailing-newline trimming, no CRLF conversion; the validator should not normalize `text`.
- **PR spam** — if the producer runs on every push (not just releases/tags), shll.ai gets a PR per commit. Q1 governs this directly.

## Open Questions

- **Q1 (CI trigger/workflow placement) — RESOLVED**: User directed a two-workflow split: (1) a **new `ci.yml`** for build verification + tests, triggered on **push to `main`** (does NOT run the help-dump); (2) **`release.yml`** runs the **release pipeline** and **carries the help-dump→shll.ai step**. **Trigger mechanism resolved (clarify)**: a **release PR merge to `main`** (release-labeled / version-bumping) triggers a tagging step that pushes a `v*` tag; the **existing tag-push path** in `release.yml` then runs the full release pipeline (release notes, GitHub release, Homebrew tap, **help-dump→shll.ai PR + auto-merge**). Releases stay **tag-anchored** — the Homebrew tap update and version stamping already depend on tags — while a PR merge becomes the new entry point. This is the smallest, most compatible change to the current `release.yml`.
- **Q2 (recursion default)**: The producer is flat today (`commands: []`). If/when tu grows per-subcommand help screens, recursion walks each subcommand's `--help`. Default assumption: build the producer flat now, leave a clearly-marked extension point — do not pre-build speculative recursion. Confirm acceptable.
- **Q3 (`captured_at` precision)**: ISO-8601 UTC with `Z` suffix is mandated. Millisecond (`...T12:34:56.000Z`, JS `toISOString()` default) vs second precision (`...T12:34:56Z`)? Default: match `wt.json` exactly once available; until then use JS `toISOString()` default (millisecond, `Z`). Confirm.
- **Q4 (producer invocation site)**: Does the producer run automatically as the **tail of `scripts/build.sh`** (so `npm run build` always refreshes `help/tu.json` locally too), or **only as a separate CI step / `npm run help-dump`**? Default: separate script + CI step; keep `build.sh` focused on bundling. Lean, but confirm.
- **Q5 (commit `help/tu.json` to the tu repo?) — RESOLVED**: **Transient in CI** — the release job generates `help/tu.json`, opens the shll.ai PR, and auto-merges it. Not committed to the tu repo. shll.ai is the single source of truth; the tu repo stays clean and there is no regen-on-bump staleness risk. (Clarified: user directed "both generate the PR and merge it" — the artifact's home is shll.ai, full PR-and-merge.) The producer still *writes* the file to a `help/tu.json` path in the CI workspace; that path is `.gitignore`'d (or simply never committed) in the tu repo.
- **Q6 (auto-merge method)**: `--squash` vs `--merge` vs `--rebase` for the shll.ai PR. Default: `--squash` (clean single-commit history per tool update); still Confident (#15) — governed by shll.ai branch settings, trivially changed. The clarify direction confirmed the PR **must actually merge** (not sit open), reinforcing auto-merge; the exact method stays the `--squash` default.

## Clarifications

### Session 2026-06-03

| # | Question | Resolution |
|---|----------|------------|
| Q5 / #13 | Commit `help/tu.json` to the tu repo, or generate transiently in CI? | **Transient in CI.** User directed "both generate the PR and merge it" — the artifact's home is shll.ai (full PR-and-merge), not the tu repo. tu repo stays clean (`help/` gitignored); no regen-on-bump staleness. Reinforces auto-merge (#6/#15). → Certain |
| Q1-followon / #17 | Concrete `release.yml` trigger for "run on PR merges"? | **Release-PR merge → tag → existing pipeline.** A release-labeled / version-bumping PR merging to `main` triggers a tagging step that pushes `v*`; the existing tag-push path runs the full release pipeline + help-dump. Releases stay tag-anchored (Homebrew tap + version stamping depend on tags); PR merge is the new entry point. → Certain |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | JSON contract shape is `{tool, version, captured_at, schema_version:1, root:Node}` with `Node={name,path,short,usage,text,commands}`, taken verbatim from the frozen backlog spec | Contract is explicitly frozen in the backlog and fully specified inline; no interpretation needed | S:98 R:70 A:95 D:98 |
| 2 | Certain | `root.commands = []` and the document is flat (no recursion) | Backlog states "effectively flat: root Node carries the full 'tu --help' text and commands is an empty array []"; confirmed tu prints no per-subcommand help screens (cli.ts dispatch + completions.ts) | S:95 R:75 A:95 D:95 |
| 3 | Certain | Capture help via the **built** binary `node dist/tu.mjs --help`, byte-for-byte, after build | Backlog: "Capture the literal output of running the built CLI with --help"; build produces `dist/tu.mjs` (build.sh) | S:95 R:65 A:92 D:90 |
| 4 | Certain | `version` read from `package.json` at runtime (currently 0.4.14, not the backlog's stale 0.4.13) | Backlog says "read version from package.json"; verified package.json version is 0.4.14; runtime read self-corrects | S:95 R:80 A:98 D:95 |
| 5 | Certain | Bespoke Node producer using `node:` built-ins only; no new runtime dependency; not the shared Go producer | Backlog mandates bespoke Node producer; constitution III forbids runtime deps; tu has no Cobra tree | S:95 R:60 A:95 D:92 |
| 6 | Certain | Push via PR into `sahil87/shll.ai` at `help/tu.json` using `SHLLAI_TOKEN`, with auto-merge; never a direct push to main | Backlog mandates this explicitly, including the multi-repo-push-race rationale | S:98 R:55 A:90 D:95 |
| 7 | Certain | `tu --help` output is identical to `tu -h` and `tu help`, all = `FULL_HELP` constant; `--version` is a separate non-help branch | Verified cli.ts:1122-1132 — `--help` hits the help branch and prints FULL_HELP | S:95 R:85 A:98 D:90 |
| 8 | Certain | Producer **fails the CI build** (non-zero exit) on capture error, empty output, or JSON validation failure — never PRs a malformed artifact | Backlog explicitly says "validate it parses"; constitution II (graceful degradation) is runtime-only, so a build artifact MUST fail loud — determined by stated requirement + constitution | S:85 R:65 A:90 D:90 |
| 9 | Certain | Capture help with color disabled (`NO_COLOR=1`) to guarantee plain byte-for-byte `text` | The frozen contract mandates RAW byte-for-byte help text; forcing NO_COLOR is the determined consequence of that requirement (tu honors NO_COLOR/--no-color); FULL_HELP carries no ANSI so output is plain | S:80 R:75 A:88 D:85 |
| 10 | Confident | `SHLLAI_TOKEN` secret already exists in tu's Actions secrets with contents+PR write on shll.ai, and shll.ai permits auto-merge; provisioning is out of scope | Backlog says "the existing repo secret SHLLAI_TOKEN" (existing) and the consumer is tracked separately; but it's an external-state assumption this change can't itself verify → Confident, not Certain | S:80 R:55 A:75 D:80 |
| 11 | Certain | `root.short` sourced from package.json.description / config description ("AI coding assistant cost tracking CLI"); `root.usage` = the `Usage:` line of FULL_HELP; `root.name`=`root.path`=`"tu"` | The contract field names are frozen and each maps to one deterministic verified source (package.json / config.yaml / the verified first line of FULL_HELP) — determined by contract + codebase | S:80 R:70 A:85 D:85 |
| 12 | Certain | Producer lives at `scripts/help-dump.mjs` (or `.ts` via tsx), exposed as `npm run help-dump`, run as a separate CI step (NOT folded into build.sh) | `scripts/` already holds build.sh/release.sh/release-notes.sh — project convention deterministically places a new build script there; keeping build.sh focused on bundling follows the existing single-responsibility pattern | S:75 R:80 A:88 D:85 |
| 13 | Certain | `help/tu.json` is generated **transiently in CI** and not committed to the tu repo; the release job generates it, opens the shll.ai PR, **and auto-merges it** — shll.ai is the single source of truth | Q5 — Clarified: user directed "both generate the PR and merge it" (destination is shll.ai, full PR-and-merge), which is the transient-in-CI path; reinforces the auto-merge in #6/#15 | S:95 R:70 A:65 D:55 |
| 14 | Certain | `captured_at` uses JS `toISOString()` (millisecond, `Z`) — the standard JS UTC ISO-8601 output | The contract mandates ISO-8601 UTC with `Z` suffix; `toISOString()` IS exactly that output — determined by the contract (sub-second precision is the JS default and trivially trimmed if wt.json later pins seconds) | S:80 R:88 A:88 D:88 |
| 15 | Confident | Auto-merge via `--squash` | Q6 — `--squash` is the conventional clean-history default for automated single-file bot PRs; ultimately governed by shll.ai branch settings and trivially changed | S:65 R:85 A:75 D:80 |
| 16 | Certain | Help-dump→shll.ai step lives in **`release.yml`** (release pipeline, fires on PR merges); a **new `ci.yml`** (build verify + tests on push to `main`) is split out and does NOT carry the help-dump | Q1 — Clarified: user explicitly directed the two-workflow split and placed help-dump in release.yml | S:95 R:60 A:95 D:95 |
| 17 | Certain | `release.yml` trigger: a **release PR merge to `main`** (release-labeled / version-bumping) triggers a tagging step that pushes `v*`; the **existing tag-push path** then runs the release pipeline + help-dump. Releases stay tag-anchored (Homebrew tap + version stamping depend on tags); PR-merge is the new entry point | Q1-followon — Clarified: user selected "Release-PR merge → tag → release"; most compatible with current release.yml | S:95 R:55 A:60 D:50 |

17 assumptions (15 certain, 2 confident, 0 tentative, 0 unresolved).
