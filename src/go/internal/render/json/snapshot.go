// Package json is the JSON encoder: it renders the snapshot object exactly as
// the TS emitJson (JSON.stringify(obj, null, 2) plus console.log's trailing
// newline) does, and the two history shapes (history.go: a bare entry array
// for one tool, an object keyed by display name with [] inline for empty
// series). The writer is hand-ordered — Go maps are unordered and struct
// marshalling cannot emit the conditional "label" key first — using
// encoding/json only for scalar encoding. Returned as lines; nothing here
// writes to a stream.
package json

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/sahil87/tu/internal/view"
)

// Snapshot renders the snapshot JSON (layouts §12): an object keyed by
// display name in the given order; per tool, "label" FIRST when Label != "",
// then the six pinned totals keys; two-space indent. The trailing newline of
// console.log is the caller's Fprintln per line.
func Snapshot(rows []view.ToolTotals) []string {
	lines := []string{"{"}
	for i, r := range rows {
		lines = append(lines, "  "+encodeString(r.Name)+": {")
		if r.Label != "" {
			lines = append(lines, `    "label": `+encodeString(r.Label)+",")
		}
		lines = append(lines,
			`    "totalCost": `+encodeFloat(r.TotalCost)+",",
			`    "inputTokens": `+strconv.FormatInt(r.InputTokens, 10)+",",
			`    "outputTokens": `+strconv.FormatInt(r.OutputTokens, 10)+",",
			`    "cacheCreationTokens": `+strconv.FormatInt(r.CacheCreationTokens, 10)+",",
			`    "cacheReadTokens": `+strconv.FormatInt(r.CacheReadTokens, 10)+",",
			`    "totalTokens": `+strconv.FormatInt(r.TotalTokens, 10),
		)
		close := "  }"
		if i < len(rows)-1 {
			close += ","
		}
		lines = append(lines, close)
	}
	return append(lines, "}")
}

// encodeString encodes a JSON string without HTML escaping (display names and
// labels carry none, but the encoder is pinned anyway).
func encodeString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return `""` // unreachable for a string
	}
	return string(bytes.TrimRight(buf.Bytes(), "\n"))
}

// encodeFloat encodes a float64 following the ES6 rules encoding/json
// implements (shortest repr, 1e+21/1e-7 exponent thresholds). -0 is
// normalized to 0: Go writes "-0", V8 writes "0".
func encodeFloat(f float64) string {
	if f == 0 {
		f = 0 // normalize -0 to +0
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return "0" // NaN/Inf cannot occur in summed costs
	}
	return string(raw)
}
