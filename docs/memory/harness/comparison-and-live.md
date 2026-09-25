---
type: memory
description: "How tudiff decides red — first-divergence byte comparison per channel with the golden side in the oracle position after $HOME/$VERSION/identity normalisation, the tree.json comparison over the written metrics repo, and the DC-keyed harness/expected-diffs.json file with its match rule and stale-entry failure; the live real-git sequence is in live.md."
---
# Comparison

**Domain**: harness

## Overview

After the Go side runs, `harness.Compare` and the `tree.json` comparison decide a case's colour against the golden corpus ([golden-corpus](/harness/golden-corpus.md)), and `harness/expected-diffs.json` marks the divergences a spec decision makes intentional. The matrix run that feeds these is in [differential-harness](/harness/differential-harness.md); the real-git sync and repair sequence is in [live](/harness/live.md).

## Requirements

### Requirement: Expected-diffs file
`harness/expected-diffs.json` (committed, JSON, `schema: 1`, sibling of `matrix.json`) declares the divergences a spec drop decision makes intentional: `{"schema": 1, "expected": [{"id": "DC-05", "cases": ["snapshot-*-csv", "…"], "reason": "…"}]}`, keyed by the spec's stable `DC-NN` ids. `LoadExpected(path)` (`src/go/internal/harness/expected.go`) decodes with `DisallowUnknownFields` plus a trailing-value rejection (the `LoadMatrix` pattern) and validates: `schema == 1`; the `expected` key present (an empty array is valid, a missing key invalid — the matrix's `args` rule); per entry an `id` matching `^DC-\d{2}$` that is unique across entries, a non-empty `cases` array whose every pattern `path.Match` accepts (an `ErrBadPattern` is a load error), and a non-empty `reason` — every error names the offending entry `id` (when known) and field. `(*Expected).Match(caseID, group)` tries entries in file order and the first matching entry wins: a pattern matches when `path.Match(pattern, caseID)` is true, and a pattern containing no `/` is additionally tried against the group name. Case IDs are `<group>/<conf>/<env>/<io>/<tz>` for `run` and the bare step name under group `live` for `live`, so `snapshot-*-csv` names whole groups, `sync-dry-run/*/*/*/alt` one axis slice, `sync-dry-run` both the matrix group and the live step of that name, and `live` every live step (`*` never crosses `/`, which is why axis-level patterns spell all five segments). A nil or empty set never matches. The committed set is empty: every `[DECIDE]` marker in `docs/specs/usage.md` § Drop at cutover is unresolved, and unresolved means keep — a later `drop` resolution is one entry here, not a code change. (489t)

#### Scenario: First matching entry wins
- **GIVEN** entries `DC-05: ["snapshot-*-csv"]` and `DC-21: ["sync-dry-run/*/*/*/alt", "sync-dry-run"]`
- **WHEN** matching `snapshot-all-csv/multi/default/pipe/fixed` (group `snapshot-all-csv`), `sync-dry-run/multi/default/pipe/alt` (group `sync-dry-run`), `sync-dry-run/multi/default/pipe/fixed` (group `sync-dry-run`), and `sync-dry-run` (group `live`)
- **THEN** the results are `DC-05`, `DC-21` (the axis-slice pattern hits the full ID), `DC-21` (the bare pattern hits the group), and `DC-21` (the bare pattern hits the live step ID)

### Requirement: Comparison — first divergence per case
`Compare` byte-diffs channels in order and stops at the first difference: pipe cases compare `exit`, `stdout`, `stderr`; tty cases compare `exit`, `tty`. The golden's stored bytes are already normalised (home, version, identity — the capture-time pipeline in [golden-corpus](/harness/golden-corpus.md)); the Go side's fresh capture is run through the same pipeline — `NormalizeHome` against its staged `$HOME`, then `NormalizeVersion` with the probed version, then `NormalizeIdentity` with the probed host identity — before comparing. The golden channels load into the oracle-position `SideCapture`: the divergence offset, 1-based line (counting `\n` in the golden bytes — the oracle side is the line-numbering reference), and `strconv.Quote` excerpts of at most 40 bytes are computed against it. `SideCapture.Home` carries the Go side's staged home (set by `runCase`; `Compare`'s own home normalisation is a no-op on the already-normalised bytes). Every compared channel identical → **green**; a timeout on the side → **timeout** (channel `timeout`), counted as red for the exit code but listed separately in the summary. Otherwise the case is **red** with `Channel` naming the first differing one; an exit divergence records the two codes. (lsml, 6wpm)

