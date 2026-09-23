---
type: memory
description: "How tudiff decides red — first-divergence byte comparison per channel after $HOME normalization, the tree channel over the written metrics repo, informational call-log diffs, the DC-keyed harness/expected-diffs.json file with its match rule and stale-entry failure, and tudiff live's real-git sync and repair parity flow on temp bare repos."
---
# Comparison and Live

**Domain**: harness

## Overview

After both sides run, `harness.Compare` and `harness.CompareTrees` decide a case's colour, `harness/expected-diffs.json` marks the divergences a spec decision makes intentional, and `tudiff live` repeats the sync and repair steps against real git. The matrix run that feeds these is in [differential-harness](/harness/differential-harness.md).

## Requirements

### Requirement: Expected-diffs file
`harness/expected-diffs.json` (committed, JSON, `schema: 1`, sibling of `matrix.json`) declares the divergences a spec drop decision makes intentional: `{"schema": 1, "expected": [{"id": "DC-05", "cases": ["snapshot-*-csv", "…"], "reason": "…"}]}`, keyed by the spec's stable `DC-NN` ids. `LoadExpected(path)` (`src/go/internal/harness/expected.go`) decodes with `DisallowUnknownFields` plus a trailing-value rejection (the `LoadMatrix` pattern) and validates: `schema == 1`; the `expected` key present (an empty array is valid, a missing key invalid — the matrix's `args` rule); per entry an `id` matching `^DC-\d{2}$` that is unique across entries, a non-empty `cases` array whose every pattern `path.Match` accepts (an `ErrBadPattern` is a load error), and a non-empty `reason` — every error names the offending entry `id` (when known) and field. `(*Expected).Match(caseID, group)` tries entries in file order and the first matching entry wins: a pattern matches when `path.Match(pattern, caseID)` is true, and a pattern containing no `/` is additionally tried against the group name. Case IDs are `<group>/<conf>/<env>/<io>/<tz>` for `run` and the bare step name under group `live` for `live`, so `snapshot-*-csv` names whole groups, `sync-dry-run/*/*/*/alt` one axis slice, `sync-dry-run` both the matrix group and the live step of that name, and `live` every live step (`*` never crosses `/`, which is why axis-level patterns spell all five segments). A nil or empty set never matches. The committed set is empty: every `[DECIDE]` marker in `docs/specs/usage.md` § Drop at cutover is unresolved, and unresolved means keep — a later `drop` resolution is one entry here, not a code change. (489t)

#### Scenario: First matching entry wins
- **GIVEN** entries `DC-05: ["snapshot-*-csv"]` and `DC-21: ["sync-dry-run/*/*/*/alt", "sync-dry-run"]`
- **WHEN** matching `snapshot-all-csv/multi/default/pipe/fixed` (group `snapshot-all-csv`), `sync-dry-run/multi/default/pipe/alt` (group `sync-dry-run`), `sync-dry-run/multi/default/pipe/fixed` (group `sync-dry-run`), and `sync-dry-run` (group `live`)
- **THEN** the results are `DC-05`, `DC-21` (the axis-slice pattern hits the full ID), `DC-21` (the bare pattern hits the group), and `DC-21` (the bare pattern hits the live step ID)

