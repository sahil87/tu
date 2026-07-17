# Plan: Toolkit Standards Conformance

**Change**: 260717-rdo3-toolkit-standards-conformance
**Intake**: `intake.md`

## Requirements

Requirements are grouped by the surface each governs. RFC-2119 keywords are used
throughout. IDs are stable across the change.

### Standards Audit: Runtime enumeration

#### R1: Audit against the runtime-enumerated standard set
The audit SHALL be performed against the standards `shll standards` serves at
apply time, not against memory or website copies, and SHALL record the `shll`
version it ran against.

- **GIVEN** `shll standards` exits 0 and lists the governing standards
- **WHEN** the audit runs
- **THEN** each listed standard is fetched via `shll standards <name>` and audited, and `shll version`'s `shll` row is recorded in the audit record
- **AND** if `shll standards` is unavailable, the audit stops and reports rather than proceeding from memory

### help-dump: Machine-readable help contract (mechanical)

#### R2: The help-dump envelope MUST NOT carry `captured_at`
`tu help-dump` MUST emit an envelope of exactly `{tool, version, schema_version, root}`.
The `captured_at` field MUST NOT be emitted — its ownership belongs to shll.ai's
puller, which stamps it after capture.

- **GIVEN** the current help-dump standard (shll v0.0.23) forbids `captured_at`
- **WHEN** `tu help-dump` runs
- **THEN** the parsed JSON envelope has keys exactly `{tool, version, schema_version, root}` and no `captured_at`
- **AND** a pinning test asserts `captured_at` is absent

#### R3: help-dump satisfies the full conformance checklist
`tu help-dump` MUST exit 0, write valid JSON to stdout only with empty stderr,
carry a `version` reflecting the built binary (not a literal), emit
`schema_version: 1`, and exclude `completion`/`help`/hidden commands from the tree.

- **GIVEN** the standard's "Verifying conformance" checklist
- **WHEN** `tu help-dump` is exercised against the built binary
- **THEN** every checklist item passes (exit 0, stdout-only valid JSON, empty stderr, built-binary version, `schema_version: 1`, no hidden/`completion`/`help` nodes)
- **AND** a minimal test pins exit 0, valid JSON, and the expected `tool`/`schema_version`

### readme-extraction: README + docs/site structure (mechanical)

