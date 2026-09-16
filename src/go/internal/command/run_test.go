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

func (f *fakeFetcher) Fetch(_ context.Context, tool fact.Tool, _ string, _ []string, fresh bool) ([]fact.Record, *source.Error) {
	f.calls = append(f.calls, fetchCall{tool: tool.Key, fresh: fresh})
	return f.byTool[tool.Key], f.fetchErrs[tool.Key]
}

func (f *fakeFetcher) FetchAll(_ context.Context, _ string, _ []string, fresh bool) ([]fact.Record, []*source.Error) {
	f.calls = append(f.calls, fetchCall{fresh: fresh})
	var recs []fact.Record
	var errs []*source.Error
	for _, tool := range fact.Tools {
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
	return Deps{Source: f, Now: fixedNow, Colors: ansi.Colors{Enabled: true}, Width: 80}
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
		{"leaderboard", Request{Display: Leaderboard, Flags: base}, config.Single},
		{"lbh", Request{Display: LeaderboardHistory, Flags: base}, config.Single},
		{"watch", Request{Flags: Flags{Watch: true, Interval: 10}}, config.Single},
		{"sync", Request{Flags: Flags{Sync: true, Interval: 10}}, config.Single},
		{"dry-run", Request{Flags: Flags{DryRun: true, Interval: 10}}, config.Single},
		{"by-machine", Request{Flags: Flags{ByMachine: true, Interval: 10}}, config.Single},
		{"no-rain", Request{Flags: Flags{NoRain: true, Interval: 10}}, config.Single},
		{"skip-brew-update", Request{Flags: Flags{SkipBrewUpdate: true, Interval: 10}}, config.Single},
		{"user", Request{Flags: Flags{User: "alice", Interval: 10}}, config.Single},
		{"top", Request{Flags: Flags{Top: 3, Interval: 10}}, config.Single},
		{"command", Request{Command: "help", Flags: base}, config.Single},
		{"version", Request{Version: true, Flags: base}, config.Single},
		// History forms of the still-unported surfaces (B3/B4).
		{"history multi mode", Request{Display: History, Flags: base}, config.Multi},
		{"history by-machine", Request{Display: History, Flags: Flags{ByMachine: true, Interval: 10}}, config.Single},
		{"history user", Request{Display: History, Flags: Flags{User: "alice", Interval: 10}}, config.Single},
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

// ── B2: history ────────────────────────────────────────────────────────────

// historyNow is outside the placeholder window, so no row carries the
// current-period marker and the lines equal the intake §12 captures.
func historyNow() time.Time {
	return time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
}

func historyDeps(f *fakeFetcher, width int) Deps {
	return Deps{Source: f, Now: historyNow, Colors: ansi.Colors{Enabled: true}, Width: width}
}

// placeholderCorpus is the fake Fetcher content mirroring the harness corpus:
// three days, every tool at $0.50 / 24,400 tokens.
func placeholderCorpus() map[string][]fact.Record {
	byTool := make(map[string][]fact.Record)
	for _, tool := range fact.Tools {
		for _, d := range []string{"2026-01-05", "2026-01-06", "2026-01-07"} {
			byTool[tool.Key] = append(byTool[tool.Key], fact.Record{Date: d, Tool: tool.Key, Totals: fakeTotals})
		}
	}
	return byTool
}

// R17: the single-source window renders the intake §12 cc-h-window bytes.
func TestRunHistorySingleSourceWindow(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Source:  "cc",
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notices) != 0 {
		t.Errorf("Notices = %v, want empty", res.Notices)
	}
	if len(f.calls) != 1 || f.calls[0].tool != "cc" {
		t.Errorf("calls = %+v, want exactly one Fetch for cc", f.calls)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"\x1b[1;37m📊 Claude Code (daily)\x1b[0m",
		"2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50",
		"\x1b[1;37mTotal       \x1b[0m | \x1b[1;37m         9,000\x1b[0m",
		"\x1b[2mavg $0.50/day · peak $0.50 (2026-01-05)\x1b[0m",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("history lines missing %q:\n%s", want, joined)
		}
	}

	// R18: stats and the buildCostMap key scheme, valued in the display metric.
	if res.TotalCost != 1.5 || res.TotalTokens != 73200 {
		t.Errorf("stats = %v / %v, want 1.5 / 73200", res.TotalCost, res.TotalTokens)
	}
	if res.CostByItem["Claude Code:2026-01-05"] != 0.5 || res.CostByItem["total:2026-01-05"] != 0.5 {
		t.Errorf("CostByItem = %v", res.CostByItem)
	}
}