### Requirement: Comparison — first divergence per case
`Compare` byte-diffs channels in order and stops at the first difference: pipe cases compare `exit`, `stdout`, `stderr`; tty cases compare `exit`, `tty`. Each side's byte channels are home-normalized before comparing: `NormalizeHome(b, home)` replaces every occurrence of the side's staged `$HOME` path in `b` with the literal `$HOME` (byte replacement, no regexp; an empty home is a no-op — distinct from `Redact`, which anonymizes the developer's real home for the committed fixture corpus). The two staged homes differ only in the side segment, and the Goal-frozen absolute-path messages (the setup commands' `Already initialized`/`Cloned` lines, the version warning's conf path) legitimately embed `$HOME`, so unnormalized captures would diverge even when both binaries are correct. `SideCapture.Home` carries each side's staged home (set by `runCase`); the divergence offset, 1-based line (counting `\n` in the node capture — the oracle side is the line-numbering reference), and `strconv.Quote` excerpts of at most 40 bytes are computed on the normalized bytes. Every compared channel identical → **green**; a timeout on either side → **timeout** (channel `timeout`), counted as red for the exit code but listed separately in the summary. Otherwise the case is **red** with `Channel` naming the first differing one; an exit divergence records the two codes. The call logs are compared separately as sorted sets of `{tool, argv}` pairs — `CompareCallLogs(nodePath, goPath, nodeHome, goHome)` normalizes each side's argv entries with that side's staged home before forming the set, so the `-C <dir>` and `clone <url> <dir>` shapes compare equal (`cwd` differs by side by construction; `matched` is alias metadata) — into `CallsDiffer`/`NodeCalls`/`GoCalls` — **informational only**: a difference appends `[calls differ: node=<n> go=<n>]` to the case line and never reddens a case.

#### Scenario: First divergence on stdout
- **GIVEN** node stdout `"Usage: tu\n"` and go stdout `""` with equal exit codes
- **WHEN** compared
- **THEN** the case is red on channel `stdout` at offset 0, line 1, with node excerpt `"Usage: tu\n"` and go excerpt `""`

### Requirement: The tree channel compares the written metrics repo
After the byte comparison, a case that is still green is checked by `harness.CompareTrees(nodeHome, goHome)` (wired into `runCase` after `Compare`): the two sides' `<home>/.tu/metrics_repo/` trees — the set of slash-separated relative file paths and each file's bytes (day-file content carries no home path, so no normalization) — plus the **presence** of `<home>/.tu/.last-sync` (its content is a wall-clock timestamp, so presence only). Nothing under `.tu/cache/` is compared (the two cache formats differ by design). A difference reddens the case with `Channel: "tree"` and the first differing relative path (or `.last-sync`) in `NodeExcerpt`/`GoExcerpt` (present/absent or a short byte excerpt); a case already red keeps its first channel. The report renders the `tree` channel like any other red channel. Every multi-mode data case thereby also proves the write — a `--json`-style float or key-order slip in a written day-file becomes visible. (lsml)

#### Scenario: A day-file byte divergence is red on the tree channel
- **GIVEN** a multi-mode data case where the Go side wrote a day-file with different bytes
- **WHEN** `tudiff run` compares it
- **THEN** the case is red on channel `tree` naming that path; identical trees with `.last-sync` on both sides stay green

### Requirement: tudiff live — real-git parity
`tudiff live [--node] [--go] [--turepair] [--harness-bin] [--report] [--expected] [--keep]` (`src/go/cmd/tudiff/live.go`, run via `just go-live`) runs the real-git half of the sync gate where `run` answers every git call with the fake. Preflight: real `git` on `PATH`, `node`, the oracle bundle, both Go binaries, and the fake `ccusage`, then the expected-diffs file is loaded with the same flag, resolution rule, and preflight messages as `run`; the child `PATH` is a livebin holding only the fake `ccusage` followed by the process `PATH` — the fake git is deliberately absent. Seeding: one bare repo (`git init --bare --initial-branch=main`); a scratch clone receives `harness/metrics-repo/` in one `seed` commit and pushes; the bare is then copied into two identical remotes, never shared; each side's `multi` staged home (via `harness.StageHome`) has its `.tu/metrics_repo` replaced by a real clone of its own bare. Every child carries a pinned identity (`GIT_AUTHOR_NAME/EMAIL`, `GIT_COMMITTER_NAME/EMAIL`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `commit.gpgsign=false` via `GIT_CONFIG_COUNT/KEY_0/VALUE_0`) and FIXED `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` (`2026-01-09T12:00:00Z`), so identical trees yield identical commit hashes. Seeding and comparison reuse the exported `harness.CopyTree`/`CopyFile`/`Excerpt` helpers.

The nine steps run on both sides with `TZ=UTC` and the same midnight-rollover re-run rule as `runCase`; each step's stdout/stderr/exit is compared with `harness.Compare` after home normalization: `sync --dry-run` (then both trees provably unchanged — the no-mutation guarantee), `sync` (then day-file trees byte-equal, `git status --porcelain` empty on both, `git log --format=%H%n%s -p main` byte-identical including hashes, `.last-sync` present on both), a second `sync` (the steady-state no-commit path; the log unchanged), a foreign commit pushed to each bare then `sync` with a one-off fixture alias raising cc `2026-01-07` (both sides' `.tu/cache` wiped first — the cache key excludes `TUDIFF_FIXTURES`, so without the wipe the earlier steps' warm cache would hide the alias's raised value; the sync then commits the local change and `pull --rebase` integrates; the logs identical), a fabricated `.git/rebase-merge` then `sync` (the `Warning: recovering from interrupted rebase` line; the abort's failure swallowed), a broken `origin` then `sync` (the pull-failure bytes carrying real git's stderr verbatim under the `$HOME`-normalized `git -C $HOME/.tu/metrics_repo pull --rebase origin main` line, then `Error: sync failed — check network and remote config.`, exit 1), and `cc --sync` (`syncing metrics... synced.` then the table). Then the repair parity flow: the shrunk-history fixture seeded in one repo, copied twice, `node scripts/repair-metrics.mjs --repo <A>` vs `bin/turepair --repo <B>` compared after replacing each side's repo path with `$REPO` — dry-run, then both with `--write`, outputs and trees compared again. The report lands under `--report` (default `bin/harness/report-live/`) in the `run` report's shape (per-step lines, first divergence, summary); a red step is annotated with the matching expected-diffs entry exactly as in `run`, and the exit-code rule is `run`'s (exit 1 iff unexpected + timeout + unconfirmed > 0 or any entry is stale). CI runs it as the `tudiff` job's second step, and that job gates `ci-gate` — see [toolchain](/build/toolchain.md) (lsml, 489t).

