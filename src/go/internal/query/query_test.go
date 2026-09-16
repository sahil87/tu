package query

import (
	"reflect"
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

func rec(date, tool string, t fact.Totals) fact.Record {
	return fact.Record{Date: date, Tool: tool, Totals: t}
}

var unit = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

func TestWindow(t *testing.T) {
	recs := []fact.Record{
		rec("2026-01-05", "cc", unit),
		rec("2026-01-06", "cc", unit),
		rec("2026-01-07", "cc", unit),
	}
	orig := append([]fact.Record{}, recs...)

	cases := []struct {
		name         string
		since, until string
		want         []string
	}{
		{"inclusive both", "2026-01-05", "2026-01-07", []string{"2026-01-05", "2026-01-06", "2026-01-07"}},
		{"open until", "2026-01-06", "", []string{"2026-01-06", "2026-01-07"}},
		{"open since", "", "2026-01-06", []string{"2026-01-05", "2026-01-06"}},
		{"both open", "", "", []string{"2026-01-05", "2026-01-06", "2026-01-07"}},
		{"empty result", "2026-02-01", "2026-02-28", nil},
		{"single day", "2026-01-06", "2026-01-06", []string{"2026-01-06"}},
	}
	for _, c := range cases {
		got := Window(recs, c.since, c.until)
		var dates []string
		for _, r := range got {
			dates = append(dates, r.Date)
		}
		if !reflect.DeepEqual(dates, c.want) {
			t.Errorf("%s: Window dates = %v, want %v", c.name, dates, c.want)
		}
	}
	if !reflect.DeepEqual(recs, orig) {
		t.Error("Window mutated its input")
	}
}

func TestByToolByUser(t *testing.T) {
	recs := []fact.Record{
		{Date: "2026-01-05", Tool: "cc", User: "alice"},
		{Date: "2026-01-05", Tool: "codex", User: "bob"},
		{Date: "2026-01-06", Tool: "cc", User: "bob"},
	}
	if got := ByTool(recs, "cc"); len(got) != 2 || got[0].Tool != "cc" || got[1].Tool != "cc" {
		t.Errorf("ByTool(cc) = %+v", got)
	}
	if got := ByTool(recs, "kimi"); len(got) != 0 {
		t.Errorf("ByTool(kimi) = %+v, want none", got)
	}
	got := ByUser(recs, "bob")
	if len(got) != 2 || got[0].User != "bob" || got[1].User != "bob" {
		t.Errorf("ByUser(bob) = %+v", got)
	}
}

func TestRollUpMonthly(t *testing.T) {
	var recs []fact.Record
	for _, d := range []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-10"} {
		recs = append(recs, rec(d, "cc", unit))
	}
	orig := append([]fact.Record{}, recs...)

	got := RollUp(recs, Monthly)
	if len(got) != 1 {
		t.Fatalf("RollUp(Monthly) yielded %d records, want 1", len(got))
	}
	want := fact.Totals{TotalCost: 3.0, InputTokens: 18000, OutputTokens: 2400, CacheCreationTokens: 6000, CacheReadTokens: 120000, TotalTokens: 146400}
	if got[0].Date != "2026-01" || got[0].Totals != want {
		t.Errorf("RollUp(Monthly) = %+v, want Date 2026-01 Totals %+v", got[0], want)
	}
	if !reflect.DeepEqual(recs, orig) {
		t.Error("RollUp mutated its input")
	}
}

func TestRollUpWeekly(t *testing.T) {
	// Seven days: 2026-01-05..11. Jan 4 is a Sunday, so 05–10 share week
	// 2026-01-04 and 11 starts week 2026-01-11 (the plan's R3 dates stopped at
	// 01-10, which lands entirely inside the first week).
	var recs []fact.Record
	for _, d := range []string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-10", "2026-01-11"} {
		recs = append(recs, rec(d, "cc", unit))
	}
	got := RollUp(recs, Weekly)
	if len(got) != 2 {
		t.Fatalf("RollUp(Weekly) yielded %d records, want 2", len(got))
	}
	if got[0].Date != "2026-01-04" || got[1].Date != "2026-01-11" {
		t.Errorf("weekly labels = %q, %q, want 2026-01-04, 2026-01-11", got[0].Date, got[1].Date)
	}
	if got[0].Totals.TotalTokens != 6*24400 || got[1].Totals.TotalTokens != 24400 {
		t.Errorf("weekly sums = %d, %d", got[0].Totals.TotalTokens, got[1].Totals.TotalTokens)
	}
}

func TestRollUpWeeklyAcrossMonthAndYear(t *testing.T) {
	recs := []fact.Record{
		rec("2026-08-30", "cc", unit), // Sunday: new week starts on the month's last day
		rec("2026-08-31", "cc", unit), // same week as 08-30
		rec("2025-12-29", "cc", unit), // Monday: week label 2025-12-28
		rec("2026-01-01", "cc", unit), // Thursday: same week as 2025-12-29
	}
	got := RollUp(recs, Weekly)
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2", len(got))
	}
	if got[0].Date != "2025-12-28" || got[1].Date != "2026-08-30" {
		t.Errorf("labels = %q, %q (must sort ascending across the year boundary)", got[0].Date, got[1].Date)
	}
	if got[0].Totals.TotalTokens != 2*24400 || got[1].Totals.TotalTokens != 2*24400 {
		t.Errorf("sums = %d, %d", got[0].Totals.TotalTokens, got[1].Totals.TotalTokens)
	}
}

