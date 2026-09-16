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

// singleCfg is the post-guard single-mode config the B2 tests run under.
var singleCfg = config.Config{Mode: config.Single}

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
	res, err := Run(context.Background(), Request{Format: JSON}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{Period: query.Monthly, Format: JSON}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{Source: "cc", Format: JSON}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{Source: "cc", Format: JSON, Period: query.Weekly}, singleCfg, fakeDeps(f))
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
	if _, err := Run(context.Background(), req, singleCfg, fakeDeps(f)); err != nil {
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
	res, err := Run(context.Background(), Request{}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{}, singleCfg, fakeDeps(f))
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
	res, err := Run(context.Background(), Request{}, singleCfg, fakeDeps(f))
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
	multiCfg := config.Config{Mode: config.Multi}
	cases := []struct {
		name string
		req  Request
		cfg  config.Config
	}{
		{"watch", Request{Flags: Flags{Watch: true, Interval: 10}}, singleCfg},
		{"sync", Request{Flags: Flags{Sync: true, Interval: 10}}, singleCfg},
		{"dry-run", Request{Flags: Flags{DryRun: true, Interval: 10}}, singleCfg},
		{"no-rain", Request{Flags: Flags{NoRain: true, Interval: 10}}, singleCfg},
		{"skip-brew-update", Request{Flags: Flags{SkipBrewUpdate: true, Interval: 10}}, singleCfg},
		{"command", Request{Command: "help", Flags: base}, singleCfg},
		{"version", Request{Version: true, Flags: base}, singleCfg},
		{"lb watch", Request{Display: Leaderboard, Flags: Flags{Watch: true, Interval: 10}}, multiCfg},
		{"lb sync", Request{Display: Leaderboard, Flags: Flags{Sync: true, Interval: 10}}, multiCfg},
		{"multi watch", Request{Flags: Flags{Watch: true, Interval: 10}}, multiCfg},
		{"multi sync", Request{Flags: Flags{Sync: true, Interval: 10}}, multiCfg},
		{"multi dry-run", Request{Flags: Flags{DryRun: true, Interval: 10}}, multiCfg},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeFetcher{}
			_, err := Run(context.Background(), c.req, c.cfg, fakeDeps(f))
			if !errors.Is(err, ErrUnported) {
				t.Errorf("err = %v, want ErrUnported", err)
			}
			if len(f.calls) != 0 {
				t.Errorf("unported request must not fetch; calls = %+v", f.calls)
			}
		})
	}
}

// R1: the multi-mode gate — lb/lbh in single mode return ErrLeaderboardMode
// before Normalize (no notices, no lines, no fetch); the message names lb for
// lbh (DC-14).
func TestRunLeaderboardSingleModeGate(t *testing.T) {
	cases := []struct {
		name string
		req  Request
	}{
		{"lb", Request{Display: Leaderboard, Flags: Flags{Interval: 10}}},
		{"lbh", Request{Display: LeaderboardHistory, Flags: Flags{Interval: 10}}},
		{"lb -u name (no notice precedes the gate)", Request{Display: Leaderboard, Flags: Flags{User: "other-user", Interval: 10}}},
		{"lbh by-machine", Request{Display: LeaderboardHistory, Flags: Flags{ByMachine: true, Interval: 10}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeFetcher{}
			res, err := Run(context.Background(), c.req, singleCfg, fakeDeps(f))
			if !errors.Is(err, ErrLeaderboardMode) {
				t.Fatalf("err = %v, want ErrLeaderboardMode", err)
			}
			if err.Error() != "Error: lb requires multi mode — run tu init-metrics <repo-url> to set up a metrics repo" {
				t.Errorf("message = %q", err.Error())
			}
			if len(res.Notices) != 0 || len(res.Lines) != 0 {
				t.Errorf("Result = %+v, want empty (no notices, no lines)", res)
			}
			if len(f.calls) != 0 {
				t.Errorf("the gate must precede any fetch; calls = %+v", f.calls)
			}
		})
	}
}

