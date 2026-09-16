# Intake: Placeholder Confirmation Ledger

**Change**: 260916-di0s-placeholder-confirmation-ledger
**Created**: 2026-09-16

## Origin

One-shot `/fab-new` invocation from the Go-port plan, row P3c (`fab/plans/sahil/26-09-15-go-port.md`). The user's raw input:

> Context: fab/plans/sahil/26-09-15-go-port.md, row P3c. Read the Decisions and Target architecture sections before writing the intake. Do not change any external surface listed under Goal. `tudiff placeholder` hard-codes `unconfirmed: true` on every entry, so a hand-edited flip in `manifest.json` is silently reverted by the next regeneration (the `sha256` pins fixture bytes, not the manifest). Add a committed `harness/fixtures/_placeholder/confirmed.json` — a JSON object keyed by source name with machine, date, and ccusage_version fields — that the generator merges into the manifest (unconfirmed: false plus a confirmed_by field for listed sources); the corpus test asserts manifest and ledger agree. Seed it with the four sources confirmed in P3b (claude, codex, gemini, kimi).

The plan's own P3c row (read at `origin/main` `f40c895`) gives the ledger shape verbatim: `{"<source>": {"machine": …, "date": …, "ccusage_version": …}}`, and orders the row "after P4, before G0". Row P3b records the confirmation facts this change seeds: **done 2026-09-16 on dev-ws-sahil02 for claude, codex, gemini, kimi — all four key-path sets identical to their placeholders**; opencode and copilot are empty there and stay unconfirmed until another machine with data reports back.

Key facts established while reading the code (not assumptions):

