# Plan: Placeholder Confirmation Ledger

**Change**: 260916-di0s-placeholder-confirmation-ledger
**Intake**: `intake.md`

## Requirements

### Harness: Confirmation ledger

#### R1: Committed per-alias ledger `confirmed.json`
`harness/fixtures/_placeholder/confirmed.json` SHALL be a committed JSON object keyed by ccusage source name, each value an object with exactly the fields `machine` (non-empty string), `date` (`YYYY-MM-DD`), and `ccusage_version` (non-empty string). It MUST be seeded with `claude`, `codex`, `gemini`, `kimi`, each `{"machine": "dev-ws-sahil02", "date": "2026-09-16", "ccusage_version": "20.0.19"}`, keys in registry order, 2-space indent, trailing newline. `.gitignore` MUST NOT exclude it (the existing `!harness/fixtures/_placeholder/` negation already covers it).

- **GIVEN** the committed repository
- **WHEN** `harness/fixtures/_placeholder/confirmed.json` is decoded
- **THEN** it has exactly the keys `claude`, `codex`, `gemini`, `kimi`, each with the three fields above

#### R2: `ReadConfirmed` loads and validates the ledger
`harness.ReadConfirmed(dir string) (map[string]ConfirmedBy, error)` in `src/go/internal/harness/placeholder.go` MUST read `<dir>/confirmed.json` (`const ConfirmedLedgerFile = "confirmed.json"`). A missing file MUST return an empty (non-nil) map and no error. It MUST return an error, naming the file and the problem, when: the JSON is malformed; a key is not in `DefaultSources`; an entry lacks `machine`, `date`, or `ccusage_version`; `date` does not match `^\d{4}-\d{2}-\d{2}$`; or an entry carries an unknown field (`json.Decoder.DisallowUnknownFields`).

- **GIVEN** a directory with no `confirmed.json`
- **WHEN** `ReadConfirmed` is called
- **THEN** it returns an empty map and `nil`
- **GIVEN** a ledger `{"opencod": {"machine":"m","date":"2026-09-16","ccusage_version":"20.0.19"}}`
- **WHEN** `ReadConfirmed` is called
- **THEN** it returns an error containing `unknown source "opencod"`
- **GIVEN** a ledger whose `claude` entry has `"date": "16-09-2026"`
- **WHEN** `ReadConfirmed` is called
- **THEN** it returns an error containing `claude` and `YYYY-MM-DD`

#### R3: Manifest schema carries optional `confirmed_by`
`Fixture` in `src/go/internal/harness/manifest.go` MUST gain `ConfirmedBy *ConfirmedBy \`json:"confirmed_by,omitempty"\`` as its last field, after `Unconfirmed`, where `ConfirmedBy` is a struct with `Machine` (`machine`), `Date` (`date`), `CcusageVersion` (`ccusage_version`) in that order. `SchemaVersion` MUST remain `1`. A `Fixture` with `ConfirmedBy == nil` MUST serialize with no `confirmed_by` key (real-capture manifests are byte-identical to today); a non-nil value MUST round-trip through `WriteManifest`/`ReadManifest`.

- **GIVEN** a `Fixture` with `ConfirmedBy: nil`
- **WHEN** written via `WriteManifest`
- **THEN** the bytes contain no `confirmed_by`
- **GIVEN** a `Fixture` with `ConfirmedBy: &ConfirmedBy{"dev-ws-sahil02","2026-09-16","20.0.19"}`
- **WHEN** written and read back
- **THEN** the read `Fixture` deep-equals the original and the bytes contain `"confirmed_by": {` followed by `"machine"`, `"date"`, `"ccusage_version"` in that order

#### R4: Generator merges the ledger into the manifest
`WritePlaceholders(out, sources)` MUST call `ReadConfirmed(out)` after the existing real-alias refusal and before writing any file; a ledger error MUST abort with that error and no files written or modified. For each generated source listed in the ledger the manifest entry MUST have `unconfirmed: false` and `confirmed_by` equal to the ledger entry; for each unlisted source it MUST have `unconfirmed: true` and no `confirmed_by`. Ledger keys not in this run's `sources` are validated but produce no entry. `PlaceholderDerivedFrom` and all other header fields are unchanged. Fixture bytes and `sha256` values are unaffected by the ledger.