// R3: the leaderboard displays and --top are in scope in multi mode; watch
// and friends stay out even on lb.
func TestRunLeaderboardScope(t *testing.T) {
	f := &fakeFetcher{}
	deps := multiDeps(f, seedRepo())
	if _, err := Run(context.Background(), Request{
		Display: Leaderboard,
		Flags:   Flags{Top: 3, Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, deps); err != nil {
		t.Errorf("lb --top in multi mode: err = %v, want nil", err)
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
	}, singleCfg, historyDeps(f, 80))
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
	}, singleCfg, historyDeps(f, 80))
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
	}, singleCfg, historyDeps(f, 80))
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
	res, err := Run(context.Background(), Request{Display: History, Flags: Flags{Interval: 10}}, singleCfg, historyDeps(f, 80))
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
	res, err := Run(context.Background(), Request{Display: History, Flags: Flags{Full: true, Interval: 10}}, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), req, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), Request{Display: History, Format: JSON, Flags: Flags{Interval: 10}}, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), req, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), Request{Display: History, Format: CSV, Flags: Flags{Interval: 10}}, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), req, singleCfg, historyDeps(f, 80))
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
		res, err := Run(context.Background(), Request{Display: History, Format: Markdown, Flags: Flags{Interval: 10}}, singleCfg, historyDeps(f, 80))
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
	res, err := Run(context.Background(), Request{Flags: Flags{Since: "2026-13-01", Interval: 10}}, singleCfg, historyDeps(f, 80))
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
	}, singleCfg, historyDeps(f, 80))
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
	}, singleCfg, deps)
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
	}, singleCfg, historyDeps(f, 120))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "\x1b[32m") {
		t.Errorf("expected green bars at width 120:\n%s", joined)
	}
}

// ── B3: metrics-repo source and multi-mode merge ───────────────────────────

// fakeRepo is an in-memory Repo mirroring the committed seed; it records its
// Read calls as "user/tool".
type fakeRepo struct {
	users []string
	recs  map[string]map[string][]fact.Record // user → tool key → records (walk order)
	calls []string
}

func (f *fakeRepo) Users() []string { return f.users }

func (f *fakeRepo) Read(user string, tool fact.Tool) []fact.Record {
	f.calls = append(f.calls, user+"/"+tool.Key)
	return f.recs[user][tool.Key]
}

// multiCfg is the seeded multi-mode config (the harness conf pins these).
var multiCfg = config.Config{Mode: config.Multi, User: "harness-user", Machine: "harness-machine"}

// storedRec stamps a repo record (User + Machine set, like metrics.Source).
func storedRec(user, machine, date, toolKey string, cost float64) fact.Record {
	t := fakeTotals
	t.TotalCost = cost
	return fact.Record{Date: date, Tool: toolKey, User: user, Machine: machine, Totals: t}
}

// seedRepo mirrors harness/metrics-repo: harness-user cc 01-05 (0.25) and
// 01-06 (0.75) on harness-machine, cc 01-06 (0.40) + codex 01-07 (0.30) on
// other-box; other-user cc 01-05 (1.10) + gemini 01-06 (0.20) on laptop.
func seedRepo() *fakeRepo {
	return &fakeRepo{
		users: []string{"harness-user", "other-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {
				"cc": {
					storedRec("harness-user", "harness-machine", "2026-01-05", "cc", 0.25),
					storedRec("harness-user", "harness-machine", "2026-01-06", "cc", 0.75),
					storedRec("harness-user", "other-box", "2026-01-06", "cc", 0.40),
				},
				"codex": {storedRec("harness-user", "other-box", "2026-01-07", "codex", 0.30)},
			},
			"other-user": {
				"cc":     {storedRec("other-user", "laptop", "2026-01-05", "cc", 1.10)},
				"gemini": {storedRec("other-user", "laptop", "2026-01-06", "gemini", 0.20)},
			},
		},
	}
}

// liveCorpus is the placeholder live fetch stamped with the harness identity
// (as the edge's ccusage.Source stamps cfg.User/cfg.Machine).
func liveCorpus() map[string][]fact.Record {
	byTool := placeholderCorpus()
	for key, recs := range byTool {
		for i := range recs {
			recs[i].User = "harness-user"
			recs[i].Machine = "harness-machine"
		}
		byTool[key] = recs
	}
	return byTool
}

