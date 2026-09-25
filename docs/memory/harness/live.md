---
type: memory
description: "tudiff live — the real-git half of the differential gate: the seven sync steps plus two repair steps run on one staged home against a temp bare remote with a pinned git identity and fixed commit dates, compared against harness/golden/live/<step>/ goldens (bytes, tree.json, log.txt/status.txt), with $TMP/$REPO normalisation, the --update writer, and run's gate rule."
---
# Live Harness

**Domain**: harness

## Overview

`tudiff live` (`src/go/cmd/tudiff/live.go`, run via `just go-live`) runs the real-git half of the sync gate where `run` answers every git call with the fake: the sync and repair sequence executes against real git on temp bare repos and is compared against the `harness/golden/live/<step>/` goldens. The comparison primitives are in [comparison-and-live](/harness/comparison-and-live.md), the corpus format in [golden-corpus](/harness/golden-corpus.md), the staged homes and child environment in [matrix-and-staging](/harness/matrix-and-staging.md), and the report shape in [report](/harness/report.md).

## Requirements

### Requirement: tudiff live — real-git parity against live/ goldens
`tudiff live [--go] [--turepair] [--harness-bin] [--report] [--expected] [--keep] [--golden] [--update] [--now]` runs the sequence and compares each step against its golden. Preflight: real `git` on `PATH`, both Go binaries, and the fake `ccusage`, the placeholder manifest the fake replays from, then the expected-diffs file with the same flag, resolution rule, and preflight messages as `run`, then the golden guards — the manifest present and valid (`tudiff: harness/golden/manifest.json not found (run tudiff live --update)`), its `matrix_sha256` matching the current matrix, and a golden dir per step (`tudiff: no golden for live/<step> (run tudiff live --update)`); `--update` skips the hash and per-step guards. The child `PATH` is a livebin holding only the fake `ccusage` followed by the process `PATH` — the fake git is deliberately absent. Seeding: one bare repo (`git init --bare --initial-branch=main`); a scratch clone receives `harness/metrics-repo/` in one `seed` commit and pushes; the staged `multi` home (via `harness.StageHome`) has its `.tu/metrics_repo` replaced by a real clone of the bare. Every child carries a pinned identity (`GIT_AUTHOR_NAME/EMAIL`, `GIT_COMMITTER_NAME/EMAIL`, `GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, `commit.gpgsign=false` via `GIT_CONFIG_COUNT/KEY_0/VALUE_0`) and FIXED `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` (`2026-01-09T12:00:00Z`), so identical trees yield identical commit hashes — which is what makes `log.txt` (hashes included) reproducible. Seeding and comparison reuse the exported `harness.CopyTree`/`CopyFile`/`Excerpt` helpers.

The nine steps run on the one staged side with `TZ=UTC`, `TUDIFF_NOW=<manifest.now>`, and the same midnight-rollover re-run rule as `runCase`; each step is compared against `live/<step>/`: stdout/stderr/exit via `harness.Compare`, the clone's written tree against `tree.json` (`CompareLiveTree`, `.git/` excluded), and `status.txt`/`log.txt` via the `checkEqual` text comparison where the step pins them (identity-normalised on both sides) — a step's golden stores exactly the artefacts that step checks: `sync --dry-run` (then tree and status provably unchanged — the no-mutation guarantee), `sync` (then day-file trees match, `git status --porcelain` empty, the log byte-identical including hashes, `.last-sync` present), a second `sync` (the steady-state no-commit path; `log.txt` is the post-sync log, so matching it proves no new commit), a foreign commit pushed to the bare then `sync` with a one-off fixture alias raising cc `2026-01-07` (the side's `.tu/cache` wiped first — the cache key excludes `TUDIFF_FIXTURES`, so without the wipe the earlier steps' warm cache would hide the alias's raised value; the sync then commits the local change and `pull --rebase` integrates), a fabricated `.git/rebase-merge` then `sync` (byte equality plus a both-positions assertion that stderr carries the `Warning: recovering from interrupted rebase` line), a broken `origin` then `sync` (the pull-failure bytes carrying real git's stderr verbatim under the `$HOME`-normalized `git -C $HOME/.tu/metrics_repo pull --rebase origin main` line, then `Error: sync failed — check network and remote config.`, and a both-positions exit-1 assertion), and `cc --sync` (`syncing metrics... synced.` then the table). Then the repair flow: the shrunk-history fixture seeded in one repo and copied once, `bin/turepair --repo <repo>` dry-run then `--write`, outputs and the repo's gitless tree (`TreeSnapshotRepo`) compared against `live/repair-dry-run/` and `live/repair-write/`. The report lands under `--report` (default `bin/harness/report-live/`) in the `run` report's shape; a red step is annotated with the matching expected-diffs entry exactly as in `run`, and the exit-code rule is `run`'s (exit 1 iff unexpected + timeout + unconfirmed > 0 or any entry is stale). `--update` writes each step's golden instead and aborts on the first capture or pre-step harness failure (the manifest is never written after one), then rewrites the manifest with `live_steps` set and `cases` preserved. CI runs it as the `tudiff` job's second step, and that job gates `ci-gate` — see [toolchain](/build/toolchain.md) (lsml, 489t, 6wpm).

#### Scenario: Nine green steps against the committed corpus
- **GIVEN** the committed live goldens
- **WHEN** `just go-live` runs
- **THEN** the summary is `9 cases — 9 green …` (seven sync steps + two repair steps) and the exit code is 0

## Design Decisions

### Live channels also normalise $TMP and $REPO
**Decision**: Live captures replace the run's temp root with the literal `$TMP`, and the repair steps replace the `--repo` path with `$REPO`, before the byte channels are stored or compared.
**Why**: Real git's error text (the pull-failure step) embeds the missing remote's path verbatim and the repair report prints its repo path; both are per-run temp paths the frozen corpus cannot hold.
**Rejected**: Seeding the repos at fixed paths under the repo root (the harness stages under `os.MkdirTemp` by design; fixed paths would serialise concurrent runs).
*Introduced by*: 260925-6wpm-remove-src-node

### One remote, one clone, one home
**Decision**: The live sequence stages a single side — one bare remote seeded from `harness/metrics-repo/`, one real clone as the home's metrics repo, one staged `multi` home.
**Why**: The golden corpus is the expected side, so there is no second runtime to isolate; the pinned git identity and fixed commit dates make the single side's hashes reproducible against `log.txt`.
**Rejected**: Seeding two identical remotes and homes for symmetry (dead weight — nothing runs on a second side).
*Introduced by*: 260925-6wpm-remove-src-node