- **GIVEN** `<out>/confirmed.json` listing `codex`
- **WHEN** `WritePlaceholders(out, ["codex","opencode"])` runs
- **THEN** the manifest's codex entry is `unconfirmed: false` with the ledger's `confirmed_by`, the opencode entry is `unconfirmed: true` with no `confirmed_by`, and the bytes contain `"confirmed_by"` exactly once
- **GIVEN** no `<out>/confirmed.json`
- **WHEN** `WritePlaceholders` runs
- **THEN** every entry is `unconfirmed: true` and the manifest bytes contain no `confirmed_by` (today's output)
- **GIVEN** a malformed `<out>/confirmed.json`
- **WHEN** `WritePlaceholders` runs against an empty `out`
- **THEN** it returns an error and `out` contains no `manifest.json` and no fixture files

#### R5: `tudiff placeholder` reports confirmation state per source
`runPlaceholder` in `src/go/cmd/tudiff/main.go` SHALL print, per source, `<source> daily --json  placeholder (confirmed: <machine> <date>)  -> <out>/<file>` for a ledger-listed source and `<source> daily --json  placeholder (unconfirmed)  -> <out>/<file>` otherwise. It MUST obtain the state from the manifest `WritePlaceholders` just wrote (read back via `ReadManifest`), not by re-reading the ledger, so stdout reflects what was actually written.

- **GIVEN** `--out <tmp>` with a `confirmed.json` listing `opencode` as `box-a` / `2026-09-20`
- **WHEN** `tudiff placeholder --source opencode --source copilot --out <tmp>` runs
- **THEN** stdout contains `opencode daily --json  placeholder (confirmed: box-a 2026-09-20)` and `copilot daily --json  placeholder (unconfirmed)` and exit is 0

#### R6: Corpus test asserts manifest and ledger agree
`TestCorpus` in `src/go/internal/harness/corpus_test.go` MUST, for the `_placeholder` alias, load the ledger via `ReadConfirmed` (a malformed ledger fails the test) and assert for every fixture entry: `Unconfirmed == !listed`; when listed, `*ConfirmedBy` equals the ledger entry; when unlisted, `ConfirmedBy == nil`; and every ledger key has a manifest entry with `Period == "daily"`. Failure messages MUST tell the reader to re-run `tudiff placeholder`. For every other alias it MUST assert `ConfirmedBy == nil` (alongside the existing `Unconfirmed`-only-under-`_placeholder` check). The test MUST pass against the committed `_placeholder/` pair.

- **GIVEN** the committed `confirmed.json` (four sources) and a `manifest.json` still carrying `unconfirmed: true` for `claude`
- **WHEN** `go test ./internal/harness/ -run TestCorpus` runs
- **THEN** it fails naming `_placeholder/claude/daily.json` and `confirmed.json`

#### R7: Regenerated committed manifest
`harness/fixtures/_placeholder/manifest.json` MUST be regenerated with `tudiff placeholder` (default flags) after the ledger is seeded and committed alongside it: the four listed sources are `unconfirmed: false` with `confirmed_by`, `opencode` and `copilot` remain `unconfirmed: true`, and every `sha256` is unchanged from the current manifest.

- **GIVEN** the regenerated manifest
- **WHEN** compared against `git show HEAD:harness/fixtures/_placeholder/manifest.json`
- **THEN** only `captured_at` and the four flipped entries differ; the six `sha256` values are identical

#### R8: Plan bookkeeping
`fab/plans/sahil/26-09-15-go-port.md` MUST have row P3c's PR column set to `260916-di0s-placeholder-confirmation-ledger` and its Status column to a short landed line, and gate G0's "After" cell MUST read `Phase 0 (P0, P1, P2, P3a, P4, P3c)`. No other plan edits.

- **GIVEN** the edited plan
- **WHEN** row P3c and the G0 row are read
- **THEN** both reflect the values above and `git diff --stat` for the plan shows only those two lines changed

### Non-Goals

- Confirming opencode or copilot — remains P3b, by a human on a machine with data.
- Changes to `tudiff run`, `UnconfirmedReplays`, the report, `fakeccusage`, `fakegit`, `matrix.json`, or `Capture`.
- A `just harness-placeholder` recipe.
- Validating the ledger's `ccusage_version` against `PlaceholderVersion`.
- Any edit under `src/node/`, `dist/`, `Formula/`, `.github/workflows/`, or any surface under the plan's Goal.

### Design Decisions

#### Confirmation is a committed ledger merged by the generator, never a manifest edit
**Decision**: Human confirmation of a placeholder shape lives in `harness/fixtures/_placeholder/confirmed.json`; `WritePlaceholders` merges it into `manifest.json` (`unconfirmed: false` + `confirmed_by`), and the corpus test asserts the two agree.
**Why**: `manifest.json` is generated and rewritten wholesale, so a hand flip is reverted on the next regeneration and the `sha256` guard cannot see it. A separate human-owned input keeps the manifest a pure function of fixtures plus ledger, so regeneration is idempotent with respect to confirmations and a stale pair fails CI.
**Rejected**: Hand-editing the manifest and never regenerating (the failure this removes); encoding confirmation in the corpus-wide `derived_from` string (cannot carry per-source state, also overwritten); `--confirmed` flags on `tudiff placeholder` (state would live in shell history, not the repo); bumping `schema` to 2 (the field is optional and additive; real-capture manifests and the fake ccusage are unaffected).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

#### Ledger errors abort before any write
**Decision**: `ReadConfirmed` runs before the first file write and any error (malformed JSON, unknown source, missing/invalid field, unknown field) aborts `WritePlaceholders` with nothing touched.
**Why**: The ledger is hand-edited; a typo is the realistic failure, and half-written output would be worse than a loud refusal. Matches the existing all-or-nothing real-alias refusal.
**Rejected**: Warn-and-continue (the misspelled source silently stays unconfirmed, which is exactly the state the ledger exists to record).
*Introduced by*: 260916-di0s-placeholder-confirmation-ledger

## Tasks

### Phase 1: Core Implementation

- [x] T001 Add `ConfirmedBy` struct and the `Fixture.ConfirmedBy *ConfirmedBy` (`confirmed_by,omitempty`) field after `Unconfirmed` in `src/go/internal/harness/manifest.go`; in `src/go/internal/harness/manifest_test.go` extend `TestManifestRoundTrip` to assert the bytes contain no `confirmed_by` for the nil case and add `TestManifestConfirmedByRoundTrip` (non-nil value round-trips; key order `machine`, `date`, `ccusage_version`) <!-- R3 -->
- [x] T002 In `src/go/internal/harness/placeholder.go` add `ConfirmedLedgerFile`, `ReadConfirmed(dir)` with the R2 validation rules, and the R4 merge in `WritePlaceholders` (read ledger after the refusal, before any write; set `Unconfirmed`/`ConfirmedBy` per source); in `src/go/internal/harness/placeholder_test.go` add `TestReadConfirmed` (table: missing file → empty map; malformed; unknown source; missing field; bad date; unknown field), `TestWritePlaceholdersMergesLedger` (R4 scenario 1, `"confirmed_by"` exactly once in the bytes), `TestWritePlaceholdersLedgerErrorWritesNothing` (R4 scenario 3), and extend `TestWritePlaceholders` to assert no `confirmed_by` in the no-ledger bytes <!-- R2, R4 -->
- [x] T003 In `src/go/cmd/tudiff/main.go` `runPlaceholder`, read the written manifest back with `harness.ReadManifest(*out)` and print the R5 per-source suffix; in `src/go/cmd/tudiff/main_test.go` add `TestPlaceholderReportsConfirmation` (R5 scenario) and keep `TestPlaceholderWrites` passing <!-- R5 -->
- [x] T004 In `src/go/internal/harness/corpus_test.go` `TestCorpus`, add the R6 assertions: load the ledger for `_placeholder` via `ReadConfirmed`, check `Unconfirmed == !listed`, `ConfirmedBy` equality/nil, every ledger key present with `Period == "daily"`, and `ConfirmedBy == nil` for every other alias; failure messages end with `— re-run tudiff placeholder` <!-- R6 -->

### Phase 2: Corpus and bookkeeping

- [x] T005 Write `harness/fixtures/_placeholder/confirmed.json` with the R1 seed; run `just harness-build && bin/harness/tudiff placeholder` to regenerate `harness/fixtures/_placeholder/manifest.json` and verify every `sha256` is unchanged against `git show HEAD:…`; run `cd src/go && go test ./internal/harness/ ./cmd/tudiff/ -count=1` then `just go-test` and `just go-lint`; edit `fab/plans/sahil/26-09-15-go-port.md` row P3c (PR + Status) and gate G0's "After" cell <!-- R1, R7, R8 -->

## Acceptance

### Functional Completeness

- [x] A-001 R1: `harness/fixtures/_placeholder/confirmed.json` is committed with exactly `claude`, `codex`, `gemini`, `kimi`, each `dev-ws-sahil02` / `2026-09-16` / `20.0.19`, and `git check-ignore` does not match it
- [x] A-002 R2: `ReadConfirmed` returns an empty map for a missing file and an error for malformed JSON, unknown source, missing field, bad date, and unknown field, each error naming `confirmed.json` and the offending source/field
- [x] A-003 R3: `Fixture.ConfirmedBy` is a `*ConfirmedBy` with `omitempty`, `SchemaVersion` is still 1, and the nil case serializes with no `confirmed_by` key
- [x] A-004 R4: `WritePlaceholders` sets `unconfirmed: false` + `confirmed_by` for ledger-listed sources and leaves unlisted sources unchanged from today's output
- [x] A-005 R5: `tudiff placeholder` stdout carries `(confirmed: <machine> <date>)` or `(unconfirmed)` per source, sourced from the written manifest
- [x] A-006 R6: `TestCorpus` asserts manifest/ledger agreement under `_placeholder` and `ConfirmedBy == nil` under every other alias
- [x] A-007 R7: The committed `manifest.json` has four `unconfirmed: false` entries with `confirmed_by`, two `unconfirmed: true` entries without, and all six `sha256` values unchanged
- [x] A-008 R8: Plan row P3c (PR, Status) and gate G0's "After" cell are updated and nothing else in the plan changed

### Behavioral Correctness

- [x] A-009 R4: With no `confirmed.json` present, `WritePlaceholders` output is byte-identical to the pre-change output apart from `captured_at` (existing `TestWritePlaceholders`, `TestPlaceholderDefaultsToAllSources`, `TestPlaceholderWrites` pass unmodified in intent)
- [x] A-010 R3: A real-capture manifest written by `Capture` (no `ConfirmedBy`) serializes byte-identically to before (`TestManifestRoundTrip` still passes with the original bytes)

### Scenario Coverage

- [x] A-011 R4: `TestWritePlaceholdersMergesLedger` covers the listed + unlisted mix and asserts `"confirmed_by"` appears exactly once
- [x] A-012 R6: Flipping one committed entry's `unconfirmed` back to `true` by hand makes `TestCorpus` fail naming that fixture and `confirmed.json` (verified once locally during T004/T005, then reverted)

### Edge Cases & Error Handling

- [x] A-013 R4: A malformed `confirmed.json` aborts `WritePlaceholders` before any file is written (`TestWritePlaceholdersLedgerErrorWritesNothing`)
- [x] A-014 R2: An unknown-field entry (e.g. `ccusage_verison`) is rejected rather than silently dropped
- [x] A-015 R4: Ledger keys outside the `--source` subset are validated but produce no manifest entry and no error

### Code Quality

- [x] A-016 Pattern consistency: new code follows the package's existing shape — stdlib only, `tudiff:`-prefixed error strings, doc comments on exported identifiers, `_test.go` siblings, no `__tests__/` under `src/go/`
- [x] A-017 No unnecessary duplication: `ReadManifest`/`WriteManifest`/`DefaultSources`/`Summarize` are reused; no second JSON manifest reader or source list
- [x] A-018 No swallowed errors: every ledger failure surfaces on stderr with exit 1 through the existing `runPlaceholder` error path
- [x] A-019 Minimum pathways: `runPlaceholder` derives its stdout from the written manifest (one source of truth), not from a second ledger read
- [x] A-020 Named constants: the ledger filename is `ConfirmedLedgerFile`, not a repeated literal
- [x] A-021 `gofmt -l` is clean and `go vet ./...` passes (`just go-lint`)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)
- If an item is not applicable, mark checked and prefix with **N/A**: `- [x] A-NNN **N/A**: {reason}`

