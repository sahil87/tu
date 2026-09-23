---
type: memory
description: The terminal-table encoder — Colors as a value honoring --no-color/NO_COLOR, the SGR palette, the Table and CompactTable encoders (cells, bars, deltas, separators, footer), and the golden-file test contract
---

# ANSI Table Encoder

**Domain**: render

## Overview

`internal/render/ansi` turns a [view.Table or view.CompactTable](/view/table-model.md) into output lines with ANSI color, honoring `--no-color`/`NO_COLOR` through the `Colors` value. Encoders return `[]string`; nothing here writes to a stream — the [command edge](/command/run-and-result.md) prints the lines.

## Requirements

### Requirement: Colors is a value, computed once at the edge

`ansi.Colors{Enabled bool}` (`internal/render/ansi/color.go`) MUST carry the color-enabled decision as a value; the [command entry point](/command/entry-point.md) computes it once as `ansi.Colors{Enabled: !NoColor && os.Getenv("NO_COLOR") == ""}` and passes it down. The package reads no environment and probes no TTY: color is emitted whether or not stdout is a TTY, and `--no-color` or a non-empty `NO_COLOR` yields byte-identical uncolored output because every style method returns its input unchanged when `Enabled` is false.

### Requirement: The SGR palette

`internal/render/ansi/color.go` defines the style methods, each a wrap closed by `\x1b[0m`: `Bold` `\x1b[1m`, `Dim` `\x1b[2m`, `Green` `\x1b[32m`, `Red` `\x1b[31m`, `Cyan` `\x1b[36m`, `Yellow` `\x1b[33m`, `Magenta` `\x1b[35m`, `Blue` `\x1b[34m`, `BoldWhite` `\x1b[1;37m`, `BoldCyan` `\x1b[1;36m`, `BrightGreen` `\x1b[92m`, `DimGreen` `\x1b[2;32m`, reset `\x1b[0m`. `palette(slot)` maps a stacked-bar/legend slot to `Green`/`Magenta`/`Blue`/`Cyan` for slots 0–3 and an uncolored identity for slot ≥ 4; `Yellow` is excluded from the palette, reserved for the two-zone overflow.

### Requirement: Text primitives

`StripANSI` removes SGR sequences matching `\x1b\[[0-9;]*m`. `PadLeft`/`PadRight` pad to a width by **rune count** (snapshot text is ASCII; the 📊 title is never padded).

### Requirement: Table emits the line layout

`ansi.Table(t view.Table, c Colors) []string` (`internal/render/ansi/table.go`) MUST emit: `""`, `BoldWhite(title)`, `""`; when `Empty != ""`, the empty text and `""` — plus `Dim(Footer)` and `""` when `Footer != ""` (the leaderboard's staleness line on the empty state); else header cells padded per column then `BoldCyan`-wrapped **individually** and joined by unstyled `" | "`, `Dim` divider/separator rows of `"─"×width` per column joined by `"─|─"` plus `barDiv` (`"─" + "─"×Scale.Width` when `Scale.Width > 0`) — trailing the last column, or following the `BarAfter` column's dashes for the leaderboard's mid-row bar — then Data rows, the `Divider`+`Total` pair (each cell padded then `BoldWhite`-wrapped individually), `Dim(Footer)` with the legend appended, a blank line plus `Dim(Note)` when `Note != ""`, and a trailing `""`. Per-cell encoding on Data rows: pad first, then wrap — the first cell by its `LabelStyle` (`BoldWhite` for `Current`, `Dim` for `Weekend`), any `Dim` cell `Dim`-wrapped, a `Leader` cell `BoldWhite`-wrapped after padding and any `Dim` wrap (a zero leader cell is `BoldWhite(Dim(…))`). Header/Total cells are never dimmed. The [view model](/view/table-model.md) owns cell semantics.

#### Scenario: empty leaderboard still prints its footer
- **GIVEN** a `view.Table` with `Empty = "  No data"` and `Footer = "never synced · tu sync to refresh"`
- **WHEN** `ansi.Table` runs
- **THEN** the lines are `""`, the bold title, `""`, `"  No data"`, `""`, the dim footer, `""`

### Requirement: Deltas and bars compose per the model

A `Cell.Delta` composes per the table's `DeltaInCell` BEFORE any `Dim` wrap: `DeltaPadsArrow` pads text+arrow together (rune count over the colored string — the arrow's escape codes count toward the width, so a colored cell renders effectively unpadded) and `DeltaAfterPad` appends `" " + arrow` after the padded text; the `Dim` wrap covers the composite either way. A row-level `Row.Delta` follows the last cell — `Green("↑")`/`Red("↓")`, prefixed with `" "` only when `DeltaSpaced` (the pivot renders the arrow abutting the cell). The bar follows: single-zone `" " + fill(Main)` only when `Main != ""`; two-zone ALWAYS `" " + fill(Main) + pad + Dim("┊") + Yellow(Overflow) + pad`, including a zero row — the wrap has no empty-string guard, so `Green("")`/`Yellow("")` emit the bare escape pair `ESC[32mESC[0m` / `ESC[33mESC[0m`.

