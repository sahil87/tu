package harness

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlaceholderDeterministicAndConsistent(t *testing.T) {
	a, err := EncodePretty(Placeholder("opencode"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodePretty(Placeholder("opencode"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("placeholder output is not deterministic")
	}

	var report DailyReport
	if err := json.Unmarshal(a, &report); err != nil {
		t.Fatalf("placeholder does not decode into the schema structs: %v", err)
	}
	if len(report.Daily) != 3 {
		t.Fatalf("daily has %d entries, want 3", len(report.Daily))
	}
	for i, date := range PlaceholderDates {
		e := report.Daily[i]
		if e.Date != date {
			t.Errorf("daily[%d].date = %q, want %q", i, e.Date, date)
		}
		if e.TotalTokens != e.InputTokens+e.OutputTokens+e.CacheCreationTokens+e.CacheReadTokens {
			t.Errorf("daily[%d] totalTokens != sum of counters", i)
		}
		if len(e.ModelBreakdowns) != 1 || e.ModelBreakdowns[0].ModelName != "placeholder-opencode-model" {
			t.Errorf("daily[%d] modelBreakdowns = %+v", i, e.ModelBreakdowns)
		}
	}
	var sumTokens int
	var sumCost float64
	for _, e := range report.Daily {
		sumTokens += e.TotalTokens
		sumCost += e.TotalCost
	}
	if report.Totals.TotalTokens != sumTokens || report.Totals.TotalCost != sumCost {
		t.Errorf("totals = %+v, want totalTokens %d, totalCost %v", report.Totals, sumTokens, sumCost)
	}
	if report.Totals.TotalTokens != report.Totals.InputTokens+report.Totals.OutputTokens+report.Totals.CacheCreationTokens+report.Totals.CacheReadTokens {
		t.Error("totals.totalTokens != sum of totals counters")
	}
}

// Keys must come out alphabetically ordered with 2-space indentation, the
// ccusage serializer layout.
func TestPlaceholderKeyOrderAndIndent(t *testing.T) {
	raw, err := EncodePretty(Placeholder("copilot"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "\n  \"daily\": [") {
		t.Errorf("not 2-space indented:\n%s", s)
	}
	// Alphabetical within an entry: cacheCreationTokens < cacheReadTokens <
	// date < inputTokens < modelBreakdowns < modelsUsed < outputTokens <
	// totalCost < totalTokens.
	order := []string{"cacheCreationTokens", "cacheReadTokens", "\"date\"", "inputTokens", "modelBreakdowns", "modelsUsed", "outputTokens", "totalCost", "totalTokens"}
	assertKeyOrder(t, s, order)
	if !strings.HasSuffix(s, "}\n") {
		t.Error("missing trailing newline")
	}
}

// codex is the observed outlier shape: costUSD, a models{} map, and
// reasoningOutputTokens — no modelBreakdowns/modelsUsed/totalCost.
func TestPlaceholderCodexShape(t *testing.T) {
	raw, err := EncodePretty(PlaceholderFor("codex"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, forbidden := range []string{"totalCost", "modelBreakdowns", "modelsUsed"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("codex placeholder must not carry claude-style key %q", forbidden)
		}
	}
	assertKeyOrder(t, s, []string{"cacheCreationTokens", "cacheReadTokens", "costUSD", "\"date\"", "inputTokens", "\"models\"", "outputTokens", "reasoningOutputTokens", "totalTokens"})

	var report CodexDailyReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("codex placeholder does not decode into the codex structs: %v", err)
	}
	if len(report.Daily) != 3 {
		t.Fatalf("daily has %d entries, want 3", len(report.Daily))
	}
	var sumTokens, sumReasoning int
	var sumCost float64
	for i, e := range report.Daily {
		if e.TotalTokens != e.InputTokens+e.OutputTokens+e.CacheCreationTokens+e.CacheReadTokens {
			t.Errorf("daily[%d] totalTokens != sum of counters", i)
		}
		usage, ok := e.Models["placeholder-codex-model"]
		if !ok || len(e.Models) != 1 {
			t.Fatalf("daily[%d] models = %+v", i, e.Models)
		}
		if usage.TotalTokens != e.TotalTokens || usage.ReasoningOutputTokens != e.ReasoningOutputTokens {
			t.Errorf("daily[%d] model usage %+v disagrees with entry", i, usage)
		}
		sumTokens += e.TotalTokens
		sumReasoning += e.ReasoningOutputTokens
		sumCost += e.CostUSD
	}
	if report.Totals.TotalTokens != sumTokens || report.Totals.CostUSD != sumCost || report.Totals.ReasoningOutputTokens != sumReasoning {
		t.Errorf("totals = %+v, want totalTokens %d, costUSD %v, reasoning %d", report.Totals, sumTokens, sumCost, sumReasoning)
	}

	// The claude-style dispatcher path must still hold for every other source.
	if _, ok := PlaceholderFor("kimi").(DailyReport); !ok {
		t.Error("PlaceholderFor(kimi) must be the claude-style DailyReport")
	}
}

func assertKeyOrder(t *testing.T, s string, order []string) {
	t.Helper()
	prev := 0
	for _, key := range order {
		idx := strings.Index(s[prev:], key)
		if idx == -1 {
			t.Fatalf("key %s missing after offset %d", key, prev)
		}
		prev += idx + len(key)
	}
}

func TestWritePlaceholders(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, PlaceholderAlias)
	if err := WritePlaceholders(out, []string{"opencode", "codex"}); err != nil {
		t.Fatalf("WritePlaceholders: %v", err)
	}
	for _, source := range []string{"opencode", "codex"} {
		if _, err := os.Stat(filepath.Join(out, source, "daily.json")); err != nil {
			t.Errorf("missing fixture for %s: %v", source, err)
		}
	}
	m, err := ReadManifest(out)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if m.Machine != PlaceholderAlias || m.CcusageVersion != PlaceholderVersion || m.CcusagePath != "" || m.DerivedFrom != PlaceholderDerivedFrom {
		t.Errorf("manifest header = %+v", m)
	}
	if len(m.Fixtures) != 2 {
		t.Fatalf("manifest has %d fixtures, want 2", len(m.Fixtures))
	}
	for _, fx := range m.Fixtures {
		if !fx.Unconfirmed {
			t.Errorf("%s: unconfirmed must be true", fx.Source)
		}
		if fx.Days != 3 || fx.Empty || fx.FirstDate != PlaceholderDates[0] || fx.LastDate != PlaceholderDates[2] {
			t.Errorf("%s: days=%d empty=%v range=%s..%s, want 3/false/%s..%s", fx.Source, fx.Days, fx.Empty, fx.FirstDate, fx.LastDate, PlaceholderDates[0], PlaceholderDates[2])
		}
	}
}

// Real captures are local-only and may sit beside the placeholder alias; the
// generator must not refuse or touch them.
func TestWritePlaceholdersCoexistsWithLocalCapture(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "dev-ws-sahil02")
	real := &Manifest{
		Schema:  SchemaVersion,
		Machine: "dev-ws-sahil02",
		Fixtures: []Fixture{{
			Source: "codex", Period: "daily", Args: []string{"--json"},
			File: "codex/daily.json", Days: 17, FirstDate: "2026-08-05", LastDate: "2026-09-13",
		}},
	}
	if err := WriteManifest(realDir, real); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(realDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := WritePlaceholders(filepath.Join(root, PlaceholderAlias), []string{"codex"}); err != nil {
		t.Fatalf("a local capture must not block the placeholder corpus: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(realDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("local capture manifest was modified")
	}
}

// WritePlaceholders must refuse an --out that already holds a real alias's
// corpus, so a mistaken --out cannot clobber captured fixtures.
func TestWritePlaceholdersRefusesRealAliasOut(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "dev-ws-sahil02")
	real := &Manifest{
		Schema:  SchemaVersion,
		Machine: "dev-ws-sahil02",
		Fixtures: []Fixture{{
			Source: "codex", Period: "daily", Args: []string{"--json"},
			File: "codex/daily.json", Days: 17, FirstDate: "2026-08-05", LastDate: "2026-09-13",
		}},
	}
	if err := WriteManifest(realDir, real); err != nil {
		t.Fatal(err)
	}
	if err := WritePlaceholders(realDir, []string{"gemini"}); err == nil {
		t.Fatal("expected refusal to overwrite a real alias, got nil error")
	}
	// The real manifest must survive untouched.
	m, err := ReadManifest(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Machine != "dev-ws-sahil02" || len(m.Fixtures) != 1 || m.Fixtures[0].Source != "codex" {
		t.Errorf("real alias manifest was modified: %+v", m)
	}
}
