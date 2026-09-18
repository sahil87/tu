package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
)

// Expected is the expected-diffs file (harness/expected-diffs.json): the set
// of divergences a spec drop decision makes intentional, keyed by the spec's
// stable DC-NN ids, so a drop resolution is one data entry rather than a code
// change to the gate. The committed set is empty while every [DECIDE] marker
// is unresolved (unresolved means keep).
type Expected struct {
	Schema  int             `json:"schema"`
	Entries []ExpectedEntry `json:"expected"`
}

// ExpectedEntry is one expected divergence: the spec id behind the drop
// decision, the case patterns it explains, and the human reason.
type ExpectedEntry struct {
	ID     string   `json:"id"`
	Cases  []string `json:"cases"`
	Reason string   `json:"reason"`
}

// ExpectedStat is one entry's tally over the executed results: Matched counts
// every executed case the entry's patterns hit regardless of status, Red the
// matched cases that are red.
type ExpectedStat struct {
	ID      string
	Matched int
	Red     int
}

// expectedIDPattern is the spec's stable DC-NN id shape.
var expectedIDPattern = regexp.MustCompile(`^DC-\d{2}$`)

// LoadExpected reads and validates an expected-diffs file, LoadMatrix-style:
// DisallowUnknownFields, a trailing-value rejection, and a validate() whose
// every error names the offending entry id (when known) and field.
func LoadExpected(path string) (*Expected, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// expectedJSON distinguishes a missing "expected" key (invalid) from an
	// empty array (valid) — the matrix's args rule.
	type expectedJSON struct {
		Schema  int              `json:"schema"`
		Entries *[]ExpectedEntry `json:"expected"`
	}
	var doc expectedJSON
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	switch err := dec.Decode(&extra); err {
	case io.EOF:
	case nil:
		return nil, fmt.Errorf("trailing JSON value after expected-diffs document")
	default:
		return nil, fmt.Errorf("trailing content after expected-diffs document: %w", err)
	}
	if doc.Schema != 1 {
		return nil, fmt.Errorf("schema must be 1, got %d", doc.Schema)
	}
	if doc.Entries == nil {
		return nil, fmt.Errorf("missing expected key")
	}
	e := &Expected{Schema: doc.Schema, Entries: *doc.Entries}
	if err := e.validate(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Expected) validate() error {
	seen := map[string]bool{}
	for i, en := range e.Entries {
		where := fmt.Sprintf("entry %d", i)
		if en.ID != "" {
			where = fmt.Sprintf("entry %q", en.ID)
		}
		if !expectedIDPattern.MatchString(en.ID) {
			return fmt.Errorf("%s: invalid id %q (want DC-NN)", where, en.ID)
		}
		if seen[en.ID] {
			return fmt.Errorf("entry %q: duplicate id", en.ID)
		}
		seen[en.ID] = true
		if len(en.Cases) == 0 {
			return fmt.Errorf("entry %q: cases is an empty array", en.ID)
		}
		for _, p := range en.Cases {
			if _, err := path.Match(p, ""); err != nil {
				return fmt.Errorf("entry %q: invalid cases pattern %q: %v", en.ID, p, err)
			}
		}
		if en.Reason == "" {
			return fmt.Errorf("entry %q: empty reason", en.ID)
		}
	}
	return nil
}

// Match returns the id of the first entry (file order) with a pattern matching
// the case: path.Match against the full case ID, plus — for a pattern without
// a slash — against the group name (so "sync-dry-run" names both the matrix
// group and the live step, and "live" every live step). A nil or empty set
// never matches.
func (e *Expected) Match(caseID, group string) (id string, ok bool) {
	if e == nil {
		return "", false
	}
	for _, en := range e.Entries {
		if entryMatches(en, caseID, group) {
			return en.ID, true
		}
	}
	return "", false
}

// entryMatches reports whether any of the entry's patterns matches the case,
// independently of file order (Match's first-wins rule is separate).
func entryMatches(en ExpectedEntry, caseID, group string) bool {
	for _, p := range en.Cases {
		if ok, _ := path.Match(p, caseID); ok {
			return true
		}
		if !strings.Contains(p, "/") {
			if ok, _ := path.Match(p, group); ok {
				return true
			}
		}
	}
	return false
}

// ExpectedStats tallies each entry over the executed (post-filter) results and
// derives the stale entries: ids that matched at least one executed case and
// none red. An entry matching zero executed cases is reported but is neither
// stale nor an error — --filter and live legitimately exclude cases.
func ExpectedStats(exp *Expected, results []Result) (stats []ExpectedStat, stale []string) {
	if exp == nil {
		return nil, nil
	}
	for _, en := range exp.Entries {
		st := ExpectedStat{ID: en.ID}
		for _, r := range results {
			if entryMatches(en, r.Case.ID, r.Case.Group) {
				st.Matched++
				if r.Status == StatusRed {
					st.Red++
				}
			}
		}
		if st.Matched >= 1 && st.Red == 0 {
			stale = append(stale, en.ID)
		}
		stats = append(stats, st)
	}
	return stats, stale
}
