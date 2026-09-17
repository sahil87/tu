// Package metrics reads the metrics repo clone: the second adapter under
// source. It walks {Dir}/{user}/{year}/{machine}/{tool}-{date}.jsonl and
// returns []fact.Record stamped with User = the profile directory and
// Machine = the machine directory. It never writes (B6 owns the writer),
// never prints, and never returns an error: a missing or unreadable
// directory or file is simply absent data, exactly as the TS readers
// swallow every fs error (the repo's absence is reported once, by the
// metrics-dir guard).
package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sahil87/tu/internal/fact"
)

// nonUserDirs are the top-level metrics-repo directories that are not user
// profiles (the TS NON_USER_DIRS); Users also skips dot-prefixed names.
var nonUserDirs = map[string]bool{"docs": true}

// Source reads one metrics repo clone rooted at Dir.
type Source struct{ Dir string }

// Users lists the profile directories: direct children of Dir that are
// directories, excluding dot-prefixed names (".git") and the nonUserDirs set,
// sorted ascending (byte order — the TS Array.sort on ASCII names). A missing
// or unreadable Dir yields nil.
func (s Source) Users() []string {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil
	}
	var users []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || nonUserDirs[e.Name()] {
			continue
		}
		users = append(users, e.Name())
	}
	sort.Strings(users)
	return users
}

// DayFile is the one JSON object a {tool}-{date}.jsonl file holds, in the
// exact key order the TS toUsageEntry spread produces (label first, then the
// six totals in UsageTotals order). The writer marshals it; the reader
// unmarshals it. A missing totals key decodes as 0 (the TS would propagate
// NaN through += undefined, but only a hand-corrupted file can lack a key;
// the never-shrink writer always emits all six).
type DayFile struct {
	Label string `json:"label"`
	fact.Totals
}

// Name is the day-file basename: "{tool}-{date}.jsonl".
func Name(tool fact.Tool, date string) string {
	return tool.Key + "-" + date + ".jsonl"
}

// Path is the day-file path: {dir}/{user}/{year}/{machine}/{Name} where year
// is the label's first four characters (the TS label.slice(0, 4); a shorter
// label is used whole).
func Path(dir, user, machine string, tool fact.Tool, date string) string {
	year := date
	if len(year) > 4 {
		year = year[:4]
	}
	return filepath.Join(dir, user, year, machine, Name(tool, date))
}

// Read returns user's records for tool across every machine, in walk order:
// year directories ascending, machine directories ascending, files ascending —
// each file whose name has prefix "{tool.Key}-" and suffix ".jsonl". Per file:
// read, TrimSpace, skip when empty; decode the single JSON object; skip
// silently on any decode error. The record's Date is the JSON "label" (NOT the
// filename date); Tool is tool.Key; User/Machine are the directory names. A
// missing user directory, an unreadable level, or a non-directory at the year
// or machine level is skipped silently (nil when nothing is read).
func (s Source) Read(user string, tool fact.Tool) []fact.Record {
	// user is a profile directory name, never a path: reject path-like
	// values ("../outside") so a crafted -u cannot make the walk escape Dir.
	if user == "" || user == "." || user == ".." || strings.ContainsAny(user, `/\`) {
		return nil
	}
	prefix := tool.Key + "-"
	var out []fact.Record
	for _, year := range readSubDirs(filepath.Join(s.Dir, user)) {
		for _, machine := range readSubDirs(filepath.Join(s.Dir, user, year)) {
			machPath := filepath.Join(s.Dir, user, year, machine)
			for _, file := range readDayFiles(machPath, prefix) {
				if r, ok := readDayFile(filepath.Join(machPath, file)); ok {
					r.Tool = tool.Key
					r.User = user
					r.Machine = machine
					out = append(out, r)
				}
			}
		}
	}
	return out
}

// readSubDirs lists dir's subdirectories ascending; a missing or unreadable
// dir yields nil (the TS try/catch around each readdir).
func readSubDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// readDayFiles lists dir's entries matching the "{prefix}*.jsonl" shape,
// ascending (the TS readdir + startsWith/endsWith filter — file-type agnostic;
// a directory so named fails the read below, as in the TS).
func readDayFiles(dir, prefix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// readDayFile decodes the single JSON object in path into a record keyed by
// its label. Empty/whitespace content and any read or decode error are skipped
// silently (the TS swallows them identically — absent data, never an error).
func readDayFile(path string) (fact.Record, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fact.Record{}, false
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed[0] != '{' {
		// Only a JSON object is a day-file: "null" unmarshals cleanly into
		// the struct and would surface as a zero record with an empty label.
		return fact.Record{}, false
	}
	var d DayFile
	if err := json.Unmarshal([]byte(trimmed), &d); err != nil {
		return fact.Record{}, false
	}
	return fact.Record{Date: d.Label, Totals: d.Totals}, true
}
