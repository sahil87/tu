---
type: memory
description: "tudiff's argument matrix and per-case staging — harness/matrix.json case groups over the conf/env/io/tz axes and their expansion to stable case IDs, the four $HOME skeletons and the seeded harness/metrics-repo copy, and the from-scratch child environment including TUDIFF_NOW and the TUDIFF_GIT_SCRIPT failure rule sets."
---
# Matrix and Staging

**Domain**: harness

## Overview

`harness/matrix.json` declares the case groups `tudiff run` expands over the conf/env/io/tz axes, and `src/go/internal/harness` stages each case's `$HOME` and a from-scratch child environment. Execution, fixture resolution and reporting live in [differential-harness](/harness/differential-harness.md); the comparison rules and the golden corpus in [comparison-and-live](/harness/comparison-and-live.md) and [golden-corpus](/harness/golden-corpus.md).

## Requirements

### Requirement: The argument matrix
`harness/matrix.json` (committed, JSON, `schema: 1`) lists **case groups** — `{"id", "args", "conf"?, "env"?, "io"?, "tz"?}` decoded with `DisallowUnknownFields`; a missing `args` key is invalid while an empty array is valid. Axis values: `conf ∈ {single, multi, org, legacy}` (which `$HOME` skeleton is staged), `env ∈ {default, nocolor, envrepo, pullfail, pushfail, dirty}` (`nocolor` sets `NO_COLOR=1`; `envrepo` sets `TU_METRICS_REPO=git@example.invalid:harness/tu-metrics.git`, flipping even `single` into multi mode through the env layer of the cascade; the last three set `TUDIFF_GIT_SCRIPT` to scripted git failure rule sets — see the child-environment requirement (lsml)), `io ∈ {pipe, tty}` (separate pipes vs. a pseudo-terminal via `script`), `tz ∈ {fixed, alt}` (`fixed` = `TZ=UTC`; `alt` = `TZ=Asia/Kolkata`, a non-integer offset and the zone the local captures were bucketed in). An omitted axis takes its base value (`single`/`default`/`pipe`/`fixed`). `LoadMatrix` rejects a schema other than 1, an empty/duplicate/non-path-component `id`, an unknown axis value, and a duplicate value within one axis — every error names the offending group id and field. `Expand` crosses each group in the nested order conf → env → io → tz; the expanded **case ID** is `<id>/<conf>/<env>/<io>/<tz>` — every axis always appears, so IDs stay stable when a group later gains an axis. `Filter` keeps cases whose ID contains the substring. The committed matrix covers the toolkit commands, snapshots for every source token, format/flag variants, windowed and full history, the `--by-machine` pivot (the original empty-state flag cases plus nine populated groups — the windowed and monthly single-tool history in table/json/csv/md, token mode, `-u all` on both, and the `snapshot-cc-by-machine-json` single-source case that pins the zero-fill quirk), the leaderboards (the baseline `lb*`/`lbh*` gate/empty-state groups plus 28 populated groups — the seeded January window in table/json/csv/md, `--by-machine` including the `-t` three-way tie, `--top 1` on both displays, `-t`, `-u other-user`, `--until`-only/`--since`-only, `cc lb`, the `m lbh` family, `lbh --full` under both time zones, the `top-snapshot` warn-and-ignore guard, and the `lb-tty` case — the leaderboard's one `io: tty` case, which exercises the mid-row bar on a real 120-column terminal), multi-mode flags, the six sync groups (`sync-cmd` on the four confs × `default`/`pullfail`/`pushfail`/`dirty`, `sync-dry-run` on the four confs × both time zones — the `tz: alt` cases are the DC-21 byte check, a UTC commit date beside local day-file dates — `sync-flag` and `cc-sync` on `single`/`multi` × `default`/`pullfail`, `cc-sync-json`, and `sync-json` — every multi-mode sync case also proving the written tree through the `tree` channel) (lsml), setup commands, usage errors, and the watch flag family (4pze) — the exit-2 incompatibilities `watch-json` (`--watch --json`), `watch-csv` (`-w --csv`) and `watch-md` (`-w --md`), the `--interval` validation cases `watch-interval-min` (`-w -i 3`), `watch-interval-max` (`-w -i 4000`) and `watch-interval-nan` (`-w -i abc`), and the DC-03 silent-acceptance cases `no-rain-without-watch` (`--no-rain`; conf single/multi × io pipe/tty) and `interval-without-watch-long` (`--interval 30`) — 159 groups expanding to 452 cases — and contains no `update` group. No live `-w` session case exists: a watch frame is wall-clock- and RNG-dependent (the elapsed counter, the countdown, the rain PRNG), so no deterministic golden exists; the Go watch loop is gated instead by golden frames under a fake clock plus a fake-terminal loop test ([watch/compositor](/watch/compositor.md), [loop-and-terminal](/watch/loop-and-terminal.md)). The file is data: coverage is reviewed at G0 and edited without code changes. Editing it invalidates the goldens' `matrix_sha256` — compare mode refuses to run until `--update` regenerates them ([golden-corpus](/harness/golden-corpus.md)).

#### Scenario: Expansion order and base defaults
- **GIVEN** a group `{"id":"x","args":[],"conf":["single","multi"],"io":["pipe","tty"]}`
- **WHEN** expanded
- **THEN** the IDs are exactly, in order, `x/single/default/pipe/fixed`, `x/single/default/tty/fixed`, `x/multi/default/pipe/fixed`, `x/multi/default/tty/fixed`

### Requirement: Per-case $HOME staging and the seeded metrics repo
For each expanded case the driver stages **one** HOME — `<tmp>/cases/<safeID>/go/home` — via `StageHome(dir, variant, seedDir)`: `single` creates the directory and nothing inside it; `multi` writes `.config/tu/tu.conf`; `org` writes `.config/tu/org.conf` (no `tu.conf`); `legacy` writes `.tu.conf` (no `.config/`). The three conf files are byte-identical and exactly:

```
version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
```

Only the file's location varies between variants; the pinned `machine`/`user` defeat the `$HOSTNAME`/`$USER` sentinels, and the `.invalid` TLD can never resolve. For the three multi variants the committed seed `harness/metrics-repo/` is copied recursively to `<dir>/.tu/metrics_repo/`; no `.git/` is created (the metrics-dir guard is an existence check, and the fake git answers every call). The seed follows the metrics-repo layout spec (`<user>/<year>/<machine>/<tool>-<date>.jsonl`, one `UsageEntry` JSON line each, `label` equal to the filename's date) on the placeholder dates 2026-01-05..07 with the placeholder token counters: two profiles (`harness-user`, `other-user`) across two machines so `lb`/`lbh`/`-u` cases have rows; the own-machine cc day-files cost `0.25` and `0.75`, straddling the placeholder fixtures' `0.5` so both arms of the own-machine max-merge (live wins / stored wins via the never-shrink guard) are exercised; `docs/README.md` documents the tree and is never scanned as a user (the spec excludes `docs/`).

