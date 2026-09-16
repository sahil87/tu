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
// then the six pinned totals keys, then — when the breakdown carries ≥1 slice
// for the row — a "machines" object (one "name": cost line per slice in
// FIRST-SEEN slice order, never sorted — the TS attachMachinesJson copies the
// Map in insertion order; values are raw doubles and always cost, even under
// -t). Rows with no slices are byte-identical to the no-breakdown output; a
// zero-usage single-source tool MAY carry machines with 0 values (the TS
// zero-fill) and no label. Two-space indent. The trailing newline of
// console.log is the caller's Fprintln per line.
func Snapshot(rows []view.ToolTotals, bd *view.Breakdown) []string {
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
			`    "totalTokens": `+strconv.FormatInt(r.TotalTokens, 10)+machineComma(bd, r.Name),
		)
		lines = append(lines, machineLines(bd, r.Name, "    ")...)
		close := "  }"
		if i < len(rows)-1 {
			close += ","
		}
		lines = append(lines, close)
	}
	return append(lines, "}")
}

// machineComma is the comma on the totalTokens line when a machines object
// follows (the TS spread `{...val, machines}`).
func machineComma(bd *view.Breakdown, key string) string {
	if len(bd.Slices(key)) > 0 {
		return ","
	}
	return ""
}

// machineLines renders one row's "machines" object at the row's indent (the
// opening line included): one "name": cost line per slice in slice order,
// values always cost (DC-10 raw doubles). No slices → no lines.
func machineLines(bd *view.Breakdown, key, indent string) []string {
	slices := bd.Slices(key)
	if len(slices) == 0 {
		return nil
	}
	lines := []string{indent + `"machines": {`}
	for i, s := range slices {
		comma := ","
		if i == len(slices)-1 {
			comma = ""
		}
		lines = append(lines, indent+"  "+encodeString(s.Name)+": "+encodeFloat(s.TotalCost)+comma)
	}
	return append(lines, indent+"}")
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
