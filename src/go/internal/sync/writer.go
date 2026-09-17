package sync

import (
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source/metrics"
)

// Action is a per-file write decision.
type Action string

const (
	ActionWrite Action = "write"
	ActionSkip  Action = "skip" // the never-shrink guard would skip this write
)

// Decision is the TS WriteDecision: produced for every record in BOTH live
// and dry-run mode, so the preview shares the exact decision path as the
// write (toolkit principle №5). Path is metrics.Path — absolute when dir is,
// relative when a relative metrics_dir is configured.
type Decision struct {
	Path         string
	Action       Action
	IncomingCost float64
	ExistingCost *float64 // nil unless an existing parseable file was read
}

// Write is the TS writeMetrics: one Decision per record, in record order. In
// live mode a write-decision creates the file's directory (0o755) and writes
// the day-file (0o644) as DayFile JSON + "\n"; in dry-run mode nothing is
// created or written. Only the filesystem effects are gated on dryRun. A
// filesystem error stops the walk and is returned (the TS throws — an
// uncaught crash the edge prints, exit 1). Nothing is printed, ever — the
// never-shrink skip is silent, as in the TS.
func Write(dir, user, machine string, tool fact.Tool, recs []fact.Record, dryRun bool) ([]Decision, error) {
	decisions := make([]Decision, 0, len(recs))
	for _, rec := range recs {
		path := metrics.Path(dir, user, machine, tool, rec.Date)
		shrinking, existing := shrinkState(path, rec.TotalCost)
		d := Decision{Path: path, Action: ActionWrite, IncomingCost: rec.TotalCost, ExistingCost: existing}
		if shrinking {
			d.Action = ActionSkip
		}
		decisions = append(decisions, d)
		if shrinking || dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		data, err := json.Marshal(metrics.DayFile{Label: rec.Date, Totals: rec.Totals})
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return nil, err
		}
	}
	return decisions, nil
}

// shrinkState is the TS readShrinkState, one file read. The never-shrink
// guard: day-file snapshots are high-water marks of complete data — Claude
// Code purges transcripts older than ~30 days, so a live fetch for an old
// date collapses toward zero and must never overwrite correct history. A read
// error, empty/whitespace content, undecodable JSON, or a coerced cost that
// is not finite is treated as absent (write, no existing cost), matching the
// read path's skip-silently posture. A finite existing cost yields shrinking
// = incoming < existing: strictly lower skips, equal or greater writes (so
// today's file keeps refreshing as the day grows).
func shrinkState(path string, incoming float64) (shrinking bool, existing *float64) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, nil // file absent or unreadable → write
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return false, nil // empty file → treat as absent
	}
	var doc any
	if err := json.Unmarshal([]byte(trimmed), &doc); err != nil {
		return false, nil // unparseable → treat as absent
	}
	obj, ok := doc.(map[string]any)
	if !ok {
		return false, nil // top-level non-object → Number(undefined) = NaN → absent
	}
	value, present := obj["totalCost"]
	if !present {
		return false, nil // missing key → Number(undefined) = NaN → absent
	}
	cost := jsNumber(value)
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return false, nil // not a finite number → treat as absent
	}
	return incoming < cost, &cost
}

// jsNumber is the JS Number() coercion the TS applies to existing?.totalCost:
// a JSON number → itself, null → 0, true/false → 1/0, a string → the
// StringNumericLiteral parse (jsNumberString), an array → Number() of its
// join(",") string ([5] → 5, [] → 0), an object → NaN ("[object Object]").
func jsNumber(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case nil: // JSON null (a missing key is handled by the caller)
		return 0
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		return jsNumberString(x)
	case []any:
		return jsNumberString(jsJoin(x))
	default: // object
		return math.NaN()
	}
}

// jsNumberString is Number() on a string: trimmed; empty → 0; a 0x/0X/0o/0O/
// 0b/0B prefix (no sign allowed — Number("-0x10") is NaN) → the integer in
// that base, parsed as a big.Float so arbitrarily long digit strings round to
// the nearest double as JS does (failure → NaN); otherwise strconv.ParseFloat
// (failure → NaN). ParseFloat is never offered a base-prefixed string — it
// would accept a hex float like "0x1.8p1" that JS rejects.
func jsNumberString(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if len(s) > 2 && s[0] == '0' {
		var base int
		switch s[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			digits := s[2:]
			for i := 0; i < len(digits); i++ {
				if strings.IndexByte("0123456789abcdef"[:base], lowerByte(digits[i])) < 0 {
					return math.NaN() // e.g. the hex float "0x1.8p1" JS rejects
				}
			}
			f, _, err := big.ParseFloat(digits, base, 53, big.ToNearestEven)
			if err != nil {
				return math.NaN()
			}
			v, _ := f.Float64()
			return v
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return f
}

// lowerByte folds ASCII uppercase to lowercase for the base-prefix digit scan.
func lowerByte(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// jsJoin is Array.prototype.join(",") for a JSON array: elements are
// ToString'd — null → "", bool → "true"/"false", number → its shortest
// round-trip form, string → itself, a nested array → its own join, an object
// → "[object Object]".
func jsJoin(arr []any) string {
	parts := make([]string, len(arr))
	for i, el := range arr {
		switch x := el.(type) {
		case nil:
			parts[i] = ""
		case bool:
			parts[i] = strconv.FormatBool(x)
		case float64:
			parts[i] = strconv.FormatFloat(x, 'g', -1, 64)
		case string:
			parts[i] = x
		case []any:
			parts[i] = jsJoin(x)
		default: // object
			parts[i] = "[object Object]"
		}
	}
	return strings.Join(parts, ",")
}

// Writer adapts the package Write to command.Writer (asserted where cmd/tu
// assigns it).
type Writer struct{ Dir string }

// Write calls the package Write in live mode and drops the decisions.
func (w Writer) Write(user, machine string, tool fact.Tool, recs []fact.Record) error {
	_, err := Write(w.Dir, user, machine, tool, recs, false)
	return err
}