#### Scenario: Legacy variant staging
- **GIVEN** variant `legacy` staged into an empty dir
- **WHEN** `StageHome` returns
- **THEN** `<dir>/.tu.conf` exists with the bytes above, `<dir>/.config` does not exist, and `<dir>/.tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-06.jsonl` exists

### Requirement: The from-scratch child environment
`BuildEnv` constructs the child environment from scratch — exactly `PATH=<abs harness-bin>:<harness process's PATH>` (fakes first), `HOME=<the staged home>`, `TZ` per the axis, `LANG=C.UTF-8`, `LC_ALL=C.UTF-8`, `TUDIFF_FIXTURES=<resolved alias dirs, absolute, OS path list>`, `TUDIFF_CALL_LOG=<report>/cases/<case>/go.calls.jsonl`, plus `TUDIFF_NOW=<the golden manifest's pinned now>` whenever `EnvSpec.Now` is non-empty (the Go binary's edge `now()` seam reads it — see [differential-harness](/harness/differential-harness.md)), `TERM=xterm-256color` only for `io: tty`, `NO_COLOR=1` only for `env: nocolor`, and `TU_METRICS_REPO` only for `env: envrepo`. No other variable from the parent environment is present, so an exported `TU_METRICS_REPO` or `NO_COLOR` in the developer's shell cannot tilt a case. The side's working directory is `<tmp>/cases/<case>/go/` (its `home` a sibling), never the repo root. `TUDIFF_GIT_SCRIPT` is appended by `BuildEnv` (in `internal/harness/diff.go`) only for the three scripting env values, from the `gitScripts` table whose JSON text is part of the byte contract (lsml):

| `env` | `TUDIFF_GIT_SCRIPT` | Exercises |
|-------|--------------------|-----------|
| `pullfail` | `[{"match":["pull"],"stderr":"fatal: couldn't find remote ref main\n","exit":1}]` | the pull-failure line, the follow-up `rebase --abort` call, exit 1 / `sync failed — using local data.` |
| `pushfail` | `[{"match":["push"],"stderr":"error: failed to push some refs\n","exit":1}]` | the single retry (two `push` calls) and the post-retry warning line |
| `dirty` | `[{"match":["status","--porcelain"],"stdout":" M harness-user/x\n","exit":0}]` | the commit path and its `commit -m "# harness-user: update {UTC date}"` argv |

For every other env value the variable is unset and the fake git is the silent exit-0 stub for every call.

## Design Decisions

### JSON case groups with axis defaults, not a cross-product
**Decision**: `harness/matrix.json` lists case groups; omitted axes take one base value.
**Why**: A full product of 4×3×2×2 over ~90 argv lines is thousands of cases; explicit axes keep CI to minutes and make G0's coverage review a read of one file.
**Rejected**: A line-oriented text file (no per-line axis control without inventing syntax); a full cross-product with a `--sample` flag (nondeterministic coverage).
*Introduced by*: 260916-i9hc-harness-differential

### One staged $HOME per case
**Decision**: Each case stages a single skeleton home for the Go side; the golden side has no runtime.
**Why**: The oracle is a committed corpus, not a process ([golden-corpus](/harness/golden-corpus.md)), so there is no second side to isolate; the one home still gets the full skeleton and seed so the written tree lands in `tree.json`.
**Rejected**: Staging a second home for symmetry (dead weight — nothing runs against it).
*Introduced by*: 260925-6wpm-remove-src-node

### Git failure scripting rides the `env` axis
**Decision**: `pullfail`/`pushfail`/`dirty` are `env` values setting `TUDIFF_GIT_SCRIPT`.
**Why**: The axis means "which extra environment variable is set"; a new axis would rename every existing case ID.
**Rejected**: A fifth `git` axis.
*Introduced by*: 260916-lsml-sync-metrics-writer