- The worktree branch `p3c-placeholder-confirmation-ledger` was 5 commits behind `origin/main` with no unique commits; it was fast-forwarded to `f40c895` (P4 `tudiff run`, #82) before this intake was written, so everything below describes the post-P4 tree.
- `src/go/internal/harness/placeholder.go` `WritePlaceholders(out, sources)` builds each `Fixture{…, Unconfirmed: true}` literally and rewrites `<out>/manifest.json` wholesale. Nothing reads any per-source confirmation state.
- `src/go/internal/harness/manifest.go` `Fixture` has no field for who confirmed a shape; `Manifest.DerivedFrom` is one corpus-wide provenance sentence (`PlaceholderDerivedFrom`).
- `src/go/internal/harness/corpus_test.go` `TestCorpus` asserts `unconfirmed: true` occurs only under `_placeholder/` and nothing else about the flag.
- `src/go/internal/harness/fixtures.go` `UnconfirmedReplays` (P4) reads `fx.Unconfirmed` from the manifest to mark `[unconfirmed]` cases in the `tudiff run` report. It needs no change: once the manifest says `unconfirmed: false` for a source, cases replaying that source stop being flagged.
- `.gitignore` ignores `harness/fixtures/*/` except `_placeholder/`; `confirmed.json` under `_placeholder/` is committable as-is (verified with `git check-ignore`).

## Why

**The pain point.** P3b is a human step: compare a real capture's key paths against a placeholder, then mark the source confirmed. The only place that mark can live today is `manifest.json`, and `manifest.json` is a generated file. The generator hard-codes `unconfirmed: true`, so the next `tudiff placeholder` run (needed whenever a placeholder shape or the fixed counters change) silently reverts the human's flip. The `sha256` in the manifest pins the fixture *bytes*, not the manifest itself, so the corpus test cannot catch the revert either. Four of six sources are already confirmed (P3b, 2026-09-16) and there is no durable place to say so.

**The consequence of not fixing it.** The `tudiff run` report keeps counting claude/codex/gemini/kimi cases as `[unconfirmed]`, which drowns the two genuinely unconfirmed sources (opencode, copilot) in noise; gate G2 ("no fixture is still `unconfirmed`") and the R3 gate (unconfirmed fixtures fail the run) become unreachable without either hand-editing generated output before every gate or never regenerating placeholders again. Either is a trap the operator will fall into.

**Why this approach.** A small committed ledger, `confirmed.json`, is the human-owned input; `manifest.json` stays a pure function of the fixtures plus the ledger. The generator merges the ledger in, so regeneration is idempotent with respect to confirmations. The corpus test asserts the two files agree, so a stale manifest (ledger edited, generator not re-run) or a stale ledger (a source removed from the corpus) fails CI the same way a tampered fixture already does. This keeps D7's invariant (only `_placeholder/` is ever unconfirmed) and D6's report contract intact, adds no dependency (stdlib `encoding/json`), and touches no external surface listed under the plan's Goal — `harness/` and `src/go/cmd/tudiff` are developer tooling, unshipped.

Alternatives rejected:

- **Hand-edit `manifest.json` and never regenerate** — the exact failure this row exists to remove.
- **Encode confirmation in `derived_from`** (the original P3b wording) — one string cannot carry per-source state, and the generator overwrites it too.
- **Flags on `tudiff placeholder` (`--confirmed claude=dev-ws-sahil02:2026-09-16`)** — the state would live in the invoker's shell history, not the repo.
- **Bump `schema` to 2** — the new field is optional and additive; real-capture manifests are unaffected and the fake ccusage reads only `fixtures[].{source,period,args,file,stderr_file,exit_code}`.

## What Changes

### 1. The ledger file: `harness/fixtures/_placeholder/confirmed.json`

A committed JSON object keyed by ccusage source name. Each value records who confirmed that the placeholder's key-path set matches a real capture. Seed content (exact bytes to commit, 2-space indent, trailing newline, keys in registry order):

```json
{
  "claude": {
    "machine": "dev-ws-sahil02",
    "date": "2026-09-16",
    "ccusage_version": "20.0.19"
  },
  "codex": {
    "machine": "dev-ws-sahil02",
    "date": "2026-09-16",
    "ccusage_version": "20.0.19"
  },
  "gemini": {
    "machine": "dev-ws-sahil02",
    "date": "2026-09-16",
    "ccusage_version": "20.0.19"
  },
  "kimi": {
    "machine": "dev-ws-sahil02",
    "date": "2026-09-16",
    "ccusage_version": "20.0.19"
  }
}
```

Rules:

- The file is optional. Absent → every generated entry is `unconfirmed: true` exactly as today (this is what keeps the existing `--out <tmp>` tests and `TestPlaceholderDefaultsToAllSources` green without edits).
- Keys MUST be members of `harness.DefaultSources` (`claude, codex, opencode, gemini, copilot, kimi`). Any other key is a typo guard failure: the generator exits 1 with `tudiff: confirmed.json: unknown source "opencod" (known: claude, codex, opencode, gemini, copilot, kimi)`.
- Every entry MUST have non-empty `machine`, `date` matching `^\d{4}-\d{2}-\d{2}$`, and non-empty `ccusage_version`. A missing or malformed field exits 1 with `tudiff: confirmed.json: <source>: <field> is required` / `… date must be YYYY-MM-DD`.
- Unparseable JSON exits 1 with `tudiff: confirmed.json: <json error>`.
- Unknown extra fields inside an entry are rejected (`json.Decoder.DisallowUnknownFields`) so a misspelled `ccusage_verison` cannot be silently dropped.
- The ledger is **only** read from `<out>/confirmed.json` — the same alias directory the generator writes to. A real alias directory never has one; the generator never looks outside `--out`.
- `ccusage_version` in the ledger is recorded, not validated against `PlaceholderVersion` (`20.0.19`). A confirmation made against a different ccusage release is still a confirmation of the key-path set; the version is provenance for a human reading the manifest (Assumption 10).

### 2. Manifest schema: a new optional `confirmed_by` on `Fixture`

In `src/go/internal/harness/manifest.go`:

```go
// ConfirmedBy records the human confirmation that a placeholder's key-path
// set matches a real capture (plan row P3b). It is read from
// <alias>/confirmed.json by WritePlaceholders and appears only on
// _placeholder entries whose source is listed there.
type ConfirmedBy struct {
	Machine        string `json:"machine"`
	Date           string `json:"date"` // YYYY-MM-DD
	CcusageVersion string `json:"ccusage_version"`
}

type Fixture struct {
	// … existing fields unchanged, in the same order …
	Unconfirmed bool         `json:"unconfirmed"`
	ConfirmedBy *ConfirmedBy `json:"confirmed_by,omitempty"`
}
```

- `SchemaVersion` stays `1`. The field is a pointer with `omitempty`, so real-capture manifests (written by `Capture`) and unconfirmed placeholder entries serialize byte-for-byte as today.
- Invariant: `ConfirmedBy != nil ⇔ Unconfirmed == false` **on `_placeholder` entries**; on every other alias `ConfirmedBy` is always `nil` (a real capture is confirmed by being real, not by a ledger).
- Resulting manifest entry for a confirmed source:

```json
    {
      "source": "claude",
      "period": "daily",
      "args": ["--json"],
      "file": "claude/daily.json",
      "stderr_file": "",
      "exit_code": 0,
      "sha256": "b7341ac5bc442c28ad155c07e4020df28d8121d2172c722c04210a187f8dace5",
      "days": 3,
      "first_date": "2026-01-05",
      "last_date": "2026-01-07",
      "empty": false,
      "redactions": 0,
      "unconfirmed": false,
      "confirmed_by": {
        "machine": "dev-ws-sahil02",
        "date": "2026-09-16",
        "ccusage_version": "20.0.19"
      }
    }
```

  and an unlisted source (opencode, copilot) keeps today's shape ending at `"unconfirmed": true` with no `confirmed_by` key.

### 3. Ledger loading and merge in the generator

In `src/go/internal/harness/placeholder.go`:

```go
// ConfirmedLedgerFile is the per-alias ledger of human-confirmed placeholder
// shapes, read by WritePlaceholders from <out>/confirmed.json.
const ConfirmedLedgerFile = "confirmed.json"

// ReadConfirmed loads <dir>/confirmed.json. A missing file is not an error
// and yields an empty ledger; malformed JSON, an unknown source key, or a
// missing/invalid field is.
func ReadConfirmed(dir string) (map[string]ConfirmedBy, error)
```

`WritePlaceholders(out, sources)`:

1. Keeps the existing refusal (an `--out` holding a real alias's manifest).
2. Calls `ReadConfirmed(out)` **before** writing anything; an error aborts the run with no files touched (the same all-or-nothing posture as the refusal).
3. For each generated source: `if cb, ok := ledger[source]; ok { fx.Unconfirmed = false; fx.ConfirmedBy = &cb }` else the entry stays `Unconfirmed: true, ConfirmedBy: nil`.
4. Ledger keys whose source is **not** in this run's `sources` (e.g. `--source opencode` with the four-entry ledger) are validated but not emitted — the manifest is regenerated wholesale from `sources` today and that does not change. (The committed corpus is always generated with the default all-six set, so the committed manifest always covers every ledger key; the corpus test enforces that.)
5. `PlaceholderDerivedFrom` and every other manifest header field are unchanged.

`tudiff placeholder` per-source stdout line gains the confirmation state so the operator sees the merge happen:

```
claude daily --json  placeholder (confirmed: dev-ws-sahil02 2026-09-16)  -> harness/fixtures/_placeholder/claude/daily.json
opencode daily --json  placeholder (unconfirmed)  -> harness/fixtures/_placeholder/opencode/daily.json
```

(`tudiff` is developer tooling under `bin/harness/`, not a shipped surface; its stdout is not covered by Output Stability.)

### 4. Corpus test: manifest and ledger MUST agree

In `src/go/internal/harness/corpus_test.go` `TestCorpus`, for the `_placeholder` alias only:

- Read `<dir>/confirmed.json` via `ReadConfirmed` (a missing file is an empty ledger; a malformed one fails the test).
- For every fixture entry: `fx.Unconfirmed == !listed` and, when listed, `*fx.ConfirmedBy == ledger[fx.Source]` (all three fields); when not listed, `fx.ConfirmedBy == nil`. Failure message names the fix: `… manifest disagrees with confirmed.json — re-run tudiff placeholder`.
- Every ledger key MUST have a manifest entry for `(source, "daily")`: a source confirmed in the ledger but absent from the corpus is a stale ledger and fails.

For every **other** alias (a local real capture): `fx.ConfirmedBy == nil` — extending the existing `unconfirmed: true occurs only under _placeholder/` assertion to the new field.

Unit tests (in `placeholder_test.go` / `manifest_test.go` / `cmd/tudiff/main_test.go`):

- `TestWritePlaceholders` gains the no-ledger case explicitly (all six unconfirmed, no `confirmed_by` key in the bytes).
- New `TestWritePlaceholdersMergesLedger`: write a `confirmed.json` listing `codex` into the temp out dir, generate `codex,opencode`, assert codex is `unconfirmed:false` with the ledger's `ConfirmedBy` and opencode is `unconfirmed:true`, `ConfirmedBy == nil`; assert the manifest bytes contain `"confirmed_by"` exactly once.
- New `TestReadConfirmedRejects…` table: unknown source, missing machine, bad date, unknown field, malformed JSON → non-nil error naming the problem; missing file → empty map, nil error.
- New round-trip test that a manifest with `ConfirmedBy` set survives `WriteManifest`/`ReadManifest` and that `ConfirmedBy == nil` omits the key.
- `TestPlaceholderWrites` in `cmd/tudiff` asserts the `(confirmed: …)` / `(unconfirmed)` stdout suffix.

### 5. Regenerate the committed corpus

Run `bin/harness/tudiff placeholder` (default `--out`) after seeding the ledger and commit the regenerated `harness/fixtures/_placeholder/manifest.json`. Fixture bytes and every `sha256` are unchanged; only `captured_at` and the four flipped entries differ. `just go-test` must pass with the committed pair.

### 6. Plan bookkeeping

`fab/plans/sahil/26-09-15-go-port.md`: fill row P3c's **PR** column with `260916-di0s-placeholder-confirmation-ledger` and its **Status** with a short landed line (`ledger + confirmed_by landed; 4/6 confirmed`); add P3c to the G0 row's "After" list (`Phase 0 (P0, P1, P2, P3a, P4, P3c)`) to match the queue block, which already lists it. No other plan edits.

### Non-goals

- Confirming opencode or copilot — that remains P3b, by a human on a machine with data. This change makes their eventual confirmation a one-line ledger edit plus a regeneration.
- Any change to `tudiff run`, `UnconfirmedReplays`, the report format, `fakeccusage`, `fakegit`, `matrix.json`, or `Capture`.
- A new `just` recipe for regenerating placeholders (`bin/harness/tudiff placeholder` after `just harness-build` is the documented path; Assumption 11).
- Any edit under `src/node/`, `dist/`, the formula, CI workflows, or any surface listed under the plan's Goal.

## Affected Memory

- `harness/differential-harness`: (modify) — **Manifest schema v1** requirement gains the optional per-entry `confirmed_by` object (`machine`, `date`, `ccusage_version`) and the invariant that it is present exactly when `unconfirmed: false` on a `_placeholder` entry; **tudiff placeholder** requirement replaces "`unconfirmed: true` on every entry" with the ledger merge (`<out>/confirmed.json`, validation rules, the no-ledger fallback, the stdout suffix); **Corpus validation test** requirement gains the manifest/ledger agreement assertions and the `confirmed_by`-only-under-`_placeholder` check; **Fixture corpus layout** requirement lists `confirmed.json` as the one hand-written file under `_placeholder/`; Design Decision "Real empty captures are confirmed; only `_placeholder/` is unconfirmed" is refined and a new Design Decision "Confirmation is a committed ledger merged by the generator, never a manifest edit" is added with the rejected alternatives from § Why.

No spec change: `docs/specs/usage.md` and `layouts.md` describe tu's external contract, which this change does not touch.

## Impact

- **New files**: `harness/fixtures/_placeholder/confirmed.json` (committed, ~22 lines).
- **Modified Go** (all under `src/go/`, stdlib only): `internal/harness/manifest.go` (+`ConfirmedBy` type, +1 field), `internal/harness/placeholder.go` (+`ReadConfirmed`, merge in `WritePlaceholders`), `cmd/tudiff/main.go` (stdout suffix in `runPlaceholder`), and their `_test.go` siblings plus `internal/harness/corpus_test.go`.
- **Regenerated**: `harness/fixtures/_placeholder/manifest.json` (four entries flip, `captured_at` moves; fixture bytes and `sha256` values unchanged).
- **Docs**: `docs/memory/harness/differential-harness.md` (hydrate), `fab/plans/sahil/26-09-15-go-port.md` (row P3c, gate G0).
- **Downstream behaviour**: `tudiff run` report stops flagging cases that replay claude/codex/gemini/kimi placeholders as `[unconfirmed]`; the summary's "cases replayed unconfirmed fixtures" count drops to the opencode/copilot cases. No code change there — it reads the manifest.
- **Not touched**: `src/node/`, `dist/`, `Formula/`, `.github/workflows/`, `harness/matrix.json`, `harness/metrics-repo/`, the fakes, `Capture`, any external surface under the plan's Goal.
- **Tests to run**: `just go-test` (scoped first: `cd src/go && go test ./internal/harness/ ./cmd/tudiff/ -count=1`), then `just go-lint`.

## Open Questions

- None blocking. Whether the ledger's `ccusage_version` should be required to equal `PlaceholderVersion` is recorded as Assumption 10 (informational only) and can be tightened in `/fab-clarify` or later without changing the file shape.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Ledger path, shape, and seed: `harness/fixtures/_placeholder/confirmed.json`, `{"<source>": {"machine","date","ccusage_version"}}`, seeded with claude/codex/gemini/kimi as `dev-ws-sahil02` / `2026-09-16` / `20.0.19` | Given verbatim by the user and by plan rows P3b/P3c; the seed values are the facts P3b recorded | S:95 R:90 A:95 D:95 |
| 2 | Certain | Generator reads the ledger only from `<out>/confirmed.json`; a missing file yields all-unconfirmed output identical to today | Same-directory placement is the only reading of "committed under `_placeholder/`"; the fallback keeps every existing `--out <tmp>` test green and is the D7 default | S:85 R:90 A:95 D:90 |
| 3 | Confident | `confirmed_by` is an object mirroring the ledger entry (`*ConfirmedBy`, `omitempty`), placed after `unconfirmed`; `SchemaVersion` stays 1 | User said "a confirmed_by field"; an object preserves all three provenance fields, `omitempty` keeps real-capture manifests byte-identical, and an optional additive field does not warrant a schema bump | S:70 R:80 A:85 D:75 |
| 4 | Confident | Ledger validation: keys must be in `DefaultSources`, all three fields required, `date` is `YYYY-MM-DD`, unknown fields rejected; any violation aborts the generator before writing | A typo in a hand-edited file is the realistic failure; `code-quality.md` forbids swallowing errors; all-or-nothing matches the existing real-alias refusal | S:60 R:90 A:85 D:80 |
| 5 | Confident | Ledger keys outside this run's `--source` subset are validated but not emitted; the corpus test (not the generator) enforces that every ledger key has a committed manifest entry | The manifest is already regenerated wholesale from `sources`; the committed corpus is always the default all-six set, so the invariant belongs with the committed pair | S:60 R:85 A:80 D:75 |
| 6 | Confident | Corpus test asserts, under `_placeholder` only: `unconfirmed == !listed`, `confirmed_by` deep-equals the ledger entry when listed and is absent otherwise, every ledger key has an entry; under other aliases `confirmed_by` is always absent | Direct reading of "the corpus test asserts manifest and ledger agree" plus the existing only-`_placeholder`-is-unconfirmed invariant | S:80 R:90 A:90 D:85 |
| 7 | Confident | `UnconfirmedReplays`, the `tudiff run` report, the fakes, and `Capture` are untouched | They read `fx.Unconfirmed` from the manifest and get the new state for free; changing them would widen scope past the row | S:70 R:90 A:95 D:90 |
| 8 | Confident | `tudiff placeholder` stdout gains a `(confirmed: <machine> <date>)` / `(unconfirmed)` suffix per source | Makes the merge observable to the human running P3b; `tudiff` is unshipped developer tooling, so Output Stability does not apply | S:50 R:95 A:85 D:70 |
| 9 | Confident | Plan bookkeeping in this change: fill P3c's PR/Status and add P3c to G0's "After" list; `PlaceholderDerivedFrom` unchanged | P2/P3a set the precedent for filling the row in the landing change; the queue block already lists P3c under Phase 0, so G0's list is simply stale; per-source provenance now lives in `confirmed_by`, so the corpus-wide sentence needs no edit | S:65 R:100 A:90 D:85 |
| 10 | Confident | Ledger `ccusage_version` is recorded, not required to equal `PlaceholderVersion` | The plan asks for the field but not for a policy; a confirmation on another release is still a key-path confirmation; tightening later is a one-line test change — the weakest-signal row here, worth a look in `/fab-clarify` | S:40 R:90 A:55 D:45 |
| 11 | Confident | No new `just harness-placeholder` recipe | Not requested; `just harness-build` + `bin/harness/tudiff placeholder` is the documented two-step path; adding a recipe is scope beyond the row | S:35 R:95 A:60 D:50 |
| 12 | Certain | Change type is `feat` (the keyword inference landed on `fix` from the prose in § Why and was overridden) | The row adds a new committed artifact and generator behaviour; the corpus-test edit is a consequence, not the change | S:80 R:100 A:95 D:90 |

12 assumptions (7 certain, 5 confident, 0 tentative, 0 unresolved).
