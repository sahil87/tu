package ccusage

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source"
)

// Parse converts one ccusage per-agent document ({"daily":[...], ...}) into
// records for tool: Date + Tool + Totals per entry; User/Machine are stamped
// by Source. The "totals" object and unknown keys (modelBreakdowns, models,
// reasoningOutputTokens, …) are ignored. A "daily" empty array is a
// legitimate zero result, not an error; empty/garbage stdout or a missing or
// non-array "daily" is a KindParse error.
func Parse(raw []byte, tool fact.Tool) ([]fact.Record, *source.Error) {
	var doc struct {
		Daily json.RawMessage `json:"daily"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, parseError(tool, "stdout is not a JSON object", err)
	}
	if doc.Daily == nil {
		return nil, parseError(tool, `stdout has no "daily" array`, nil)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(doc.Daily, &entries); err != nil || entries == nil {
		// entries stays nil for {"daily":null} — null is not an array.
		return nil, parseError(tool, `"daily" is not an array`, err)
	}

	records := make([]fact.Record, 0, len(entries))
	for _, entry := range entries {
		records = append(records, fact.Record{
			Date: normalizeLabel(labelOf(entry, invocations[tool.Key].labelKey)),
			Tool: tool.Key,
			Totals: fact.Totals{
				TotalCost:           number(entry, "totalCost", "costUSD"),
				InputTokens:         int64(number(entry, "inputTokens")),
				OutputTokens:        int64(number(entry, "outputTokens")),
				CacheCreationTokens: int64(number(entry, "cacheCreationTokens")),
				CacheReadTokens:     int64(number(entry, "cacheReadTokens", "cachedInputTokens")),
				TotalTokens:         int64(number(entry, "totalTokens")),
			},
		})
	}
	return records, nil
}

// parseError builds the KindParse error for a malformed document.
func parseError(tool fact.Tool, detail string, err error) *source.Error {
	return &source.Error{Tool: tool.Key, Name: tool.Name, Kind: source.KindParse, Detail: detail, Err: err}
}

// number reads the first present key as a JSON number, mirroring the TS
// toUsageTotals coercion: a missing key falls through to the next, and a
// present key whose value is not a JSON number coerces to 0 — never an
// error. (The TS Number("12") is 12 where Go is 0; ccusage never emits
// numeric strings.)
func number(entry map[string]json.RawMessage, keys ...string) float64 {
	for _, k := range keys {
		raw, ok := entry[k]
		if !ok {
			continue
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err != nil {
			return 0
		}
		return f
	}
	return 0
}

// labelOf reads the entry's label key as a string; a missing or non-string
// value yields "".
func labelOf(entry map[string]json.RawMessage, key string) string {
	raw, ok := entry[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

var months = map[string]string{
	"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04", "May": "05", "Jun": "06",
	"Jul": "07", "Aug": "08", "Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
}

var (
	dailyLabelRe   = regexp.MustCompile(`^(\w{3})\s+(\d{1,2}),\s+(\d{4})$`)
	monthlyLabelRe = regexp.MustCompile(`^(\w{3})\s+(\d{4})$`)
)

// normalizeLabel converts the defensive human-readable forms ("Feb 14, 2026",
// "Feb 2026") to ISO ("2026-02-14", "2026-02"). ISO labels and anything
// unrecognized pass through unchanged; an unknown 3-letter month maps to
// "00"; the day is zero-padded.
func normalizeLabel(label string) string {
	if m := dailyLabelRe.FindStringSubmatch(label); m != nil {
		day := m[2]
		if len(day) == 1 {
			day = "0" + day
		}
		return fmt.Sprintf("%s-%s-%s", m[3], monthNumber(m[1]), day)
	}
	if m := monthlyLabelRe.FindStringSubmatch(label); m != nil {
		return fmt.Sprintf("%s-%s", m[2], monthNumber(m[1]))
	}
	return label
}

func monthNumber(abbr string) string {
	if n, ok := months[abbr]; ok {
		return n
	}
	return "00"
}