### Requirement: Mid-row bar area under BarAfter

When a column carries `BarAfter` (the leaderboard's mid-row bar) the bar area — exactly `1 + Scale.Width` visible characters — MUST render immediately after that column's cell on every non-divider row: Data rows get `" " + fill(Main) + spaces(Width − runes(Main))` single-zone (the two-zone form is already full-width), and Header/Total/bar-less rows get the unstyled `" " + spaces(Scale.Width)` gap outside the cell wraps. `fill` colors raw glyphs: solid `Green(Main)` when `Segments == nil`, else contiguous rune slices colored by `palette` slot (zero-count segments skipped) — stripping ANSI yields `Main` unchanged.

#### Scenario: colored in-cell delta defeats padding
- **GIVEN** a snapshot `Cost` cell with `Cell.Delta = DeltaUp`, color enabled, `DeltaInCell = DeltaPadsArrow`
- **WHEN** the row renders
- **THEN** the padded width is computed over `text + " ↑"` including the arrow's escape codes, so the colored cell renders effectively unpadded; with color disabled the same line pads to the full column width

### Requirement: CompactTable is the narrow-terminal form

`ansi.CompactTable(t view.CompactTable, c Colors) []string` (`internal/render/ansi/compact.go`) encodes the watch-mode narrow-terminal form ([watch loop](/watch/loop-and-terminal.md)): `""`, `BoldWhite(title)`, `""`; if `Empty != ""`, the empty text and `""`; else per row `PadRight(name, view.CompactNameWidth) + " " + PadLeft(value, view.CompactValueWidth)` — the widths are `view.CompactNameWidth = 14`, `view.CompactValueWidth = 12`, `view.CompactDivWidth = 27` (`internal/view/compact.go`) — with a row's delta arrow composed into the value BEFORE padding (the `DeltaPadsArrow` raw-length rule, so a colored arrow defeats the padding); when `Total != nil`, `Dim("─"×27)` then the `BoldWhite` Total line; trailing `""`.

### Requirement: Golden files pin every byte

Each encoder's tests compare against golden files under `internal/render/ansi/testdata/`, regenerated with `go test ./internal/render/ansi/ -update` (the flag is declared in `table_test.go`). Every colored snapshot/history/pivot/leaderboard golden is asserted equal to its no-color twin under `StripANSI`, and every bar row's stripped bar equals `Main` + padding + `┊` + `Overflow` + padding. The goldens pin color-on and color-off bytes, the two-zone rule and overflow, the mid-row bar, separators, the watch `MaxRows` windows, and every compact form; the differential harness ([harness](/harness/differential-harness.md)) adds the end-to-end byte check.

## Design Decisions

### Colors is a value, not a global
**Decision**: `ansi.Colors{Enabled}` is computed once at the command edge and passed to the renderers.
**Why**: a module-global color switch would leak the output channel into the render layer; a value keeps `render` pure and testable.
**Rejected**: a package-level `SetNoColor` global.
*Introduced by*: 260916-3am6-query-view-render-snapshot

### The palette leads green and excludes yellow
**Decision**: palette slots are `Green, Magenta, Blue, Cyan` assigned in visible-column order (slot ≥ 4 uncolored); `Yellow` is reserved exclusively for the two-zone overflow, and blue takes the later slot because it is the muddiest terminal color.
**Why**: the first (dominant) column matches the single-tool history's solid green bar and a tool's color never changes across windows, preserving day-to-day comparability; yellow mid-bar would collide with the overflow signal.
**Rejected**: assigning green to the highest-cost tool (colors would swap mid-history and the legend would disagree across renders); a palette order of green/magenta/cyan/blue (gives the more legible cyan to the less-used tool).
*Introduced by*: 260816-3tah-stacked-tool-bars-pivot, 260816-w61k-green-dominant-stack-palette

### View emits raw glyphs; ansi only colors and pads
**Decision**: `view.Bar` carries the raw block-glyph strings (`Main`, `Overflow`) and per-tool `Segments`; `render/ansi` slices, colors and pads them and adds the `┊` rule.
**Why**: the strip-ANSI-equals-no-color invariant becomes structural — the colored bar is the raw bar with escapes inserted at slice boundaries — and the width budget stays in the pure layer.
**Rejected**: computing glyphs inside `render/ansi` from numeric values (duplicates the scale math in the encoder and makes the no-color twin a test assertion instead of a construction).
*Introduced by*: 260916-9ax5-history-and-periods

### One ANSI encoder, extended by zero-valued flags
**Decision**: the leaderboard renders through `ansi.Table` via `Column.BarAfter` (the mid-row bar area, always `1 + Scale.Width` visible chars) and `Cell.Leader` (`BoldWhite` after pad and any `Dim` wrap), plus a footer on the `Empty` branch.
**Why**: every other leaderboard rule (pad-then-dim, `BoldWhite` Total cells, `BoldCyan` headers, `─|─` dividers) already matches; zero values keep every existing golden byte-identical.
**Rejected**: a dedicated leaderboard encoder — a second pathway with the same rules.
*Introduced by*: 260916-2gbb-leaderboard-lb-lbh