## Deletion Candidates

None — this change adds new functionality without making existing code redundant. The old per-source stdout loop in `runPlaceholder` was replaced in place by the manifest read-back loop; no surviving symbol, file, or branch became unused.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Confident | `runPlaceholder` reads the written manifest back for its stdout suffix rather than threading the ledger through a new return value | Keeps `WritePlaceholders`'s signature and every existing caller/test unchanged; one source of truth for what was written | S:55 R:95 A:85 D:75 |
| 2 | Confident | `ReadConfirmed` returns `map[string]ConfirmedBy` (values, not pointers); the merge takes the address of a loop-local copy | Value map is simpler to compare in tests with `==`; the pointer only exists on `Fixture` for `omitempty` | S:50 R:95 A:90 D:80 |
| 3 | Confident | The `date` check is a plain `^\d{4}-\d{2}-\d{2}$` regexp, not `time.Parse` | Matches the intake's stated rule; the field is provenance, and a calendar-valid check adds no safety the corpus test needs | S:60 R:95 A:85 D:75 |
| 4 | Certain | Tasks total five, so the change runs in the light lane | The intake sizes the row S; five focused tasks map one-to-one onto the requirement groups | S:80 R:100 A:95 D:90 |

4 assumptions (1 certain, 3 confident, 0 tentative).