#### R4: The command-reference link uses the canonical URL form
The README's generated-command-reference link MUST use the exact form
`https://shll.ai/tu/commands/` (the standard's `https://shll.ai/<tool>/commands/`),
not any other path prefix.

- **GIVEN** the standard rule 8 (`README is the hub`) and the intake's exact URL
- **WHEN** the README links the command reference
- **THEN** the link target is `https://shll.ai/tu/commands/`
- **AND** the previously-shipped `https://shll.ai/tools/tu/commands/` (extra `/tools/` segment) no longer appears

#### R5: README prose matches the shipped CLI flags (rule 7)
The README SHOULD NOT mention commands or flags that do not exist, and its
curated Flags block SHOULD reflect the shipped flag surface so a reader is not
misled about the tool's capabilities.

- **GIVEN** the README `### Flags` block predates several shipped flags (`--csv`, `--md`, `--since`/`-s`, `--until`, `--full`, `-j`)
- **WHEN** the README is spot-checked against `tu --help`/`tu help-dump`
- **THEN** every flag the README lists exists in the shipped CLI (no phantom flags), and the block is updated to include the shipped format/window flags it omits

#### R6: The remaining readme-extraction checklist items stay conformant
The README head order, footer boundary, image absoluteness, absence of
mermaid/`#gh-*-mode-only` fragments, docs/site closure, and reserved-name rules
MUST remain conformant.

- **GIVEN** the standard's full "Verifying conformance" checklist
- **WHEN** the README and `docs/site/**` are audited
- **THEN** head order (H1 → toolkit blockquote → badges → prose), no footer-name heading truncating site content, all images absolute, no mermaid fences, no `#gh-*-mode-only`, docs/site relative links resolve inside the tree, and no page named `overview`/`readme`/`commands` all hold

### principles: The ten toolkit CLI principles (foundation)

#### R7: Usage-error diagnostics route to stderr, not stdout (№2)
On a usage-error path, the usage hint is a diagnostic and MUST be written to
stderr, not stdout. Data the caller asked for goes to stdout; status, hints, and
errors go to stderr.

- **GIVEN** an invalid invocation (`tu bogusarg`, `tu cc bogus`, an unknown tool key)
- **WHEN** tu rejects it
- **THEN** the error line AND the accompanying usage hint (`SHORT_USAGE`) are both written to stderr, and stdout is empty
- **AND** the process still exits non-zero

#### R8: The remaining principles are assessed and conformant-or-dispositioned
Each of the ten principles MUST be assessed against tu's actual runtime behavior;
every gap MUST be dispositioned as fixed-here (small/additive) or deferred
(restructuring), per the intake's proportionality boundary.

- **GIVEN** the ten principles at shll v0.0.23
- **WHEN** each is assessed against the exercised CLI (help/version/error paths, stream split, exit codes, `--json`/`--no-color`, idempotency, output volume)
- **THEN** each principle is marked PASS or carries a dispositioned gap in the audit record (§ Notes)

### skill: Agent skill bundle (deferred)

#### R9: The skill standard is dispositioned "deferred, not yet adopted"
tu MUST NOT implement a `tu skill` subcommand in this change. The skill standard
is dispositioned "deferred, not yet adopted" per the toolkit's phased per-repo
rollout, and a backlog entry MUST track future adoption.

- **GIVEN** no `tu skill` subcommand and no `docs/site/skill.md` exist, and the standard states "No tool ships `skill` today"
- **WHEN** the skill standard is audited
- **THEN** the disposition recorded is "deferred, not yet adopted", a `fab/backlog.md` entry tracks adoption of `tu skill` + `docs/site/skill.md`, and no `tu skill` is implemented

### Governance: Constitution amendment

#### R10: The constitution carries the Toolkit Standards article
`fab/project/constitution.md` MUST gain a `### Toolkit Standards` subsection
(wt's consumer variant, verbatim) as the last subsection under
`## Additional Constraints` before `## Governance`, with the version bumped
1.0.0 → 1.1.0 and Last Amended set to 2026-07-18.

- **GIVEN** tu's constitution is at v1.0.0 with no Toolkit Standards article, while sibling repos wt/shll gained it today at v1.1.0
- **WHEN** the amendment is applied
- **THEN** the `### Toolkit Standards` subsection appears immediately before `## Governance`, the Governance line reads `**Version**: 1.1.0` with `**Last Amended**: 2026-07-18`, and Ratified is unchanged

### Deferred-gap and reporting conventions

#### R11: Deferred gaps are recorded as backlog entries referenced from the report
Every deferred gap (from any standard) MUST get a `fab/backlog.md` entry with a
fresh 4-char lowercase-alphanumeric ID and enough context to act on cold, and the
conformance report MUST reference each deferral by its ID.

- **GIVEN** the repo's backlog convention (`- [ ] [4char] YYYY-MM-DD: description`)
- **WHEN** a gap is deferred
- **THEN** a backlog entry with a fresh unique ID and actionable context exists, and the audit record (§ Notes) references it

#### R12: The audit record is complete enough for the ship stage to build the PR report
The plan's `## Notes` section MUST hold the full per-standard audit results — every
checklist item verdict, every principle verdict, each gap's disposition, the
`shll` version audited against, and the stale "tu exception" upstream-doc
observation — so the ship stage can assemble the PR-body conformance report from it.

- **GIVEN** the deliverable is a PR-body conformance report (one section per standard)
- **WHEN** the ship stage builds the report
- **THEN** all inputs (per-item verdicts, dispositions with backlog IDs, shll version, upstream-doc note) are present in § Notes

### Non-Goals

- Implementing `tu skill` or `docs/site/skill.md` — deferred (R9).
- Migrating usage-error exit codes from `1` to `2` — deferred (systematic, machine-observable contract change; see § Notes and backlog `8h6g`).
- Adding `--dry-run` to the sync mutation — deferred (feature addition; backlog `xuhk`).
- Recursing tu's real subcommands in help-dump — not needed; the flat contract is checklist-conformant (see § Notes; the standard's stale "tu exception" prose is an upstream doc issue, not a tu violation).
- Reworking caching for statelessness (№6) — the constitution's Article IV mandates TTL caching; no unilateral "fix" (see § Notes).

## Tasks

### Phase 1: Runtime audit (evidence gathering)

- [x] T001 Enumerate standards at runtime: `shll version` (record `shll` row), `shll standards`, then `shll standards <name>` for each entry; capture verbatim content as audit input <!-- R1 -->
- [x] T002 Build the binary (`npm run build`) and exercise the real CLI: `tu help-dump`, `tu --version`, `tu help`/`-h`, and error paths (unknown arg, unknown tool, unknown shell, incompatible flags, bad interval, bad dates) capturing stdout/stderr/exit separately <!-- R3 R7 R8 -->
- [x] T003 Record the full per-standard audit (every checklist item verdict, all ten principle verdicts, dispositions, shll version, upstream-doc note) into `## Notes` <!-- R1 R8 R12 -->

### Phase 2: Mechanical-contract fixes

- [x] T004 Remove `captured_at` from the help-dump producer at `src/node/core/help-dump.ts`: drop it from the `HelpDoc` interface, the `buildHelpDoc` return object, and the contract comment; envelope becomes exactly `{tool, version, schema_version, root}` <!-- R2 -->
- [x] T005 Update the pinning test `src/node/core/__tests__/help-dump.test.ts`: replace the two `captured_at` assertions with assertions that `captured_at` is ABSENT (in both the `buildHelpDoc` and `runHelpDump` suites) <!-- R2 R3 -->
- [x] T006 Update the local wrapper `scripts/help-dump.mjs`: remove the `captured_at` validation line so the wrapper's self-validation matches the new envelope (no drift) <!-- R2 -->
- [x] T007 Fix the README command-reference URL in `README.md`: `https://shll.ai/tools/tu/commands/` → `https://shll.ai/tu/commands/` <!-- R4 -->
- [x] T008 Update the README `### Flags` block in `README.md` to reflect the shipped flag surface — add the omitted `--csv`, `--md`, `--since`/`-s`, `--until`, `--full`, and the `-j` alias — without introducing any phantom flags <!-- R5 -->

### Phase 3: Principle fix (small, additive)

- [x] T009 Route the usage-error hint to stderr in `src/node/core/cli.ts`: change `console.log(SHORT_USAGE)` → `console.error(SHORT_USAGE)` on the two usage-error paths (the unknown-tool path in `dispatchSingleTool`, and the `parseDataArgs` catch in `main`) so both the error line and the usage hint land on stderr <!-- R7 -->

### Phase 4: Governance + deferrals + verification

- [x] T010 Amend `fab/project/constitution.md`: add `### Toolkit Standards` (wt's verbatim text) as the last subsection under `## Additional Constraints` before `## Governance`; bump `**Version**: 1.0.0` → `1.1.0` and `**Last Amended**: 2026-03-06` → `2026-07-18` (Ratified unchanged) <!-- R10 -->
- [x] T011 Add `fab/backlog.md` deferral entries: `[uch0]` (skill-standard adoption: `tu skill` + `docs/site/skill.md`, embedded/≤150-line/drift-guarded), `[8h6g]` (usage-error exit code `1` → `2` convention), `[xuhk]` (`--dry-run` for the sync mutation, №5) <!-- R9 R11 -->
- [x] T012 Run the test suite (`npm test`) in an isolated env (clean `HOME`, `TU_*` unset) and confirm green; if the command tree changed, re-verify the help-dump checklist afterward <!-- R2 R3 R7 -->

## Execution Order

- T001–T002 (evidence) precede T003 (audit record) and the fixes.
- T004 → T005 (test follows producer) → T006 (wrapper matches new envelope).
- T012 (verification) runs last, after all source/doc edits.
- T007, T008, T009, T010, T011 are independent of each other.

## Acceptance

### Functional Completeness

- [x] A-001 R1: The audit was run against the runtime `shll standards` set (4 standards at shll v0.0.23) and the shll version is recorded in § Notes
- [x] A-002 R2: `tu help-dump` emits an envelope with keys exactly `{tool, version, schema_version, root}` and no `captured_at`
- [x] A-003 R3: `tu help-dump` passes the full help-dump checklist (exit 0, stdout-only valid JSON, empty stderr, built-binary version, `schema_version: 1`, no hidden/`completion`/`help` nodes) with a pinning test
- [x] A-004 R4: The README command-reference link is exactly `https://shll.ai/tu/commands/` and the `/tools/` form is gone
- [x] A-005 R5: Every flag the README lists exists in the shipped CLI, and the Flags block includes the shipped format/window flags
- [x] A-006 R6: The remaining readme-extraction checklist items (head order, footer boundary, absolute images, no mermaid/gh-mode, docs/site closure, reserved names) all pass
- [x] A-007 R7: On usage-error paths, both the error line and the usage hint go to stderr; stdout is empty; exit is non-zero
- [x] A-008 R8: All ten principles are assessed with a PASS or a dispositioned gap recorded in § Notes
- [x] A-009 R9: No `tu skill` subcommand is implemented; the skill standard is dispositioned "deferred, not yet adopted" with a tracking backlog entry
- [x] A-010 R10: The constitution carries `### Toolkit Standards` before `## Governance`, Version 1.1.0, Last Amended 2026-07-18
- [x] A-011 R11: Each deferred gap has a `fab/backlog.md` entry with a fresh unique 4-char ID referenced from § Notes
- [x] A-012 R12: § Notes holds the complete per-standard audit record sufficient for the ship stage to build the PR conformance report

### Behavioral Correctness

- [x] A-013 R2: The help-dump pinning test asserts `captured_at` is absent (a change from the prior test that asserted its presence), and the `scripts/help-dump.mjs` self-validation no longer requires `captured_at`
- [x] A-014 R7: The stream-routing change is verified against the built binary (`tu bogusarg` writes nothing to stdout; the error + usage hint appear on stderr)

### Scenario Coverage

- [x] A-015 R3: The help-dump checklist was re-verified against the built binary after the `captured_at` removal (command-tree-touching change)
- [x] A-016 R10: The constitution's `### Toolkit Standards` text is byte-verbatim the wt consumer variant specified in the intake

### Edge Cases & Error Handling

- [x] A-017 R7: The `--json`/`--csv`/`--watch` incompatibility errors (already stderr-routed) remain unchanged; only the `SHORT_USAGE` hint routing is corrected

### Code Quality

- [x] A-018 Pattern consistency: New code follows the surrounding style — functional, `console.error` for stderr diagnostics, `type` imports, `node:` builtins (matches code-quality.md)
- [x] A-019 No unnecessary duplication: The `captured_at` removal touches only the three emit/validate paths; no logic is duplicated (minimum pathways per code-quality.md)
- [x] A-020 No swallowed errors: The stream-routing fix does not swallow any error — the error still prints (now to stderr) and still exits non-zero (code-quality.md anti-pattern check)
- [x] A-021 Test integrity: The updated help-dump test asserts the NEW spec (envelope without `captured_at`), conforming to the standard as source of truth — not bent to accommodate the old fixture (constitution Test Integrity)

## Notes

- Check items as you review: `- [x]`
- All acceptance items must pass before `/fab-continue` (hydrate)

### Audit record — for the ship-stage PR conformance report

**Audited against**: `shll v0.0.23` (`shll standards`, 2026-07-18). `shll version`
also reported wt v0.0.24, idea v0.0.15, tu v0.8.1, run-kit v3.7.2, hop v0.1.18,
fab-kit v2.15.6. `shll standards` listed exactly four standards: `principles`
(foundation), `help-dump` (binary), `readme-extraction` (repo), `skill`
(binary+repo) — matching the intake; no `shll update` needed.

**Output-Stability note (constitution)**: removing `captured_at` from the
help-dump JSON is a schema-visible change to a machine format. It is
standard-mandated (the current help-dump standard forbids emitting it) and
shll.ai is the only consumer (it re-stamps `captured_at` itself). Per the
intake, this is noted for the release flow to decide the version bump — this
change does NOT bump the tool version itself.

#### principles — 2 gaps (1 fixed, 1 deferred)

| № | Principle | Verdict | Disposition |
|---|-----------|---------|-------------|
| 1 | Non-interactive by default | PASS | tu has no interactive prompts of its own; watch-mode raw stdin is `isTTY`-gated (`watch.ts`); auto-clone sets `GIT_TERMINAL_PROMPT=0` (`cli.ts` checkMetricsDirGuard). Observation (not a violation): `init-metrics` clone and `sync.ts` git push/pull do not set `GIT_TERMINAL_PROMPT=0`, so a credential-less transport could theoretically block — but this is git's prompt, not tu's, and SSH-key auth is the norm; left as-is. |
| 2 | stdout is data, stderr is diagnostics | gap → fixed | Usage-error paths printed the `SHORT_USAGE` hint to **stdout** via `console.log` (`cli.ts` lines 963 + 1331) while the error line went to stderr — mixing a diagnostic into the data stream. Fixed here: both routed to stderr (`console.error`). Machine formats (`--json`) are stable and `schema_version`-versioned → conformant. |
| 3 | Help is a published contract | PASS | `tu help-dump` exists as a real in-binary subcommand emitting the JSON tree; help text is layered (summary → examples → flags). skill (the usage-briefing companion) is SHOULD and deferred (see skill section). |
| 4 | Fail fast with actionable errors | gap → deferred `[8h6g]` | Errors are detected early and print a message; BUT the toolkit exit-code convention is `0` success / `1` operational failure / `2` usage error, and tu uses `1` for BOTH operational failures and usage errors (unknown arg, unknown shell, incompatible flags, `-u` missing value, `--interval`/`--since`/`--until` errors — ~14 sites). Migrating usage errors to `2` is systematic, changes a machine-observable contract scripts branch on, and touches ~20+ existing test assertions pinning `exitCode === 1`; that exceeds the intake's "a[n] undocumented exit code made consistent" small-fix boundary and lands in restructuring → deferred to backlog `[8h6g]`. Error *wording* already carries what-failed + a usage hint. |
| 5 | Visible mutation boundaries | gap → deferred `[xuhk]` | Read vs. write is clear from names (`sync`/`--sync` mutate via git push; the default read-only display and `status`/`help-dump` are reads) — naming half PASSES. The `--dry-run` half is absent for the sync mutation; adding an accurate-preview `--dry-run` is a feature addition (restructuring the sync flow) → deferred to backlog `[xuhk]`. tu's sync is additive with a never-shrink write guard, so no destructive-consent gap exists. |
| 6 | Stateless, therefore retry-safe | PASS (with noted tension) | State is re-derived at request time; there is no state DB. tu caches ccusage fetches with a 60s TTL — the intake flags a №6-vs-Constitution-Article-IV tension (Article IV *mandates* TTL caching for fast startup). Per the intake, this tension is reported, not unilaterally "fixed": the cache is a bounded freshness optimization, not authoritative state, and sync/writeMetrics is idempotent (never-shrink guard, per-day whole-entry max). No fix. |
| 7 | Compose, don't reinvent | PASS | tu shells out to `ccusage` (`execFile`), `git` (sync), and `brew` (update) rather than reimplementing them. Capability is probed, not assumed: `tu update` passes `--skip-brew-update` only to the tap-refresh step by design, matching the cross-toolkit contract flag name. |
| 8 | Graceful degradation | PASS (with note) | Missing ccusage sources warn on stderr and fall back to zero data (`fetcher.ts`), never crash — constitution Article II. Missing metrics repo warns and falls back to single mode. Color is controllable via `--no-color` and honors `NO_COLOR` (`colors.ts`). Observation (not a violation): color is NOT auto-gated on `!stdout.isTTY`, so ANSI can leak into a pipe; but `NO_COLOR`/`--no-color` is the standard opt-out mechanism and auto-TTY-gating would be an output-stability change — left as-is. |
| 9 | Bounded, high-signal output | PASS | Daily/weekly history defaults to an implicit 3-month cap with an explicit "last 3 months" heading hint (silent truncation avoided); `--full` uncaps; monthly is one compact row per month. No `--quiet`, but output is already bounded and high-signal (no unbounded surface dumps ten-thousand-line output). |
| 10 | Agent-discoverable documentation | PASS (SHOULD; skill deferred) | README is structured for mechanical extraction and `docs/site/` renders on shll.ai (after the command-ref URL fix, see readme-extraction). The `<tool> skill` bundle — the most forward-leaning P10 obligation — is a SHOULD and phased; deferred (backlog `[uch0]`). |

#### help-dump — 1 gap (fixed)

- `captured_at` emitted in the envelope → **fixed here** (`src/node/core/help-dump.ts`, test `src/node/core/__tests__/help-dump.test.ts`, wrapper `scripts/help-dump.mjs`). Envelope is now exactly `{tool, version, schema_version, root}`.
- `tu help-dump` exits 0, valid JSON to stdout only, stderr empty → PASS (verified against the built binary).
- Envelope is `{tool, version, schema_version, root}` — no `captured_at` → PASS (after fix).
- `completion`, `help`, and all hidden commands absent from the tree → PASS (tu's tree is flat by design — `root.commands: []` — so there are no such nodes to filter; trivially satisfied).
- `version` reflects the built binary, not a literal → PASS (build-time `--define` `__PKG_VERSION__` = `0.8.1`; dev falls back to package.json).
- Minimal test pins exit 0 / valid JSON / expected `tool`+`schema_version` → PASS (retained; `captured_at` assertions inverted to absence).
- **Upstream-doc observation (report as a shll-repo doc issue, NOT a tu violation)**: the help-dump standard's "The `tu` exception" prose still describes tu as "flag-based with no subcommands," emitting `root.commands: []`. tu has since grown real subcommands (`shell-init`, hidden `help-dump`, plus the `init-conf`/`init-metrics`/`sync`/`status`/`update` non-data commands dispatched in `cli.ts`). Conformance is judged by the shape-agnostic invocation contract + verification checklist, which tu's flat document passes; tu's tree is NOT flattened to match the stale prose. Recursing tu's real subcommands is NOT done here (tu prints no per-subcommand `--help` pages, so there is no per-command `text` to emit — recursing would be a producer redesign, not a small additive fix; backlog `[v76l]` recorded "if tu ever grows real subcommands, recurse the same way" as future intent). The stale prose should be corrected in the shll repo's `docs/site/standards/help-dump.md`.

#### readme-extraction — 1 gap (fixed) + 1 additive doc fix

- Command-reference URL was `https://shll.ai/tools/tu/commands/` (extra `/tools/` segment) → **fixed here** to the canonical `https://shll.ai/tu/commands/` (standard rule 8 + intake §4).
- Rule 7 (command/flag accuracy): no phantom flags were mentioned (every README flag exists) → PASS; the curated `### Flags` block omitted several shipped flags (`--csv`, `--md`, `--since`/`-s`, `--until`, `--full`, `-j`). Not strictly a rule-7 violation (rule 7 flags *nonexistent* mentions), but the intake §4 invites fixing stale README mentions → **additive doc fix here** (block updated to the shipped surface).
- Head order (`#` H1 → toolkit blockquote (exact line) → contiguous badges → tagline prose) → PASS.
- Tail / footer boundary: no `Contributing`/`Development`/`Building`/`License`/`Acknowledgements` heading present; the final section (`## CI / branch protection`) is site-worthy and not a footer name → PASS.
- Relative targets: README relative links point only into `docs/site/` (`install.md`, `workflows.md`, both natural repo-relative — auto-rewritten); docs/site pages link only to each other relatively and use absolute `https://` for the external shll link → PASS (closure holds, no `..` escape).
- Images absolute everywhere: the README screenshot is an absolute `<img src="https://github.com/user-attachments/...">`; no relative images anywhere; no images in docs/site → PASS.
- No ```` ```mermaid ```` fences; no `#gh-dark-mode-only`/`#gh-light-mode-only` fragments → PASS.
- No `docs/site/` page named `overview`/`readme`/`commands` (current pages: `install.md`, `workflows.md`) → PASS.
- README cross-links its docs/site pages (natural relative) and the command reference (absolute, after fix) → PASS.

#### skill — deferred, not yet adopted

Phased per-repo adoption (no seven-repo flag-day); the standard itself states "No
tool ships `skill` today… A tool without a `skill` subcommand is not yet in
violation." Verified: no `tu skill` subcommand exists (the `cli.ts` non-data
command dispatch has no `skill` case) and no `docs/site/skill.md` exists. NOT
implemented in this change. Tracked for future adoption as backlog `[uch0]`
(`tu skill` + canonical `docs/site/skill.md`, embedded/≤150-line/byte-identical
drift-guarded per the standard's sync+drift-guard pattern).

### Deferred backlog IDs (referenced above)

- `[uch0]` — skill-standard adoption (`tu skill` + `docs/site/skill.md`).
- `[8h6g]` — usage-error exit code `1` → `2` (principle №4 convention).
- `[xuhk]` — `--dry-run` for the sync mutation (principle №5).

## Deletion Candidates

None — this change removes an emitted field (`captured_at`) and reroutes two diagnostics to stderr; it makes no existing code redundant or unused (the removal left no orphaned imports, helpers, or branches in `src/node/core/help-dump.ts`, `scripts/help-dump.mjs`, or `src/node/core/cli.ts`).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Envelope after removal is exactly `{tool, version, schema_version, root}`; the pinning test inverts from asserting `captured_at` present to asserting it absent | The standard specifies the exact envelope verbatim; the intake mandates the test update | S:95 R:90 A:95 D:95 |
| 2 | Certain | skill standard dispositioned "deferred, not yet adopted"; no `tu skill` implemented; backlog entry `[uch0]` tracks adoption | Task + standard both state this verbatim; verified no `tu skill`/`skill.md` exists | S:100 R:90 A:100 D:100 |
| 3 | Certain | Constitution gains `### Toolkit Standards` (wt verbatim) before `## Governance`, Version → 1.1.0, Last Amended → 2026-07-18 | Intake supplies the exact text and governance deltas; matches sibling repos | S:95 R:85 A:90 D:90 |
| 4 | Confident | The `SHORT_USAGE`-to-stdout misroute (№2) is fixed here (`console.log`→`console.error` at two sites) — a small, additive stream-routing fix with no test breakage | Matches the intake's in-scope example ("a status/hint line printed to stdout instead of stderr"); no test asserts SHORT_USAGE on stdout | S:80 R:85 A:80 D:80 |
| 5 | Confident | Usage-error exit code `1`→`2` (№4) is DEFERRED, not fixed here | Systematic across ~14 sites + ~20 test assertions; changes a machine-observable contract; exceeds the intake's single-undocumented-code small-fix boundary → restructuring | S:70 R:65 A:75 D:65 |
| 6 | Confident | The help-dump flat tree is NOT restructured to recurse real subcommands; the stale "tu exception" prose is reported as an upstream shll-repo doc issue | Checklist + invocation contract are shape-agnostic and pass; tu prints no per-subcommand help pages, so recursion is a producer redesign (defer intent recorded in `[v76l]`) | S:60 R:75 A:65 D:60 |
| 7 | Confident | The README `### Flags` block is updated to add the shipped flags it omits (`--csv`/`--md`/`--since`/`--until`/`--full`/`-j`) as an additive doc fix | Intake §4 invites fixing stale README mentions; low-effort and keeps the README accurate to the shipped CLI; no phantom flags introduced | S:65 R:90 A:70 D:65 |
| 8 | Confident | `--dry-run` absence on the sync mutation (№5) is DEFERRED (`[xuhk]`); the mutation-naming half passes | Adding an accurate-preview dry-run is a feature addition restructuring the sync flow — out of scope per the intake | S:70 R:80 A:75 D:70 |
| 9 | Confident | Tests are verified green in an ISOLATED env (clean HOME, `TU_*` unset); the dirty-dev-box `TU_METRICS_REPO` failures are pre-existing env flakes (backlog 2026-06-03), not this change's | Intake states "green on a clean env is the bar"; the isolated baseline was 769/769 green before edits | S:80 R:90 A:85 D:80 |

9 assumptions (3 certain, 6 confident, 0 tentative).
