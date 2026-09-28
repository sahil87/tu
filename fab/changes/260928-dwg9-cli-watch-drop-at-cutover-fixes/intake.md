# Intake: CLI and Watch Drop-at-Cutover Fixes

**Change**: 260928-dwg9-cli-watch-drop-at-cutover-fixes
**Created**: 2026-09-28

## Origin

> CLI and watch drop-at-cutover fixes (DC-04, DC-09, DC-17 in docs/specs/usage.md § Drop at cutover; backlog [o89t]).

Conversational. In a `/fab-discuss` session the user reviewed the 24 `[DECIDE: keep|drop]` markers with the agent's keep/drop lean and said "wherever you have strong recommendations — go ahead with those". The strong set was split into three changes: sync (DC-18, 21, 22, 23 — PR #103, change 260928-ubws), output shapes/headings (DC-01, 07, 08, 13, 14, 15, 24 + keeps DC-05, 06, 11, 16 — PR #104, change 260928-lfj9), and this CLI/watch change. DC-02, 03, 10, 12, 19, 20 stay open.

## Why

- **DC-04** — `--help`/`-h` works only as the first argument (or after `update`): `tu cc --help` and `tu h -h` exit 2 with `Unknown argument`. Asking for help after typing part of a command is the most natural moment to ask for it; failing it is hostile, and it is the common CLI convention.
- **DC-09** — `--version`/`-V`/`-v` are missing from the `--help` text; the lowercase `-v` alias is discoverable only from memory docs and completions. Toolkit principle №3 makes help a published contract (and `help-dump` republishes it on hexokit.com), so an undocumented flag is a contract gap.
- **DC-17** — watch mode does not guard lines wider than the terminal: the single-tool history (110 chars) at 100 columns, or `lbh` with many users, wraps inside the frame. The compositor counts one terminal row per line, so every wrapped line desynchronizes its accounting — the footer, rain zone, and redraw overwrite the wrong rows and the screen corrupts. This is a real bug.

## What Changes

### DC-04 — `--help`/`-h` anywhere (`src/go/internal/command/parse.go` ~L48–132)

