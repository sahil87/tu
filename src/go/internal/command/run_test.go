package command

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/query"
	"github.com/sahil87/tu/internal/render/ansi"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/ccusage"
)

// fakeFetcher is an in-memory Fetcher: it hands back records per tool and
// records its calls.
type fakeFetcher struct {
	byTool    map[string][]fact.Record
	fetchErrs map[string]*source.Error
	calls     []fetchCall
}

type fetchCall struct {
	tool  string // "" for FetchAll
	fresh bool
}

func (f *fakeFetcher) Fetch(_ context.Context, tool ccusage.Tool, _ string, _ []string, fresh bool) ([]fact.Record, *source.Error) {
	f.calls = append(f.calls, fetchCall{tool: tool.Key, fresh: fresh})
	return f.byTool[tool.Key], f.fetchErrs[tool.Key]
}

func (f *fakeFetcher) FetchAll(_ context.Context, _ string, _ []string, fresh bool) ([]fact.Record, []*source.Error) {
	f.calls = append(f.calls, fetchCall{fresh: fresh})
	var recs []fact.Record
	var errs []*source.Error
	for _, tool := range ccusage.Tools {
		recs = append(recs, f.byTool[tool.Key]...)
		if err := f.fetchErrs[tool.Key]; err != nil {
			errs = append(errs, err)
		}
	}
	return recs, errs
}

var fakeTotals = fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

// fixedNow is 2026-01-06 12:00 in +05:30: a date the placeholder corpus
// covers, so the daily window matches fixture records.
func fixedNow() time.Time {
	return time.Date(2026, 1, 6, 12, 0, 0, 0, time.FixedZone("IST", 5*3600+1800))
}

func fakeDeps(f *fakeFetcher) Deps {
	return Deps{Source: f, Now: fixedNow, Colors: ansi.Colors{Enabled: true}}
}