func multiDeps(f *fakeFetcher, r *fakeRepo) Deps {
	d := historyDeps(f, 80)
	d.Repo = r
	return d
}

// R9: the own-user merge — live wins 01-05, stored wins 01-06, other-box sums
// on — renders the intake §9 cc-h-window rows; exactly one live fetch.
func TestRunMultiOwnUserMerge(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	repo := seedRepo()
	res, err := Run(context.Background(), Request{
		Source:  "cc",
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}, multiCfg, multiDeps(f, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	for _, want := range []string{
		"2026-01-05   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50",
		"2026-01-06   |          6,000 |            800 |          2,000 |         40,000 |         48,800 |     $1.15",
		"2026-01-07   |          3,000 |            400 |          1,000 |         20,000 |         24,400 |     $0.50",
		"12,000",
		"$2.15",
		"avg $0.72/day · peak $1.15 (2026-01-06)",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("merged history missing %q:\n%s", want, joined)
		}
	}
	if len(f.calls) != 1 || f.calls[0].tool != "cc" {
		t.Errorf("fetch calls = %+v, want exactly one Fetch for cc", f.calls)
	}
	if !reflect.DeepEqual(repo.calls, []string{"harness-user/cc"}) {
		t.Errorf("repo calls = %v, want [harness-user/cc]", repo.calls)
	}
}

// R9/A-024: the repo-only paths make no fetch call and carry no warnings;
// -u <other> reads only that user's tree, -u all every profile.
func TestRunMultiRepoOnlyPaths(t *testing.T) {
	t.Run("-u other", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		repo := seedRepo()
		res, err := Run(context.Background(), Request{
			Display: History,
			Flags:   Flags{User: "other-user", Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
		}, multiCfg, multiDeps(f, repo))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 0 {
			t.Errorf("fetch calls = %+v, want none on the repo-only path", f.calls)
		}
		if len(res.Warnings) != 0 {
			t.Errorf("Warnings = %v, want empty", res.Warnings)
		}
		joined := strings.Join(res.Lines, "\n")
		for _, want := range []string{"2026-01-05", "$1.10", "2026-01-06", "$0.20", "$1.30"} {
			if !strings.Contains(joined, want) {
				t.Errorf("-u other-user history missing %q:\n%s", want, joined)
			}
		}
		for _, c := range repo.calls {
			if !strings.HasPrefix(c, "other-user/") {
				t.Errorf("repo read %q, want only other-user reads", c)
			}
		}
	})

	t.Run("-u other missing from repo", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		repo := seedRepo()
		res, err := Run(context.Background(), Request{
			Display: History,
			Flags:   Flags{User: "nobody", Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
		}, multiCfg, multiDeps(f, repo))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 0 || len(res.Warnings) != 0 {
			t.Errorf("calls = %+v, warnings = %v, want both empty", f.calls, res.Warnings)
		}
		if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, "  No data") {
			t.Errorf("-u nobody must render the empty history:\n%s", joined)
		}
	})

	t.Run("-u all", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		repo := seedRepo()
		res, err := Run(context.Background(), Request{
			Display: History,
			Flags:   Flags{User: "all", Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
		}, multiCfg, multiDeps(f, repo))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 0 {
			t.Errorf("fetch calls = %+v, want none for -u all", f.calls)
		}
		if len(res.Warnings) != 0 {
			t.Errorf("Warnings = %v, want empty", res.Warnings)
		}
		// cc 01-05: 0.25 + 1.10; cc 01-06: 0.75 + 0.40; gemini 01-06: 0.20;
		// codex 01-07: 0.30. Runtime variables, not constants: constant
		// arithmetic is exact and would not match the decoded-float sums.
		c25, c40, c75 := 0.25, 0.40, 0.75
		c110, c20, c30 := 1.10, 0.20, 0.30
		if got, want := res.CostByItem["total:2026-01-05"], c25+c110; got != want {
			t.Errorf("total:2026-01-05 = %v, want %v", got, want)
		}
		if got, want := res.CostByItem["total:2026-01-06"], (c75+c40)+c20; got != want {
			t.Errorf("total:2026-01-06 = %v, want %v", got, want)
		}
		if got, want := res.CostByItem["total:2026-01-07"], c30; got != want {
			t.Errorf("total:2026-01-07 = %v, want %v", got, want)
		}
		// TotalCost accumulates in registry series order: cc days, then codex,
		// then gemini.
		if got, want := res.TotalCost, ((c25+c110+(c75+c40))+c30)+c20; got != want {
			t.Errorf("TotalCost = %v, want %v", got, want)
		}
		// Users ascending, each user's tools in registry order.
		wantCalls := []string{
			"harness-user/cc", "harness-user/codex", "harness-user/oc",
			"harness-user/gemini", "harness-user/copilot", "harness-user/kimi",
			"other-user/cc", "other-user/codex", "other-user/oc",
			"other-user/gemini", "other-user/copilot", "other-user/kimi",
		}
		if !reflect.DeepEqual(repo.calls, wantCalls) {
			t.Errorf("repo calls = %v, want %v", repo.calls, wantCalls)
		}
	})
}

