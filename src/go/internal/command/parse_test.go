package command

import (
	"testing"

	"github.com/sahil87/tu/internal/query"
)

// TestParseValid covers the argv shapes the grammar accepts — including every
// harness/matrix.json case group that reaches the parser without a usage
// error.
func TestParseValid(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want Request
	}{
		{"bare", nil, Request{}},
		{"cc", []string{"cc"}, Request{Source: "cc"}},
		{"codex", []string{"codex"}, Request{Source: "codex"}},
		{"co alias", []string{"co"}, Request{Source: "codex"}},
		{"oc", []string{"oc"}, Request{Source: "oc"}},
		{"gemini", []string{"gemini"}, Request{Source: "gemini"}},
		{"gem alias", []string{"gem"}, Request{Source: "gemini"}},
		{"copilot", []string{"copilot"}, Request{Source: "copilot"}},
		{"cop alias", []string{"cop"}, Request{Source: "copilot"}},
		{"kimi", []string{"kimi"}, Request{Source: "kimi"}},
		{"ki alias", []string{"ki"}, Request{Source: "kimi"}},
		{"all", []string{"all"}, Request{Source: ""}},
		{"w", []string{"w"}, Request{Period: query.Weekly}},
		{"m", []string{"m"}, Request{Period: query.Monthly}},
		{"cc m", []string{"cc", "m"}, Request{Source: "cc", Period: query.Monthly}},
		{"cc mh", []string{"cc", "mh"}, Request{Source: "cc", Period: query.Monthly, Display: History}},
		{"wh", []string{"wh"}, Request{Period: query.Weekly, Display: History}},
		{"dh", []string{"dh"}, Request{Display: History}},
		{"h", []string{"h"}, Request{Display: History}},
		{"history", []string{"history"}, Request{Display: History}},
		{"lb", []string{"lb"}, Request{Display: Leaderboard}},
		{"m lb", []string{"m", "lb"}, Request{Period: query.Monthly, Display: Leaderboard}},
		{"cc m lb", []string{"cc", "m", "lb"}, Request{Source: "cc", Period: query.Monthly, Display: Leaderboard}},
		{"lbh", []string{"lbh"}, Request{Display: LeaderboardHistory}},
		{"w lbh", []string{"w", "lbh"}, Request{Period: query.Weekly, Display: LeaderboardHistory}},
		{"--json", []string{"--json"}, Request{Format: JSON}},
		{"-j", []string{"-j"}, Request{Format: JSON}},
		{"--csv", []string{"--csv"}, Request{Format: CSV}},
		{"--md", []string{"--md"}, Request{Format: Markdown}},
		{"cc --json", []string{"cc", "--json"}, Request{Source: "cc", Format: JSON}},
		{"--by-machine", []string{"--by-machine"}, Request{Flags: Flags{ByMachine: true, Interval: 10}}},
		{"-t", []string{"-t"}, Request{Flags: Flags{Interval: 10, Metric: Tokens}}},
		{"--metric tokens", []string{"--metric", "tokens"}, Request{Flags: Flags{Interval: 10, Metric: Tokens}}},
		{"--metric cost", []string{"--metric", "cost"}, Request{Flags: Flags{Interval: 10, Metric: Cost}}},
		{"-t --metric tokens", []string{"-t", "--metric", "tokens"}, Request{Flags: Flags{Interval: 10, Metric: Tokens}}},
		{"--fresh", []string{"--fresh"}, Request{Flags: Flags{Fresh: true, Interval: 10}}},
		{"-f", []string{"-f"}, Request{Flags: Flags{Fresh: true, Interval: 10}}},
		{"cc --fresh", []string{"cc", "--fresh"}, Request{Source: "cc", Flags: Flags{Fresh: true, Interval: 10}}},
		{"--no-color", []string{"--no-color"}, Request{Flags: Flags{NoColor: true, Interval: 10}}},
		{"--sync", []string{"--sync"}, Request{Flags: Flags{Sync: true, Interval: 10}}},
		{"cc --sync", []string{"cc", "--sync"}, Request{Source: "cc", Flags: Flags{Sync: true, Interval: 10}}},
		{"--dry-run", []string{"--dry-run"}, Request{Flags: Flags{DryRun: true, Interval: 10}}},
		{"sync --dry-run", []string{"sync", "--dry-run"}, Request{Command: "sync", Flags: Flags{DryRun: true, Interval: 10}}},
		{"--full history", []string{"h", "--full"}, Request{Display: History, Flags: Flags{Full: true, Interval: 10}}},
		{"window", []string{"h", "--since", "2026-01-01", "--until", "2026-01-31"}, Request{Display: History, Flags: Flags{Interval: 10, Since: "2026-01-01", Until: "2026-01-31"}}},
		{"since compact", []string{"--since", "20260101"}, Request{Flags: Flags{Interval: 10, Since: "2026-01-01"}}},
		{"since impossible shape ok", []string{"--since", "2026-13-01"}, Request{Flags: Flags{Interval: 10, Since: "2026-13-01"}}},
		{"-u user", []string{"-u", "other-user"}, Request{Flags: Flags{Interval: 10, User: "other-user"}}},
		{"lb -u all", []string{"lb", "-u", "all"}, Request{Display: Leaderboard, Flags: Flags{Interval: 10, User: "all"}}},
		{"lb --top 1", []string{"lb", "--top", "1"}, Request{Display: Leaderboard, Flags: Flags{Interval: 10, Top: 1}}},
		{"interval without watch ignored", []string{"--interval", "3"}, Request{Flags: Flags{Interval: 10}}},
		// A non-numeric --interval value is NOT consumed: it becomes a
		// positional and is an unknown argument (the TS behaves the same).
		{"watch interval", []string{"--watch", "--interval", "30"}, Request{Flags: Flags{Watch: true, Interval: 30}}},
		{"ki m -j --fresh", []string{"ki", "m", "-j", "--fresh"}, Request{Source: "kimi", Period: query.Monthly, Format: JSON, Flags: Flags{Fresh: true, Interval: 10}}},
		{"help", []string{"help"}, Request{Command: "help"}},
		{"-h", []string{"-h"}, Request{Command: "-h"}},
		{"--help", []string{"--help"}, Request{Command: "--help"}},
		{"help-dump", []string{"help-dump"}, Request{Command: "help-dump"}},
		{"skill", []string{"skill"}, Request{Command: "skill"}},
		{"shell-init bash", []string{"shell-init", "bash"}, Request{Command: "shell-init"}},
		{"shell-init missing", []string{"shell-init"}, Request{Command: "shell-init"}},
		{"status", []string{"status"}, Request{Command: "status"}},
		{"init-conf", []string{"init-conf"}, Request{Command: "init-conf"}},
		{"init-metrics url", []string{"init-metrics", "git@example.invalid:harness/tu-metrics.git"}, Request{Command: "init-metrics"}},
		{"sync", []string{"sync"}, Request{Command: "sync"}},
		{"update", []string{"update"}, Request{Command: "update"}},
		{"version", []string{"--version"}, Request{Version: true}},
		{"-V", []string{"-V"}, Request{Version: true}},
		{"-v", []string{"-v"}, Request{Version: true}},
		{"cc --version", []string{"cc", "--version"}, Request{Version: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := c.want
			if want.Flags.Interval == 0 {
				want.Flags.Interval = 10 // the default, omitted from the table for brevity
			}
			got, uerr := Parse(c.args)
			if uerr != nil {
				t.Fatalf("Parse(%q) error = %v, want success", c.args, uerr.Message)
			}
			if got != want {
				t.Errorf("Parse(%q) = %+v, want %+v", c.args, got, want)
			}
		})
	}
}

