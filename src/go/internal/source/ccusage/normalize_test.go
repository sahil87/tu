package ccusage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source"
)

// placeholderRoot is the committed placeholder fixture corpus, resolved from
// this package's directory (Go tests run with cwd = package dir):
// src/go/internal/source/ccusage is five levels below the repo root. The same
// relative-walk convention as internal/harness/corpus_test.go, shifted one
// level.
const placeholderRoot = "../../../../../harness/fixtures/_placeholder"

var placeholderTotals = fact.Totals{
	TotalCost:           0.5,
	InputTokens:         3000,
	OutputTokens:        400,
	CacheCreationTokens: 1000,
	CacheReadTokens:     20000,
	TotalTokens:         24400,
}

var placeholderDates = []string{"2026-01-05", "2026-01-06", "2026-01-07"}

// TestParsePlaceholderCorpus parses all six placeholder fixtures and asserts
// the shared placeholder shape. It fails — never skips — when the corpus is
// absent, because the placeholder corpus is a committed deliverable of this
// repository.
func TestParsePlaceholderCorpus(t *testing.T) {
	if _, err := os.Stat(placeholderRoot); err != nil {
		t.Fatalf("placeholder corpus missing at %s: %v", placeholderRoot, err)
	}

	for _, tool := range fact.Tools {
		t.Run(tool.Key, func(t *testing.T) {
			path := filepath.Join(placeholderRoot, invocations[tool.Key].prefixArgs[0], "daily.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("fixture missing: %v", err)
			}
			records, perr := Parse(raw, tool)
			if perr != nil {
				t.Fatalf("Parse error: %v", perr)
			}
			if len(records) != 3 {
				t.Fatalf("got %d records, want 3", len(records))
			}
			for i, r := range records {
				if r.Date != placeholderDates[i] {
					t.Errorf("record %d Date = %q, want %q", i, r.Date, placeholderDates[i])
				}
				if r.Tool != tool.Key {
					t.Errorf("record %d Tool = %q, want %q", i, r.Tool, tool.Key)
				}
				if r.User != "" || r.Machine != "" {
					t.Errorf("record %d User/Machine = %q/%q, want unstamped", i, r.User, r.Machine)
				}
				if r.Totals != placeholderTotals {
					t.Errorf("record %d Totals = %+v, want %+v", i, r.Totals, placeholderTotals)
				}
				sum := r.InputTokens + r.OutputTokens + r.CacheCreationTokens + r.CacheReadTokens
				if sum != r.TotalTokens {
					t.Errorf("record %d counter sum %d != TotalTokens %d", i, sum, r.TotalTokens)
				}
			}
		})
	}
}

func TestParseCoercionAndLabels(t *testing.T) {
	tool, _ := fact.Lookup("cc")

	t.Run("non-numeric value zeroes, cachedInputTokens fallback", func(t *testing.T) {
		raw := `{"daily":[{"date":"Feb 14, 2026","inputTokens":"12","cachedInputTokens":7}]}`
		records, perr := Parse([]byte(raw), tool)
		if perr != nil {
			t.Fatal(perr)
		}
		if len(records) != 1 {
			t.Fatalf("got %d records, want 1", len(records))
		}
		r := records[0]
		if r.Date != "2026-02-14" {
			t.Errorf("Date = %q", r.Date)
		}
		if r.InputTokens != 0 {
			t.Errorf("InputTokens = %d, want 0 for a non-numeric value", r.InputTokens)
		}
		if r.CacheReadTokens != 7 {
			t.Errorf("CacheReadTokens = %d, want 7 via cachedInputTokens", r.CacheReadTokens)
		}
	})

	t.Run("costUSD fallback", func(t *testing.T) {
		raw := `{"daily":[{"date":"2026-01-05","costUSD":1.25}]}`
		records, perr := Parse([]byte(raw), tool)
		if perr != nil {
			t.Fatal(perr)
		}
		if records[0].TotalCost != 1.25 {
			t.Errorf("TotalCost = %v, want 1.25 via costUSD", records[0].TotalCost)
		}
	})

	t.Run("missing label and fields zero out", func(t *testing.T) {
		raw := `{"daily":[{}]}`
		records, perr := Parse([]byte(raw), tool)
		if perr != nil {
			t.Fatal(perr)
		}
		if records[0].Date != "" {
			t.Errorf("Date = %q, want empty", records[0].Date)
		}
		if !records[0].Totals.IsZero() {
			t.Errorf("Totals = %+v, want zero", records[0].Totals)
		}
	})

	t.Run("totals object and unknown keys ignored", func(t *testing.T) {
		raw := `{"daily":[{"date":"2026-01-05","modelBreakdowns":[{"cost":9}],"reasoningOutputTokens":5}],"totals":{"totalCost":-0.0}}`
		records, perr := Parse([]byte(raw), tool)
		if perr != nil {
			t.Fatal(perr)
		}
		if len(records) != 1 || records[0].Date != "2026-01-05" {
			t.Errorf("records = %+v", records)
		}
	})
}