// R9: -u <cfg.User> is literally the no-flag path.
func TestRunMultiUserSelfEqualsNoFlag(t *testing.T) {
	req := Request{
		Source:  "cc",
		Display: History,
		Flags:   Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10},
	}
	plain, err := Run(context.Background(), req, multiCfg, multiDeps(&fakeFetcher{byTool: liveCorpus()}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	selfReq := req
	selfReq.Flags.User = "harness-user"
	self, err := Run(context.Background(), selfReq, multiCfg, multiDeps(&fakeFetcher{byTool: liveCorpus()}, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(self.Lines, plain.Lines) {
		t.Errorf("-u harness-user lines differ from no-flag:\n%s\nvs\n%s", self.Lines, plain.Lines)
	}
	if !reflect.DeepEqual(self.Notices, plain.Notices) {
		t.Errorf("notices differ: %v vs %v", self.Notices, plain.Notices)
	}
}

// R10/A-015: the daily collapse precedes the roll-up, so the monthly JSON
// carries 2.15, not 2.1500000000000004.
func TestRunMultiMonthlyJSON(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	res, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Period: query.Monthly, Format: JSON,
		Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(f, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	want := "[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 2.15,\n    \"inputTokens\": 12000,\n    \"outputTokens\": 1600,\n    \"cacheCreationTokens\": 4000,\n    \"cacheReadTokens\": 80000,\n    \"totalTokens\": 97600\n  }\n]"
	if got := strings.Join(res.Lines, "\n"); got != want {
		t.Errorf("cc mh --json =\n%s\nwant:\n%s", got, want)
	}
}

// R10: the summation order is contract. Costs 0.1 (01-05) and 0.2 + 0.3
// (01-06, two machines) roll to a month as (d1) + (d2a + d2b) = 0.6 — the
// per-machine-first association (0.1 + 0.2) + 0.3 = 0.6000000000000001 would
// print differently.
func TestRunMultiAssociationOrder(t *testing.T) {
	a, b, c := 0.1, 0.2, 0.3
	if a+(b+c) == (a+b)+c {
		t.Fatal("the test values do not distinguish the summation orders")
	}
	repo := &fakeRepo{recs: map[string]map[string][]fact.Record{
		"harness-user": {"cc": {
			storedRec("harness-user", "machine-a", "2026-01-05", "cc", a),
			storedRec("harness-user", "machine-a", "2026-01-06", "cc", b),
			storedRec("harness-user", "machine-b", "2026-01-06", "cc", c),
		}},
	}}
	f := &fakeFetcher{} // no live records: the repo path alone carries the sum
	res, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Period: query.Monthly, Format: JSON,
		Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(f, repo))
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, `"totalCost": 0.6,`) {
		t.Errorf("monthly totalCost must associate per-day-first (0.6):\n%s", joined)
	}
}

// PR #91 review: when dates interleave across machines (own machine holds the
// 01-05 and 01-07 records, another machine the 01-06 one), the collapsed
// daily sequence is machine-major — but the TS mergeEntries sorts its daily
// merge by label before the period aggregation. The roll-up must therefore
// sum the bucket date-sorted: (0.2 + 0.1) + 0.3 = 0.6000000000000001, not
// the machine-major (0.2 + 0.3) + 0.1 = 0.6.
func TestRunMultiInterleavedDateAssociation(t *testing.T) {
	a, b, c := 0.2, 0.1, 0.3
	if (a+c)+b == (a+b)+c {
		t.Fatal("the test values do not distinguish the summation orders")
	}
	repo := &fakeRepo{recs: map[string]map[string][]fact.Record{
		"harness-user": {"cc": {
			storedRec("harness-user", "harness-machine", "2026-01-05", "cc", a),
			storedRec("harness-user", "harness-machine", "2026-01-07", "cc", c),
			storedRec("harness-user", "other-box", "2026-01-06", "cc", b),
		}},
	}}
	f := &fakeFetcher{} // no live records: the repo path alone carries the sum
	res, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Period: query.Monthly, Format: JSON,
		Flags: Flags{Interval: 10},
	}, multiCfg, multiDeps(f, repo))
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, `"totalCost": 0.6000000000000001,`) {
		t.Errorf("monthly totalCost must associate date-sorted (0.6000000000000001):\n%s", joined)
	}
}

