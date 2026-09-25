# Plan: Remove src/node — golden-file harness, Node toolchain retired

**Change**: 260925-6wpm-remove-src-node
**Intake**: `intake.md`

> Read `intake.md` first — it carries the full design, the measured baseline (452/452 run, 9/9 live, node captures 1280 files / 3.8 MB), and the two user decisions (clock seam = `TUDIFF_NOW`; version anchor = git tag). This plan sequences that design. The order of the phases is load-bearing: the goldens can only be captured while `src/node/` still exists, so Phase 2 (golden mode + capture) MUST be complete and green before Phase 3 (removal) starts.

## Requirements

### cmd/tu: The edge clock seam

#### R1: `TUDIFF_NOW` pins the edge clock
`src/go/cmd/tu/main.go` MUST route every edge use of `time.Now` (the `Deps.Now`, `MetricsDirGuard`, `command.Normalize`, `config.Status`, `config.LastSync`, sync `Now`, and watch `Now` sites) through one unexported `now()` function that returns `time.ParseInLocation("2006-01-02T15:04:05", os.Getenv("TUDIFF_NOW"), time.Local)` when the variable is set and parses, and `time.Now()` otherwise. The rain PRNG seed MAY keep `time.Now().UnixNano()`. The variable MUST NOT be read in `init()` or a package-level initializer, MUST NOT appear in `--help`, `docs/specs/usage.md`, or `docs/site/skill.md`, and MUST NOT be read anywhere under `internal/`.

- **GIVEN** `TUDIFF_NOW=2026-09-26T12:00:00` and `TZ=Asia/Kolkata`
- **WHEN** `tu` runs a snapshot
- **THEN** `query.CurrentLabel` sees local date `2026-09-26`, and the same binary under `TZ=UTC` sees `2026-09-26` too

- **GIVEN** `TUDIFF_NOW` unset or malformed (`banana`)
- **WHEN** `tu` runs
- **THEN** behaviour is identical to today (`time.Now`)

### harness: The golden corpus

#### R2: `harness/golden/` layout and normalisation
The committed golden corpus lives at `harness/golden/` with: `manifest.json` (schema 1; fields `oracle`, `oracle_version`, `node_version` (empty when captured from Go), `captured_at` RFC3339 UTC, `now` (the pinned zone-less timestamp), `script` flavour, `platform` `<GOOS>/<GOARCH>`, `fixtures` `["_placeholder"]`, `matrix_sha256` (hex sha256 of `harness/matrix.json` bytes), `cases`, `live_steps`); `run/<case ID as nested dirs>/` holding `stdout`, `stderr`, `exit` for pipe cases or `tty`, `exit` for tty cases, plus `tree.json`; `live/<step>/` holding `stdout`, `stderr`, `exit`, `tree.json` and, where the step checks them today, `log.txt` and `status.txt`. Byte channels are stored **after** `NormalizeHome` and after the new `NormalizeVersion(b, version)` — which replaces the probed version string (the last field of `<bin> --version`, e.g. `v0.12.2`) and its `v`-stripped form with the literal `$VERSION`, longest form first — and after `NormalizeIdentity(b, hostname, username)`, which replaces the capturing machine's `os.Hostname()` with `$MACHINE` and its username (derived exactly as `cmd/tu` derives it for the `$USER` conf sentinel) with `$USER`, hostname first, whole-token matches only (a token boundary is any byte outside `[A-Za-z0-9_-]`, plus start/end), so `machine_dev-ws-sahil02_cost` → `machine_$MACHINE_cost` and a path segment `sahil/` → `$USER/`. `tree.json` path keys receive the same identity normalisation (day-file contents carry no identity, so hashes are unaffected). Both normalisations run at capture (`--update`) and at compare, on the Go side's probed values, so the corpus holds no real hostname or username. `exit` holds the decimal code. `tree.json` is `{"files": {"<relpath>": {"sha256": "<hex>", "size": N}}, "last_sync": bool}` over `<home>/.tu/metrics_repo/` (nothing under `.tu/cache/`) plus the presence of `<home>/.tu/.last-sync`; an absent metrics repo is `{"files": {}, "last_sync": false}`. All of this lives in a new `src/go/internal/harness/golden.go` with `_test.go` siblings; it MUST be stdlib-only.

- **GIVEN** a Go capture whose stdout is `tu version v0.12.3-2-gabcdef\n` and a probed version `v0.12.3-2-gabcdef`
- **WHEN** normalised
- **THEN** the bytes are `tu version $VERSION\n`; a help-dump body containing `"version": "0.12.3-2-gabcdef"` becomes `"version": "$VERSION"`

