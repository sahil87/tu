package json

import (
	"strconv"

	"github.com/sahil87/tu/internal/view"
)

// History renders the single-tool history JSON (tu cc h --json): a BARE array
// of entry objects — [] inline when empty — laid out as JSON.stringify(v,
// null, 2). Entry keys in order: label (always present on history entries),
// totalCost, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens,
// totalTokens. Costs are the raw summed doubles (DC-10), scalars through the
// V2 helpers.
func History(s view.Series) []string {
	if len(s.Entries) == 0 {
		return []string{"[]"}
	}
	lines := []string{"["}
	for i, e := range s.Entries {
		lines = append(lines, entryLines(e, "  ")...)
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
			lines = append(lines, entryLines(e, "    ")...)
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
// placement).
func entryLines(e view.Entry, indent string) []string {
	return []string{
		indent + "{",
		indent + `  "label": ` + encodeString(e.Label) + ",",
		indent + `  "totalCost": ` + encodeFloat(e.TotalCost) + ",",
		indent + `  "inputTokens": ` + strconv.FormatInt(e.InputTokens, 10) + ",",
		indent + `  "outputTokens": ` + strconv.FormatInt(e.OutputTokens, 10) + ",",
		indent + `  "cacheCreationTokens": ` + strconv.FormatInt(e.CacheCreationTokens, 10) + ",",
		indent + `  "cacheReadTokens": ` + strconv.FormatInt(e.CacheReadTokens, 10) + ",",
		indent + `  "totalTokens": ` + strconv.FormatInt(e.TotalTokens, 10),
	}
}
