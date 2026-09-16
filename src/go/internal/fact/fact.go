// Package fact defines the one record type every downstream stage of the Go
// port consumes: what a tool cost a user on a machine on a date. Sources
// produce []Record; query, view, and render consume it and nothing else.
//
// Date is an ISO label string ("YYYY-MM-DD" or "YYYY-MM"), never a time.Time:
// monthly roll-up labels have no instant, and lexicographic order on ISO
// labels is the total order every filter relies on. Tool holds the registry
// key (cc, codex, oc, gemini, copilot, kimi), not the display name. The
// registry itself (Tool, Tools, Lookup) lives here too: key, display name,
// and column order are properties of the fact model.
package fact

// Record is one observed usage fact. The four string fields carry no JSON
// tags (Go's default names); the pinned external names apply to Totals only.
type Record struct {
	Date    string // ISO label: "YYYY-MM-DD" or "YYYY-MM"
	Tool    string // registry key: cc, codex, oc, gemini, copilot, kimi
	User    string
	Machine string
	Totals
}

// Totals is cost plus the five token counters. The JSON tags are the pinned
// external names from docs/specs/usage.md § Data Model, so render/json and
// the cache envelope can encode the struct directly. TotalTokens is stored,
// never recomputed from the other counters (byte parity with ccusage's own
// value). The zero value is the "no data" value (the TS EMPTY).
type Totals struct {
	TotalCost           float64 `json:"totalCost"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	TotalTokens         int64   `json:"totalTokens"`
}

// Add returns the field-wise sum of t and o. It is pure: neither receiver nor
// argument is mutated.
func (t Totals) Add(o Totals) Totals {
	return Totals{
		TotalCost:           t.TotalCost + o.TotalCost,
		InputTokens:         t.InputTokens + o.InputTokens,
		OutputTokens:        t.OutputTokens + o.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens + o.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens + o.CacheReadTokens,
		TotalTokens:         t.TotalTokens + o.TotalTokens,
	}
}

// IsZero reports whether all six fields are zero.
func (t Totals) IsZero() bool {
	return t == Totals{}
}