## Design Decisions

### Compare-side home normalization
**Decision**: Each side's staged home path is replaced with the literal `$HOME` in the stdout/stderr/tty captures and the call-log argv before comparison, and the report's per-side capture files hold the normalized bytes.
**Why**: Two fresh homes per case (the decision above) is still right, and the setup commands' absolute-path messages and the git argv shapes (`-C <dir>`, `clone <url> <dir>`) are Goal-frozen surfaces that legitimately embed `$HOME` — without normalization those cases could never go green.
**Rejected**: A shared home (masks divergences through shared cache and day-files); staging both sides at the same absolute path via a symlink (fragile across `script`/tty and still leaks the side's real dir through `cwd`).
*Introduced by*: 260916-4fs0-config-and-setup-commands

### Call log is informational
**Decision**: `calls` differences are printed but never redden a case.
**Why**: The TS fetcher issues its per-tool ccusage calls via `Promise.all`; order is nondeterministic and a sorted comparison still can't distinguish "Go hasn't fetched yet" from "Go fetched differently" while every case is red.
**Rejected**: Compared channel (flaky now); dropping it (loses the call-log seam's value for V1).
*Introduced by*: 260916-i9hc-harness-differential

### Expected diffs are a committed data file keyed by spec DC id
**Decision**: `harness/expected-diffs.json` (schema 1) lists entries `{id, cases, reason}` keyed by the spec's stable `DC-NN` IDs; both `tudiff run` and `tudiff live` load it via `--expected`.
**Why**: A `drop` resolution at G0/G3 must become one data entry, not a change to the gate's code; the matrix already models coverage as reviewed data, and DC IDs are declared stable in the spec.
**Rejected**: An `expected` field on matrix groups (a drop spans groups and is keyed by spec id, not group id); a `--expect` CLI flag (state would live in CI YAML, not beside the matrix).
*Introduced by*: 260918-489t-harness-full-matrix-gate

### A missing expected-diffs file is a preflight error, never an empty set
**Decision**: `tudiff run`/`live` exit 2 when the file is absent or invalid.
**Why**: The harness never reports a vacuous green (`--filter` matching nothing and a missing placeholder manifest are already exit 2); the file is committed, so the default always exists.
**Rejected**: Treating absence as `expected: []` (a mis-resolved path would silently drop every expectation and turn intended reds into gate failures, or worse, hide a stale set).
*Introduced by*: 260918-489t-harness-full-matrix-gate

### Stale entries fail, unmatched entries only report
**Decision**: An entry that matched at least one executed case and none red exits 1 with a `(stale)` line; an entry that matched zero executed cases prints `(no executed case)` and does not affect the exit code.
**Why**: An always-green expectation is a misconfiguration worth failing on; `--filter` and `live` legitimately exclude cases, so zero-matched must not fail, and this keeps `run` and `live` decoupled (no cross-runner case list).
**Rejected**: A preflight cross-check that every entry matches some case in the full matrix (breaks `live`-only entries and `--filter` runs).
*Introduced by*: 260918-489t-harness-full-matrix-gate
