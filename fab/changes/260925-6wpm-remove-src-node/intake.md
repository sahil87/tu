# Intake: Remove src/node — golden-file harness, Node toolchain retired

**Change**: 260925-6wpm-remove-src-node
**Created**: 2026-09-25

## Origin

Plan row **Z1** of `fab/plans/sahil/26-09-15-go-port.md` (Phase 5 — Retire; decisions D10, D13). Invoked one-shot via `/fab-new` with two SRAD questions answered inline.

> Context: fab/plans/sahil/26-09-15-go-port.md, row Z1. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. Scope: two Go releases (v0.12.1, v0.12.2) have shipped past cutover; delete src/node/. Key detail: CI's tudiff job and just go-diff still byte-diff node dist/tu.mjs against bin/tu (ci.yml around line 83, justfile around line 96, src/go/cmd/tudiff/run.go defaultNode = dist/tu.mjs). Replace that oracle before deleting src/node: freeze the current Node outputs for the full matrix as committed golden captures (placeholder fixtures only, no real usage data), switch tudiff run to compare bin/tu against those goldens, keep 452/452 green, then delete src/node/, package.json, package-lock.json, esbuild/tsx/typescript devDependencies, scripts/build.sh, scripts/help-dump.mjs, and every setup-node/npm step in CI and release.yml. The ccusage vendor fetch in scripts/package-go.sh does not use npm and stays. Old tags (pre-Z1) keep src/node in their own git history, so rollback via the tap is unaffected.

Decisions taken in the conversation (asked, user chose the recommended option both times):

