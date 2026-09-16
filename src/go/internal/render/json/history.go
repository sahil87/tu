package json

import (
	"strconv"

	"github.com/sahil87/tu/internal/view"
)

// History renders the single-tool history JSON (tu cc h --json): a BARE array
// of entry objects — [] inline when empty — laid out as JSON.stringify(v,
// null, 2). Entry keys in order: label (always present on history entries),
// totalCost, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens,
// totalTokens, then — when the breakdown carries ≥1 slice for the entry's
// label — a "machines" object in first-seen slice order (cost values; the TS
// attachMachinesJson). Entries with no slices are byte-identical to the
// no-breakdown output. Costs are the raw summed doubles (DC-10), scalars
// through the V2 helpers.
func History(s view.Series, bd *view.Breakdown) []string {
	if len(s.Entries) == 0 {
		return []string{"[]"}
	}
	lines := []string{"["}
	for i, e := range s.Entries {
		lines = append(lines, entryLines(e, "  ", bd)...)
		close := "  }"
		if i < len(s.Entries)-1 {
			close += ","
		}
		lines = append(lines, close)
	}
	return append(lines, "]")
}

// TotalHistory renders the all-tools history JSON (tu h --json): an object
// keyed by display name in input (registry) order, each value the bare entry
// array — [] inline for an empty series.
func TotalHistory(series []view.Series) []string {
	lines := []string{"{"}
	for i, s := range series {
		comma := ","
		if i == len(series)-1 {
			comma = ""
		}
		if len(s.Entries) == 0 {
			lines = append(lines, "  "+encodeString(s.Name)+": []"+comma)
			continue
		}
		lines = append(lines, "  "+encodeString(s.Name)+": [")
		for j, e := range s.Entries {
			lines = append(lines, entryLines(e, "    ", nil)...)
			close := "    }"
			if j < len(s.Entries)-1 {
				close += ","
			}
			lines = append(lines, close)
		}
		lines = append(lines, "  ]"+comma)
	}
	return append(lines, "}")
}

// entryLines is one entry object's field lines at the given indent (the
// opening "{" line included, the closing "}" left to the caller's comma
// placement); a breakdown with slices for the entry's label appends the
// "machines" object after totalTokens.
func entryLines(e view.Entry, indent string, bd *view.Breakdown) []string {
	lines := []string{
		indent + "{",
		indent + `  "label": ` + encodeString(e.Label) + ",",
		indent + `  "totalCost": ` + encodeFloat(e.TotalCost) + ",",
		indent + `  "inputTokens": ` + strconv.FormatInt(e.InputTokens, 10) + ",",
		indent + `  "outputTokens": ` + strconv.FormatInt(e.OutputTokens, 10) + ",",
		indent + `  "cacheCreationTokens": ` + strconv.FormatInt(e.CacheCreationTokens, 10) + ",",
		indent + `  "cacheReadTokens": ` + strconv.FormatInt(e.CacheReadTokens, 10) + ",",
		indent + `  "totalTokens": ` + strconv.FormatInt(e.TotalTokens, 10) + machineComma(bd, e.Label),
	}
	return append(lines, machineLines(bd, e.Label, indent+"  ")...)
}