#### R3: `tudiff run` compares `bin/tu` against the goldens
`runRun` MUST drop the `--node` flag, the node-bundle and `node`-on-PATH preflight checks, `StageOracle`, the node exec branch, and the call-log comparison (`CompareCallLogs`, `NodeCalls`, `CallsDiffer`, the `[calls differ …]` annotation) — the Go call log is still written and still feeds `UnconfirmedReplays`. It MUST add `--golden <dir>` (default `harness/golden`, repo-root-relative) and `--update`. In compare mode the preflight MUST exit 2 with one `tudiff: <reason>` line when: the manifest is missing or invalid (`tudiff: harness/golden/manifest.json not found (run tudiff run --update)`), `matrix_sha256` differs from the current matrix (`tudiff: harness/matrix.json changed since the goldens were captured (run tudiff run --update and review the diff)`), any expanded (filtered) case has no golden dir (`tudiff: no golden for <id> (run tudiff run --update)`), or `--fixtures` is given (`tudiff: --fixtures needs an oracle; goldens are placeholder-only`) unless `--update` targets a non-default `--golden`. Fixture resolution in golden mode is always `_placeholder` only (no hostname alias). Per case the driver stages ONE home, runs the Go side with `TUDIFF_NOW=<manifest.now>` added by `BuildEnv` (new `EnvSpec.Now` field), then loads the golden channels into the oracle-position `SideCapture` and reuses `Compare` (the golden side stays the line-numbering reference) and compares the Go tree against `tree.json` (channel `tree`, first differing path in the excerpts). The report header line `node: …` becomes `golden: <dir> (captured <captured_at> from <oracle> <oracle_version>; now <now>)`; red-line detail uses `golden=`/`go=`; `report.json` renames `node`→`golden`, `node_version`→`golden_captured_at`, `node_excerpt`→`golden_excerpt`, `node_exit`→`golden_exit`, and drops `node_ms`, `node_calls`, `calls_differ`. `cases/<id>/` holds `go.{stdout,stderr,exit}` or `go.{tty,exit}`, `go.calls.jsonl`, and on a `tree` red the Go tree under `go.tree/`. The gate rule, `--expected` handling, `--list`, `--filter`, `--jobs`, `--timeout`, and the date-rollover re-run are unchanged. `--update` runs the Go side and writes `run/<id>/…` for every executed case, then rewrites `manifest.json` (`oracle: bin/tu` path as given, `oracle_version` from `bin/tu --version`, fresh `captured_at`/`matrix_sha256`/`cases`, `now` preserved from the existing manifest — or taken from `--now <ts>` — and `2026-09-26T12:00:00`-style noon-of-today when no manifest exists), printing `tudiff: wrote <n> goldens under <dir>/run (now <now>)` and exiting 0.

- **GIVEN** the committed goldens and a `bin/tu` built from HEAD
- **WHEN** `just go-diff --placeholder` runs
- **THEN** the summary is `452 cases — 452 green, 0 red (0 expected, 0 unexpected), 0 timeout` and the exit code is 0, with no `node` binary on `PATH`

- **GIVEN** `harness/matrix.json` edited after capture
- **WHEN** `tudiff run` runs without `--update`
- **THEN** stderr is the matrix-changed line and the exit code is 2

#### R4: `tudiff live` compares against `live/` goldens
`runLive` MUST get the same treatment: `--node` removed, `--golden`/`--update` added, one bare remote + one clone + one home, `TUDIFF_NOW` in the Go side's env, each of the nine steps compared against `live/<step>/` (stdout/stderr/exit via `Compare`; `tree.json`; `log.txt`/`status.txt`/`.last-sync` presence via the step's existing text checks — a step's golden stores exactly the artefacts that step checks today), and the repair flow comparing `bin/turepair --repo <B>` dry-run then `--write` output and trees against `live/repair-dry-run/` and `live/repair-write/` (repo path replaced with `$REPO` as today). The pinned git identity and fixed `GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` stay. The report lands under `bin/harness/report-live/` in the `run` report's shape.

- **GIVEN** the committed live goldens
- **WHEN** `just go-live` runs
- **THEN** the summary is `9 cases — 9 green …` (seven sync steps + two repair steps) and the exit code is 0

#### R5: One-time capture from the Node oracle (transitional)
While `src/node/` still exists, `run --update` and `live --update` MUST accept `--from-node <bundle>`: the goldens are captured from the staged Node oracle (existing `StageOracle` + node exec + `node scripts/repair-metrics.mjs` for the repair steps) instead of the Go binary, `--placeholder` is implied, and a capture guard refuses (exit 2, `tudiff: capture date differs between UTC and Asia/Kolkata — retry between 06:00 and 18:30 UTC`) unless the local calendar date is identical under `TZ=UTC` and `TZ=Asia/Kolkata` both before the first case and after the last; `manifest.now` is written as `<that date>T12:00:00`, `oracle` as `node <bundle>`, `oracle_version` from `node <bundle> --version`, `node_version` from `node --version`. Node captures are normalised with the node-probed version. The captured corpus is committed in its own commit before any deletion.

