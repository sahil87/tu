---
type: memory
description: "The committed golden corpus harness/golden/ — the differential harness's oracle: the manifest.json/run-<case>/live-<step> layout and manifest fields with the matrix-hash drift guard, tree.json's hash-pinned metrics-repo tree, the $HOME/$VERSION/$MACHINE/$USER normalisations (plus $TMP/$REPO in live), the pinned zone-less clock feeding TUDIFF_NOW, and the --update refresh discipline under Output Stability."
---
# Golden Corpus

**Domain**: harness

## Overview

`harness/golden/` is the committed byte corpus the differential harness compares `bin/tu` against — the retired TypeScript implementation's output for the full matrix and the live sequence, frozen as files (6wpm). The run driver that consumes it is in [differential-harness](/harness/differential-harness.md); the comparison rules in [comparison-and-live](/harness/comparison-and-live.md).

## Requirements

### Requirement: Corpus layout
`harness/golden/` holds `manifest.json` at the root, `run/<case ID as nested dirs>/` per expanded matrix case (452 today), and `live/<step>/` per live step (the seven sync steps plus `repair-dry-run` and `repair-write`). A run case dir holds `stdout`, `stderr`, `exit` for pipe cases or `tty`, `exit` for tty cases, plus `tree.json`; `exit` is the decimal exit code with a trailing newline. A live step dir holds `stdout`, `stderr`, `exit`, `tree.json`, and — exactly where the step pins them — `status.txt` (the clone's `git status --porcelain` after the step) and `log.txt` (its `git log --format=%H%n%s -p main`, hashes included; the pinned git identity and fixed commit dates make them reproducible). All loader/writer code lives in `src/go/internal/harness/golden.go` (stdlib-only): `LoadGoldenManifest`/`WriteGoldenManifest`, `MatrixSHA256`, `TreeSnapshot`/`TreeSnapshotLive`/`TreeSnapshotRepo`, `WriteTree`/`LoadTree`/`CompareTree`/`CompareLiveTree`/`CompareTreeSnapshot`, `GoldenCaseDir`/`GoldenLiveDir`, `WriteGoldenCase`/`LoadGoldenCase`, `WriteGoldenExtra`/`LoadGoldenExtra`, `NormalizeVersion`, and `ProbeIdentity`/`NormalizeIdentity`/`NormalizeTreeIdentity`.

### Requirement: manifest.json
`manifest.json` is produced from `harness.GoldenManifest` (2-space JSON, trailing newline; loading rejects unknown fields, trailing content, and any schema other than 1) with fields in serialized order: `schema`, `oracle` (the capturing binary as invoked — `bin/tu`), `oracle_version` (the probed `<oracle> --version` last field), `node_version` (the Node runtime version at capture; empty for a Go capture), `captured_at` (RFC 3339 UTC), `now` (the pinned zone-less local timestamp), `script` (the `script(1)` flavour — `util-linux` or `bsd`), `platform` (`<GOOS>/<GOARCH>`), `fixtures` (`["_placeholder"]` — the committed corpus is placeholder-only, real captures never enter it), `matrix_sha256` (hex sha256 of `harness/matrix.json` at capture time), `cases`, and `live_steps`. Compare mode refuses to run — preflight exit 2 — when the manifest is missing or invalid, when `matrix_sha256` differs from the current matrix's hash, or when an expanded case or live step has no golden dir (the exact messages are in [differential-harness](/harness/differential-harness.md)); the harness never reports a vacuous green.

### Requirement: tree.json pins the written tree by hash
`tree.json` is `{"files": {"<relpath>": {"sha256": "<hex>", "size": N}}, "last_sync": bool}` — every file under `<home>/.tu/metrics_repo/` (slash-separated relpath → hash+size; nothing under `.tu/cache/`, and the live snapshot also excludes the clone's `.git/`) plus the presence — never the content, a wall-clock timestamp — of `<home>/.tu/.last-sync`. An absent metrics repo is `{"files": {}, "last_sync": false}`. The repair steps' `tree.json` hashes the repair repo's working tree instead (`.git/` excluded, `last_sync` false). Hashes rather than bytes keep ~450 copies of the seed tree out of git while pinning paths and content exactly.

#### Scenario: Hash-pinned tree
- **GIVEN** a sync case whose Go side wrote a day-file with different bytes
- **WHEN** the written tree is compared against `tree.json`
- **THEN** the case is red on channel `tree` naming the first differing path; storing the tree as sha256+size pins that byte surface without committing the tree

### Requirement: Byte channels are stored normalised
Byte channels are stored exactly as `harness.Compare` compares them: after `NormalizeHome` (the staged `$HOME` → the literal `$HOME`), then `NormalizeVersion` (the probed `<bin> --version` value, e.g. `v0.12.2`, and its `v`-stripped bare form → `$VERSION`, longest form first so the bare form never matches inside the v-form), then `NormalizeIdentity` (the capturing machine's hostname → `$MACHINE`, whole-token; its username → `$USER`, whole path segment). The same pipeline runs on the Go side's fresh capture at compare time with the Go side's probed values, so the `--version`, `-v`, and `help-dump` goldens hold across releases and the corpus carries no real host identity. `tree.json` path keys receive the same identity normalisation (day-file contents carry none, so hashes pass through). In `live`, channels additionally replace the temp root with `$TMP` (real git's error text embeds the missing-remote path verbatim), and the repair steps replace the repo path with `$REPO`.

#### Scenario: Version normalisation
- **GIVEN** a Go capture whose stdout is `tu version v0.12.3-2-gabcdef\n` and a probed version `v0.12.3-2-gabcdef`
- **WHEN** normalised
- **THEN** the bytes are `tu version $VERSION\n`; a help-dump body containing `"version": "0.12.3-2-gabcdef"` becomes `"version": "$VERSION"`

### Requirement: The pinned clock
`manifest.now` is a zone-less local timestamp (`2006-01-02T15:04:05` shape) that `tudiff run` and `tudiff live` pass to the Go side as `TUDIFF_NOW`; `cmd/tu`'s `now()` seam parses it in the process TZ and falls back to `time.Now` when unset or malformed (the seam's contract is in [differential-harness](/harness/differential-harness.md)). Cache TTL and `.last-sync` ages computed against the pinned instant match what a sub-60 s real run produced (`<1m ago`, cache fresh).

### Requirement: The --update refresh discipline
`tudiff run --update` and `tudiff live --update` rewrite the goldens from the Go binary over the (filtered) matrix and the nine live steps, then rewrite the manifest with a fresh `captured_at` and `matrix_sha256`, `oracle`/`oracle_version` from the Go side, and `now` **preserved** from the existing manifest — `--now <ts>` overrides it, and a bootstrap capture with no manifest pins noon of today (local). `--update` forces the `_placeholder` corpus; a filtered update rewrites only the matched cases (`go test -run X -update` style); the writer prints `tudiff: wrote <n> goldens under <dir>/run (now <now>)` (or `…/live`) and exits 0. A golden diff is an output change: it MUST be reviewed in the PR like any `testdata/*.golden` regeneration, and it falls under the constitution's Output Stability rule — a breaking output change requires a minor version bump.

#### Scenario: Matrix drift is a preflight error, not a silent diff
- **GIVEN** `harness/matrix.json` edited after the goldens were captured
- **WHEN** `tudiff run` runs without `--update`
- **THEN** stderr is `tudiff: harness/matrix.json changed since the goldens were captured (run tudiff run --update and review the diff)` and the exit code is 2

## Design Decisions

### The committed corpus is the oracle — frozen goldens, not a live binary
**Decision**: `tudiff` byte-diffs `bin/tu` against committed golden captures under `harness/golden/`; the oracle the bytes came from is retired.
**Why**: An oracle that runs (the last TypeScript bundle) keeps `node` on every CI runner and developer machine; goldens make the harness self-contained and reuse the constitution's `testdata/*.golden` + `-update` discipline, lifted to the binary level.
**Rejected**: Running the last TS release tarball as the oracle (still needs node); reducing the harness to unit goldens only (loses the full-matrix gate).
*Introduced by*: 260925-6wpm-remove-src-node

### Capture provenance: the Node oracle's bytes, regenerated from the proven-identical Go binary
**Decision**: The corpus's bytes originate from the frozen TypeScript oracle (`node dist/tu.mjs`) at `v0.12.2`, captured on 2026-09-25 on Linux with util-linux `script` and `now` pinned to that date's noon; once identity normalisation (`$MACHINE`/`$USER`) landed, the corpus was regenerated in place via `--update` from the Go binary — which the committed corpus had just proven byte-identical to the Node capture — so the manifest's `oracle` reads `bin/tu` while `now` stays pinned to the original date.
**Why**: Goldens can only be captured while the oracle still runs, so the freeze preceded the toolchain's removal in the same change; regenerating from the proven-equal Go binary applied the identity normalisation without trusting any new oracle.
**Rejected**: Re-capturing after the removal (no oracle left to prove the bytes against); keeping the capture machine's hostname/username in the corpus (leaks real identity into a public repo).
*Introduced by*: 260925-6wpm-remove-src-node

### A zone-less pinned clock over per-zone timestamps
**Decision**: `now` is one zone-less local timestamp; each case's TZ interprets it, and noon is the pinned hour.
**Why**: The matrix's `tz` axis runs the same case under `TZ=UTC` and `TZ=Asia/Kolkata`; one pinned wall-clock value lands on the same local calendar date in both, and noon keeps `CommitMessage`'s UTC date on that same day in both zones.
**Rejected**: Pinning a UTC instant (the two zones would see different local dates); normalising date labels out of captures (the default history window changes which rows exist as months pass, so the row set itself would drift).
*Introduced by*: 260925-6wpm-remove-src-node
