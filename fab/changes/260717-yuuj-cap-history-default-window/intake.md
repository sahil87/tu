# Intake: Cap History Default Window

**Change**: 260717-yuuj-cap-history-default-window
**Created**: 2026-07-17

## Origin

Created via promptless dispatch (`/fab-proceed`-style deferred questioning) from a synthesized user conversation. The user requested:

> Cap the default date window of `tu` history displays (`h`/`dh`/`wh` — daily and weekly history) to the last 3 calendar months, with an escape-hatch flag to show full history. Monthly history (`mh`) is exempt. The cut should be "from the 1st of a month".

Key decisions were settled conversationally before dispatch (cap boundary, scope, filter interaction, heading hint, version bump, client-side implementation approach). One decision was left open by the user — the escape-hatch flag name (`--all` proposed by user, `--full` leaned by assistant) — and is resolved here as a Confident SRAD assumption (see Assumptions #5).

## Why

1. **Pain point**: Daily and weekly history displays print the *entire* usage history by default. As the metrics corpus grows (multi-machine sync accumulates months of JSONL day-files), `tu h` / `tu dh` / `tu wh` output grows unboundedly — dozens to hundreds of rows scroll past for what is usually a "how have recent weeks looked?" question.
2. **Consequence of not fixing**: Default output keeps degrading — the most relevant (recent) rows are pushed to the bottom of an ever-longer table, and the Total row becomes an ever-less-meaningful all-time figure. Users would have to remember to type `--since` manually on every invocation.
3. **Why this approach**: A *defaulted* `sinceFlag` reuses the exact `--since`/`--until` machinery shipped in change 260703-sntl (PR #40) — one code path, zero new filtering logic, cache behavior untouched. Monthly history stays exempt because it is already compact (one row per month) and serves as the long-term view; capping it to 3 rows would gut its purpose.

**Alternatives rejected** (from the conversation):

- **Passing `--since` through to ccusage**: extra ccusage args bypass the 60s fetch cache in `fetchHistory`, and pass-through cannot filter multi-mode remote entries read from the metrics repo. Rejected — same rationale as the shipped since/until design (cache uniformity, "minimum pathways" per code-quality.md).
- **Capping monthly history (`mh`)**: monthly is one compact row per month; a 3-row cap would destroy its value as the long-term view. Rejected.

## What Changes

### 1. Implicit 3-month cap on daily/weekly history

When ALL of the following hold:

- display is `history` (`h`/`dh`/`wh`/`history` forms),
- period is `daily` or `weekly` (NOT `monthly`),
- no explicit `--since` AND no explicit `--until` was given,
- the escape-hatch flag (`--full`) is absent,

then `sinceFlag` is defaulted to the **first day of the month two months back** — i.e. the window covers 3 calendar months *including* the current month. Example: on 2026-07-17 the implicit floor is `2026-05-01` (May, June, July). The user explicitly wanted the cut "from the 1st of a month".

**Injection point**: `src/node/core/cli.ts` `main()`, adjacent to the existing since/until history-only warn-and-clear guard (currently lines 1330–1338) — the single spot where `display`, `period`, `sinceFlag`, and `untilFlag` are all known before dispatch. Pseudocode:

```ts
// Implicit 3-month cap: daily/weekly history only, no explicit window, no --full
if (display === "history" && period !== "monthly"
    && sinceFlag === undefined && untilFlag === undefined && !fullFlag) {
  sinceFlag = threeMonthFloor(); // e.g. "2026-05-01" on 2026-07-17
  capActive = true;              // drives the heading hint
}
```

The floor computation uses the **local** system date (labels are local usage days): `YYYY-MM-01` where the month is the current local month minus 2 (year rollover handled, e.g. on 2026-01-15 → `2025-11-01`).

Downstream, the defaulted `sinceFlag` flows through the existing plumbing unchanged: dispatch functions pass it to `filterEntriesByRange` (`src/node/core/fetcher.ts:311`) applied to merged daily entries *before* weekly aggregation — exactly as an explicit `--since` does today. Caching is unaffected: the full dataset is fetched/cached, the filter is applied afterward. Multi-mode remote entries are filtered too (same call sites: cli.ts:464, 486, 504, 826, 834, 971, 1119, 1128).

**Inherited semantics** (identical to explicit `--since`, deliberately — one code path): under weekly aggregation, daily entries are filtered before `aggregateWeekly`, so the week containing the floor may appear as a partial leading week labeled with its Sunday (e.g. a `2026-04-26` row containing only May 1–2 data). No week-boundary snapping.

### 2. New escape-hatch flag: `--full`

A new boolean global flag `--full` (long-only, no short alias) disables the cap and shows full history.

- **Parsing**: added to `parseGlobalFlags` (`src/node/core/cli.ts:609`) — a new `fullFlag: boolean` field on `GlobalFlags`, stripped in the same pass as the other boolean flags (line 634 strip list).
- **`tu h --full`** → full daily history, no cap, no heading hint.
- **`tu mh --full`** → vacuous no-op (monthly is never capped; output is already full history). No warning — the flag's request ("show full history") is satisfied, not ignored.
- **`--full` on snapshot display** (e.g. `tu --full`, `tu m --full`) → warn-and-ignore on stderr, mirroring the existing since/until guard: `Warning: --full applies to daily/weekly history — ignoring.` Printed once in `main()` (not inside dispatch), so watch mode warns once at startup.
- **`--full` combined with `--since`/`--until`** → silently accepted (redundant, not contradictory — both express "no implicit cap"; the explicit window still applies).

<!-- assumed: none — flag name resolved as Confident assumption #5 below -->

### 3. Explicit `--since` OR `--until` disables the cap entirely

Any explicit `--since` or `--until` means the user chose their own window — the implicit floor is NOT intersected with it. Rationale (from the conversation): otherwise `tu h --until <past-date>` would intersect with the implicit floor and silently return nothing. This falls out naturally from the injection condition (`sinceFlag === undefined && untilFlag === undefined`).

### 4. Heading hint when the cap is active

When the implicit cap is active, the history table heading indicates it with the text `last 3 months`:

- Single-tool history (`renderHistory`, `src/node/tui/formatter.ts:104`): `📊 Claude Code (daily)` → `📊 Claude Code (daily, last 3 months)`
- Cross-tool pivot (`renderTotalHistory`, `src/node/tui/formatter.ts:310`): `📊 Combined Cost History (daily)` → `📊 Combined Cost History (daily, last 3 months)`

The user explicitly endorsed this: the default-output change means the Total row becomes a 3-month total instead of all-time, and "the hint should help there". The hint appears wherever the heading appears (table output and the Markdown emitter's `## {title}` heading); it does NOT appear when `--full` or explicit `--since`/`--until` is used (cap not active). Plumbing mechanism (a `FormatOptions` field vs. appending to the `period` string passed to renderers) is decided at plan generation — behavior above is the contract.

### 5. Help text, completions, and version bump

- **`FULL_HELP`** (`src/node/core/cli.ts:74`): new Flags line for `--full` (e.g. `--full                Show full history (default: last 3 months for daily/weekly history)`), plus a note on the `--since`/`--until` lines or Display line if natural. The help-dump/shll.ai contract picks up new help text automatically (raw FULL_HELP passthrough — additive, no drift concern).
- **Completions** (`src/node/core/completions.ts`): add `--full` to the bash `long_flags`, zsh flag list (with description `show full history (no 3-month cap)`), and fish `complete -c tu -l full` lines.
- **Version**: per the constitution's Output Stability rule, this default-output change requires a **minor version bump: 0.7.0 → 0.8.0** (`package.json`).

### 6. Tests

Per the constitution (co-located `__tests__/` folders) and code-review policy (output changes SHOULD include test coverage):

- `cli-parser.test.ts`: `--full` parsing (stripped from filteredArgs, `fullFlag` set), floor-computation cases (mid-year, year rollover — e.g. on 2026-01-15 → `2025-11-01`).
- Dispatch/filter behavior: cap applied for daily/weekly history; NOT applied for monthly history, snapshot display, explicit `--since`/`--until`, or `--full`.
- `cli-help.test.ts` / `completions.test.ts`: `--full` present in FULL_HELP and all three shells' completions.
- Formatter: heading hint present when cap active, absent otherwise.

## Affected Memory

- `cli/data-pipeline`: (modify) document the implicit 3-month default window on daily/weekly history, the `--full` escape hatch, and the explicit-window-disables-cap rule alongside the existing `--since`/`--until` documentation
- `display/formatting`: (modify) document the `(…, last 3 months)` heading hint on history tables when the cap is active

## Impact

- **`src/node/core/cli.ts`**: `GlobalFlags` interface + `parseGlobalFlags` (new `fullFlag`), `main()` cap-injection logic adjacent to the since/until guard (~line 1330), floor-computation helper, `FULL_HELP` flags block, warn-and-ignore guard for `--full` on snapshot.
- **`src/node/core/fetcher.ts`**: no changes expected — `filterEntriesByRange` is reused as-is.
- **`src/node/tui/formatter.ts`**: heading hint in `renderHistory` and `renderTotalHistory` (and the shared MD-emit title path).
- **`src/node/core/completions.ts`**: `--full` in bash/zsh/fish scripts.
- **`src/node/core/__tests__/`**: cli-parser, cli-help, completions, dispatch filter behavior; formatter tests under `src/node/tui/__tests__/` if heading tests live there.
- **`package.json`**: version 0.7.0 → 0.8.0 (minor bump, Output Stability rule).
- **`docs/specs/usage.md`**: flags/grammar prose mentions the new default window and `--full` (human-curated spec; flag for update at hydrate/ship).
- **Downstream consumers**: `tu h --json` default output now covers 3 months instead of all-time — covered by the minor bump. help-dump/shll.ai refreshes automatically from FULL_HELP.
- **Watch mode**: history displays in watch mode inherit the cap uniformly (same dispatch pathway); watch's existing `maxRows` truncation is unaffected.

## Open Questions

- None — all decision points graded Confident or higher (see Assumptions; the flag-name choice `--full` over the user-proposed `--all` is Assumption #5 and can be revisited via `/fab-clarify` before apply if desired).

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Cap floor = first day of the month two months back (3 calendar months including current); e.g. 2026-05-01 on 2026-07-17 | Discussed — user explicitly wanted the cut "from the 1st of a month" and confirmed the 3-months-including-current example | S:90 R:85 A:90 D:90 |
| 2 | Certain | Cap applies to daily and weekly history only; monthly history (`mh`) exempt | Discussed — user agreed monthly is the compact long-term view; capping it to 3 rows would gut its purpose | S:90 R:85 A:90 D:95 |
| 3 | Certain | Any explicit `--since` OR `--until` disables the implicit cap entirely (no intersection) | Discussed — agreed; otherwise `tu h --until <past-date>` would intersect the implicit floor and silently return nothing | S:90 R:85 A:90 D:90 |
| 4 | Certain | Escape hatch is a new boolean global flag that disables the cap; implemented client-side as a defaulted `sinceFlag` at the cli.ts guard seam, reusing `filterEntriesByRange` | Discussed — agreed approach; mirrors shipped 260703-sntl design (cache uniformity, multi-mode filtering, minimum pathways); codebase confirms the seam exists | S:85 R:80 A:90 D:90 |
| 5 | Confident | Flag is named `--full` (long-only), not the user-proposed `--all` | User proposed `--all`; assistant flagged the cognitive collision with the positional source token `all` (`tu all dh --all` — same word, two meanings) and leaned `--full`; user did not settle it. Trivially renameable pre-ship (one string across cli.ts/help/completions/tests) | S:45 R:90 A:55 D:50 |
| 6 | Certain | Heading hint `last 3 months` shown in the history table heading when (and only when) the cap is active | Discussed — user explicitly endorsed the hint with example text `(last 3 months)`; it explains the Total row becoming a 3-month total | S:85 R:90 A:85 D:85 |
| 7 | Certain | Minor version bump 0.7.0 → 0.8.0 | Constitution Output Stability rule: breaking default-output changes MUST carry a minor bump; agreed in conversation | S:85 R:80 A:95 D:95 |
| 8 | Confident | Cap applies uniformly across output formats (table/JSON/CSV/MD) and watch-mode history — single pathway via the defaulted `sinceFlag` | Not discussed per-format, but the mechanism (one defaulted flag through existing dispatch) and code-quality.md "minimum pathways" imply uniform behavior; format-specific carve-outs would add paths | S:60 R:80 A:85 D:80 |
| 9 | Confident | Weekly aggregation inherits exact explicit-`--since` semantics: daily entries filtered before aggregation, so a partial leading week (labeled with its Sunday, possibly before the floor) may appear | Identical code path as explicit `--since` today; introducing week-boundary snapping only for the implicit cap would fork semantics between implicit and explicit windows | S:50 R:80 A:80 D:70 |
| 10 | Confident | `--full` on snapshot display warns-and-ignores on stderr (house pattern, mirrors since/until guard); `--full` on `mh` is a silent vacuous no-op; `--full` + explicit `--since`/`--until` silently accepted | House pattern for inapplicable flags is warn-and-ignore (`-u`, `--by-machine`, `--since`/`--until` guards); on `mh` the request "show full history" is satisfied — a warning saying "ignoring" would be false | S:40 R:85 A:75 D:65 |
| 11 | Confident | Floor computed from the local system date (not UTC) | Usage labels are local-day based; "the last 3 months" should mean the user's calendar. Trivial to change | S:45 R:85 A:75 D:75 |
| 12 | Confident | Hint plumbing (a `FormatOptions` field vs. appending to the `period` string handed to renderers) is deferred to plan generation; the behavioral contract is the heading text above | Pure implementation detail with two workable options and no user-visible difference beyond the agreed heading; apply decides-and-records | S:35 R:90 A:65 D:45 |

12 assumptions (6 certain, 6 confident, 0 tentative, 0 unresolved).