Today the help check is `len(scan.positionals) > 0 && helpCommands[scan.positionals[0]]`. New rule: if **any** positional is `-h` or `--help`, `Parse` returns the help command (full help, exit 0) before any other guard (it already precedes the `--dry-run` guard). The bare word `help` stays first-position-only (`tu cc help` is still an unknown argument — `help` could be confused with a positional token). Using `scan.positionals` (not raw argv) means a token consumed as a value-taking flag's value is not treated as help. This covers non-data commands too — `tu init-metrics --help`, `tu sync --help` print help and do nothing (help must never mutate). `runUpdate`'s own `--help` probe (`cmd/tu/main.go` ~L515) becomes redundant if Parse catches it first — keep exactly one path (the update standard's `--skip-brew-update` probe still gets the full help, which carries the flag).

```
$ tu cc --help      # full help, exit 0 (was: Unknown argument: --help, exit 2)
$ tu h -h           # full help, exit 0
$ tu sync --help    # full help, exit 0, no sync
```

### DC-09 — list `--version` in the help (`src/go/internal/command/request.go` `FullHelp`)

Add a line to the `Flags:` block, aligned with the others: `  --version / -V / -v  Print the version and exit`. Update the `Help:` line if it enumerates forms (`tu help | tu -h | tu --help` — note `-h`/`--help` now work after a command). `help-dump` picks the text up automatically (it embeds `FullHelp`). Check `docs/site/skill.md` (the embedded `tu skill` bundle) and README for a flags list that should gain the line, per the skill standard.

### DC-17 — clip frame lines to the terminal width (`src/go/internal/watch/compositor.go`)

Every stats and table line written into the frame MUST be clipped to the terminal's column count before `Frame` emits it, so no line ever wraps and the one-row-per-line accounting holds. Clipping is ANSI-aware: escape sequences are copied through without counting toward width, visible characters stop at `cols`, and a clipped line that was inside an SGR sequence is closed with a reset (`\x1b[0m`). Measure width the same way the compositor measures `maxContentWidth` today (visible runes after `StripANSI`), but count wide runes (the `📊` emoji and other East-Asian-wide characters) as 2 columns if the codebase or x/term offers a width helper — otherwise document the limit. The skeleton frame goes through the same clip. Lines that fit are byte-identical to today.

### Spec, ledger, goldens

- `docs/specs/usage.md` (~L87–88 flag table, ~L458 watch) and `docs/specs/layouts.md` (~L197, ~L235, ~L566–567, and the verbatim help block) — rewrite to the new behavior and drop the `(DC-04)`/`(DC-09)`/`(DC-17)` tags; mark the three ledger entries `[DECIDED: drop]` with a `Now: … (dropped in 260928-dwg9-cli-watch-drop-at-cutover-fixes).` line — the format changes 260928-ubws/lfj9 used.
- Regenerate goldens: help text appears in harness `run/*help*` cases and `help-dump` output; watch goldens (`go test ./internal/watch -update`) if a case exercises a wide line; then `bin/tudiff --update` (run + live) and re-run every gate without `--update`.

## Affected Memory

- `command/request-and-parse`: (modify) help tokens recognized anywhere among positionals; `help` word still first-only
- `toolkit/version-and-help-dump`: (modify) the help text now lists --version/-V/-v
- `toolkit/update`: (modify) if runUpdate's own --help probe is removed in favor of Parse
- `toolkit/standards-audit`: (modify) the -v alias is now documented in help
- `watch/compositor` (or whichever watch memory file documents Lay/Frame): (modify) lines clipped to terminal width

## Impact

- Code: `src/go/internal/command/{parse,request}.go`, `cmd/tu/main.go`, `src/go/internal/watch/compositor.go` (+ skeleton), tests and goldens.
- Harness: help cases, help-dump, unknown-argument cases that used `--help` after a positional, watch cases.
- Docs: `docs/specs/usage.md`, `docs/specs/layouts.md`, possibly `docs/site/skill.md`, README.
- Output Stability: help text and argument handling change ⇒ next release is a minor bump. No release in this change.

## Open Questions

- None blocking.

## Assumptions

| # | Grade | Decision | Rationale | Scores |
|---|-------|----------|-----------|--------|
| 1 | Certain | Drop DC-04, DC-09, DC-17 | Discussed — user said go ahead with the agent's strong recommendations; these three were listed as drop | S:95 R:80 A:95 D:95 |
| 2 | Confident | `-h`/`--help` anywhere among positionals print help; the bare word `help` stays first-only | Flag-style help is the universal convention; a bare word mid-command is ambiguous | S:80 R:85 A:80 D:75 |
| 3 | Confident | Help wins over every other guard and never runs a command, including non-data commands | Help must never mutate; it already precedes the --dry-run guard | S:85 R:85 A:85 D:80 |
| 4 | Confident | Help line text: `--version / -V / -v  Print the version and exit` | Matches the existing `--flag / -x  Description` layout | S:80 R:95 A:85 D:80 |
| 5 | Confident | Watch clips every frame line ANSI-aware to the terminal width; fitting lines byte-identical | Clipping keeps the one-row-per-line invariant; wrapping is what corrupts the frame | S:85 R:80 A:80 D:80 |
| 6 | Tentative | Wide runes (📊) count as 2 columns if a width helper is available, else document the limit | No width helper confirmed in-tree yet; headings are short, so the edge rarely matters | S:60 R:85 A:60 D:60 |
| 7 | Certain | Ledger format matches changes 260928-ubws and 260928-lfj9 | Consistency with the other resolutions | S:90 R:95 A:90 D:90 |
| 8 | Certain | No release in this change; next release is a minor bump | Constitution Output Stability | S:90 R:95 A:90 D:95 |

8 assumptions (3 certain, 4 confident, 1 tentative, 0 unresolved).