// R17: the all-tools window renders the intake §12 h-window bytes.
func TestRunHistoryAllToolsWindow(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"\x1b[1;37m📊 Combined Cost History (daily)\x1b[0m",
		"\x1b[1;36mDate      \x1b[0m | \x1b[1;36mClaude Code\x1b[0m",
		"2026-01-05 |       $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $0.50 |     $3.00",
		"\x1b[2mavg $3.00/day · peak $3.00 (2026-01-05)\x1b[0m",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("pivot lines missing %q:\n%s", want, joined)
		}
	}
	if res.TotalCost != 9.0 {
		t.Errorf("TotalCost = %v, want 9.0", res.TotalCost)
	}
	if res.CostByItem["Kimi:2026-01-07"] != 0.5 || res.CostByItem["total:2026-01-07"] != 3.0 {
		t.Errorf("CostByItem = %v", res.CostByItem)
	}
}

// R2: the window applies to daily records before the weekly roll-up, so the
// single weekly row carries the leading Sunday label 2026-01-04.
func TestRunHistoryWindowBeforeRollUp(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Display: History,
		Period:  query.Weekly,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "\x1b[1;37m📊 Combined Cost History (weekly)\x1b[0m") {
		t.Errorf("missing weekly title:\n%s", joined)
	}
	if !strings.Contains(joined, "2026-01-04 |") {
		t.Errorf("missing the leading Sunday row 2026-01-04:\n%s", joined)
	}
	if !strings.Contains(joined, "|     $9.00") {
		t.Errorf("the single week sums the three days:\n%s", joined)
	}
}

// The cap: bare `h` daily history defaults Since to the three-month floor and
// carries the heading hint; the placeholder corpus (January) falls outside.
func TestRunHistoryCap(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{Display: History, Flags: Flags{Interval: 10}}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"", "\x1b[1;37m📊 Combined Cost History (daily, last 3 months)\x1b[0m", "", "  No data", ""}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Errorf("lines = %q, want %q", res.Lines, want)
	}
	if len(f.calls) != 1 {
		t.Errorf("calls = %+v, want the daily FetchAll", f.calls)
	}
}

// --full disables the cap: bare `h --full` renders the January rows with no hint.
func TestRunHistoryFull(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{Display: History, Flags: Flags{Full: true, Interval: 10}}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "📊 Combined Cost History (daily)\x1b[0m") {
		t.Errorf("missing the uncapped title:\n%s", joined)
	}
	if !strings.Contains(joined, "2026-01-05 |") {
		t.Errorf("missing the placeholder rows:\n%s", joined)
	}
}

