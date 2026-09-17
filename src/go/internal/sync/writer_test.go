package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source/metrics"
)

// rec is the TS entry(): a daily record with the test token counters.
func rec(date string, cost float64) fact.Record {
	return fact.Record{
		Date: date,
		Totals: fact.Totals{
			TotalCost:    cost,
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}
}

var ccTool = fact.Tool{Key: "cc", Name: "Claude Code"}

// readDay parses the written day-file the way the TS tests JSON.parse it.
func readDay(t *testing.T, path string) metrics.DayFile {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d metrics.DayFile
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// --- writeMetrics ---

// TS: creates date-partitioned path and writes JSONL file.
func TestWriteCreatesDatePartitionedPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.5)}, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl")
	if !fileExists(path) {
		t.Fatalf("%s not created", path)
	}
	d := readDay(t, path)
	if d.Label != "2026-02-20" || d.TotalCost != 1.5 {
		t.Errorf("day-file = %+v", d)
	}
}

// TS: writes one file per entry.
func TestWriteOneFilePerEntry(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-19", 1.0), rec("2026-02-20", 2.0)}, false); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"2026-02-19", "2026-02-20"} {
		if p := filepath.Join(dir, "sahil", "2026", "macbook", "cc-"+date+".jsonl"); !fileExists(p) {
			t.Errorf("%s not created", p)
		}
	}
}

// TS: handles entries spanning multiple years.
func TestWriteSpansMultipleYears(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2025-12-31", 1.0), rec("2026-01-01", 2.0)}, false); err != nil {
		t.Fatal(err)
	}
	if p := filepath.Join(dir, "sahil", "2025", "macbook", "cc-2025-12-31.jsonl"); !fileExists(p) {
		t.Errorf("%s not created", p)
	}
	if p := filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-01-01.jsonl"); !fileExists(p) {
		t.Errorf("%s not created", p)
	}
}

// TS: overwrites existing file on re-run.
func TestWriteOverwritesOnRerun(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.0)}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 2.0)}, false); err != nil {
		t.Fatal(err)
	}
	d := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl"))
	if d.TotalCost != 2.0 {
		t.Errorf("totalCost = %v, want 2", d.TotalCost)
	}
}

// TS: skips the write when incoming totalCost is lower than existing (shrink).
func TestWriteSkipsShrink(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 308.12)}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 9.46)}, false); err != nil {
		t.Fatal(err)
	}
	d := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-04-24.jsonl"))
	if d.TotalCost != 308.12 {
		t.Errorf("totalCost = %v, want 308.12", d.TotalCost)
	}
}

// TS: skips shrinking writes silently (no stderr warning).
func TestWriteSkipsShrinkSilently(t *testing.T) {
	dir := t.TempDir()
	out, errOut := captureProcessStreams(t, func() {
		if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 100.0)}, false); err != nil {
			t.Error(err)
		}
		if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 1.0)}, false); err != nil {
			t.Error(err)
		}
	})
	if out != "" || errOut != "" {
		t.Errorf("process streams = %q / %q, want both empty", out, errOut)
	}
}

// TS: writes when incoming totalCost equals existing (idempotent refresh).
func TestWriteEqualCostRefreshes(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.5)}, false); err != nil {
		t.Fatal(err)
	}
	// Equal cost but different token counts — the newer snapshot must win.
	updated := rec("2026-02-20", 1.5)
	updated.TotalTokens = 999
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{updated}, false); err != nil {
		t.Fatal(err)
	}
	d := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl"))
	if d.TotalCost != 1.5 || d.TotalTokens != 999 {
		t.Errorf("day-file = %+v, want totalCost 1.5 and totalTokens 999", d)
	}
}

// TS: writes when existing file is empty (treated as absent).
func TestWriteEmptyExistingTreatedAbsent(t *testing.T) {
	dir := t.TempDir()
	machDir := filepath.Join(dir, "sahil", "2026", "macbook")
	if err := os.MkdirAll(machDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(machDir, "cc-2026-02-20.jsonl")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 0.5)}, false); err != nil {
		t.Fatal(err)
	}
	if d := readDay(t, path); d.TotalCost != 0.5 {
		t.Errorf("totalCost = %v, want 0.5", d.TotalCost)
	}
}