1. **Clock seam** — frozen goldens cannot follow the calendar (snapshot "today" labels, the default history window, leaderboard windows, `.last-sync` ages, the sync commit message all derive from `time.Now` at the `cmd/tu` edge — ten call sites in `src/go/cmd/tu/main.go`). Chosen: one `now()` function in `cmd/tu/main.go` that reads **`TUDIFF_NOW`** (a zone-less local timestamp such as `2026-09-26T12:00:00`, parsed in the process's `TZ`) and falls back to `time.Now`. One binary, one code path; harness-namespaced; not documented in `docs/specs/usage.md`. Rejected: a `//go:build harness` tagged binary (two build paths, harness stops testing the exact `bin/tu`), libfaketime (absent on CI), normalising date labels in captures (the default history window changes which rows exist as months pass).
2. **Version anchor** — `package.json` is deleted, and it is today's single version anchor (justfile `go_version`, `scripts/release.sh` via `npm version`, `release.yml`'s `tag-on-release-merge` job and merge fallback, `scripts/package-go.sh`'s host smoke test). Chosen: **tag-driven like `hop`/`idea`** — `go_version` from `git describe --tags --always`, `scripts/release.sh` replaced by hop's tag-only script (computes the next tag from `git tag`, pushes it; no version commit, so no more `release: vX.Y.Z` commits), `release.yml` drops the never-used release-labeled-merge job and keeps the tag-push and `workflow_dispatch` paths, `ci.yml` checkouts get `fetch-depth: 0`. Rejected: a repo-root `VERSION` file (fab-kit shape) — smaller diff but diverges from the plan's stated intent and from the Go siblings.

Baseline measured on this branch before any edit (2026-09-25, dev-ws-sahil02, node v24.15.0, after `npm ci`): `just go-diff --placeholder` → **452 cases, 452 green** (single 182, multi 176, org 47, legacy 47; pipe 376, tty 76; tz fixed 385, alt 67); `just go-live` → **9 steps, 9 green**. The node-side captures in `bin/harness/report/cases/` are **1280 files, 3.8 MB** (`node.{stdout,stderr,exit}` per pipe case, `node.{tty,exit}` per tty case; largest single file 6.3 KB — the `skill` bundle). `--version` prints `tu version v0.12.2`; `help-dump` prints `"version": "0.12.2"` (bare).

## Why

**The Node tree exists today for exactly one consumer.** Since the cutover (plan row X1, v0.12.1 the first Go-only formula) `src/node/` no longer ships; it is kept as (a) the differential harness's oracle — `cmd/tudiff` byte-diffs `node dist/tu.mjs` against `bin/tu` on every PR — and (b) the D10 rollback build. Two releases have now shipped past cutover (v0.12.1, v0.12.2) with no rollback, which is the D10 condition for removal. Rollback remains possible without the tree: every pre-Z1 tag carries `src/node/` in its own history, and the tap can be pointed at such a tag (a higher-numbered Node release per the go-release-pipeline memory) without anything in `main`.

**What it costs to keep.** Every PR runs `npm ci` twice (the `build-and-test` lane and the `tudiff` lane) and `release.yml` runs `npm ci && npm run build` before any Go step. The repo carries a 5-package `devDependencies` set, a lockfile, `tsconfig.json`, and the esbuild bundle script; `fab/project/config.yaml` still lists `src/node/` in `source_paths` and `**/*.spec.ts` in `test_paths`; ~15 memory files justify byte-level decisions with "parity with the frozen `src/node/` oracle (until plan row Z1)". The constitution (v2.0.0) already does not govern the tree. Leaving it is dual-toolchain drag with no remaining purpose once the oracle is replaced.

**Why golden captures, not a tarball oracle.** Plan row Z1 offered two oracle replacements: keep running the last TS release tarball, or reduce `tudiff` to golden-file regression. A tarball oracle still needs `node` on every CI runner and developer machine — the thing being removed. Frozen goldens make the harness self-contained: the bytes the TS implementation produced for the whole matrix are committed once (placeholder corpus only — D7's "real captures are never committed" holds), and from then on the gate is "the Go binary still produces those bytes". This is the same `testdata/*.golden` + `-update` discipline the constitution already mandates for the render encoders, lifted to the binary level. A golden change is an output change and falls under Output Stability (minor bump), exactly as a render golden change does today.

**Why the freeze must precede the delete, in the same change.** The goldens can only be captured while the Node oracle still runs. The order is therefore: (1) golden mode lands in `tudiff` with both sides still present, (2) the goldens are captured from Node and committed, (3) `tudiff` is switched to golden comparison and the full matrix plus live sequence are green against them, (4) the Node tree and toolchain are deleted. One PR, sequenced commits; the capture commit is the provenance.

## What Changes

### 1. Golden corpus — `harness/golden/` (new, committed)

A new committed tree beside `harness/matrix.json`:

```
harness/golden/
  manifest.json                     # provenance + the pinned clock + matrix hash
  run/<case ID as nested dirs>/     # one dir per expanded matrix case (452 today)
    stdout  stderr  exit            # pipe cases   (bytes as Compare compares them)
    tty     exit                    # tty cases
    tree.json                       # written metrics-repo tree (multi/org/legacy/envrepo homes)
  live/<step>/                      # the nine tudiff live steps (sync-dry-run … cc-sync)
    stdout  stderr  exit  tree.json
    log.txt                         # `git log --format=%H%n%s -p main` where the step checks it
    status.txt                      # `git status --porcelain` where the step checks it
  live/repair-dry-run/  live/repair-write/
    stdout  stderr  exit  tree.json # the repair oracle's output and repo tree
```

- **Byte channels are stored exactly as `harness.Compare` sees them**: home-normalised (`NormalizeHome`, the side's staged `$HOME` → literal `$HOME`) **and version-normalised** (new: `NormalizeVersion` replaces the binary's own version — probed once per run via `<bin> --version`, e.g. `v0.12.2`, and its bare form `0.12.2` — with the literal `$VERSION`, longest form first). Version normalisation applies to both the golden at capture time and the Go capture at compare time, so `--version`, `-v` and `help-dump` goldens hold across releases while every other byte stays exact. The `exit` file is the decimal exit code.
- **`tree.json`** pins the written metrics repo without committing ~250 copies of the seed: `{"files": {"<relpath>": {"sha256": "…", "size": N}, …}, "last_sync": true|false}` over `<home>/.tu/metrics_repo/` (paths and bytes; nothing under `.tu/cache/`), plus the presence of `<home>/.tu/.last-sync` — the same three things `CompareTrees` compares today. A tree mismatch reddens the case on channel `tree` naming the first differing path; the report writes the Go side's tree files for that case so the bytes can be inspected without re-running.
- **`manifest.json`** (schema 1): `oracle` (`"node dist/tu.mjs"`), `oracle_version` (`"v0.12.2"`), `node_version` (`"v24.15.0"`), `captured_at` (UTC RFC3339), `now` (the pinned zone-less local timestamp, see § 2), `script` flavour (`util-linux`), `platform` (`linux/amd64`), `fixtures` (`["_placeholder"]`), `matrix_sha256` (of `harness/matrix.json`), `cases` (452), `live_steps` (11).
- Size budget: ~1300 small files, ~4 MB (measured from today's node captures; git compresses the ANSI tables well). Real usage data never enters: capture forces the `_placeholder` corpus and rejects `--fixtures` (the `.gitignore` rule for `harness/fixtures/*/` is unchanged).

### 2. Clock seam — `TUDIFF_NOW` in `cmd/tu`

`src/go/cmd/tu/main.go` gains one function replacing every direct `time.Now` use at the edge (the `Deps.Now`, `MetricsDirGuard`, `Normalize`, `Status`, `LastSync`, sync `Now`, watch `Now`, and rain seed sites — ten today):

```go
// now is the edge clock. TUDIFF_NOW (a zone-less local timestamp,
// "2006-01-02T15:04:05", read in the process TZ) pins it for the
// differential harness's golden runs; unset or malformed → time.Now.
func now() time.Time {
    if v := os.Getenv("TUDIFF_NOW"); v != "" {
        if t, err := time.ParseInLocation("2006-01-02T15:04:05", v, time.Local); err == nil {
            return t
        }
    }
    return time.Now()
}
```

- Read at call time, never in `init()` (Constitution IV). The rain seed keeps `time.Now().UnixNano()` (a PRNG seed, never a compared surface — watch sessions are not in the matrix).
- Zone-less on purpose: the matrix's `tz` axis runs cases under `TZ=UTC` and `TZ=Asia/Kolkata`; one pinned wall-clock value yields the same local calendar date in both, which is what the captured Node run had (Node ran on real time; capture is guarded so its calendar date agreed in both zones, § 3). Noon keeps `CommitMessage`'s `now.UTC()` date on the same day for both zones.
- `tudiff run` and `tudiff live` set `TUDIFF_NOW=<manifest.now>` in the Go side's environment (`harness.BuildEnv` gains the field); nothing else ever sets it. Not documented in `docs/specs/usage.md` or `--help`; documented in the harness memory as a harness seam like `TUDIFF_FIXTURES`/`TUDIFF_CALL_LOG` (which the fakes read).
- Cache TTL and `.last-sync` ages are computed against the same pinned instant they were written with, which is behaviourally what a sub-60 s real run produced (`<1m ago`, cache fresh).

### 3. `tudiff` golden mode — `run`, `live`, `--update`, and the one-time Node capture

**End state of `src/go/cmd/tudiff/run.go`** (`runRun`):

- Flags: `--matrix`, `--expected`, `--go` (`bin/tu`), `--harness-bin`, `--fixtures`, `--placeholder`, `--report`, `--filter`, `--list`, `--jobs`, `--timeout` unchanged; **`--node` removed**; new **`--golden`** (default `harness/golden`, repo-root-relative like the other defaults) and **`--update`** (rewrite the goldens from the Go side instead of comparing; mutually exclusive with `--filter`-less runs only in that a filtered `--update` rewrites just the matched cases — same as `go test ./pkg -run X -update`).
- Preflight (exit 2, one `tudiff: <reason>` line, same order discipline as today): the node-bundle and `node`-on-PATH checks are **gone**; new checks: `harness/golden/manifest.json` present and valid (`tudiff: harness/golden/manifest.json not found (run tudiff run --update)`), `matrix_sha256` equal to the current `harness/matrix.json` hash unless `--update` (`tudiff: harness/matrix.json changed since the goldens were captured (run tudiff run --update and review the diff)`), and — when not `--update` — every expanded (filtered) case has a golden dir (`tudiff: no golden for <case ID> (run tudiff run --update)`).
- Per case: stage **one** `$HOME` (the Go side; `stageCase` stops staging a node side), run the Go binary exactly as today (pipe or `script(1)` tty, `--timeout`, date-rollover re-run kept — the pinned clock makes it moot but the guard is cheap and stays), then compare against the golden with the existing `Compare`/`firstDivergence` machinery by loading the golden channels into a `SideCapture` in the oracle position (the golden side is the line-numbering reference, as the node side was), and `CompareTrees` against `tree.json`.
- **Call log**: the node-vs-go call-set comparison (`CompareCallLogs`, `[calls differ: …]`, `NodeCalls`) is **dropped** — it was informational and its oracle half no longer exists. The Go call log stays: `UnconfirmedReplays` still reads it for the gate rule, and `go.calls.jsonl` is still written to the report.
- Report: header line `node: <path> (<node --version>)` becomes `golden: harness/golden (captured <captured_at> from <oracle> <oracle_version>; now <now>)`; case lines and summary keep their shape with `node=` renamed `golden=` in the red-line detail (`exit: golden=<n> go=<n>`, `<channel> @<off> (line <l>): golden=<q> go=<q>`, `tree: golden=… go=…`); `report.json` renames `node`/`node_version`/`node_excerpt`/`node_exit`/`node_ms`/`node_calls`/`calls_differ` accordingly (`golden`, `golden_excerpt`, `golden_exit`; `node_ms`, `node_calls`, `calls_differ` removed). `cases/<id>/` holds `go.{stdout,stderr,exit}` or `go.{tty,exit}` plus `go.calls.jsonl`, and — on a `tree` red — `go.tree/…`. The gate rule (exit 1 iff unexpected red, timeout, unconfirmed replay, or stale expected entry) is unchanged; `harness/expected-diffs.json` keeps its role and match rule.
- `--update`: runs the Go side over the (filtered) matrix, writes `run/<case>/…` and `tree.json`, rewrites `manifest.json` with `oracle: "bin/tu"`, `oracle_version` from `bin/tu --version`, a fresh `captured_at` and `matrix_sha256`, and **keeps `now` unchanged** (the goldens stay pinned to the original capture date; `--now <ts>` overrides it explicitly). Exit 0 after writing; prints `tudiff: wrote <n> goldens under harness/golden/run (now <now>)`. A golden diff is reviewed in the PR like any render golden.

**End state of `src/go/cmd/tudiff/live.go`** (`runLive`): same treatment — `--node` gone, `--golden`/`--update` added, one side staged (one bare remote, one clone, one home), each of the nine steps compared against `live/<step>/` (stdout/stderr/exit via `Compare`, trees via `tree.json`, `log.txt` and `status.txt` via the existing `checkEqual` text comparison where the step checks them), the repair flow compares `bin/turepair --repo <B>` dry-run and `--write` output and trees against `live/repair-dry-run/` and `live/repair-write/` (repo path replaced with `$REPO` as today). The pinned git identity and fixed `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` stay, which is what makes `log.txt` (hashes included) reproducible.

**Transitional capture (first commits of this PR, removed before merge):** with `src/node/` still present, `run --update` and `live --update` accept a temporary **`--from-node <bundle>`** flag that captures the goldens from the staged Node oracle (existing `StageOracle` + `execSide(node=true)` code) instead of the Go binary, with a capture guard: refuse (exit 2) unless the local calendar date is identical under `TZ=UTC` and `TZ=Asia/Kolkata` at the start and again at the end of the capture (Node runs on real time; the pinned `now` written to the manifest is `<that date>T12:00:00`). The capture commits `harness/golden/` produced by `just go-diff --placeholder --update --from-node dist/tu.mjs` and `just go-live --update --from-node dist/tu.mjs` run on Linux (util-linux `script`). After the goldens are committed and `tudiff run`/`live` are green against them **without** `--from-node`, the flag, `StageOracle`, the node exec branch, and `harness.SideNode` are deleted in the same PR along with `src/node/`. The removal commit is what proves the harness no longer needs `node`.

### 4. Version anchor — `git describe`, tag-only release

- `justfile`: `go_version := \`git describe --tags --always 2>/dev/null || echo dev\`` (hop's expression); `go-build` and `go-build-target` stamp `-X main.version={{go_version}}` (the tag already carries the `v`; today's `v{{go_version}}` prefix goes). `cmd/tu/main.go`'s version comment (line ~56) updated.
- `scripts/release.sh`: replaced by hop's tag-only script verbatim (same `patch|minor|major` usage, same clean-tree and detached-HEAD refusals, computes the next tag from `git tag -l 'v*' --sort=-v:refname`, tags, pushes the tag only). The justfile `release` recipe comment updated to hop's wording.
- `.github/workflows/release.yml`: the `tag-on-release-merge` job and the `push: branches: [main]` trigger are removed (no merged PR has ever carried the `release` label; releases are `just release` tag pushes — v0.11.3 … v0.12.2 are all `release:` commits from `release.sh`). The `release` job keeps the two entry paths (tag push, `workflow_dispatch` → `scripts/release.sh` then read the tag) and its checkout `ref` expression drops the merge branch; `setup-node`, `Install dependencies` (`npm ci`), `Build bundle` (`npm run build`), and the `node -p` version fallback go; the version step becomes hop's `Extract version from tag`. The fail-loud-first ordering (Go build/package/formula before `gh release create`, tap push last) and the tap step are unchanged. Header comments rewritten (no Node rollback-build prose; the rollback note in the tap step now says: point the tap at a pre-Z1 tag's Node formula for a higher-numbered release, since `src/node/` lives only in those tags' history).
- `.github/workflows/ci.yml`: `build-and-test` lane deleted; `go-build-and-test` and `tudiff` checkouts get `fetch-depth: 0` (tags for `git describe`); the `tudiff` job drops `setup-node` and `npm ci` and runs `just go-diff --placeholder` then `just go-live`; `ci-gate` `needs: [go-build-and-test, tudiff]` with the two checks; header comment rewritten to two lanes. `scripts/ci-gate-ruleset.sh` is untouched (the required context is still `ci-gate`).
- `scripts/package-go.sh`: the `CCUSAGE_VERSION` vs `package-lock.json` guard is **deleted** (`CCUSAGE_VERSION` is the only pin now — the memory already says so); the host smoke test's `want` becomes `tu version $(git describe --tags --always)` (same expression the justfile stamped, so it agrees by construction; at release the checkout is the tag so it is `tu version vX.Y.Z`). The curl fetch from the npm registry stays exactly as is.
- `scripts/go-formula.sh` and `scripts/release-notes.sh` already use `git describe` — unchanged.

### 5. Deletions and toolchain cleanup

Deleted: `src/node/` (entire tree), `package.json`, `package-lock.json`, `tsconfig.json`, `scripts/build.sh`, `scripts/help-dump.mjs`, `scripts/repair-metrics.mjs` (its Go port `cmd/turepair` is the constitution-named maintainer tool and the live harness's repair goldens are frozen from the `.mjs` output first — see Assumptions), the old `scripts/release.sh` body (replaced, § 4).

Edited:
- `justfile`: `setup`, `test`, `run`, `build` recipes removed; `go-diff` and `go-live` lose the `build` dependency (`go-diff *ARGS: go-build harness-build`); the "Go successor" banner comment and the `go_version` comment rewritten; `harness-build` unchanged. `just` recipe list is Go-only.
- `.gitignore`: `node_modules/`, `*.tsbuildinfo` lines and the `help/` block's `npm run help-dump` prose removed (`help/` itself stays ignored — `tu help-dump > help/tu.json` is still a local convenience); `dist/` and `bin/` stay.
- `README.md` § CI / branch protection: two lanes, and the local-reproduce block becomes `just go-lint && just go-build && just go-test` plus `just go-diff --placeholder && just go-live`.
- `fab/project/config.yaml`: `source_paths` → `src/go/` only; `test_paths` → `**/*_test.go` only. `fab/project/context.md`: the "Retired TypeScript tree" section removed; Stack lines unchanged. `fab/project/code-review.md`: review-scope line drops `dist/` (generated bundle) and `node_modules/`; the stale `UsageEntry[]` rule becomes "New data sources MUST produce `[]fact.Record` and flow through `query`" (Constitution V wording).
- `src/go/internal/harness/capture.go`: `FindRepoRoot` walks up to the **`justfile`** instead of `package.json`; `tudiff capture`'s ccusage resolution drops the `node_modules/@ccusage/…` probe and its `run npm ci` hint — order becomes `--ccusage` flag → `dist/vendor/ccusage/bin/ccusage` → the brew-installed `vendor/ccusage/bin/ccusage` beside the `tu` on `PATH` (via `ccusage.ResolveBinary`) → bare `ccusage` on `PATH`; `nodePlatformArch` goes; `main.go`'s `--ccusage` help text and `usageText` (`run  byte-diff …` line) updated. `manifest_test.go`/`capture_test.go` fixtures that spell `node_modules/…` paths change to `dist/vendor/…`.
- Go tests that walk up to `package.json` to find the repo root — `config/defaults_test.go`, `source/metrics/metrics_test.go`, `toolkit/skill_test.go`, `toolkit/helpdump_test.go`, `cmd/tudiff/{run,live,main}_test.go` — walk up to `justfile`; `toolkit/helpdump_test.go`'s transitional `TestDescriptionMatchesPackage` (pins `Description` against `package.json`) is deleted (the constant is the long-term shape, per its own comment); `toolkit/version_test.go`'s comment no longer cites the TS.
- `src/go/cmd/turepair/main.go`: `usageLine` becomes `Usage: turepair [--repo <path>] [--write]` (the `node scripts/…` spelling was kept only for oracle parity; turepair is a maintainer tool outside the Goal's surfaces). Its test updated.
- Source comments that cite `src/node/*.ts` files as the origin of a byte surface (`internal/sync/{report,git,flow,localecmp}.go`, `internal/config/guard.go`, `internal/command/request.go`, `internal/toolkit/shellinit.go`, `internal/watch/terminal.go`, `cmd/tu/main.go`, `internal/render/{format,csv}.go`, `internal/source/ccusage/{exec,registry}.go`) keep their behavioural content but are reworded to cite the frozen goldens / "the retired TS implementation" rather than a file path that no longer exists. No behaviour changes in any of these packages.

Unchanged on purpose: `CCUSAGE_VERSION`, the curl-based ccusage vendor fetch, `.github/formula-template.rb`, `scripts/{go-formula,release-notes,dogfood-install,dogfood-uninstall,sync-skill,ci-gate-ruleset}.sh`, `harness/{matrix,expected-diffs}.json`, `harness/fixtures/_placeholder/`, `harness/metrics-repo/`, every external surface in the plan's Goal (grammar, `--help`, output formats, exit codes, conf files, metrics layout, toolkit contracts).

### 6. Plan bookkeeping

At ship, `fab/plans/sahil/26-09-15-go-port.md` row Z1 gets the PR link and `landed`, the header Status line notes Z1 done and Z2 (backlog re-triage) remaining, and D10/D13's "kept buildable until Z1" prose is annotated as fulfilled. Z2 (re-pointing backlog rows that cite `src/node/core/*.ts`) is **not** part of this change.

## Affected Memory

- `harness/differential-harness`: (modify) oracle → committed goldens; `--golden`/`--update`, preflight set, `TUDIFF_NOW`, report header/case-line/`report.json` renames, call-log comparison removed, gate rule unchanged
- `harness/golden-corpus`: (new) `harness/golden/` layout, `manifest.json` fields, `tree.json` shape, `$VERSION`/`$HOME` normalisation, the pinned clock, the `--update` refresh discipline and its Output Stability consequence, capture provenance (Node v0.12.2, 2026-09, Linux/util-linux)
- `harness/comparison-and-live`: (modify) golden side in the oracle position, `tree.json` comparison, `live` against `live/<step>/` goldens, repair goldens vs `bin/turepair`, call-log design decision retired
- `harness/matrix-and-staging`: (modify) one staged home per case, no oracle staging (`StageOracle` gone), `BuildEnv` carries `TUDIFF_NOW`
- `harness/replayers`: (modify) the "fake ccusage staged into the oracle's vendor slot" paragraph removed
- `harness/fixture-corpus`: (modify) `tudiff capture` ccusage resolution order (no `node_modules`), `FindRepoRoot` marker is `justfile`, `main.go` dispatch text
- `build/toolchain`: (modify) two CI lanes with `fetch-depth: 0`, `go_version` from `git describe`, tag-only `release.sh`, `go-diff`/`go-live` without `build`, the "Retired src/node tree" section and the `package.json`-anchor design decision replaced by a `git describe` decision, node-era `just` recipes gone
- `build/go-release-pipeline`: (modify) lockfile guard deleted, `release.yml` without setup-node/npm and without `tag-on-release-merge`, smoke test wants the describe string, rollback requirement reworded to "a pre-Z1 tag's Node formula"
- `toolkit/skill-bundle`: (modify) drift-guard test walks up to `justfile`
- `toolkit/version-and-help-dump`: (modify) `TestDescriptionMatchesPackage` removed; version comes from the tag
- `sync/repair`: (modify) `turepair` usage line; the `.mjs` oracle reference and its parity rationale
- `command/multi-mode`, `config/cascade`, `config/metrics-dir-guard`, `config/status`, `render/csv-and-markdown`, `render/json`, `render/number-formatting`, `source/ccusage-adapter`, `source/ccusage-json-shapes`, `source/errors-and-warnings`, `source/metrics-reader`, `sync/day-file-writer`, `sync/dry-run-report`, `sync/git-flow`, `query/aggregation`: (modify) the "parity with the frozen `src/node/` oracle (until plan row Z1)" rationale lines reworded to "parity with the frozen golden corpus (`harness/golden/`, the retired TS implementation's bytes)" — rationale only, no requirement changes

## Impact

- **Go code**: `src/go/cmd/tudiff/{run,live,main}.go` (golden mode, flags, preflight, one-sided staging, `--update`, temporary `--from-node`), `src/go/internal/harness/{diff,report,capture,homes}.go` (+ new `golden.go`: manifest, tree.json, load/write, `NormalizeVersion`), `src/go/cmd/tu/main.go` (`now()` seam), `src/go/cmd/turepair/main.go` (usage line), test files that locate the repo root, comment-only touches in ~12 files. `just go-lint` and `just go-test` stay the gate; new unit tests for golden load/write/compare, `NormalizeVersion`, manifest validation, matrix-hash preflight, `TUDIFF_NOW` parsing, and the `--update` writer sit beside the code as `_test.go` siblings.
- **Committed data**: `harness/golden/` (~1300 files, ~4 MB).
- **Build/CI/release**: `justfile`, `ci.yml`, `release.yml`, `scripts/release.sh`, `scripts/package-go.sh`, `.gitignore`, `README.md`.
- **Deleted**: `src/node/` (the TypeScript implementation and its `__tests__/`), `package.json`, `package-lock.json`, `tsconfig.json`, `scripts/{build.sh,help-dump.mjs,repair-metrics.mjs}`.
- **Process**: `just release` no longer commits; the version is the tag. `fab/project/config.yaml` scopes fab to `src/go/` only.
- **Unaffected**: every shipped surface; the formula and tap; `CCUSAGE_VERSION` and the vendor fetch; real usage captures (still gitignored, still usable via `--fixtures` for a local `tudiff run` — the goldens are placeholder-only, so a `--fixtures <machine>` run compares real-fixture Go output against placeholder goldens and is meaningless; `run` rejects `--fixtures` unless `--update`... see Assumptions #14).

## Open Questions

- None blocking. Two were asked and answered at intake (clock seam, version anchor — recorded in Origin and Assumptions).

## Clarifications

### Session 2026-09-25 (bulk confirm)

| # | Action | Detail |
|---|--------|--------|
| 3 | Confirmed | — |
| 4 | Confirmed | — |
| 5 | Confirmed | — |
| 6 | Confirmed | — |
| 7 | Confirmed | — |
| 8 | Confirmed | — |
| 9 | Confirmed | — |
| 10 | Confirmed | — |
| 11 | Confirmed | — |
| 14 | Confirmed | — |
| 16 | Confirmed | — |
| 18 | Confirmed | — |

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Golden-file regression replaces the Node oracle: placeholder-only goldens captured once from `node dist/tu.mjs` at v0.12.2 and committed under `harness/golden/`; the freeze precedes the delete inside this one change | Stated verbatim in the invocation; plan row Z1 lists it as one of its two options and the tarball option would keep `node` on CI | S:95 R:70 A:90 D:95 |
| 2 | Certain | Clock seam is a `TUDIFF_NOW` env read in one `now()` func at the `cmd/tu` edge (zone-less local timestamp, fallback `time.Now`), set by `tudiff run`/`live` from `manifest.now` for the Go side; not documented in usage.md | Asked — user chose the recommended option over a build-tagged binary; ten `time.Now` sites at the edge, none in `internal/`; code-quality "minimum pathways" | S:90 R:70 A:85 D:90 |
| 3 | Confident | Version anchor is the git tag: `go_version` from `git describe --tags --always`, hop's tag-only `release.sh`, `release.yml` drops `tag-on-release-merge` and the main-push trigger, `ci.yml` checkouts use `fetch-depth: 0`, `package-go.sh` smoke test wants the describe string | Clarified — user confirmed. Asked — user chose; plan Z1, the justfile comment and build memory all pre-announce `git describe`; no merged PR ever carried the `release` label; matches hop/idea | S:95 R:60 A:85 D:85 |
| 4 | Certain | `$VERSION` normalisation: the probed `--version` value (`v0.12.2`) and its bare form (`0.12.2`) are replaced, longest first, in every compared byte channel on both sides | Clarified — user confirmed. `--version` prints `tu version v0.12.2` and `help-dump` prints `"version": "0.12.2"` (measured); mirrors `NormalizeHome`; without it the three version cases would go red on the first tag after capture | S:95 R:85 A:80 D:75 |
| 5 | Confident | `scripts/repair-metrics.mjs` is deleted with the tree; `bin/turepair` is the repair tool and its usage line drops the `node scripts/…` spelling; the live repair goldens are frozen from the `.mjs` output first | Clarified — user confirmed. The invocation's delete list omits it, but it is a plain-`node` script whose Go port the constitution names; keeping it would keep a `node` dependency in `tudiff live` | S:95 R:85 A:60 D:55 |
| 6 | Confident | Golden encoding: byte channels verbatim per case; the written metrics-repo tree as `tree.json` (relpath → sha256 + size) plus a `last_sync` flag; live steps additionally store `log.txt`/`status.txt` | Clarified — user confirmed. Verbatim trees would commit ~250 copies of the seed; sha256 pins bytes and paths exactly as `CompareTrees` does; the report writes the Go tree on a red for inspection | S:95 R:80 A:70 D:60 |
| 7 | Confident | Goldens are captured on Linux with util-linux `script`; the manifest records the flavour; a BSD-flavour host running tty cases reports reds like any other divergence (no special-casing) | Clarified — user confirmed. CI is ubuntu-latest and the capture box is Linux; today both sides run under the same flavour so the risk is new but confined to local macOS runs of 76 tty cases | S:95 R:75 A:65 D:55 |
| 8 | Certain | The Node capture arm (`--from-node`, `StageOracle`, node exec, `SideNode`) is deleted in the same PR once the goldens are committed and green; `--update` from the Go binary is the permanent refresh path (a golden change is an Output Stability event) | Clarified — user confirmed. Dead code after the capture; the constitution's `-update` convention; keeps `tudiff` free of any `node` reference in the merged tree | S:95 R:80 A:80 D:70 |
| 9 | Certain | Repo-root marker for `FindRepoRoot` and the test walk-ups becomes `justfile` | Clarified — user confirmed. Unique, always present at the root, already the single build definition; `go.mod` sits two levels down | S:95 R:90 A:75 D:60 |
| 10 | Confident | Matrix drift guard: `manifest.matrix_sha256` must equal the current `harness/matrix.json` hash or `run` exits 2 (unless `--update`); a missing golden for an expanded case is also exit 2 | Clarified — user confirmed. The harness never reports a vacuous green (existing design decision); a matrix edit must regenerate goldens, and the regenerated diff is reviewed in the PR | S:95 R:85 A:70 D:65 |
| 11 | Certain | The node-vs-go call-log comparison is dropped; the Go call log stays for the unconfirmed-replay gate and the report | Clarified — user confirmed. It was informational only and its oracle half is gone; the gate rule is unchanged | S:95 R:85 A:80 D:70 |
| 12 | Certain | Real usage captures never enter the goldens: capture/`--update` force `_placeholder` and reject `--fixtures`; the `harness/fixtures/*/` gitignore rule is unchanged | D7 and the invocation's "placeholder fixtures only, no real usage data" | S:95 R:80 A:95 D:95 |
| 13 | Certain | No external surface in the plan's Goal changes; the only shipped-binary code change is the `now()` seam, and `turepair`'s usage line is a maintainer-tool change | Invocation constraint; the 452-green golden run after the switch is the proof | S:90 R:70 A:90 D:90 |
| 14 | Confident | `tudiff run --fixtures <alias>` (real captures) is rejected in golden mode with exit 2 (`tudiff: --fixtures needs a Node oracle; goldens are placeholder-only`) unless `--update --golden <other dir>` targets a non-default, gitignored golden dir | Clarified — user confirmed. A real-fixture Go run against placeholder goldens is meaningless; keeping a local golden set for real captures is possible but out of scope | S:95 R:85 A:70 D:55 |
| 15 | Certain | `change_type` pinned to `chore` (refresh inferred `feat`) | A retirement/removal with no new user-facing behaviour; per the g1-rework memory note, pin explicitly so refresh cannot re-infer | S:60 R:95 A:85 D:80 |
| 16 | Certain | The ~15 memory rationale lines reading "parity with the frozen `src/node/` oracle (until plan row Z1)" are reworded at hydrate to cite the golden corpus; requirements untouched | Clarified — user confirmed. Rationale-only drift; leaving them would point at a directory that no longer exists | S:95 R:90 A:80 D:70 |
| 17 | Certain | `ci.yml` keeps two lanes (`go-build-and-test`, `tudiff`) aggregated by `ci-gate`; `README.md`'s CI prose and local-reproduce block follow; `scripts/ci-gate-ruleset.sh` is untouched | Mechanical consequence of deleting the Node lane; the required check name does not change | S:85 R:85 A:90 D:90 |
| 18 | Confident | Capture-date guard for the one-time Node freeze: refuse unless the local calendar date agrees under `TZ=UTC` and `TZ=Asia/Kolkata` at start and end; `manifest.now` is `<date>T12:00:00` | Clarified — user confirmed. Node runs on real time and the tz axis needs one pinned wall-clock value that lands on the same date in both zones; `CommitMessage` uses the UTC date | S:95 R:80 A:65 D:55 |
| 19 | Certain | `CCUSAGE_VERSION` becomes the only ccusage pin; the lockfile guard in `package-go.sh` is deleted; the curl fetch stays | Invocation says the vendor fetch stays; the go-release-pipeline memory already states the guard is deleted "after the lockfile is removed (plan row Z1)" | S:90 R:90 A:95 D:95 |

19 assumptions (12 certain, 7 confident, 0 tentative, 0 unresolved).
