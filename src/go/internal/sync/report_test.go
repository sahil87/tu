package sync

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

// --- T006: Report.Format (R7, the TS formatDrySyncReport) ---

// decision builds one Decision with an optional existing cost.
func decision(path string, action Action, incoming float64, existing *float64) Decision {
	return Decision{Path: path, Action: action, IncomingCost: incoming, ExistingCost: existing}
}

// R7: the full shape — write lines (update and new), then the skip block,
// writes of every tool collected before the skips, the tildefied dir header,
// and the commit/dry-run tails.
func TestReportFormatFull(t *testing.T) {
	home := string(filepath.Separator) + "home" + string(filepath.Separator) + "tester"
	metricsDir := filepath.Join(home, ".tu", "metrics_repo")
	user, machine := "harness-user", "harness-machine"
	day := func(toolKey, date string) string {
		return filepath.Join(metricsDir, user, "2026", machine, toolKey+"-"+date+".jsonl")
	}
	r := Report{
		MetricsDir: metricsDir,
		User:       user,
		Machine:    machine,
		Tools: []ToolReport{
			{Tool: ccTool, Decisions: []Decision{
				decision(day("cc", "2026-01-05"), ActionWrite, 0.5, ptrFloat(0.25)),
				decision(day("cc", "2026-01-06"), ActionSkip, 0.5, ptrFloat(0.75)),
				decision(day("cc", "2026-01-07"), ActionWrite, 0.5, nil),
			}},
			{Tool: fact.Tool{Key: "codex", Name: "Codex"}, Decisions: []Decision{
				decision(day("codex", "2026-01-05"), ActionWrite, 0.5, nil),
			}},
		},
		WouldCommit:   true,
		CommitMessage: "# harness-user: update 2026-09-15",
	}
	want := []string{
		"Would write 3 day-file(s) under ~/.tu/metrics_repo/harness-user/:",
		"  2026/harness-machine/cc-2026-01-05.jsonl  $0.50  (update: $0.25 → $0.50)",
		"  2026/harness-machine/cc-2026-01-07.jsonl  $0.50  (new)",
		// The codex write lands before the cc skip: writes of every tool are
		// collected first, then the skips (the TS two accumulators).
		"  2026/harness-machine/codex-2026-01-05.jsonl  $0.50  (new)",
		"Would skip 1 file(s) (never-shrink guard):",
		"  2026/harness-machine/cc-2026-01-06.jsonl  incoming $0.50 < existing $0.75",
		`Would commit: "# harness-user: update 2026-09-15", then pull --rebase origin main, then push`,
		"Dry run — nothing written, committed, or pushed.",
	}
	if got := r.Format(home); !equalStrings(got, want) {
		t.Errorf("Format =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
}

// R7: no decisions and WouldCommit false — the zero-write single line, no
// skip block, the "nothing (no changes)" commit line.
func TestReportFormatZero(t *testing.T) {
	r := Report{MetricsDir: "/m", User: "u", Machine: "m"}
	want := []string{
		"Would write 0 day-file(s) under /m/u/.",
		"Would commit: nothing (no changes), then pull --rebase origin main, then push",
		"Dry run — nothing written, committed, or pushed.",
	}
	if got := r.Format(""); !equalStrings(got, want) {
		t.Errorf("Format =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
}

// R7: writes without skips emit no skip block at all.
func TestReportFormatNoSkipBlock(t *testing.T) {
	metricsDir := filepath.Join("m")
	path := filepath.Join(metricsDir, "u", "2026", "mach", "cc-2026-01-05.jsonl")
	r := Report{
		MetricsDir:    metricsDir,
		User:          "u",
		Machine:       "mach",
		Tools:         []ToolReport{{Tool: ccTool, Decisions: []Decision{decision(path, ActionWrite, 1.0, nil)}}},
		WouldCommit:   true,
		CommitMessage: "# u: update 2026-09-15",
	}
	want := []string{
		"Would write 1 day-file(s) under m/u/:",
		"  2026/mach/cc-2026-01-05.jsonl  $1.00  (new)",
		`Would commit: "# u: update 2026-09-15", then pull --rebase origin main, then push`,
		"Dry run — nothing written, committed, or pushed.",
	}
	if got := r.Format(""); !equalStrings(got, want) {
		t.Errorf("Format =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
}

// R7: a relative metrics_dir is not tildefied, the relative user prefix is
// still stripped from decision names, and a path lacking the prefix is left
// unchanged.
func TestReportFormatRelativeMetricsDir(t *testing.T) {
	metricsDir := filepath.Join("rel", "metrics")
	inPrefix := filepath.Join(metricsDir, "u", "2026", "m", "cc-2026-01-05.jsonl")
	outside := filepath.Join("elsewhere", "cc-2026-01-06.jsonl")
	r := Report{
		MetricsDir: metricsDir,
		User:       "u",
		Machine:    "m",
		Tools: []ToolReport{{Tool: ccTool, Decisions: []Decision{
			decision(inPrefix, ActionWrite, 0.5, nil),
			decision(outside, ActionWrite, 0.5, nil),
		}}},
		WouldCommit:   true,
		CommitMessage: "# u: update 2026-09-15",
	}
	want := []string{
		"Would write 2 day-file(s) under rel/metrics/u/:",
		"  2026/m/cc-2026-01-05.jsonl  $0.50  (new)",
		"  " + filepath.Join("elsewhere", "cc-2026-01-06.jsonl") + "  $0.50  (new)",
		`Would commit: "# u: update 2026-09-15", then pull --rebase origin main, then push`,
		"Dry run — nothing written, committed, or pushed.",
	}
	if got := r.Format(filepath.Join("home", "tester")); !equalStrings(got, want) {
		t.Errorf("Format =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
}

// R7/DC-22: no thousands separators in either block — $1234.50, never
// $1,234.50.
func TestReportFormatNoThousandsSeparators(t *testing.T) {
	metricsDir := "/m"
	path := filepath.Join(metricsDir, "u", "2026", "mach", "cc-2026-01-05.jsonl")
	r := Report{
		MetricsDir: metricsDir,
		User:       "u",
		Machine:    "mach",
		Tools: []ToolReport{{Tool: ccTool, Decisions: []Decision{
			decision(path, ActionWrite, 1234.5, ptrFloat(1234.5)),
			decision(path, ActionSkip, 1234.5, ptrFloat(9999.5)),
		}}},
		WouldCommit:   true,
		CommitMessage: "# u: update 2026-09-15",
	}
	want := []string{
		"Would write 1 day-file(s) under /m/u/:",
		"  2026/mach/cc-2026-01-05.jsonl  $1234.50  (update: $1234.50 → $1234.50)",
		"Would skip 1 file(s) (never-shrink guard):",
		"  2026/mach/cc-2026-01-05.jsonl  incoming $1234.50 < existing $9999.50",
		`Would commit: "# u: update 2026-09-15", then pull --rebase origin main, then push`,
		"Dry run — nothing written, committed, or pushed.",
	}
	if got := r.Format(""); !equalStrings(got, want) {
		t.Errorf("Format =\n%s\nwant\n%s", joinLines(got), joinLines(want))
	}
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
