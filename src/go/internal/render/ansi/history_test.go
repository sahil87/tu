package ansi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/view"
)

var (
	historyNow    = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	historyTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}
)

// costEntry builds an entry whose only nonzero field is the cost (bar/footer
// fixtures; token columns render zeros).
func costEntry(label string, cost float64) view.Entry {
	return view.Entry{Label: label, Totals: fact.Totals{TotalCost: cost}}
}

// claudeWindow is the intake §12 single-tool window (cc-h-window bytes).
func claudeWindow() view.Series {
	return view.Series{Name: "Claude Code", Entries: []view.Entry{
		{Label: "2026-01-05", Totals: historyTotals},
		{Label: "2026-01-06", Totals: historyTotals},
		{Label: "2026-01-07", Totals: historyTotals},
	}}
}

// historyMachines is the seeded two-machine split over the placeholder window:
// harness-machine every day, other-box only on 01-06 (a dim $0.00 elsewhere).
func historyMachines() *view.Breakdown {
	return &view.Breakdown{Noun: "Machines", Rows: map[string][]view.Slice{
		"2026-01-05": {{Name: "harness-machine", Totals: historyTotals}},
		"2026-01-06": {
			{Name: "harness-machine", Totals: fact.Totals{TotalCost: 0.75, TotalTokens: 24400}},
			{Name: "other-box", Totals: fact.Totals{TotalCost: 0.40, TotalTokens: 24400}},
		},
		"2026-01-07": {{Name: "harness-machine", Totals: historyTotals}},
	}}
}

// outlierMachines carries the two-zone window's full value on machine-a (one
// slice per label, so the machine column mirrors the Cost column).
func outlierMachines() *view.Breakdown {
	rows := make(map[string][]view.Slice, 24)
	for _, e := range outlierWindow().Entries {
		rows[e.Label] = []view.Slice{{Name: "machine-a", Totals: e.Totals}}
	}
	return &view.Breakdown{Noun: "Machines", Rows: rows}
}

var pivotToolNames = []string{"Claude Code", "Codex", "OpenCode", "Gemini", "Copilot", "Kimi"}

// sixToolWindow is the intake §12 pivot window (h-window bytes).
func sixToolWindow() []view.Series {
	series := make([]view.Series, len(pivotToolNames))
	for i, name := range pivotToolNames {
		series[i] = claudeWindow()
		series[i].Name = name
	}
	return series
}

// outlierWindow triggers the two-zone scale (the TS test's canonical sample:
// 21 rows at 100…300 plus 1,091.67 and 4,031.61 → p95 = 1012.503) and adds a
// zero row, which renders the rule with both zones padded and the empty-wrap
// escapes.
func outlierWindow() view.Series {
	entries := make([]view.Entry, 0, 24)
	for i := 0; i < 21; i++ {
		entries = append(entries, costEntry("2026-06-"+pad2(i+1), 100+float64(i)*10))
	}
	entries = append(entries,
		costEntry("2026-06-22", 1091.67),
		costEntry("2026-06-23", 4031.61),
		costEntry("2026-06-24", 0),
	)
	return view.Series{Name: "Claude Code", Entries: entries}
}

func pad2(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// separatorWindow crosses a month boundary with a Saturday (2026-01-31) and a
// Sunday (2026-02-01); the injected Now makes 2026-02-02 the current row.
func separatorWindow() view.Series {
	return view.Series{Name: "Claude Code", Entries: []view.Entry{
		{Label: "2026-01-30", Totals: historyTotals},
		{Label: "2026-01-31", Totals: historyTotals},
		{Label: "2026-02-01", Totals: historyTotals},
		{Label: "2026-02-02", Totals: historyTotals},
	}}
}

// threeToolPivot shows bars at 120 columns with three visible tools (segments
// + legend). Kimi totals $1.25 so it survives the $1.00 significance floor.
func threeToolPivot() []view.Series {
	return []view.Series{
		{Name: "Claude Code", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 1.5}},
			{Label: "2026-01-06", Totals: fact.Totals{TotalCost: 0.75}},
		}},
		{Name: "Codex", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.75}},
			{Label: "2026-01-06", Totals: fact.Totals{TotalCost: 1.5}},
		}},
		{Name: "Kimi", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.5}},
			{Label: "2026-01-06", Totals: fact.Totals{TotalCost: 0.75}},
		}},
	}
}