// TestParseUsageErrors pins the byte-exact stderr messages and ShowUsage
// flags for every rejection path (exit 2 at the edge).
func TestParseUsageErrors(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		message   string
		showUsage bool
	}{
		{"bogus", []string{"bogus"}, "Unknown argument: bogus", true},
		{"cc codex", []string{"cc", "codex"}, "Unknown argument: codex", true},
		{"m cc", []string{"m", "cc"}, "Unknown argument: cc", true},
		{"cc --help", []string{"cc", "--help"}, "Unknown argument: --help", true},
		{"unknown flag", []string{"--bogus"}, "Unknown argument: --bogus", true},
		{"json csv", []string{"--json", "--csv"}, "Error: --json and --csv are incompatible", false},
		{"-j counts as --json", []string{"-j", "--csv"}, "Error: --json and --csv are incompatible", false},
		{"watch json", []string{"--watch", "--json"}, "Error: --watch and --json are incompatible", false},
		{"json md", []string{"--json", "--md"}, "Error: --json and --md are incompatible", false},
		{"csv md", []string{"--csv", "--md"}, "Error: --csv and --md are incompatible", false},
		{"watch csv", []string{"--watch", "--csv"}, "Error: --watch and --csv are incompatible", false},
		{"watch md", []string{"--watch", "--md"}, "Error: --watch and --md are incompatible", false},
		{"user missing", []string{"-u"}, "Error: -u requires a username", false},
		{"user dash value", []string{"-u", "--json"}, "Error: -u requires a username", false},
		{"since missing", []string{"--since"}, "Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)", false},
		{"since malformed", []string{"--since", "yesterday"}, "Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)", false},
		{"until missing", []string{"--until"}, "Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)", false},
		{"until malformed", []string{"--until", "2026/01/01"}, "Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)", false},
		{"window reversed", []string{"--since", "2026-02-01", "--until", "2026-01-01"}, "Error: --since must be on or before --until", false},
		{"metric missing", []string{"--metric"}, "Error: --metric requires 'tokens' or 'cost'", false},
		{"metric unknown", []string{"--metric", "foo"}, "Error: --metric requires 'tokens' or 'cost'", false},
		{"truncate metric", []string{"-t", "--metric", "cost"}, "Error: -t and --metric cost are incompatible", false},
		{"top missing", []string{"--top"}, "Error: --top requires a positive integer", false},
		{"top invalid", []string{"--top", "x"}, "Error: --top requires a positive integer", false},
		{"top zero", []string{"--top", "0"}, "Error: --top requires a positive integer", false},
		{"watch interval missing", []string{"--watch", "--interval"}, "Error: --interval requires a numeric value", false},
		{"watch interval nonnumeric", []string{"--watch", "--interval", "abc"}, "Error: --interval requires a numeric value", false},
		{"interval nonnumeric without watch", []string{"--interval", "abc"}, "Unknown argument: abc", true},
		{"watch interval min", []string{"--watch", "--interval", "3"}, "Error: --interval minimum is 5 seconds", false},
		{"watch interval max", []string{"--watch", "--interval", "3601"}, "Error: --interval maximum is 3600 seconds", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, uerr := Parse(c.args)
			if uerr == nil {
				t.Fatalf("Parse(%q) succeeded, want error %q", c.args, c.message)
			}
			if uerr.Message != c.message {
				t.Errorf("Parse(%q) message = %q, want %q", c.args, uerr.Message, c.message)
			}
			if uerr.ShowUsage != c.showUsage {
				t.Errorf("Parse(%q) ShowUsage = %v, want %v", c.args, uerr.ShowUsage, c.showUsage)
			}
		})
	}
}