func TestRunDailyAllDropsLabels(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc":    {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
		"codex": {{Date: "2026-01-06", Tool: "codex", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{Format: JSON}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if strings.Contains(joined, `"label"`) {
		t.Errorf("daily-all JSON must not carry label keys:\n%s", joined)
	}
	if !strings.Contains(joined, "\"Claude Code\": {\n    \"totalCost\": 0.5,") {
		t.Errorf("populated Claude Code object missing:\n%s", joined)
	}
	if len(f.calls) != 1 || f.calls[0].tool != "" {
		t.Errorf("calls = %+v, want one FetchAll", f.calls)
	}
}

func TestRunMonthlyAllKeepsLabels(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc":    {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
		"codex": {{Date: "2026-01-06", Tool: "codex", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{Period: query.Monthly, Format: JSON}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if strings.Count(joined, `"label": "2026-01",`) != 2 {
		t.Errorf("want label 2026-01 on the two populated tools:\n%s", joined)
	}
}

func TestRunSingleSourceDailyKeepsLabel(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc": {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{Source: "cc", Format: JSON}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, `"label": "2026-01-06",`) {
		t.Errorf("single-source daily JSON must carry the label first:\n%s", joined)
	}
	if strings.Contains(joined, `"Codex"`) {
		t.Errorf("single-source JSON must contain only that tool:\n%s", joined)
	}
	if len(f.calls) != 1 || f.calls[0].tool != "cc" {
		t.Errorf("calls = %+v, want exactly one Fetch for cc", f.calls)
	}
}

func TestRunWeeklyLabel(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc": {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{Source: "cc", Format: JSON, Period: query.Weekly}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, `"label": "2026-01-04",`) {
		t.Errorf("weekly label is the week's Sunday:\n%s", joined)
	}
}

func TestRunFreshPropagates(t *testing.T) {
	f := &fakeFetcher{}
	req := Request{Flags: Flags{Fresh: true, Interval: 10}}
	if _, err := Run(context.Background(), req, config.Single, fakeDeps(f)); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !f.calls[0].fresh {
		t.Errorf("calls = %+v, want fresh=true", f.calls)
	}
}

func TestRunTableLines(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc":    {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
		"codex": {{Date: "2026-01-06", Tool: "codex", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"\x1b[1;37m📊 Combined Usage (daily)\x1b[0m",
		"Claude Code  |       24,400 |        3,000 |          400 |       21,000 |        $0.50",
		"\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m      48,800\x1b[0m",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("table missing %q:\n%s", want, joined)
		}
	}
}

func TestRunEmptyState(t *testing.T) {
	f := &fakeFetcher{}
	res, err := Run(context.Background(), Request{}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"", "\x1b[1;37m📊 Combined Usage (daily)\x1b[0m", "", "  No usage", ""}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
}

func TestRunResultStats(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc":    {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}},
		"codex": {{Date: "2026-01-06", Tool: "codex", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCost != 1.0 || res.TotalTokens != 48800 {
		t.Errorf("stats = %v cost / %v tokens, want 1.0 / 48800", res.TotalCost, res.TotalTokens)
	}
	wantMap := map[string]float64{"Claude Code": 0.5, "Codex": 0.5, "OpenCode": 0, "Gemini": 0, "Copilot": 0, "Kimi": 0}
	if !reflect.DeepEqual(res.CostByItem, wantMap) {
		t.Errorf("CostByItem = %v, want %v", res.CostByItem, wantMap)
	}
}

func TestRunSourceErrorBecomesWarning(t *testing.T) {
	fetchErr := &source.Error{Tool: "kimi", Name: "Kimi", Kind: source.KindExec, Detail: "spawn ccusage ENOENT"}
	f := &fakeFetcher{
		byTool:    map[string][]fact.Record{"cc": {{Date: "2026-01-06", Tool: "cc", Totals: fakeTotals}}},
		fetchErrs: map[string]*source.Error{"kimi": fetchErr},
	}
	res, err := Run(context.Background(), Request{}, config.Single, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || res.Warnings[0] != fetchErr {
		t.Errorf("Warnings = %v, want the kimi error", res.Warnings)
	}
	// The failed tool renders as zero data.
	joined := strings.Join(res.Lines, "\n")
	if strings.Contains(joined, "Kimi") {
		t.Errorf("failed tool must not render a row:\n%s", joined)
	}
}

func TestRunUnported(t *testing.T) {
	base := Flags{Interval: 10}
	cases := []struct {
		name string
		req  Request
		mode config.Mode
	}{
		{"multi mode", Request{Flags: base}, config.Multi},
		{"history", Request{Display: History, Flags: base}, config.Single},
		{"leaderboard", Request{Display: Leaderboard, Flags: base}, config.Single},
		{"lbh", Request{Display: LeaderboardHistory, Flags: base}, config.Single},
		{"csv", Request{Format: CSV, Flags: base}, config.Single},
		{"md", Request{Format: Markdown, Flags: base}, config.Single},
		{"watch", Request{Flags: Flags{Watch: true, Interval: 10}}, config.Single},
		{"sync", Request{Flags: Flags{Sync: true, Interval: 10}}, config.Single},
		{"dry-run", Request{Flags: Flags{DryRun: true, Interval: 10}}, config.Single},
		{"by-machine", Request{Flags: Flags{ByMachine: true, Interval: 10}}, config.Single},
		{"full", Request{Flags: Flags{Full: true, Interval: 10}}, config.Single},
		{"no-rain", Request{Flags: Flags{NoRain: true, Interval: 10}}, config.Single},
		{"skip-brew-update", Request{Flags: Flags{SkipBrewUpdate: true, Interval: 10}}, config.Single},
		{"user", Request{Flags: Flags{User: "alice", Interval: 10}}, config.Single},
		{"since", Request{Flags: Flags{Since: "2026-01-01", Interval: 10}}, config.Single},
		{"until", Request{Flags: Flags{Until: "2026-01-31", Interval: 10}}, config.Single},
		{"top", Request{Flags: Flags{Top: 3, Interval: 10}}, config.Single},
		{"command", Request{Command: "help", Flags: base}, config.Single},
		{"version", Request{Version: true, Flags: base}, config.Single},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeFetcher{}
			_, err := Run(context.Background(), c.req, c.mode, fakeDeps(f))
			if !errors.Is(err, ErrUnported) {
				t.Errorf("err = %v, want ErrUnported", err)
			}
			if len(f.calls) != 0 {
				t.Errorf("unported request must not fetch; calls = %+v", f.calls)
			}
		})
	}
}

// The Fetcher interface is satisfied by the real source adapter.
var _ Fetcher = (*ccusage.Source)(nil)