// R11/A-011: in multi mode a tool with a record on the current label carries
// "label" in --json; zero tools carry nothing.
func TestRunMultiSnapshotLabel(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc": {storedRec("harness-user", "harness-machine", "2026-01-06", "cc", 0.50)},
	}}
	deps := multiDeps(f, &fakeRepo{})
	deps.Now = fixedNow // 2026-01-06
	res, err := Run(context.Background(), Request{Format: JSON, Flags: Flags{Interval: 10}}, multiCfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "\"Claude Code\": {\n    \"label\": \"2026-01-06\",") {
		t.Errorf("multi-mode snapshot must carry the label on a populated tool:\n%s", joined)
	}
	if strings.Contains(joined, "\"Codex\": {\n    \"label\"") {
		t.Errorf("zero tools carry no label:\n%s", joined)
	}
}

// R8: -u in single mode warns, clears, and renders the live snapshot, exit 0.
func TestRunSingleUserWarnsAndRenders(t *testing.T) {
	f := &fakeFetcher{}
	res, err := Run(context.Background(), Request{Flags: Flags{User: "other-user", Interval: 10}}, singleCfg, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notices) != 1 || res.Notices[0] != userNotice {
		t.Errorf("Notices = %v, want the -u notice", res.Notices)
	}
	if joined := strings.Join(res.Lines, "\n"); !strings.Contains(joined, "📊 Combined Usage (daily)") {
		t.Errorf("the snapshot still renders after the -u clear:\n%s", joined)
	}
	if len(f.calls) != 1 || f.calls[0].tool != "" {
		t.Errorf("calls = %+v, want one FetchAll (the live path)", f.calls)
	}
}

// A-020: the cap applies to stored records — a day-file older than the floor
// is hidden by `h` and shown by `h --full` (Now fixed in January 2026).
func TestRunMultiCapAppliesToStored(t *testing.T) {
	repo := &fakeRepo{recs: map[string]map[string][]fact.Record{
		"harness-user": {"cc": {storedRec("harness-user", "harness-machine", "2025-10-01", "cc", 0.42)}},
	}}
	newDeps := func() Deps {
		d := multiDeps(&fakeFetcher{byTool: liveCorpus()}, repo)
		d.Now = fixedNow // 2026-01-06 → floor 2025-11-01
		return d
	}

	capped, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Flags: Flags{Interval: 10},
	}, multiCfg, newDeps())
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(capped.Lines, "\n")
	if strings.Contains(joined, "2025-10-01") {
		t.Errorf("capped history must hide the stored 2025-10-01 record:\n%s", joined)
	}
	if !strings.Contains(joined, "(daily, last 3 months)") {
		t.Errorf("missing the cap hint:\n%s", joined)
	}

	full, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Flags: Flags{Full: true, Interval: 10},
	}, multiCfg, newDeps())
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(full.Lines, "\n"); !strings.Contains(joined, "2025-10-01") {
		t.Errorf("h --full must show the stored 2025-10-01 record:\n%s", joined)
	}
}

// ── B4: machine columns ────────────────────────────────────────────────────