- **GIVEN** `dist/tu.mjs` built from this branch and `bin/tu` built from HEAD
- **WHEN** `bin/harness/tudiff run --update --from-node dist/tu.mjs` then `bin/harness/tudiff run` run
- **THEN** the second run is 452/452 green — the Go binary matches the Node bytes under the pinned clock

#### R6: The Node capture arm is deleted with the tree
After Phase 2 is green and committed, `--from-node`, `StageOracle`, `harness.SideNode`, the node exec branch in `run.go`/`live.go`, `CompareCallLogs` and its tests MUST be deleted. `tudiff`'s usage text and package comment MUST describe `run` as "byte-diff bin/tu against harness/golden over harness/matrix.json". No file under `src/go/` may reference `dist/tu.mjs`, `npm`, `node_modules`, `package.json`, or `src/node/` after this phase (comment citations of the retired TS implementation are reworded, e.g. "the retired TypeScript implementation's `formatDrySyncReport`").

- **GIVEN** the merged tree
- **WHEN** `git grep -nE 'tu\.mjs|node_modules|package\.json|src/node|\bnpm\b' -- src/go` runs
- **THEN** it prints nothing

### build: Version anchor and toolchain

#### R7: The git tag is the version anchor
`justfile` MUST define `go_version := \`git describe --tags --always 2>/dev/null || echo dev\`` and stamp `-X main.version={{go_version}}` (no `v` prefix added — the tag carries it). `scripts/release.sh` MUST become hop's tag-only script (`~/code/sahil87/hop/scripts/release.sh` verbatim, `hop`→`tu` in prose): clean-tree and on-a-branch checks, next tag computed from `git tag -l 'v*' --sort=-v:refname`, `git tag` + `git push origin <tag>`, no file edits, no commit. `scripts/package-go.sh` MUST drop the `package-lock.json` guard (`CCUSAGE_VERSION` is the only pin) and its host smoke test MUST expect `tu version $(git describe --tags --always)`. `.github/workflows/release.yml` MUST drop the `tag-on-release-merge` job, the `push: branches: [main]` trigger, `setup-node`, `npm ci`, `npm run build`, and the `node -p` fallback; keep the tag-push and `workflow_dispatch` paths (dispatch runs `scripts/release.sh "${{ inputs.bump }}"` then reads `git tag --sort=-v:refname | head -1`), the `fetch-depth: 0` checkout with `ref: ${{ github.event_name == 'workflow_dispatch' && 'main' || github.ref }}`, setup-go, setup-just, `just go-build-all`, `just go-package`, `just go-formula <tag>`, `just release-notes <tag>`, `gh release create`, and the tap step — in that fail-loud-first order. Header comments MUST be rewritten (no Node rollback-build prose; the tap step's rollback note says a rollback is a higher-numbered release whose formula points at a pre-Z1 tag, where `src/node/` still exists in history). `src/go/cmd/tu/main.go`'s `version` comment MUST say the stamp comes from `git describe`.

- **GIVEN** HEAD is tagged `v0.12.3`
- **WHEN** `just go-build && bin/tu --version` runs
- **THEN** it prints `tu version v0.12.3`; on an untagged commit it prints `tu version v0.12.2-<n>-g<sha>`

#### R8: CI without Node
`.github/workflows/ci.yml` MUST have exactly two lanes plus the gate: `go-build-and-test` (checkout with `fetch-depth: 0`, setup-go, setup-just, `just go-lint`, `just go-build`, `just go-build-all`, `just go-test`) and `tudiff` (checkout with `fetch-depth: 0`, setup-go, setup-just, `just go-diff --placeholder`, `just go-live`, the `tudiff-report` artifact upload with `if: always()`); `ci-gate` `needs: [go-build-and-test, tudiff]` and checks those two results. No `setup-node`, `npm`, or `node` anywhere in `.github/`. The header comment describes the two lanes. `justfile` MUST delete the `setup`, `test`, `run`, `build` recipes, drop the `build` dependency from `go-diff` and `go-live` (`go-diff *ARGS: go-build harness-build`), and rewrite the Go-successor banner and the `go_version`/`go-diff` comments (no `src/node`, `dist/tu.mjs`, or `package.json` mentions). `README.md` § CI / branch protection MUST describe two lanes and its reproduce block MUST be `just go-lint && just go-build && just go-test` and `just go-diff --placeholder && just go-live`. `.gitignore` MUST drop `node_modules/`, `*.tsbuildinfo`, and the `npm run help-dump`/`help-dump.mjs` prose (keep `help/`, `dist/`, `bin/`, the fixtures rule).