// TestParseValidationOrder pins the TS first-failure order: version is
// checked after validation, and validation precedes positional errors.
func TestParseValidationOrder(t *testing.T) {
	if _, uerr := Parse([]string{"--json", "--csv", "--version"}); uerr == nil || uerr.Message != "Error: --json and --csv are incompatible" {
		t.Errorf("version must not precede validation, got %v", uerr)
	}
	if _, uerr := Parse([]string{"bogus", "--json", "--csv"}); uerr == nil || uerr.Message != "Error: --json and --csv are incompatible" {
		t.Errorf("validation must precede positional errors, got %v", uerr)
	}
	// --interval validation runs before the format conflicts.
	if _, uerr := Parse([]string{"--watch", "--json", "--interval", "3"}); uerr == nil || uerr.Message != "Error: --interval minimum is 5 seconds" {
		t.Errorf("interval check must precede format conflicts, got %v", uerr)
	}
}

func TestShortUsageBytes(t *testing.T) {
	want := "Usage: tu [source] [period] [display]\n\n  tu                Today's cost, all tools\n  tu cc             Today's cost, Claude Code\n  tu mh             Monthly cost history, all tools\n  tu -h             Show full help\n\nRun 'tu help' for all commands."
	if ShortUsage != want {
		t.Errorf("ShortUsage = %q", ShortUsage)
	}
}