// TS: writes when existing file is not valid JSON (treated as absent).
func TestWriteUnparseableExistingTreatedAbsent(t *testing.T) {
	dir := t.TempDir()
	machDir := filepath.Join(dir, "sahil", "2026", "macbook")
	if err := os.MkdirAll(machDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(machDir, "cc-2026-02-20.jsonl")
	if err := os.WriteFile(path, []byte("not json at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 0.5)}, false); err != nil {
		t.Fatal(err)
	}
	if d := readDay(t, path); d.TotalCost != 0.5 {
		t.Errorf("totalCost = %v, want 0.5", d.TotalCost)
	}
}

// TS: writes when existing JSON has no numeric totalCost (not a UsageEntry).
func TestWriteNoNumericTotalCostTreatedAbsent(t *testing.T) {
	dir := t.TempDir()
	machDir := filepath.Join(dir, "sahil", "2026", "macbook")
	if err := os.MkdirAll(machDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(machDir, "cc-2026-02-20.jsonl")
	if err := os.WriteFile(path, []byte(`{"label":"2026-02-20","totalCost":"junk"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 0.5)}, false); err != nil {
		t.Fatal(err)
	}
	if d := readDay(t, path); d.TotalCost != 0.5 {
		t.Errorf("totalCost = %v, want 0.5", d.TotalCost)
	}
}

// TS: guards per entry within one batch (skips shrunk, writes grown).
func TestWriteGuardsPerEntryInBatch(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 308.12), rec("2026-04-25", 1.0)}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 9.46), rec("2026-04-25", 5.0)}, false); err != nil {
		t.Fatal(err)
	}
	machDir := filepath.Join(dir, "sahil", "2026", "macbook")
	if d := readDay(t, filepath.Join(machDir, "cc-2026-04-24.jsonl")); d.TotalCost != 308.12 {
		t.Errorf("day24 totalCost = %v, want 308.12 (shrunk → skipped)", d.TotalCost)
	}
	if d := readDay(t, filepath.Join(machDir, "cc-2026-04-25.jsonl")); d.TotalCost != 5.0 {
		t.Errorf("day25 totalCost = %v, want 5 (grown → written)", d.TotalCost)
	}
}

// TS: live mode returns a write decision and still writes the file.
func TestWriteLiveReturnsWriteDecision(t *testing.T) {
	dir := t.TempDir()
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.5)}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatalf("len(decisions) = %d, want 1", len(decisions))
	}
	d := decisions[0]
	if d.Action != ActionWrite || d.IncomingCost != 1.5 || d.ExistingCost != nil {
		t.Errorf("decision = %+v, want write 1.5 with no existing cost", d)
	}
	if !fileExists(filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl")) {
		t.Error("file not written in live mode")
	}
}

// TS: live mode reports a skip decision when the never-shrink guard fires.
func TestWriteLiveReportsSkipDecision(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 308.12)}, false); err != nil {
		t.Fatal(err)
	}
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 9.46)}, false)
	if err != nil {
		t.Fatal(err)
	}
	d := decisions[0]
	if d.Action != ActionSkip || d.IncomingCost != 9.46 {
		t.Errorf("decision = %+v, want skip 9.46", d)
	}
	if d.ExistingCost == nil || *d.ExistingCost != 308.12 {
		t.Errorf("ExistingCost = %v, want 308.12", d.ExistingCost)
	}
}

// --- writeMetrics (dry-run) ---

// TS: reports a new-file write and leaves the filesystem untouched.
func TestWriteDryRunNewFile(t *testing.T) {
	dir := t.TempDir()
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-07-18", 3.21)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 1 {
		t.Fatalf("len(decisions) = %d, want 1", len(decisions))
	}
	d := decisions[0]
	if d.Action != ActionWrite || d.IncomingCost != 3.21 || d.ExistingCost != nil {
		t.Errorf("decision = %+v, want write 3.21 with no existing cost", d)
	}
	// Nothing written — not even the year/machine directory.
	if fileExists(filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-07-18.jsonl")) {
		t.Error("dry-run wrote the day-file")
	}
	if fileExists(filepath.Join(dir, "sahil", "2026", "macbook")) {
		t.Error("dry-run created the machine directory")
	}
}

// TS: reports an update (existing < incoming) with the prior cost.
func TestWriteDryRunUpdate(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-07-18", 10.2)}, false); err != nil {
		t.Fatal(err)
	}
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-07-18", 12.34)}, true)
	if err != nil {
		t.Fatal(err)
	}
	d := decisions[0]
	if d.Action != ActionWrite || d.IncomingCost != 12.34 {
		t.Errorf("decision = %+v, want write 12.34", d)
	}
	if d.ExistingCost == nil || *d.ExistingCost != 10.2 {
		t.Errorf("ExistingCost = %v, want 10.2", d.ExistingCost)
	}
	// Existing file is unchanged by the dry-run.
	if got := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-07-18.jsonl")); got.TotalCost != 10.2 {
		t.Errorf("totalCost = %v, want 10.2 (unchanged)", got.TotalCost)
	}
}

// TS: reports a never-shrink skip (incoming < existing) with the prior cost.
func TestWriteDryRunSkip(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-06-01", 45.67)}, false); err != nil {
		t.Fatal(err)
	}
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-06-01", 0.0)}, true)
	if err != nil {
		t.Fatal(err)
	}
	d := decisions[0]
	if d.Action != ActionSkip || d.IncomingCost != 0.0 {
		t.Errorf("decision = %+v, want skip 0", d)
	}
	if d.ExistingCost == nil || *d.ExistingCost != 45.67 {
		t.Errorf("ExistingCost = %v, want 45.67", d.ExistingCost)
	}
}

// TS: treats absent/empty/unparseable existing files as absent (write, no existingCost).
func TestWriteDryRunAbsentEmptyUnparseable(t *testing.T) {
	dir := t.TempDir()
	machDir := filepath.Join(dir, "sahil", "2026", "macbook")
	if err := os.MkdirAll(machDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(machDir, "cc-2026-07-18.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(machDir, "cc-2026-07-19.jsonl"), []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{
		rec("2026-07-17", 1.0), // absent
		rec("2026-07-18", 1.0), // empty existing
		rec("2026-07-19", 1.0), // unparseable existing
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 3 {
		t.Fatalf("len(decisions) = %d, want 3", len(decisions))
	}
	for _, d := range decisions {
		if d.Action != ActionWrite || d.ExistingCost != nil {
			t.Errorf("decision = %+v, want write with no existing cost", d)
		}
	}
}

// TS: reports per-entry decisions within one batch without writing.
func TestWriteDryRunPerEntryBatch(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-04-24", 308.12), rec("2026-04-25", 1.0)}, false); err != nil {
		t.Fatal(err)
	}
	decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{
		rec("2026-04-24", 9.46), // shrunk → skip
		rec("2026-04-25", 5.0),  // grown → write
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if decisions[0].Action != ActionSkip {
		t.Errorf("decisions[0].Action = %q, want skip", decisions[0].Action)
	}
	if decisions[1].Action != ActionWrite {
		t.Errorf("decisions[1].Action = %q, want write", decisions[1].Action)
	}
	// The grown day-file on disk is still the original 1.0 — dry-run wrote nothing.
	if d := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-04-25.jsonl")); d.TotalCost != 1.0 {
		t.Errorf("day25 totalCost = %v, want 1 (unchanged)", d.TotalCost)
	}
}

// --- R2: the Number() coercion table ---

// TS readShrinkState via Number(existing?.totalCost): existing content empty,
// garbage, {"label":"x"}, null and 5 are treated as absent (write, no
// existing cost); {"totalCost":"1.5"} coerces to 1.5, so incoming 1.0 skips.
func TestShrinkStateCoercionTable(t *testing.T) {
	cases := []struct {
		name         string
		content      string
		wantAction   Action
		wantExisting *float64
	}{
		{"empty", "", ActionWrite, nil},
		{"garbage", "garbage", ActionWrite, nil},
		{"missing totalCost", `{"label":"x"}`, ActionWrite, nil},
		{"top-level null", "null", ActionWrite, nil},
		{"top-level number", "5", ActionWrite, nil},
		{"string totalCost", `{"totalCost":"1.5"}`, ActionSkip, ptrFloat(1.5)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			machDir := filepath.Join(dir, "sahil", "2026", "macbook")
			if err := os.MkdirAll(machDir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(machDir, "cc-2026-02-20.jsonl")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			decisions, err := Write(dir, "sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.0)}, true)
			if err != nil {
				t.Fatal(err)
			}
			d := decisions[0]
			if d.Action != tc.wantAction {
				t.Errorf("Action = %q, want %q", d.Action, tc.wantAction)
			}
			if tc.wantExisting == nil {
				if d.ExistingCost != nil {
					t.Errorf("ExistingCost = %v, want nil", *d.ExistingCost)
				}
			} else if d.ExistingCost == nil || *d.ExistingCost != *tc.wantExisting {
				t.Errorf("ExistingCost = %v, want %v", d.ExistingCost, *tc.wantExisting)
			}
		})
	}
}

func ptrFloat(f float64) *float64 { return &f }

// T007: the Writer adapter writes in live mode through the package Write.
func TestWriterAdapter(t *testing.T) {
	dir := t.TempDir()
	if err := (Writer{Dir: dir}).Write("sahil", "macbook", ccTool, []fact.Record{rec("2026-02-20", 1.5)}); err != nil {
		t.Fatal(err)
	}
	d := readDay(t, filepath.Join(dir, "sahil", "2026", "macbook", "cc-2026-02-20.jsonl"))
	if d.Label != "2026-02-20" || d.TotalCost != 1.5 {
		t.Errorf("day-file = %+v", d)
	}
}