- **GIVEN** the merged tree
- **WHEN** `git grep -nE 'setup-node|\bnpm\b|\bnode\b' -- .github justfile scripts README.md .gitignore` runs
- **THEN** the only hits are `scripts/package-go.sh`'s npm-registry URL/arch-spelling comments and the ccusage `npm_pkg`/`npm_arch` variable names

### repo: Tree deletion and root marker

#### R9: `src/node/` and the Node toolchain are gone; the repo root is found by `justfile`
`src/node/`, `package.json`, `package-lock.json`, `tsconfig.json`, `scripts/build.sh`, `scripts/help-dump.mjs`, `scripts/repair-metrics.mjs` MUST be deleted (`git rm -r`). `harness.FindRepoRoot` MUST walk up to the directory containing `justfile` (error text `tudiff: no justfile found above <start>`); the walk-ups in `internal/config/defaults_test.go`, `internal/source/metrics/metrics_test.go`, `internal/toolkit/skill_test.go`, `internal/toolkit/helpdump_test.go`, and the temp-root fixtures in `cmd/tudiff/{run,live,main}_test.go` and `internal/harness/capture_test.go` MUST use `justfile`. `toolkit/helpdump_test.go`'s `TestDescriptionMatchesPackage` and its `findRepoRoot` helper MUST be deleted; `helpdump.go`'s `Description` comment no longer cites `package.json`. `tudiff capture`'s ccusage resolution MUST become `--ccusage` → `dist/vendor/ccusage/bin/ccusage` under the repo root → the `vendor/ccusage/bin/ccusage` beside the `tu` found on `PATH` (resolve symlinks, as `ccusage.ResolveBinary` does) → bare `ccusage` on `PATH`, with the miss error `tudiff: no ccusage binary found (pass --ccusage)`; `nodePlatformArch` and the `node_modules` probe are deleted; `manifest_test.go`/`capture_test.go` fixtures spell `dist/vendor/…`. `cmd/turepair/main.go`'s `usageLine` MUST be `Usage: turepair [--repo <path>] [--write]` (test updated). `fab/project/config.yaml` `source_paths` MUST be `src/go/` only and `test_paths` `**/*_test.go` only; `fab/project/context.md` MUST drop the "Retired TypeScript tree" section; `fab/project/code-review.md` review scope drops `dist/`/`node_modules/` and its first project rule reads `New data sources MUST produce \`[]fact.Record\` and flow through \`query\``.

- **GIVEN** the merged tree
- **WHEN** `just go-lint && just go-test` run
- **THEN** both exit 0 with no skipped-because-no-package.json tests

### plan: Bookkeeping

#### R10: The plan document reflects Z1
`fab/plans/sahil/26-09-15-go-port.md` row Z1 MUST carry `260925-6wpm-remove-src-node` in the PR column and status `in review` (ship replaces it with the PR link), and the header **Status** line MUST say Z1 is in review with Z2 the only remaining row.

- **GIVEN** the plan file after apply
- **WHEN** the Z1 row is read
- **THEN** it names this change and its status is no longer `not started`

### Non-Goals
- Z2 (re-pointing backlog rows that cite `src/node/core/*.ts`) — its own row.
- Any behaviour change to the shipped binary beyond the `now()` seam.
- A golden set for real (gitignored) captures.
- Rewording the ~15 memory rationale lines — hydrate's job (intake Affected Memory).

### Design Decisions

#### Frozen goldens over a tarball oracle
**Decision**: `tudiff` compares the Go binary against committed byte captures; the Node oracle is captured once and retired.
**Why**: A tarball oracle keeps `node` on every runner and developer box; goldens make the harness self-contained and reuse the constitution's `-update` discipline.
**Rejected**: Running the last TS release tarball (needs node); reducing the harness to unit goldens only (loses the full-matrix gate).
*Introduced by*: 260925-6wpm-remove-src-node

#### `TUDIFF_NOW` in the shipped binary, not a build tag
**Decision**: One env read in `cmd/tu`'s `now()`, zone-less, harness-namespaced.
**Why**: Frozen goldens cannot follow the calendar; one binary and one code path (code-quality "minimum pathways"); the harness keeps testing the exact `bin/tu`.
**Rejected**: A `//go:build harness` clock (two binaries); libfaketime (absent on CI); normalising date labels (the default history window changes which rows exist).
*Introduced by*: 260925-6wpm-remove-src-node