// omissionPivot drops a $0.04 tool and a $0.00 tool; Codex's missing day
// renders a dim $0.00 cell.
func omissionPivot() []view.Series {
	return []view.Series{
		{Name: "Claude Code", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: historyTotals},
			{Label: "2026-01-06", Totals: historyTotals},
			{Label: "2026-01-07", Totals: historyTotals},
		}},
		{Name: "Codex", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: historyTotals},
			{Label: "2026-01-07", Totals: historyTotals},
		}},
		{Name: "Copilot", Entries: []view.Entry{
			{Label: "2026-01-05", Totals: fact.Totals{TotalCost: 0.04}},
		}},
		{Name: "Kimi"},
	}
}

func dailyAt(width int) view.HistoryOptions {
	return view.HistoryOptions{Period: query.Daily, Now: historyNow, Width: width}
}

func TestHistoryGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	monday := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC) // 2026-02-02 is a Monday
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"history 80col color", "history_80col_color.golden", Table(view.History(claudeWindow(), dailyAt(80), nil), color)},
		{"history 80col no-color", "history_80col_nocolor.golden", Table(view.History(claudeWindow(), dailyAt(80), nil), Colors{})},
		{"history wide bars", "history_wide_bars.golden", Table(view.History(claudeWindow(), dailyAt(140), nil), color)},
		{"history two-zone", "history_two_zone.golden", Table(view.History(outlierWindow(), dailyAt(140), nil), color)},
		{"history single row", "history_single_row.golden", Table(view.History(view.Series{Name: "Claude Code", Entries: []view.Entry{{Label: "2026-01", Totals: fact.Totals{TotalCost: 1.5, InputTokens: 9000, OutputTokens: 1200, CacheCreationTokens: 3000, CacheReadTokens: 60000, TotalTokens: 73200}}}}, view.HistoryOptions{Period: query.Monthly, Now: historyNow, Width: 80}, nil), color)},
		{"history empty", "history_empty.golden", Table(view.History(view.Series{Name: "Claude Code"}, view.HistoryOptions{Period: query.Daily, Now: historyNow, Width: 80, CapActive: true}, nil), color)},
		{"history tokens", "history_tokens.golden", Table(view.History(claudeWindow(), view.HistoryOptions{Period: query.Daily, Now: historyNow, Width: 80, Metric: view.Tokens}, nil), color)},
		{"history separators weekend today", "history_separators_weekend_today.golden", Table(view.History(separatorWindow(), view.HistoryOptions{Period: query.Daily, Now: monday, Width: 80}, nil), color)},
		{"history machines 80col", "history_machines_80col.golden", Table(view.History(claudeWindow(), dailyAt(80), historyMachines()), color)},
		{"history machines wide", "history_machines_wide.golden", Table(view.History(claudeWindow(), dailyAt(160), historyMachines()), color)},
		{"history machines two-zone", "history_machines_two_zone.golden", Table(view.History(outlierWindow(), dailyAt(160), outlierMachines()), color)},
		{"pivot 80col color", "pivot_80col_color.golden", Table(view.TotalHistory(sixToolWindow(), dailyAt(80)), color)},
		{"pivot 80col no-color", "pivot_80col_nocolor.golden", Table(view.TotalHistory(sixToolWindow(), dailyAt(80)), Colors{})},
		{"pivot wide stacked", "pivot_wide_stacked.golden", Table(view.TotalHistory(threeToolPivot(), dailyAt(120)), color)},
		{"pivot wide no-color", "pivot_wide_nocolor.golden", Table(view.TotalHistory(threeToolPivot(), dailyAt(120)), Colors{})},
		{"pivot omission dim zero", "pivot_omission_dim_zero.golden", Table(view.TotalHistory(omissionPivot(), dailyAt(80)), color)},
		{"pivot single row", "pivot_single_row.golden", Table(view.TotalHistory([]view.Series{{Name: "Claude Code", Entries: []view.Entry{{Label: "2026-01", Totals: fact.Totals{TotalCost: 1.5}}}}}, view.HistoryOptions{Period: query.Monthly, Now: historyNow, Width: 80}), color)},
		{"pivot empty", "pivot_empty.golden", Table(view.TotalHistory([]view.Series{{Name: "Claude Code"}, {Name: "Codex"}}, view.HistoryOptions{Period: query.Daily, Now: historyNow, Width: 80, CapActive: true}), color)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(c.lines, "\n") + "\n"
			path := filepath.Join("testdata", c.golden)
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(raw) {
				t.Errorf("table output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}

// Every colored history golden must equal its no-color twin (or the same
// table rendered without color) under StripANSI — the structural invariant
// behind --no-color/NO_COLOR equivalence.
func TestHistoryStripANSIInvariant(t *testing.T) {
	monday := time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC)
	tables := []view.Table{
		view.History(claudeWindow(), dailyAt(80), nil),
		view.History(claudeWindow(), dailyAt(140), nil),
		view.History(outlierWindow(), dailyAt(140), nil),
		view.History(separatorWindow(), view.HistoryOptions{Period: query.Daily, Now: monday, Width: 80}, nil),
		view.History(claudeWindow(), dailyAt(80), historyMachines()),
		view.History(claudeWindow(), dailyAt(160), historyMachines()),
		view.History(outlierWindow(), dailyAt(160), outlierMachines()),
		view.TotalHistory(sixToolWindow(), dailyAt(80)),
		view.TotalHistory(threeToolPivot(), dailyAt(120)),
		view.TotalHistory(omissionPivot(), dailyAt(80)),
	}
	for i, tab := range tables {
		color := Table(tab, Colors{Enabled: true})
		plain := Table(tab, Colors{})
		if len(color) != len(plain) {
			t.Fatalf("table %d: line counts differ: %d vs %d", i, len(color), len(plain))
		}
		for j := range color {
			if tab.Legend != nil && j == len(color)-2 {
				// The footer carries the legend only when color is enabled
				// (the TS gates swatches on colorDisabled()): the stripped
				// form is the plain footer plus the legend text.
				stripped := StripANSI(color[j])
				if !strings.HasPrefix(stripped, plain[j]+" · ") {
					t.Errorf("table %d footer: stripped %q lacks the plain prefix %q", i, stripped, plain[j])
				}
				continue
			}
			if StripANSI(color[j]) != plain[j] {
				t.Errorf("table %d line %d: StripANSI = %q, want %q", i, j, StripANSI(color[j]), plain[j])
			}
		}
	}
}

// The two-zone encoder emits the scale-break rule in every row, and a zero
// row still wraps both empty zones in color escapes (the TS wrap() has no
// empty guard) — pinned here independent of the golden file.
func TestTwoZoneZeroRowBytes(t *testing.T) {
	tab := view.History(outlierWindow(), dailyAt(140), nil)
	if !tab.Scale.TwoZone {
		t.Fatalf("outlier window must be two-zone: %+v", tab.Scale)
	}
	var zeroRow string
	for i, r := range tab.Rows {
		if r.Kind == view.Data && r.Cells[6].Text == "$0.00" {
			zeroRow = Table(tab, Colors{Enabled: true})[i+3] // "", title, "" precede the rows
		}
	}
	if zeroRow == "" {
		t.Fatal("no zero row found")
	}
	s := tab.Scale
	want := " " + "\x1b[32m\x1b[0m" + strings.Repeat(" ", s.MainZone) + "\x1b[2m┊\x1b[0m" + "\x1b[33m\x1b[0m" + strings.Repeat(" ", s.OverflowZone)
	if !strings.HasSuffix(zeroRow, want) {
		t.Errorf("zero row bar = %q, want suffix %q", zeroRow, want)
	}
	if !strings.Contains(zeroRow, "\x1b[2m    $0.00\x1b[0m") {
		t.Errorf("zero row metric cell must be dim: %q", zeroRow)
	}
}

// The intake §12 captures pin the two 80-column history tables byte for byte.
func TestIntakeByteReferences(t *testing.T) {
	color := Colors{Enabled: true}
	got := strings.Join(Table(view.History(claudeWindow(), dailyAt(80), nil), color), "\n") + "\n"
	want := "\n" +
		"\x1b[1;37m📊 Claude Code (daily)\x1b[0m\n" +
		"\n" +
		"\x1b[1;36mDate        \x1b[0m | \x1b[1;36m         Input\x1b[0m | \x1b[1;36m        Output\x1b[0m | \x1b[1;36m   Cache Write\x1b[0m | \x1b[1;36m    Cache Read\x1b[0m | \x1b[1;36m         Total\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n" +
		"\x1b[2m" + strings.Repeat("─", 12) + "─|─" + strings.Join(rep("─", 14, 5), "─|─") + "─|─" + strings.Repeat("─", 9) + "\x1b[0m\n" +
		"2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n" +
		"2026-01-06   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n" +
		"2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50\n" +
		"\x1b[2m" + strings.Repeat("─", 12) + "─|─" + strings.Join(rep("─", 14, 5), "─|─") + "─|─" + strings.Repeat("─", 9) + "\x1b[0m\n" +
		"\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m         9,000\x1b[0m | \x1b[1;37m         1,200\x1b[0m | \x1b[1;37m         3,000\x1b[0m | \x1b[1;37m        60,000\x1b[0m | \x1b[1;37m        73,200\x1b[0m | \x1b[1;37m    $1.50\x1b[0m\n" +
		"\x1b[2mavg $0.50/day · peak $0.50 (2026-01-05)\x1b[0m\n" +
		"\n"
	if got != want {
		t.Errorf("cc-h-window bytes differ\ngot:\n%q\nwant:\n%q", got, want)
	}

	got = strings.Join(Table(view.TotalHistory(sixToolWindow(), dailyAt(80)), color), "\n") + "\n"
	divider := "\x1b[2m" + strings.Repeat("─", 10) + "─|─" + strings.Repeat("─", 11) + "─|─" + strings.Join(rep("─", 9, 5), "─|─") + "─|─" + strings.Repeat("─", 9) + "\x1b[0m"
	want = "\n" +
		"\x1b[1;37m📊 Combined Cost History (daily)\x1b[0m\n" +
		"\n" +
		"\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m | \x1b[1;36m    Codex\x1b[0m | \x1b[1;36m OpenCode\x1b[0m | \x1b[1;36m   Gemini\x1b[0m | \x1b[1;36m  Copilot\x1b[0m | \x1b[1;36m     Kimi\x1b[0m | \x1b[1;36m     Cost\x1b[0m\n" +
		divider + "\n" +
		"2026-01-05 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n" +
		"2026-01-06 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n" +
		"2026-01-07 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00\n" +
		divider + "\n" +
		"\x1b[1;37mTotal     \x1b[0m | \x1b[1;37m      $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $1.50\x1b[0m | \x1b[1;37m    $9.00\x1b[0m\n" +
		"\x1b[2mavg $3.00/day · peak $3.00 (2026-01-05)\x1b[0m\n" +
		"\n"
	if got != want {
		t.Errorf("h-window bytes differ\ngot:\n%q\nwant:\n%q", got, want)
	}
}

// rep is strings.Repeat per element.
func rep(s string, w, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = strings.Repeat(s, w)
	}
	return out
}

// The watch row budget (B7): History keeps the last MaxRows entries after the
// empty check and TotalHistory the last MaxRows labels before it; the footer
// (avg over the window) and the bar scale are computed on the truncated
// window. The outlier window's 24 rows become the last 15 — the two-zone rule
// and the zero row survive truncation.
func TestMaxRowsGoldens(t *testing.T) {
	color := Colors{Enabled: true}
	withRows := func(o view.HistoryOptions, n int) view.HistoryOptions {
		o.MaxRows = n
		return o
	}
	cases := []struct {
		name   string
		golden string
		lines  []string
	}{
		{"history maxrows", "history_maxrows.golden", Table(view.History(outlierWindow(), withRows(dailyAt(140), 15), nil), color)},
		{"pivot maxrows", "pivot_maxrows.golden", Table(view.TotalHistory(sixToolWindow(), withRows(dailyAt(80), 2)), color)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(c.lines, "\n") + "\n"
			path := filepath.Join("testdata", c.golden)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if got != string(raw) {
				t.Errorf("table output differs from %s (-update to regenerate)\ngot:\n%q\nwant:\n%q", c.golden, got, string(raw))
			}
		})
	}
}