func TestRollUpGroupsByAllDims(t *testing.T) {
	// Same date and tool but different user/machine must NOT merge.
	recs := []fact.Record{
		{Date: "2026-01-05", Tool: "cc", User: "alice", Machine: "m1", Totals: unit},
		{Date: "2026-01-05", Tool: "cc", User: "bob", Machine: "m1", Totals: unit},
		{Date: "2026-01-05", Tool: "cc", User: "alice", Machine: "m2", Totals: unit},
		{Date: "2026-01-06", Tool: "cc", User: "alice", Machine: "m1", Totals: unit},
	}
	got := RollUp(recs, Daily)
	if len(got) != 4 {
		t.Fatalf("RollUp(Daily) merged across dims: %d records, want 4", len(got))
	}
	if got[0].Date != "2026-01-05" || got[3].Date != "2026-01-06" {
		t.Errorf("ascending order broken: %q … %q", got[0].Date, got[3].Date)
	}
	// Same (date, tool, user, machine) DOES merge.
	dup := append(append([]fact.Record{}, recs[0]), recs[0])
	merged := RollUp(dup, Daily)
	if len(merged) != 1 || merged[0].Totals.TotalTokens != 2*24400 {
		t.Errorf("duplicate merge = %+v", merged)
	}
}

func TestRollUpDailyReturnsCopy(t *testing.T) {
	recs := []fact.Record{rec("2026-01-05", "cc", unit)}
	got := RollUp(recs, Daily)
	if len(got) != 1 {
		t.Fatalf("got %d records, want 1", len(got))
	}
	if &got[0] == &recs[0] {
		t.Error("RollUp(Daily) aliased the input element")
	}
	got[0].Totals.TotalTokens = -1
	if recs[0].Totals.TotalTokens != 24400 {
		t.Error("writing the RollUp result changed the input")
	}
}

func TestGroupBy(t *testing.T) {
	recs := []fact.Record{
		rec("2026-01-05", "cc", unit),
		rec("2026-01-05", "codex", unit),
		rec("2026-01-06", "cc", unit),
	}
	orig := append([]fact.Record{}, recs...)

	groups := GroupBy(recs, Tool)
	if len(groups) != 2 {
		t.Fatalf("GroupBy(Tool) yielded %d groups, want 2", len(groups))
	}
	if groups[0].Key.Tool != "cc" || groups[1].Key.Tool != "codex" {
		t.Errorf("first-seen order broken: %q, %q", groups[0].Key.Tool, groups[1].Key.Tool)
	}
	if groups[0].Totals.TotalTokens != 2*24400 || groups[0].Totals.TotalCost != 1.0 {
		t.Errorf("cc group totals = %+v", groups[0].Totals)
	}
	if groups[0].Key.Date != "" || groups[0].Key.User != "" || groups[0].Key.Machine != "" {
		t.Errorf("ungrouped dims must be empty: %+v", groups[0].Key)
	}
	if !reflect.DeepEqual(recs, orig) {
		t.Error("GroupBy mutated its input")
	}
}

func TestGroupByToolMachine(t *testing.T) {
	recs := []fact.Record{
		{Date: "2026-01-05", Tool: "cc", Machine: "m1", Totals: unit},
		{Date: "2026-01-05", Tool: "cc", Machine: "m2", Totals: unit},
		{Date: "2026-01-06", Tool: "cc", Machine: "m1", Totals: unit},
	}
	groups := GroupBy(recs, Tool, Machine)
	if len(groups) != 2 {
		t.Fatalf("GroupBy(Tool, Machine) yielded %d groups, want 2", len(groups))
	}
	if groups[0].Key.Machine != "m1" || groups[1].Key.Machine != "m2" {
		t.Errorf("keys = %+v, %+v", groups[0].Key, groups[1].Key)
	}
	if groups[0].Totals.TotalTokens != 2*24400 || groups[0].Key.Date != "" {
		t.Errorf("group = %+v", groups[0])
	}
}

func TestGroupByDate(t *testing.T) {
	recs := []fact.Record{
		rec("2026-01-06", "cc", unit),
		rec("2026-01-05", "codex", unit),
		rec("2026-01-06", "codex", unit),
	}
	groups := GroupBy(recs, Date)
	if len(groups) != 2 {
		t.Fatalf("GroupBy(Date) yielded %d groups, want 2", len(groups))
	}
	// First-seen order, NOT sorted: 2026-01-06 appears first in the input.
	if groups[0].Key.Date != "2026-01-06" || groups[1].Key.Date != "2026-01-05" {
		t.Errorf("order = %q, %q", groups[0].Key.Date, groups[1].Key.Date)
	}
	if groups[0].Totals.TotalTokens != 2*24400 {
		t.Errorf("group = %+v", groups[0])
	}
}

// The history path windows daily records BEFORE rolling up (intake §3): a
// window that starts mid-week yields a leading partial week labeled by its
// Sunday, which precedes `since`.
func TestWindowThenRollUpWeekly(t *testing.T) {
	recs := []fact.Record{
		rec("2026-01-05", "cc", unit),
		rec("2026-01-06", "cc", unit),
		rec("2026-01-07", "cc", unit),
	}
	rolled := RollUp(Window(recs, "2026-01-01", "2026-01-31"), Weekly)
	if len(rolled) != 1 {
		t.Fatalf("rolled = %d records, want 1: %+v", len(rolled), rolled)
	}
	if rolled[0].Date != "2026-01-04" {
		t.Errorf("label = %q, want the Sunday 2026-01-04 (precedes since)", rolled[0].Date)
	}
	if rolled[0].Totals.TotalCost != 1.5 || rolled[0].Totals.TotalTokens != 3*24400 {
		t.Errorf("totals = %+v, want the three days summed", rolled[0].Totals)
	}
}