#### Tag-driven version like the Go siblings
**Decision**: `git describe --tags --always` stamps the binary; `release.sh` only tags.
**Why**: The plan pre-announced it; no more version commits; identical to hop/idea; `$VERSION` normalisation keeps the goldens version-agnostic.
**Rejected**: A `VERSION` file (fab-kit shape) — keeps a commit per release and diverges from the siblings.
*Introduced by*: 260925-6wpm-remove-src-node

### Deprecated Requirements

#### The Node oracle in `tudiff run`/`live`
**Reason**: `src/node/` is deleted (D10 condition met: two Go releases shipped).
**Migration**: `harness/golden/` + `--update`.

#### `package.json` as the version anchor
**Reason**: The file is deleted.
**Migration**: the `v*` git tag via `git describe`.

#### The `build-and-test` CI lane and the `CCUSAGE_VERSION` lockfile guard
**Reason**: No Node tree to build or lockfile to compare.
**Migration**: N/A — `CCUSAGE_VERSION` is the sole pin.

#### The node-vs-go call-log comparison
**Reason**: Informational only; its oracle half is gone.
**Migration**: N/A — the Go call log still feeds the unconfirmed-replay gate.

## Tasks

> Verification discipline for every task: run the named commands yourself and paste their real output into your result; never report green from memory. Run every harness command with a clean env prefix `env -u TU_METRICS_REPO -u NO_COLOR` (this box exports both, and they leak into tests). Never run `git checkout -- <path>`, `git stash`, or `git reset` — the working tree is shared state. Commit at the two checkpoints named below (no push); commit messages carry no AI attribution lines.

### Phase 1: Setup

- [x] T001 Add `now()` to `src/go/cmd/tu/main.go` (per R1) and route the ten edge `time.Now` sites through it (keep the rain seed on `time.Now`); add `cmd/tu/now_test.go` covering set/unset/malformed `TUDIFF_NOW` and the two-zone same-date property. Verify: `cd src/go && go test ./cmd/tu -count=1`. <!-- R1 -->
- [x] T002 [P] <!-- rework: review must-fix — add NormalizeIdentity ($MACHINE/$USER, whole-token, hostname first) + ProbeIdentity; unit-test it including the machine_<host>_cost and <user>/<year>/<host>/ path shapes --> Create `src/go/internal/harness/golden.go` + `golden_test.go`: `Manifest` struct with `LoadManifest`/`WriteManifest` (2-space JSON, trailing newline, `DisallowUnknownFields`, schema check), `MatrixSHA256(path)`, `NormalizeVersion(b []byte, version string) []byte`, `ProbeVersion(bin string, args ...string) (string, error)` (last whitespace-separated field of the first line of `--version`), `TreeSnapshot(home string) (Tree, error)` + `WriteTree`/`LoadTree`/`CompareTree` (first differing path → `TreeDiff` excerpts, reusing the existing `TreeDiff`), `GoldenCaseDir(root, caseID)`, `WriteGoldenCase(dir, SideCapture, Tree)`, `LoadGoldenCase(dir, io) (SideCapture, Tree, error)`. Stdlib-only, table-driven tests. Verify: `go test ./internal/harness -count=1`. <!-- R2 -->

### Phase 2: Golden mode + capture (src/node still present)