#### Scenario: First divergence on stdout
- **GIVEN** golden stdout `"Usage: tu\n"` and go stdout `""` with equal exit codes
- **WHEN** compared
- **THEN** the case is red on channel `stdout` at offset 0, line 1, with golden excerpt `"Usage: tu\n"` and go excerpt `""`

### Requirement: The tree channel compares the written metrics repo against tree.json
After the byte comparison, a case that is still green is checked by `harness.CompareTree(goldenTree, goHome, hostname, username)` (wired into `runCase` after `Compare`): the Go side's `<home>/.tu/metrics_repo/` tree — snapshotted as slash-separated relpath → sha256+size, path keys identity-normalised, `.git/` excluded in live — is compared against the golden's `tree.json`, plus the **presence** of `<home>/.tu/.last-sync` (its content is a wall-clock timestamp, so presence only). Nothing under `.tu/cache/` is compared. A difference reddens the case with `Channel: "tree"` and the first differing relative path (or `.last-sync`) in the excerpts (present/absent, or a hash+size descriptor of each side for a content difference); a case already red keeps its first channel. The report renders the `tree` channel like any other red channel and writes the Go side's tree files under `cases/<id>/go.tree/` so the bytes can be inspected without re-running. Every multi-mode data case thereby also proves the write — a `--json`-style float or key-order slip in a written day-file becomes visible. (lsml, 6wpm)

#### Scenario: A day-file byte divergence is red on the tree channel
- **GIVEN** a multi-mode data case where the Go side wrote a day-file with different bytes
- **WHEN** `tudiff run` compares it
- **THEN** the case is red on channel `tree` naming that path; identical trees with `.last-sync` on the Go side matching the golden's flag stay green

## Design Decisions

### Compare-side home normalization
**Decision**: The staged home path is replaced with the literal `$HOME` in the stdout/stderr/tty captures before comparison and storage, and the report's per-case capture files hold the normalized bytes.
**Why**: The setup commands' absolute-path messages (the version warning's conf path, the live pull-failure `git -C <dir>` line) are Goal-frozen surfaces that legitimately embed `$HOME`, and the staged home is a per-run temp path the committed corpus cannot hold — without normalization those cases could never go green.
**Rejected**: A shared, fixed home path (fragile across `script`/tty and still leaks the side's real dir through `cwd`).
*Introduced by*: 260916-4fs0-config-and-setup-commands

### Identity normalisation: whole-token hostname, path-segment username
**Decision**: `NormalizeIdentity` replaces the capturing machine's hostname as a whole token — the token alphabet is `[A-Za-z0-9-]`, so `_` is a boundary and `machine_dev-ws-sahil02_cost` normalises to `machine_$MACHINE_cost` while `dev-ws-sahil02x` never matches — and its username only as a whole slash-delimited path segment, hostname first since the hostname may embed the username.
**Why**: The hostname surfaces as a token in compared bytes (column names, path components); the username flows into compared surfaces exclusively as the metrics-repo layout's `<user>/` segment, and a whole-token rule would false-positive on ordinary text when the username is a common word (CI's `root` is a help-dump JSON key). Both run at capture and at compare, so the corpus provably holds no real host identity and a run on any machine or as any user matches it.
**Rejected**: Whole-token username replacement (false positives on common words); plain substring replacement (mangles `dev-ws-sahil02x` and `sahil02` against user `sahil`).
*Introduced by*: 260925-6wpm-remove-src-node

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