// R17: the history formats route through the encoders.
func TestRunHistoryFormats(t *testing.T) {
	mh := Request{
		Source: "cc", Display: History, Period: query.Monthly,
		Flags: Flags{Interval: 10},
	}

	t.Run("json single", func(t *testing.T) {
		f := &fakeFetcher{byTool: placeholderCorpus()}
		req := mh
		req.Format = JSON
		res, err := Run(context.Background(), req, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		want := "[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 1.5,\n    \"inputTokens\": 9000,\n    \"outputTokens\": 1200,\n    \"cacheCreationTokens\": 3000,\n    \"cacheReadTokens\": 60000,\n    \"totalTokens\": 73200\n  }\n]"
		if got := strings.Join(res.Lines, "\n"); got != want {
			t.Errorf("json =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("json all empty", func(t *testing.T) {
		// The cap floors the window past the placeholder corpus: six empty arrays.
		f := &fakeFetcher{byTool: placeholderCorpus()}
		res, err := Run(context.Background(), Request{Display: History, Format: JSON, Flags: Flags{Interval: 10}}, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		want := "{\n  \"Claude Code\": [],\n  \"Codex\": [],\n  \"OpenCode\": [],\n  \"Gemini\": [],\n  \"Copilot\": [],\n  \"Kimi\": []\n}"
		if got := strings.Join(res.Lines, "\n"); got != want {
			t.Errorf("json =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("csv single", func(t *testing.T) {
		f := &fakeFetcher{byTool: placeholderCorpus()}
		req := mh
		req.Format = CSV
		res, err := Run(context.Background(), req, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"date,input,output,cache_write,cache_read,total,cost", "2026-01,9000,1200,3000,60000,73200,1.50"}
		if !reflect.DeepEqual(res.Lines, want) {
			t.Errorf("csv = %q, want %q", res.Lines, want)
		}
	})

	t.Run("csv all empty", func(t *testing.T) {
		f := &fakeFetcher{byTool: placeholderCorpus()}
		res, err := Run(context.Background(), Request{Display: History, Format: CSV, Flags: Flags{Interval: 10}}, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"date,Claude Code,Codex,OpenCode,Gemini,Copilot,Kimi,total"}
		if !reflect.DeepEqual(res.Lines, want) {
			t.Errorf("csv = %q, want %q", res.Lines, want)
		}
	})

	t.Run("markdown single", func(t *testing.T) {
		f := &fakeFetcher{byTool: placeholderCorpus()}
		req := mh
		req.Format = Markdown
		res, err := Run(context.Background(), req, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			"## Claude Code (monthly)",
			"",
			"| Date | Input | Output | Cache Write | Cache Read | Total | Cost |",
			"| :--- | ---: | ---: | ---: | ---: | ---: | ---: |",
			"| 2026-01 | 9,000 | 1,200 | 3,000 | 60,000 | 73,200 | $1.50 |",
			"",
		}
		if !reflect.DeepEqual(res.Lines, want) {
			t.Errorf("md = %q, want %q", res.Lines, want)
		}
	})

	t.Run("markdown all empty capped", func(t *testing.T) {
		f := &fakeFetcher{byTool: placeholderCorpus()}
		res, err := Run(context.Background(), Request{Display: History, Format: Markdown, Flags: Flags{Interval: 10}}, config.Single, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		if res.Lines[0] != "## Combined Cost History (daily, last 3 months)" {
			t.Errorf("md title = %q, want the cap hint", res.Lines[0])
		}
	})
}

// R15: a snapshot with --since warns (a Notice) and still renders, in scope.
func TestRunSnapshotSinceWarnsAndRenders(t *testing.T) {
	f := &fakeFetcher{}
	res, err := Run(context.Background(), Request{Flags: Flags{Since: "2026-13-01", Interval: 10}}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notices) != 1 || res.Notices[0] != "Warning: --since/--until apply to history display — ignoring." {
		t.Errorf("Notices = %v", res.Notices)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "📊 Combined Usage (daily)") {
		t.Errorf("snapshot did not render:\n%s", joined)
	}
}

// R9: token mode on the history tables swaps the unit, title and thresholds.
func TestRunHistoryTokenMode(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Display: History,
		Flags:   Flags{Metric: Tokens, Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{"📊 Combined Token History (daily)", "   Tokens\x1b[0m", "24,400", "avg 146,400/day"} {
		if !strings.Contains(joined, want) {
			t.Errorf("token pivot missing %q:\n%s", want, joined)
		}
	}
	if res.CostByItem["total:2026-01-05"] != 146400 {
		t.Errorf("token-valued CostByItem = %v", res.CostByItem)
	}
}

// The current-period marker: with Now inside the window the matching row's
// label cell renders boldWhite (invisible in the harness; pinned here).
func TestRunHistoryCurrentMarker(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	deps := historyDeps(f, 80)
	deps.Now = fixedNow // 2026-01-06
	res, err := Run(context.Background(), Request{
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, deps)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range res.Lines {
		if strings.Contains(l, "2026-01-06") && !strings.Contains(l, "\x1b[1;37m2026-01-06") {
			t.Errorf("the current row's label must be boldWhite: %q", l)
		}
	}
}

// Width 120 leaves room for bars on the single-source table.
func TestRunHistoryBarsAtWidth120(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Source:  "cc",
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, config.Single, historyDeps(f, 120))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "\x1b[32m") {
		t.Errorf("expected green bars at width 120:\n%s", joined)
	}
}