- [x] T003 <!-- rework: apply NormalizeIdentity to byte channels and tree.json path keys at capture and compare in run.go --> Rework `src/go/cmd/tudiff/run.go` per R3 and R5: add `--golden`, `--update`, `--now`, and the temporary `--from-node <bundle>`; new preflight set; golden-mode fixture rule; single-home `stageCase`; `EnvSpec.Now` → `TUDIFF_NOW` in `harness.BuildEnv` (`diff.go`); compare path loads the golden into the oracle-position `SideCapture` and compares `tree.json`; `--update` writer; report/`report.json` renames in `internal/harness/report.go` (`golden=`/`go=`, `golden`, `golden_captured_at`, `golden_excerpt`, `golden_exit`; drop `node_ms`/`node_calls`/`calls_differ`) and `WriteCaseCaptures` writing `go.*` only plus `go.tree/` on a tree red. Keep `--from-node` isolated in clearly marked functions so T006 can delete them. Update `run_test.go`/`report_test.go`/`diff_test.go`. Verify: `go test ./cmd/tudiff ./internal/harness -count=1` and `just go-lint`. <!-- R3 -->
- [x] T004 <!-- rework: same identity normalisation in live.go (channels, tree.json keys, log/status text) --> Rework `src/go/cmd/tudiff/live.go` per R4 and R5 the same way (one remote/clone/home, `--golden`/`--update`/`--from-node`, `live/<step>/` goldens incl. `log.txt`/`status.txt` where checked, repair goldens vs `bin/turepair`, `TUDIFF_NOW` in env). Update `live_test.go`. Verify: `go test ./cmd/tudiff -count=1`. <!-- R4 -->
- [x] T005 Capture the goldens from Node and prove the Go side matches: `env -u TU_METRICS_REPO -u NO_COLOR npm ci && npm run build && just go-build && just harness-build`; then `env -u TU_METRICS_REPO -u NO_COLOR bin/harness/tudiff run --update --from-node dist/tu.mjs` (respect the 06:00–18:30 UTC capture-date window; retry later if the guard refuses) and `… bin/harness/tudiff live --update --from-node dist/tu.mjs`; inspect `harness/golden/manifest.json` (`now`, `oracle_version` `v0.12.2`, `cases` 452, `live_steps` 9) and spot-check `run/version/single/default/pipe/fixed/stdout` is `tu version $VERSION`; then `env -u TU_METRICS_REPO -u NO_COLOR bin/harness/tudiff run` and `… tudiff live` WITHOUT `--from-node` — both must be fully green. Then `git add harness/golden src/go` and commit: `harness: golden corpus captured from the Node oracle at v0.12.2 (plan row Z1)`. Paste both summaries into your result. <!-- R5 -->

### Phase 3: Removal (only after T005 is green and committed)

- [x] T006 Delete the Node capture arm per R6: `--from-node` and its helpers in `run.go`/`live.go`, `StageOracle`, `SideNode`, `CompareCallLogs`/`callSet`/`equalStrings` and their tests, the node exec branch; reword `tudiff` usage text and package comment; delete `scripts/repair-metrics.mjs`; set `cmd/turepair/main.go` `usageLine` to `Usage: turepair [--repo <path>] [--write]` and fix its test. Verify: `go test ./... -count=1` from `src/go`. <!-- R6 -->
- [x] T007 Delete the tree and toolchain per R9: `git rm -r src/node package.json package-lock.json tsconfig.json scripts/build.sh scripts/help-dump.mjs`; `FindRepoRoot` → `justfile`; the seven test walk-ups/fixtures → `justfile`; delete `TestDescriptionMatchesPackage`; `tudiff capture` ccusage resolution (`--ccusage` → `dist/vendor` → vendor beside `tu` on PATH → PATH), delete `nodePlatformArch`, fix `manifest_test.go`/`capture_test.go`; `fab/project/config.yaml`, `context.md`, `code-review.md` edits. Verify: `cd src/go && go test ./... -count=1 && cd ../.. && just go-lint`. <!-- R9 -->
- [x] T008 Build/CI/release per R7 and R8: `justfile` (`go_version` from `git describe`, `-X main.version={{go_version}}`, delete `setup`/`test`/`run`/`build`, `go-diff`/`go-live` without `build`, comments), `scripts/release.sh` (hop's tag-only script), `scripts/package-go.sh` (drop the lockfile guard; smoke `want` from `git describe`), `.github/workflows/ci.yml` (two lanes, `fetch-depth: 0`, no node), `.github/workflows/release.yml` (no `tag-on-release-merge`, no main-push trigger, no node steps, hop's version step, comments rewritten), `.gitignore`, `README.md`, `cmd/tu/main.go` version comment. Verify: `just go-build && bin/tu --version` prints `tu version v0.12.2-<n>-g<sha>`; `just go-build-all`; `bash -n scripts/release.sh scripts/package-go.sh`; `python3 -c 'import yaml,sys;[yaml.safe_load(open(f)) for f in sys.argv[1:]]' .github/workflows/ci.yml .github/workflows/release.yml` (or `ruby -ryaml`). <!-- R7 -->
- [x] T009 Comment sweep per R6/R9: reword every `src/go` comment that cites `src/node/…`, `dist/tu.mjs`, `npm`, `node_modules`, or `package.json` (files listed in intake § What Changes 5) — behaviour untouched. Verify: `git grep -nE 'tu\.mjs|node_modules|package\.json|src/node|\bnpm\b' -- src/go` prints nothing; `git grep -nE 'setup-node|\bnpm ci\b|npm run|node -p' -- .github justfile scripts README.md .gitignore` prints nothing. <!-- R6 -->
- [x] T010 Full verification with Node off PATH: `export PATH=$(echo "$PATH" | tr ':' '\n' | grep -v '/.nvm/' | paste -sd:)` then `command -v node || echo no-node`; `just go-lint && just go-test && just go-build && just go-build-all`; `env -u TU_METRICS_REPO -u NO_COLOR just go-diff --placeholder` → `452 green`; `env -u TU_METRICS_REPO -u NO_COLOR just go-live` → `9 green`; `git status --porcelain` shows no stray untracked files (`harness/golden` tracked, `node_modules/`/`dist/` ignored). Paste the two summaries. Commit: `chore: remove src/node and the Node toolchain (plan row Z1)`. <!-- R8 -->

