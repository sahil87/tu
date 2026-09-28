# Intake: Output Drop-at-Cutover Fixes

**Change**: 260928-lfj9-output-drop-at-cutover-fixes
**Created**: 2026-09-28

## Origin

> Output shape and heading drop-at-cutover fixes (DC-01, 07, 08, 13, 14, 15, 24) + record keeps DC-05, 06, 11, 16 (docs/specs/usage.md § Drop at cutover; backlog [o89t]).

Conversational. In a `/fab-discuss` session the user reviewed the 24 `[DECIDE: keep|drop]` markers with the agent's keep/drop lean and said "wherever you have strong recommendations — go ahead with those". The strong set was split into three changes: sync (DC-18, 21, 22, 23 — shipped as PR #103, change 260928-ubws), this output-shapes/headings change, and a CLI/watch change (DC-04, 09, 17). DC-02, 03, 10, 12, 19, 20 stay open. This change also records the four strong **keeps** (DC-05, 06, 11, 16) in the ledger.

## Why

The Go binary reproduced the Node output byte-for-byte, including seven output quirks that look accidental. Go is the only codebase now, so each drop is an ordinary behavior change (next minor release, Output Stability):

- **DC-01** — snapshot `--json` key sets depend on data presence: tools with no usage omit `label` (and `machines` under `--by-machine`), so every consumer must special-case missing keys.
- **DC-07** — `lb` is the only table heading without the `📊 ` prefix.
- **DC-08** — under `lbh --top n` the folded `others` column sorts by its own total and can render first; an aggregate "everyone else" bucket belongs last.
- **DC-13** — an `--until`-only leaderboard window renders the heading `· → 2026-09-10 ·` with an empty left side.
- **DC-14** — the single-mode guard says `Error: lb requires multi mode …` even when the user ran `lbh`.
- **DC-15** — a single-source snapshot (`tu cc`) is titled `📊 Combined Usage (daily)` although it shows one tool; single-source histories are titled by the tool.
- **DC-24** — snapshot numeric columns are fixed at 12 chars, so a value like `16,809,796,832` overflows its cell and shifts that row out of alignment with the header and dividers.

The keeps are deliberate contracts, not accidents: DC-05 (CSV snapshot rows are keyed by tool name, so omitting zero rows is safe, while the history CSV is positional and must keep every column), DC-06 (each format serves a different reader: ANSI hides noise, Markdown drops only exact zeros, CSV is complete data), DC-11 (share/delta are fractions, not money — trimmed 3-decimal precision is right), DC-16 (CSV/Markdown column order must be stable across runs — alphabetical — while the ANSI table is a ranking).

## What Changes

### DC-01 — stable snapshot JSON key set (`src/go/internal/render/json/snapshot.go`, `src/go/internal/command/run.go`)

Every tool object in snapshot `--json` carries `label` first, for tools with and without data, valued with the current period's label (the same label a tool with data carries, e.g. `"2026-09-16"`, `"2026-09"`, week-start Sunday for weekly). This includes the single-mode daily path where `run.go` (~L402) currently clears every row's `Label` — that clear goes away for JSON (check which other encoders rely on it; the ANSI table does not print labels). Under `--by-machine`, every tool object carries a trailing `machines` object — `{}` when the tool has no slices.

```json
{
  "Claude Code": { "label": "2026-09-16", "totalCost": 12.3, …, "machines": { "devbox": 12.3 } },
  "Codex":       { "label": "2026-09-16", "totalCost": 0, …, "machines": {} }
}
```

### DC-07 — `📊 ` on the leaderboard heading (`src/go/internal/view/leaderboard.go` ~L111)

`📊 Leaderboard ({period}) · {window} · by {cost|tokens}`, matching every other heading. Check the Markdown leaderboard heading follows whatever convention the other Markdown headings use (Markdown headings drop the emoji today — keep that consistent; only the ANSI heading changes if so).

### DC-08 — `others` column always last in `lbh --top` (`src/go/internal/command/leaderboard.go` ~L340–385 and wherever the lbh columns are ordered by total)

The `others` series is excluded from the descending-total column sort and always renders as the last user column (before the row total). CSV/Markdown already place it after the alphabetical users — unchanged.

### DC-13 — one-sided leaderboard window headings (`src/go/internal/command/leaderboard.go` ~L52–66)

`--until`-only: `{window}` becomes `until 2026-09-10`; for symmetry `--since`-only becomes `since 2026-09-01`; a two-sided window stays `2026-09-01 → 2026-09-10`. Δ semantics are unchanged (an `--until`-only window still has no previous window, so every row is `new` — that part of DC-13 is correct and stays).

### DC-14 — guard names the invoked command (`src/go/internal/command/run.go` ~L106, `cmd/tu/main.go` ~L274)

`Error: lbh requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo` when the command was `lbh`; `lb` keeps its message. `ErrLeaderboardMode` becomes a function or a typed error carrying the command token; exit code 1 unchanged.

### DC-15 — single-source snapshot heading names the tool (`src/go/internal/view/snapshot.go` ~L73, `compact.go` ~L44, Markdown snapshot)

