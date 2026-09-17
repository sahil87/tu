package view

import (
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
)

var dayTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

func kinds(rows []Row) []RowKind {
	out := make([]RowKind, len(rows))
	for i, r := range rows {
		out[i] = r.Kind
	}
	return out
}

func equalKinds(got []RowKind, want ...RowKind) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSnapshotPopulated(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: dayTotals},
		{Name: "Codex", Label: "2026-09-16", Totals: dayTotals},
		{Name: "OpenCode"}, // all-zero: omitted from rows, counted in Total
	}
	tab := Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Cost})

	if tab.Title != "📊 Combined Usage (daily)" {
		t.Errorf("Title = %q", tab.Title)
	}
	if tab.Empty != "" {
		t.Errorf("Empty = %q, want empty", tab.Empty)
	}
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data, Data, Divider, Total) {
		t.Errorf("row kinds = %v", kinds(tab.Rows))
	}
	data := tab.Rows[2].Cells
	want := []string{"Claude Code", "24,400", "3,000", "400", "21,000", "$0.50"}
	for i, w := range want {
		if data[i].Text != w {
			t.Errorf("data cell %d = %q, want %q", i, data[i].Text, w)
		}
	}
	total := tab.Rows[5].Cells
	wantTotal := []string{"Total", "48,800", "6,000", "800", "42,000", "$1.00"}
	for i, w := range wantTotal {
		if total[i].Text != w {
			t.Errorf("total cell %d = %q, want %q", i, total[i].Text, w)
		}
	}
}

func TestSnapshotHiddenCostCounted(t *testing.T) {
	hidden := fact.Totals{TotalCost: 0.25} // zero tokens: hidden, still summed
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex", Totals: dayTotals},
		{Name: "OpenCode", Totals: hidden},
	}
	tab := Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Cost})
	total := tab.Rows[len(tab.Rows)-1]
	if total.Kind != Total {
		t.Fatalf("last row kind = %v, want Total", total.Kind)
	}
	if got := total.Cells[5].Text; got != "$1.25" {
		t.Errorf("Total cost = %q, want $1.25 (hidden row counted)", got)
	}
}

func TestSnapshotSingleRowNoTotal(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex"},
	}
	tab := Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Cost})
	if !equalKinds(kinds(tab.Rows), Header, Divider, Data) {
		t.Errorf("row kinds = %v, want Header Divider Data (Total only when >1 visible)", kinds(tab.Rows))
	}
}

func TestSnapshotEmpty(t *testing.T) {
	tab := Snapshot([]ToolTotals{{Name: "Claude Code"}, {Name: "Codex"}}, query.Daily, nil, SnapshotOptions{Metric: Cost})
	if tab.Empty != "  No usage" {
		t.Errorf("Empty = %q, want %q", tab.Empty, "  No usage")
	}
	if tab.Rows != nil {
		t.Errorf("Rows = %v, want nil in the empty state", tab.Rows)
	}
}

func TestSnapshotHeadingPerPeriod(t *testing.T) {
	for p, want := range map[query.Period]string{
		query.Daily:   "📊 Combined Usage (daily)",
		query.Weekly:  "📊 Combined Usage (weekly)",
		query.Monthly: "📊 Combined Usage (monthly)",
	} {
		if got := Snapshot(nil, p, nil, SnapshotOptions{Metric: Cost}).Title; got != want {
			t.Errorf("Snapshot(nil, %v).Title = %q, want %q", p, got, want)
		}
	}
}

func TestSnapshotFixedWidthsOverflow(t *testing.T) {
	for _, c := range snapshotColumns {
		if c.Width != 12 {
			t.Errorf("column %q width = %d, want 12 (fixed, never data-sized)", c.Title, c.Width)
		}
	}
	// A wider value is carried verbatim — it overflows the cell at render time.
	big := fact.Totals{TotalCost: 1, TotalTokens: 1234567890123}
	tab := Snapshot([]ToolTotals{{Name: "Claude Code", Totals: big}}, query.Daily, nil, SnapshotOptions{Metric: Cost})
	if got := tab.Rows[2].Cells[1].Text; got != "1,234,567,890,123" {
		t.Errorf("overflowing cell = %q", got)
	}
}

// ── B4: machine columns ────────────────────────────────────────────────────

// snapshotBreakdown mirrors R7's given: cc with two machines under Cost.
func snapshotBreakdown() *Breakdown {
	return &Breakdown{Noun: "Machines", Rows: map[string][]Slice{
		"Claude Code": {
			{Name: "Sahils-Mac-mini.local", Totals: fact.Totals{TotalCost: 8.27}},
			{Name: "dev-ws-sahil02", Totals: fact.Totals{TotalCost: 6288.75}},
		},
	}}
}