### Phase 4: Polish

- [x] T012 Regenerate the corpus with identity normalisation: `env -u TU_METRICS_REPO -u NO_COLOR just go-build harness-build`, then `bin/harness/tudiff run --update` and `bin/harness/tudiff live --update` (from the Go binary — proven byte-identical to the Node capture in df9b08f; `now` is preserved from the manifest). Confirm `git diff --stat harness/golden` touches only the 17 identity-bearing files plus `manifest.json` (and nothing else changes bytes), `grep -rw 'dev-ws-sahil02\|sahil' harness/golden` is empty, then prove host independence: `unshare -u -r sh -c 'hostname fake-ci-runner; env -u TU_METRICS_REPO -u NO_COLOR bin/harness/tudiff run --placeholder'` → 452 green and the same for `tudiff live` (under `unshare -r` the user is `root`, which also exercises `$USER`). Also add the `2>/dev/null || echo dev` fallback to `scripts/package-go.sh`'s `want` line (review nice-to-have). Commit: `harness: normalise host identity in the goldens ($MACHINE/$USER), regenerated via --update`. <!-- R2 -->
- [x] T011 Update `fab/plans/sahil/26-09-15-go-port.md` per R10 (Z1 row PR column → `260925-6wpm-remove-src-node`, status `in review`; header Status line). Amend it into the T010 commit or make a third commit `plan: Z1 in review`. <!-- R10 -->

## Execution Order

- T001 and T002 are independent; T003 needs both; T004 needs T003 (shares the golden helpers and flag plumbing).
- T005 MUST run after T003+T004 and MUST be green and committed before T006 starts — it is the only moment both oracle and Go binary exist.
- T006 → T007 → T008 → T009 → T010 sequential (each verification depends on the previous deletion being complete).
- T011 last.

## Acceptance

### Functional Completeness

- [x] A-001 R1: `cmd/tu/main.go` has one `now()`; every edge `time.Now` except the rain seed goes through it; `now_test.go` covers set/unset/malformed and the two-zone date property; no `TUDIFF_NOW` read under `internal/`, no mention in `--help`, `usage.md`, or `skill.md`.
- [x] A-002 R2: `harness/golden/manifest.json`, `run/<452 cases>/`, and `live/<9 steps>/` exist with the specified files; `golden.go` is stdlib-only with sibling tests for manifest, normalisation, tree snapshot/compare, and case load/write. (Re-verified with identity normalisation: 452 exits / 376 stdout / 76 tty / 452 tree.json; `NormalizeIdentity` covered by golden_test.go.)
- [x] A-003 R3: `tudiff run` has no `--node`; `--golden`/`--update`/`--now` exist; the four new preflight errors exit 2 with the specified text; `just go-diff --placeholder` is 452/452 green.
- [x] A-004 R4: `tudiff live` compares the seven sync steps and two repair steps against `live/`; `just go-live` is 9/9 green.
- [x] A-005 R5: the golden commit exists on the branch before the removal commit; `manifest.json` records `oracle: node dist/tu.mjs`, `oracle_version: v0.12.2`, a noon `now`.
- [x] A-006 R6: no `--from-node`, `StageOracle`, `SideNode`, or `CompareCallLogs` symbol remains; `git grep` over `src/go` for the five Node tokens is empty.
- [x] A-007 R7: `go_version` is `git describe`-based; `bin/tu --version` prints a describe string; `release.sh` matches hop's tag-only shape; `package-go.sh` has no lockfile guard and its smoke test derives `want` from `git describe`; `release.yml` has no `tag-on-release-merge` job and no node.
- [x] A-008 R8: `ci.yml` has exactly `go-build-and-test`, `tudiff`, `ci-gate`; both checkouts use `fetch-depth: 0`; `justfile` has no `setup`/`test`/`run`/`build`; README and `.gitignore` updated.
- [x] A-009 R9: the seven deleted paths are gone from the tree; `FindRepoRoot` and all test walk-ups use `justfile`; `TestDescriptionMatchesPackage` is gone; `tudiff capture` resolves ccusage without `node_modules`; `turepair` usage line updated; the three `fab/project/` files updated.
- [x] A-010 R10: the plan file's Z1 row names this change with status `in review`.

### Behavioral Correctness