func TestParseEmptyAndGarbage(t *testing.T) {
	tool, _ := fact.Lookup("cc")

	t.Run("empty daily is zero records, no error", func(t *testing.T) {
		records, perr := Parse([]byte(`{"daily":[],"totals":{"totalCost":-0.0}}`), tool)
		if perr != nil {
			t.Fatalf("Parse error: %v", perr)
		}
		if records == nil || len(records) != 0 {
			t.Errorf("records = %#v, want non-nil empty slice", records)
		}
	})

	garbage := []string{
		"",
		"   \n ",
		"not json",
		"[]",
		`{"totals":{}}`,
		`{"daily":{}}`,
		`{"daily":"daily"}`,
		`{"daily":null}`,
	}
	for _, raw := range garbage {
		t.Run("garbage "+raw, func(t *testing.T) {
			records, perr := Parse([]byte(raw), tool)
			if perr == nil {
				t.Fatalf("Parse(%q) = nil error, want KindParse", raw)
			}
			if perr.Kind != source.KindParse {
				t.Errorf("Kind = %v, want KindParse", perr.Kind)
			}
			if perr.Warns() {
				t.Error("KindParse must not warn")
			}
			if records != nil {
				t.Errorf("records = %#v, want nil", records)
			}
		})
	}
}

func TestNormalizeLabel(t *testing.T) {
	tests := []struct{ in, want string }{
		{"2026-02-14", "2026-02-14"}, // ISO daily passes through
		{"2026-02", "2026-02"},       // ISO monthly passes through
		{"Feb 14, 2026", "2026-02-14"},
		{"Jan 3, 2026", "2026-01-03"}, // zero-padded day
		{"Feb 2026", "2026-02"},
		{"Dec 2026", "2026-12"},
		{"Xyz 5, 2026", "2026-00-05"}, // unknown month
		{"Xyz 2026", "2026-00"},
		{"something else", "something else"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeLabel(tt.in); got != tt.want {
			t.Errorf("normalizeLabel(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestParseUsesLabelKey pins that Parse reads the entry label via the
// invocation's labelKey (all "date" at ccusage v20).
func TestParseUsesLabelKey(t *testing.T) {
	invocations["x"] = invocation{labelKey: "period"}
	defer delete(invocations, "x")
	tool := fact.Tool{Key: "x", Name: "X"}
	raw := `{"daily":[{"period":"Feb 2026","totalCost":2}]}`
	records, perr := Parse([]byte(raw), tool)
	if perr != nil {
		t.Fatal(perr)
	}
	if records[0].Date != "2026-02" {
		t.Errorf("Date = %q, want 2026-02 via LabelKey period", records[0].Date)
	}
	if !reflect.DeepEqual(records[0].Totals, fact.Totals{TotalCost: 2}) {
		t.Errorf("Totals = %+v", records[0].Totals)
	}
}