// R7: machine columns follow Cost, letter-coded with a shared width, dim zero
// cells, per-name Total sums over visible rows, and the legend in Note.
func TestSnapshotMachineColumns(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Label: "2026-09-16", Totals: dayTotals},
		{Name: "Codex", Label: "2026-09-16", Totals: dayTotals},
		{Name: "OpenCode"}, // hidden: no cells, no sums
	}
	tab := Snapshot(rows, query.Daily, snapshotBreakdown(), SnapshotOptions{Metric: Cost})

	if got := tab.Columns[6].Title; got != "A" {
		t.Errorf("column 6 = %q, want A", got)
	}
	if got := tab.Columns[7].Title; got != "B" {
		t.Errorf("column 7 = %q, want B", got)
	}
	if tab.Columns[6].Width != 9 || tab.Columns[7].Width != 9 {
		t.Errorf("machine widths = %d/%d, want 9 ($6,288.75 fits the floor)", tab.Columns[6].Width, tab.Columns[7].Width)
	}
	cc := tab.Rows[2].Cells
	if cc[6].Text != "$8.27" || cc[7].Text != "$6,288.75" {
		t.Errorf("cc machine cells = %q, %q", cc[6].Text, cc[7].Text)
	}
	codex := tab.Rows[3].Cells
	if codex[6].Text != "$0.00" || !codex[6].Dim || codex[7].Text != "$0.00" || !codex[7].Dim {
		t.Errorf("codex machine cells = %+v, want dim zeros", codex[6:])
	}
	total := tab.Rows[5].Cells
	if total[6].Text != "$8.27" || total[7].Text != "$6,288.75" || total[6].Dim || total[7].Dim {
		t.Errorf("total machine cells = %+v, want $8.27 / $6,288.75 never dim", total[6:])
	}
	if tab.Note != "Machines: A = Sahils-Mac-mini.local, B = dev-ws-sahil02" {
		t.Errorf("Note = %q", tab.Note)
	}
}

// R7: a nil breakdown is today's output byte-for-byte (no columns, no Note).
func TestSnapshotNilBreakdown(t *testing.T) {
	rows := []ToolTotals{{Name: "Claude Code", Totals: dayTotals}, {Name: "Codex", Totals: dayTotals}}
	tab := Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Cost})
	if len(tab.Columns) != 6 || tab.Note != "" {
		t.Errorf("nil breakdown changed the table: %d columns, Note %q", len(tab.Columns), tab.Note)
	}
	// A breakdown with no slices behaves the same.
	empty := &Breakdown{Noun: "Machines", Rows: map[string][]Slice{}}
	tab = Snapshot(rows, query.Daily, empty, SnapshotOptions{Metric: Cost})
	if len(tab.Columns) != 6 || tab.Note != "" {
		t.Errorf("empty breakdown changed the table: %d columns, Note %q", len(tab.Columns), tab.Note)
	}
}

// A-018: the empty state early-returns — no machine columns, no Note.
func TestSnapshotMachinesEmptyState(t *testing.T) {
	tab := Snapshot([]ToolTotals{{Name: "Claude Code"}}, query.Daily, snapshotBreakdown(), SnapshotOptions{Metric: Cost})
	if tab.Empty != "  No usage" || len(tab.Columns) != 6 || tab.Note != "" {
		t.Errorf("empty state = %q with %d columns, Note %q", tab.Empty, len(tab.Columns), tab.Note)
	}
}

// R7: under -t the machine cells render tokens beside the $ Cost column.
func TestSnapshotMachinesTokenMetric(t *testing.T) {
	bd := &Breakdown{Noun: "Machines", Rows: map[string][]Slice{
		"Claude Code": {{Name: "m1", Totals: fact.Totals{TotalCost: 8.27, TotalTokens: 12000}}},
	}}
	rows := []ToolTotals{{Name: "Claude Code", Totals: dayTotals}}
	tab := Snapshot(rows, query.Daily, bd, SnapshotOptions{Metric: Tokens})
	cell := tab.Rows[2].Cells[6]
	if cell.Text != "12,000" {
		t.Errorf("token machine cell = %q, want 12,000", cell.Text)
	}
	if got := tab.Rows[2].Cells[5].Text; got != "$0.50" {
		t.Errorf("Cost cell = %q, want unchanged $0.50", got)
	}
}

// The watch delta (B7): under cost the Cost cell carries the arrow from
// Prev[Name]; under tokens the Tokens cell does and Cost stays plain. The
// table selects DeltaPadsArrow (the JS raw-length padding placement).
func TestSnapshotDeltaCells(t *testing.T) {
	rows := []ToolTotals{
		{Name: "Claude Code", Totals: dayTotals},
		{Name: "Codex", Totals: dayTotals},
	}
	tab := Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Cost, Prev: map[string]float64{
		"Claude Code": 0.40, // up
		"Codex":       0.60, // down
		"Kimi":        1.00, // hidden row: no cell
	}})
	if tab.DeltaInCell != DeltaPadsArrow {
		t.Errorf("DeltaInCell = %v, want DeltaPadsArrow", tab.DeltaInCell)
	}
	if got := tab.Rows[2].Cells[5].Delta; got != DeltaUp {
		t.Errorf("Claude Code cost delta = %v, want DeltaUp", got)
	}
	if got := tab.Rows[2].Cells[1].Delta; got != DeltaNone {
		t.Errorf("Claude Code tokens delta = %v, want DeltaNone under cost", got)
	}
	if got := tab.Rows[3].Cells[5].Delta; got != DeltaDown {
		t.Errorf("Codex cost delta = %v, want DeltaDown", got)
	}
	if got := tab.Rows[len(tab.Rows)-1].Cells[5].Delta; got != DeltaNone {
		t.Errorf("Total row delta = %v, want DeltaNone", got)
	}

	tab = Snapshot(rows, query.Daily, nil, SnapshotOptions{Metric: Tokens, Prev: map[string]float64{
		"Claude Code": 20000, // up (24,400)
		"Codex":       24400, // equal: no arrow
	}})
	if got := tab.Rows[2].Cells[1].Delta; got != DeltaUp {
		t.Errorf("Claude Code tokens delta = %v, want DeltaUp", got)
	}
	if got := tab.Rows[2].Cells[5].Delta; got != DeltaNone {
		t.Errorf("Claude Code cost delta = %v, want DeltaNone under tokens", got)
	}
	if got := tab.Rows[3].Cells[1].Delta; got != DeltaNone {
		t.Errorf("Codex tokens delta = %v, want DeltaNone on equal", got)
	}
}
