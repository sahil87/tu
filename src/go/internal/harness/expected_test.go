package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExpectedFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expected-diffs.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadExpectedValid(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		exp, err := LoadExpected(writeExpectedFile(t, "{\n  \"schema\": 1,\n  \"expected\": []\n}\n"))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if exp.Schema != 1 || len(exp.Entries) != 0 {
			t.Errorf("exp = %+v", exp)
		}
	})
	t.Run("populated", func(t *testing.T) {
		exp, err := LoadExpected(writeExpectedFile(t,
			`{"schema":1,"expected":[{"id":"DC-05","cases":["snapshot-*-csv","snapshot-all-csv/*/*/*/*"],"reason":"x"}]}`))
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(exp.Entries) != 1 || exp.Entries[0].ID != "DC-05" ||
			len(exp.Entries[0].Cases) != 2 || exp.Entries[0].Reason != "x" {
			t.Errorf("entries = %+v", exp.Entries)
		}
	})
}

// R1: every validation error names the offending entry id (when known) and
// field.
func TestLoadExpectedErrors(t *testing.T) {
	cases := []struct {
		name, body, wantSub string
	}{
		{"schema 2", `{"schema":2,"expected":[]}`, "schema must be 1, got 2"},
		{"missing expected key", `{"schema":1}`, "missing expected key"},
		{"unknown field", `{"schema":1,"expected":[],"bogus":1}`, "bogus"},
		{"trailing value", `{"schema":1,"expected":[]} {}`, "trailing"},
		{"bad id", `{"schema":1,"expected":[{"id":"DC-5","cases":["x"],"reason":"r"}]}`,
			`entry "DC-5": invalid id "DC-5"`},
		{"empty id", `{"schema":1,"expected":[{"id":"","cases":["x"],"reason":"r"}]}`,
			`entry 0: invalid id ""`},
		{"duplicate id", `{"schema":1,"expected":[{"id":"DC-05","cases":["x"],"reason":"r"},{"id":"DC-05","cases":["y"],"reason":"r"}]}`,
			`entry "DC-05": duplicate id`},
		{"empty cases", `{"schema":1,"expected":[{"id":"DC-05","cases":[],"reason":"r"}]}`,
			`entry "DC-05": cases`},
		{"bad pattern", `{"schema":1,"expected":[{"id":"DC-05","cases":["[unclosed"],"reason":"r"}]}`,
			`entry "DC-05": invalid cases pattern "[unclosed"`},
		{"empty reason", `{"schema":1,"expected":[{"id":"DC-05","cases":["x"],"reason":""}]}`,
			`entry "DC-05": empty reason`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exp, err := LoadExpected(writeExpectedFile(t, tc.body))
			if err == nil {
				t.Fatalf("no error for %s", tc.body)
			}
			if exp != nil {
				t.Errorf("exp = %+v, want nil on error", exp)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("err = %q, want substring %q", err, tc.wantSub)
			}
		})
	}
}

// R2: file order, path.Match against the full case ID, the bare-pattern group
// fallback, and the nil/empty sets.
func TestExpectedMatch(t *testing.T) {
	exp := &Expected{Entries: []ExpectedEntry{
		{ID: "DC-05", Cases: []string{"snapshot-*-csv"}, Reason: "x"},
		{ID: "DC-21", Cases: []string{"sync-dry-run/*/*/*/alt", "sync-dry-run"}, Reason: "y"},
		{ID: "DC-99", Cases: []string{"live"}, Reason: "z"},
	}}
	cases := []struct {
		name, caseID, group, want string
		ok                        bool
	}{
		{"group pattern", "snapshot-all-csv/multi/default/pipe/fixed", "snapshot-all-csv", "DC-05", true},
		{"axis slice", "sync-dry-run/multi/default/pipe/alt", "sync-dry-run", "DC-21", true},
		{"bare pattern hits the group", "sync-dry-run/multi/default/pipe/fixed", "sync-dry-run", "DC-21", true},
		{"bare pattern hits the live step ID", "sync-dry-run", "live", "DC-21", true},
		{"bare group live", "repair-write", "live", "DC-99", true},
		{"no match", "snapshot-all/single/default/pipe/fixed", "snapshot-all", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := exp.Match(tc.caseID, tc.group)
			if id != tc.want || ok != tc.ok {
				t.Errorf("Match(%q, %q) = %q, %t; want %q, %t", tc.caseID, tc.group, id, ok, tc.want, tc.ok)
			}
		})
	}

	t.Run("first entry wins", func(t *testing.T) {
		two := &Expected{Entries: []ExpectedEntry{
			{ID: "DC-01", Cases: []string{"sync-dry-run"}, Reason: "a"},
			{ID: "DC-02", Cases: []string{"sync-dry-run/multi/default/pipe/fixed"}, Reason: "b"},
		}}
		if id, ok := two.Match("sync-dry-run/multi/default/pipe/fixed", "sync-dry-run"); !ok || id != "DC-01" {
			t.Errorf("Match = %q, %t; want DC-01, true", id, ok)
		}
	})
	t.Run("nil and empty sets never match", func(t *testing.T) {
		var nilSet *Expected
		if id, ok := nilSet.Match("sync-dry-run", "live"); ok || id != "" {
			t.Errorf("nil set matched: %q", id)
		}
		if id, ok := (&Expected{}).Match("sync-dry-run", "live"); ok || id != "" {
			t.Errorf("empty set matched: %q", id)
		}
	})
}

// R4: per-entry tallies over the executed results and the stale derivation.
func TestExpectedStats(t *testing.T) {
	exp := &Expected{Entries: []ExpectedEntry{
		{ID: "DC-05", Cases: []string{"diff"}, Reason: "x"},
		{ID: "DC-06", Cases: []string{"absent"}, Reason: "y"},
	}}
	mk := func(id, group, status string) Result {
		return Result{Case: Case{ID: id, Group: group}, Status: status}
	}
	red := mk("diff/single/default/pipe/fixed", "diff", StatusRed)
	red.Expected = "DC-05"
	green := mk("same/single/default/pipe/fixed", "same", StatusGreen)

	stats, stale := ExpectedStats(exp, []Result{green, red})
	if len(stats) != 2 || stats[0] != (ExpectedStat{ID: "DC-05", Matched: 1, Red: 1}) {
		t.Errorf("stats = %+v", stats)
	}
	if len(stale) != 0 {
		t.Errorf("stale = %v", stale)
	}
	if stats[1].Matched != 0 {
		t.Errorf("DC-06 matched %d, want 0 (no executed case)", stats[1].Matched)
	}

	// The same set with every case green: DC-05 matched the diff case but
	// nothing is red, so it is stale; DC-06 matched nothing.
	greenDiff := mk("diff/single/default/pipe/fixed", "diff", StatusGreen)
	stats, stale = ExpectedStats(exp, []Result{green, greenDiff})
	if len(stale) != 1 || stale[0] != "DC-05" {
		t.Errorf("stale = %v, want [DC-05]", stale)
	}
	if stats[0] != (ExpectedStat{ID: "DC-05", Matched: 1, Red: 0}) {
		t.Errorf("stats[0] = %+v", stats[0])
	}

	if stats, stale := ExpectedStats(nil, []Result{red}); stats != nil || stale != nil {
		t.Errorf("nil set: stats = %+v, stale = %v", stats, stale)
	}
}