// R3: the dimension switch — machines by default, users under multi-mode
// -u all; single mode cleared -u and falls back to machines (with the -u
// notice).
func TestRunByMachineDimensionSwitch(t *testing.T) {
	window := Flags{Since: "2026-01-01", Until: "2026-01-31", Interval: 10}

	t.Run("multi machines", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		flags := window
		flags.ByMachine = true
		res, err := Run(context.Background(), Request{Source: "cc", Display: History, Flags: flags}, multiCfg, multiDeps(f, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Lines, "\n")
		for _, want := range []string{"Machines: A = harness-machine, B = other-box", "$0.75", "$0.40"} {
			if !strings.Contains(joined, want) {
				t.Errorf("machine history missing %q:\n%s", want, joined)
			}
		}
	})

	t.Run("multi -u all users", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		flags := window
		flags.ByMachine = true
		flags.User = "all"
		res, err := Run(context.Background(), Request{Source: "cc", Display: History, Flags: flags}, multiCfg, multiDeps(f, seedRepo()))
		if err != nil {
			t.Fatal(err)
		}
		if len(f.calls) != 0 {
			t.Errorf("fetch calls = %+v, want none for -u all", f.calls)
		}
		joined := strings.Join(res.Lines, "\n")
		if !strings.Contains(joined, "Users: A = harness-user, B = other-user") {
			t.Errorf("user columns missing:\n%s", joined)
		}
		// 01-05: harness-user $0.25, other-user $1.10; the row total sums both.
		if !strings.Contains(joined, "$1.35") {
			t.Errorf("the collapsed row total $1.35 missing:\n%s", joined)
		}
	})

	t.Run("single -u all falls back to machines", func(t *testing.T) {
		f := &fakeFetcher{byTool: liveCorpus()}
		flags := window
		flags.ByMachine = true
		flags.User = "all"
		res, err := Run(context.Background(), Request{Source: "cc", Display: History, Flags: flags}, singleCfg, historyDeps(f, 80))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Notices) != 1 || res.Notices[0] != userNotice {
			t.Errorf("Notices = %v, want the -u notice", res.Notices)
		}
		joined := strings.Join(res.Lines, "\n")
		if !strings.Contains(joined, "Machines: A = harness-machine") {
			t.Errorf("single-mode machine column missing:\n%s", joined)
		}
	})
}

// R6/A-020: the single-source snapshot zero-fill — every historical machine
// in first-seen order (own first) with 0 values on a zero-usage day, and no
// "label" (no current-label group). A G0 candidate vs the DC-01 sentence.
func TestRunByMachineSingleSourceZeroFill(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	deps := multiDeps(f, seedRepo()) // Now = historyNow, outside the seed window
	res, err := Run(context.Background(), Request{
		Source: "cc", Format: JSON,
		Flags: Flags{ByMachine: true, Interval: 10},
	}, multiCfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"Claude Code\": {\n" +
		"    \"totalCost\": 0,\n" +
		"    \"inputTokens\": 0,\n" +
		"    \"outputTokens\": 0,\n" +
		"    \"cacheCreationTokens\": 0,\n" +
		"    \"cacheReadTokens\": 0,\n" +
		"    \"totalTokens\": 0,\n" +
		"    \"machines\": {\n" +
		"      \"harness-machine\": 0,\n" +
		"      \"other-box\": 0\n" +
		"    }\n" +
		"  }\n" +
		"}"
	if got := strings.Join(res.Lines, "\n"); got != want {
		t.Errorf("cc --by-machine --json on a zero-usage day =\n%s\nwant:\n%s", got, want)
	}
}

// R6/A-015: the history machines key order is first-seen — the own machine
// before the walk-ordered others — and the monthly values associate in record
// input order.
func TestRunByMachineHistoryJSON(t *testing.T) {
	f := &fakeFetcher{byTool: liveCorpus()}
	res, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Period: query.Monthly, Format: JSON,
		Flags: Flags{ByMachine: true, Interval: 10},
	}, multiCfg, multiDeps(f, seedRepo()))
	if err != nil {
		t.Fatal(err)
	}
	want := "[\n  {\n    \"label\": \"2026-01\",\n    \"totalCost\": 2.15,\n    \"inputTokens\": 12000,\n    \"outputTokens\": 1600,\n    \"cacheCreationTokens\": 4000,\n    \"cacheReadTokens\": 80000,\n    \"totalTokens\": 97600,\n    \"machines\": {\n      \"harness-machine\": 1.75,\n      \"other-box\": 0.4\n    }\n  }\n]"
	if got := strings.Join(res.Lines, "\n"); got != want {
		t.Errorf("cc mh --by-machine --json =\n%s\nwant:\n%s", got, want)
	}
}