- [x] A-011 R1: with `TUDIFF_NOW` unset, `bin/tu` output for a snapshot equals a build without the seam (spot-check by diffing against `git stash`-free means: run the binary from the golden commit vs HEAD on the same case).
- [x] A-012 R3: a deliberately edited `harness/matrix.json` makes `tudiff run` exit 2 with the matrix-changed line; a deliberately edited golden `stdout` makes the matching case red on channel `stdout` with `golden=`/`go=` excerpts.
- [x] A-013 R7: `just go-package` on a tagged checkout passes its host smoke test with the tag string (verify structurally: the script's `want` line, since packaging needs the registry).

### Removal Verification

- [x] A-014 R9: `ls src/node package.json package-lock.json tsconfig.json scripts/build.sh scripts/help-dump.mjs scripts/repair-metrics.mjs` all fail.
- [x] A-015 R8: `git grep -n setup-node -- .github` is empty; `git grep -nE '\bnpm (ci|run|install|version)\b|node -p' -- .github justfile scripts` is empty.
- [x] A-016 R6: `git grep -n CompareCallLogs -- src/go` is empty.

### Scenario Coverage

- [x] A-017 R3: `tudiff run --list` still prints 452 IDs; `tudiff run --filter version` is green and its report header shows the `golden:` line.
- [x] A-018 R5: the golden `run/version/single/default/pipe/fixed/stdout` is exactly `tu version $VERSION\n`; `run/help-dump/single/default/pipe/fixed/stdout` contains `"version": "$VERSION"`.
- [x] A-019 R4: `live/sync/log.txt` exists and contains commit hashes; `live/repair-write/tree.json` exists.

### Edge Cases & Error Handling

- [x] A-029 R2: no golden file contains the capturing machine's hostname or username; under `unshare -u -r` with a fake hostname `tudiff run --placeholder` and `tudiff live` are fully green. (Re-verified: `grep -rnw 'dev-ws-sahil02\|sahil' harness/golden` empty; under `unshare -u -r` + `hostname fake-ci-runner` run = 452/452 exit 0, live = 9/9 exit 0.)

- [x] A-020 R3: `tudiff run --fixtures dev-ws-sahil02` exits 2 with the placeholder-only message; `--update --fixtures x --golden /tmp/x` is accepted.
- [x] A-021 R3: a missing golden dir for one filtered case exits 2 naming the case.
- [x] A-022 R1: `TUDIFF_NOW=banana` falls back to `time.Now` without an error line.

### Code Quality

- [x] A-023 Pattern consistency: new harness code follows the package's existing style (flag sets, `fail()` closures, `tudiff: ` error prefix, exported helpers with doc comments), `gofmt`/`go vet` clean.
- [x] A-024 No unnecessary duplication: `Compare`, `firstDivergence`, `TreeDiff`, `CopyFile`, `Excerpt` are reused, not re-implemented; `run` and `live` share the golden load/write helpers.
- [x] A-025 No I/O in `init()` or package-level initializers (Constitution IV) — `now()` reads the env at call time.
- [x] A-026 Errors returned, not printed, inside `internal/harness`; `cmd/tudiff` is the only writer to its streams.
- [x] A-027 Readability: no function over ~50 lines without reason; the temporary node arm was isolated so its deletion is a clean removal, not a rewrite.
- [x] A-028 No magic strings: `$VERSION`, `$HOME`, `TUDIFF_NOW`, the manifest schema number, and the golden dir default are named constants.

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `report.json`'s `node_version` field becomes `golden_captured_at` rather than being dropped | Keeps a provenance field in the machine-readable report; nothing downstream parses `node_version` | S:50 R:90 A:80 D:70 |
| 2 | Confident | `--update` with no existing manifest pins `now` to today at noon (local) | Only reachable when the corpus is absent; mirrors the capture guard's noon convention | S:45 R:85 A:75 D:65 |
| 3 | Confident | The capture window is 06:00–18:30 UTC (UTC and Asia/Kolkata share a calendar date) | UTC+5:30: the dates differ only between 18:30 and 24:00 UTC | S:60 R:90 A:90 D:85 |
| 4 | Confident | `tudiff capture`'s new PATH-vendor probe resolves symlinks like `ccusage.ResolveBinary` | Reuses the shipped resolution rule; the brew install symlinks `bin/tu` into libexec | S:50 R:85 A:80 D:70 |
| 5 | Certain | Two checkpoint commits (goldens, then removal), no push, no attribution lines | The golden commit is the provenance the PR reviewer needs; user rule forbids AI attribution | S:85 R:90 A:90 D:90 |

5 assumptions (1 certain, 4 confident, 0 tentative).