A single-source snapshot (`tu cc`, `tu codex`, …) is titled `📊 {Tool} Usage ({period})` (e.g. `📊 Claude Code Usage (daily)`); the all-tools snapshot keeps `📊 Combined Usage ({period})`. The watch compact snapshot and the Markdown snapshot heading follow the same rule. `view.Snapshot` needs the source to know which title applies (option field or caller-supplied title — follow the existing `SnapshotOptions` pattern).

### DC-24 — data-sized snapshot numeric columns (`src/go/internal/view/snapshot.go` ~L18–20 and the column builder)

Each numeric column's width is `max(12, widest rendered cell in that column across header, data and Total rows)`; the Tool column stays 12. With normal values the table is byte-identical (87 chars); only a wider value widens its column, keeping header, dividers and rows aligned. Follow the existing data-sizing pattern the machine columns use (`machineWidth`).

### Keeps — ledger only (DC-05, DC-06, DC-11, DC-16)

In `docs/specs/usage.md` § Drop at cutover, mark these four `[DECIDED: keep]` with a `Now:` line giving the one-line reason from § Why. Their spec lines elsewhere keep describing the behavior (they may keep the `(DC-NN)` cross-reference).

### Spec, ledger, goldens

- `docs/specs/usage.md` and `docs/specs/layouts.md`: rewrite every line carrying `(DC-01)`, `(DC-07)`, `(DC-08)`, `(DC-13)`, `(DC-14)`, `(DC-15)`, `(DC-24)` to the new behavior, drop those tags; mark each of the seven ledger entries `[DECIDED: drop]` with a `Now: … (dropped in 260928-lfj9-output-drop-at-cutover-fixes).` line — the same format change 260928-ubws used for DC-18/21/22/23.
- Regenerate package goldens (`go test ./internal/render/{ansi,json,csv,markdown} -update`, `./internal/watch -update` if the compact title changes) and the harness corpus (`bin/tudiff --update`, run + live), then re-run the gates without `--update`.

## Affected Memory

- `render/json`: (modify) stable snapshot key set — label always, machines always under --by-machine
- `view/snapshot`: (modify) single-source title, data-sized numeric columns
- `view/leaderboard`: (modify) 📊 heading, one-sided window labels, others column last
- `command/guards`: (modify) the leaderboard-mode guard message names lb or lbh (or `command/run-and-result` / `command/entry-point` if that is where ErrLeaderboardMode is documented)
- `render/csv-and-markdown`: (modify) only if the Markdown snapshot heading or lbh others placement text changes

## Impact

- Code: `src/go/internal/{view,render/json,command}` and `cmd/tu/main.go`, with tests and goldens.
- Harness: many `harness/golden/run/*` cases (snapshot JSON, lb headings, single-source snapshots, lbh --top, lb/lbh single-mode guard).
- Specs: `docs/specs/usage.md`, `docs/specs/layouts.md`.
- Output Stability: JSON shape and table headings change ⇒ next release is a minor bump. No release in this change.

## Open Questions

- None blocking.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop DC-01, 07, 08, 13, 14, 15, 24; keep DC-05, 06, 11, 16 | Discussed — user said go ahead with the agent's strong recommendations; this is that set | S:95 R:80 A:95 D:95 |
| 2 | Confident | Zero-usage JSON tools get the current period label; machines is {} when a tool has no slices | A stable key set is the point of DC-01; the period label is known regardless of data | S:80 R:75 A:80 D:75 |
| 3 | Confident | The single-mode daily label clear stops applying to JSON | Otherwise DC-01's always-label rule fails on the most common invocation | S:75 R:75 A:75 D:70 |
| 4 | Confident | Single-source snapshot title is `📊 {Tool} Usage ({period})` | Mirrors `Combined Usage`; single-source history titles already name the tool | S:80 R:90 A:80 D:75 |
| 5 | Confident | One-sided windows read `since X` / `until X`; two-sided stays `X → Y` | DC-13 names the until-only case; since-only has the same empty-side problem | S:75 R:90 A:80 D:70 |
| 6 | Confident | Numeric column width = max(12, widest cell); normal output byte-identical | Fixes DC-24 without changing any typical layout | S:80 R:85 A:85 D:80 |
| 7 | Confident | `others` excluded from the total sort and rendered last | An aggregate bucket is not a ranked user | S:85 R:90 A:85 D:85 |
| 8 | Confident | lbh guard message names lbh; exit 1 unchanged | DC-14 is only about the wording | S:90 R:90 A:90 D:90 |
| 9 | Certain | Ledger format matches change 260928-ubws (`[DECIDED: drop]` / `[DECIDED: keep]` + Now line) | Consistency with the already-shipped resolutions | S:90 R:95 A:90 D:90 |
| 10 | Certain | No release in this change; next release is a minor bump | Constitution Output Stability | S:90 R:95 A:90 D:95 |

10 assumptions (3 certain, 7 confident, 0 tentative, 0 unresolved).