// R6/A-015: under -u all the user's monthly slice sums in RECORD INPUT ORDER
// — ((a1+a2)+b1)+b2 over the flattened walk-ordered entries — never
// per-machine-first ((a1+a2)+(b1+b2)): 1.5, not 1.5000000000000002.
func TestRunByMachineUserAssociation(t *testing.T) {
	a1, a2, b1, b2 := 0.1, 0.2, 0.4, 0.8
	if ((a1+a2)+b1)+b2 == (a1+a2)+(b1+b2) {
		t.Fatal("the test values do not distinguish the summation orders")
	}
	repo := &fakeRepo{
		users: []string{"harness-user"},
		recs: map[string]map[string][]fact.Record{
			"harness-user": {"cc": {
				storedRec("harness-user", "machine-a", "2026-01-05", "cc", a1),
				storedRec("harness-user", "machine-a", "2026-01-06", "cc", a2),
				storedRec("harness-user", "machine-b", "2026-01-05", "cc", b1),
				storedRec("harness-user", "machine-b", "2026-01-06", "cc", b2),
			}},
		},
	}
	f := &fakeFetcher{} // -u all: repo-only, no fetch
	res, err := Run(context.Background(), Request{
		Source: "cc", Display: History, Period: query.Monthly, Format: JSON,
		Flags: Flags{User: "all", ByMachine: true, Interval: 10},
	}, multiCfg, multiDeps(f, repo))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, `"harness-user": 1.5`) {
		t.Errorf("the user slice must associate in input order (1.5):\n%s", joined)
	}
	if strings.Contains(joined, "1.5000000000000002") {
		t.Errorf("per-machine-first association leaked:\n%s", joined)
	}
}

// R12: the single-mode daily-all label clear does NOT apply under
// --by-machine — `tu --by-machine --json` carries "label".
func TestRunByMachineKeepsLabels(t *testing.T) {
	f := &fakeFetcher{byTool: map[string][]fact.Record{
		"cc": {{Date: "2026-01-06", Tool: "cc", User: "u", Machine: "m1", Totals: fakeTotals}},
	}}
	res, err := Run(context.Background(), Request{
		Format: JSON,
		Flags:  Flags{ByMachine: true, Interval: 10},
	}, singleCfg, fakeDeps(f))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "\"Claude Code\": {\n    \"label\": \"2026-01-06\",") {
		t.Errorf("by-machine daily-all JSON must carry the label:\n%s", joined)
	}
	if !strings.Contains(joined, "\"machines\": {\n      \"m1\": 0.5\n    }") {
		t.Errorf("the machine slice missing:\n%s", joined)
	}
}

// R1: the all-tools history pivot warns and renders the (empty) pivot, exit
// 0 path — the flag is cleared before inScope, so no machine columns.
func TestRunByMachineAllToolsHistoryWarns(t *testing.T) {
	f := &fakeFetcher{byTool: placeholderCorpus()}
	res, err := Run(context.Background(), Request{
		Display: History,
		Flags:   Flags{ByMachine: true, Interval: 10},
	}, singleCfg, historyDeps(f, 80))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Notices) != 1 || res.Notices[0] != byMachinePivotNotice {
		t.Errorf("Notices = %v, want the pivot notice", res.Notices)
	}
	joined := strings.Join(res.Lines, "\n")
	if !strings.Contains(joined, "📊 Combined Cost History (daily, last 3 months)") || !strings.Contains(joined, "  No data") {
		t.Errorf("the capped pivot must render after the clear:\n%s", joined)
	}
	if strings.Contains(joined, "Machines:") {
		t.Errorf("no legend after the pivot clear:\n%s", joined)
	}
}
