package metrics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

var (
	ccTool, _    = fact.Lookup("cc")
	codexTool, _ = fact.Lookup("codex")
)

// seedTotals are the placeholder token counters every seed day-file carries.
var seedTotals = fact.Totals{InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}

func seedRec(date, machine string, cost float64) fact.Record {
	t := seedTotals
	t.TotalCost = cost
	return fact.Record{Date: date, Tool: "cc", User: "harness-user", Machine: machine, Totals: t}
}

// seedDir returns a temp-dir copy of the committed harness/metrics-repo seed,
// located by walking up to justfile (the defaults_test.go convention).
func seedDir(t *testing.T) string {
	t.Helper()
	root := findJustfileRoot(t)
	dst := filepath.Join(t.TempDir(), "metrics_repo")
	copyTree(t, filepath.Join(root, "harness", "metrics-repo"), dst)
	return dst
}

// findJustfileRoot walks up from the package directory (the test's cwd) to
// the first directory containing justfile — the repo root.
func findJustfileRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "justfile")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no justfile found walking up from the package directory")
		}
		dir = parent
	}
}

// copyTree recursively copies src to dst (files 0644, dirs 0755).
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		raw, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeDayFile writes one {tool}-{date}.jsonl file under dir/user/year/machine.
func writeDayFile(t *testing.T, dir, user, year, machine, name, content string) {
	t.Helper()
	mach := filepath.Join(dir, user, year, machine)
	if err := os.MkdirAll(mach, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mach, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// R1: Users returns the sorted profile directories, excluding dot-prefixed
// names, nonUserDirs, and plain files; a missing Dir yields nil.
func TestUsers(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"other-user", "harness-user", "docs", ".git"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, want := (Source{Dir: dir}).Users(), []string{"harness-user", "other-user"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Users = %v, want %v", got, want)
	}
	if got := (Source{Dir: filepath.Join(dir, "nonexistent")}).Users(); got != nil {
		t.Errorf("Users on a missing Dir = %v, want nil", got)
	}
}

// R1 against the committed seed: docs and README are excluded.
func TestUsersOnSeed(t *testing.T) {
	if got, want := (Source{Dir: seedDir(t)}).Users(), []string{"harness-user", "other-user"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Users = %v, want %v", got, want)
	}
}

// R2: Read walks the seed in year/machine/file order, stamps User and Machine
// from the directories, and takes Date from the JSON label.
func TestReadSeedWalkOrder(t *testing.T) {
	dir := seedDir(t)
	want := []fact.Record{
		seedRec("2026-01-05", "harness-machine", 0.25),
		seedRec("2026-01-06", "harness-machine", 0.75),
		seedRec("2026-01-06", "other-box", 0.40),
	}
	if got := (Source{Dir: dir}).Read("harness-user", ccTool); !reflect.DeepEqual(got, want) {
		t.Errorf("Read cc = %+v, want %+v", got, want)
	}

	codexWant := []fact.Record{
		{Date: "2026-01-07", Tool: "codex", User: "harness-user", Machine: "other-box",
			Totals: fact.Totals{TotalCost: 0.30, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}},
	}
	if got := (Source{Dir: dir}).Read("harness-user", codexTool); !reflect.DeepEqual(got, codexWant) {
		t.Errorf("Read codex = %+v, want %+v", got, codexWant)
	}

	if got := (Source{Dir: dir}).Read("nobody", ccTool); got != nil {
		t.Errorf("Read for a missing user = %v, want nil", got)
	}
}

// R2 edge cases: the label (not the filename) keys the record; the prefix is
// exact to "{key}-"; empty, whitespace, and garbage files are skipped; a
// missing token key decodes as 0; non-directories at the year/machine level
// are skipped.
func TestReadEdgeCases(t *testing.T) {
	dir := t.TempDir()
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-05.jsonl",
		`{"label":"2026-02-03","totalCost":0.25,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}`)
	// A file lacking cacheReadTokens decodes that field as 0.
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-06.jsonl",
		`{"label":"2026-01-06","totalCost":0.75,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"totalTokens":24400}`)
	// Never matched for tool cc: wrong prefix, wrong suffix, a bare directory.
	writeDayFile(t, dir, "u", "2026", "m", "ccx-2026-01-05.jsonl", `{"label":"2026-01-05","totalCost":9}`)
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-05.json", `{"label":"2026-01-05","totalCost":9}`)
	// Empty / whitespace / garbage files are skipped silently.
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-07.jsonl", "")
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-08.jsonl", "  \n\t ")
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-09.jsonl", "not json")
	// A JSON null unmarshals cleanly into the struct but is no day-file.
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-10.jsonl", "null")
	// Non-directories at the year and machine level are skipped.
	if err := os.WriteFile(filepath.Join(dir, "u", "stray-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "u", "2026", "stray-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	want := []fact.Record{
		{Date: "2026-02-03", Tool: "cc", User: "u", Machine: "m",
			Totals: fact.Totals{TotalCost: 0.25, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}},
		{Date: "2026-01-06", Tool: "cc", User: "u", Machine: "m",
			Totals: fact.Totals{TotalCost: 0.75, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 0, TotalTokens: 24400}},
	}
	if got := (Source{Dir: dir}).Read("u", ccTool); !reflect.DeepEqual(got, want) {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

// R2: a missing Dir reads as nil, no error, no panic.
func TestReadMissingDir(t *testing.T) {
	if got := (Source{Dir: filepath.Join(t.TempDir(), "nonexistent")}).Read("u", ccTool); got != nil {
		t.Errorf("Read = %v, want nil", got)
	}
}

// user is a profile directory name, never a path: path-like values are
// rejected so the walk cannot escape Dir.
func TestReadPathLikeUser(t *testing.T) {
	dir := t.TempDir()
	writeDayFile(t, dir, "u", "2026", "m", "cc-2026-01-05.jsonl",
		`{"label":"2026-01-05","totalCost":0.25,"inputTokens":3000,"outputTokens":400,"cacheCreationTokens":1000,"cacheReadTokens":20000,"totalTokens":24400}`)
	for _, user := range []string{"", ".", "..", "../u", `..\u`, "a/b"} {
		if got := (Source{Dir: dir}).Read(user, ccTool); got != nil {
			t.Errorf("Read(%q) = %v, want nil", user, got)
		}
	}
}

// R1: marshalling DayFile with encoding/json's defaults reproduces the
// committed seed file's bytes (minus its trailing newline) — the same key
// order JSON.stringify(toUsageEntry(...)) produces.
func TestDayFileMarshalSeedBytes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(findJustfileRoot(t),
		"harness", "metrics-repo", "harness-user", "2026", "harness-machine", "cc-2026-01-05.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))

	d := DayFile{Label: "2026-01-05", Totals: seedTotals}
	d.TotalCost = 0.25
	got, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("Marshal(DayFile) = %s, want %s", got, want)
	}
}

// R1: finite doubles marshal exactly as JavaScript prints them (every
// expected string below verified with `node -p 'JSON.stringify(...)'`):
// integers without a fraction, shortest round-trip, exponent form only below
// 1e-6 or at 1e21 and above, and the e-07 → e-7 cleanup.
func TestDayFileMarshalFloatShapes(t *testing.T) {
	// 0.1 + 0.2 must be a runtime sum: Go folds untyped constant arithmetic
	// exactly (0.3), while JavaScript's doubles produce 0.30000000000000004.
	a, b := 0.1, 0.2
	shapes := []struct {
		cost float64
		want string // Node-verified JSON.stringify output
	}{
		{0.5, "0.5"},
		{211.8, "211.8"},
		{1e-7, "1e-7"},
		{1e21, "1e+21"},
		{a + b, "0.30000000000000004"},
	}
	for _, tt := range shapes {
		got, err := json.Marshal(DayFile{Label: "f", Totals: fact.Totals{TotalCost: tt.cost}})
		if err != nil {
			t.Fatal(err)
		}
		want := `{"label":"f","totalCost":` + tt.want +
			`,"inputTokens":0,"outputTokens":0,"cacheCreationTokens":0,"cacheReadTokens":0,"totalTokens":0}`
		if string(got) != want {
			t.Errorf("Marshal(totalCost %v) = %s, want %s", tt.cost, got, want)
		}
	}
}

// R1: Name is "{tool}-{date}.jsonl".
func TestName(t *testing.T) {
	if got, want := Name(ccTool, "2026-01-05"), "cc-2026-01-05.jsonl"; got != want {
		t.Errorf("Name(cc, 2026-01-05) = %q, want %q", got, want)
	}
	if got, want := Name(codexTool, "2026-02"), "codex-2026-02.jsonl"; got != want {
		t.Errorf("Name(codex, 2026-02) = %q, want %q", got, want)
	}
}

// R1: Path is {dir}/{user}/{year}/{machine}/{Name}; year is the label's
// first four characters, or the whole label when shorter (the TS
// label.slice(0, 4)).
func TestPath(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2026-01-05", filepath.Join("/repo", "u", "2026", "m", "cc-2026-01-05.jsonl")},
		{"2026-02", filepath.Join("/repo", "u", "2026", "m", "cc-2026-02.jsonl")},
		{"2026", filepath.Join("/repo", "u", "2026", "m", "cc-2026.jsonl")},
		{"202", filepath.Join("/repo", "u", "202", "m", "cc-202.jsonl")},
		{"", filepath.Join("/repo", "u", "", "m", "cc-.jsonl")},
	}
	for _, tt := range cases {
		if got := Path("/repo", "u", "m", ccTool, tt.date); got != tt.want {
			t.Errorf("Path(..., %q) = %q, want %q", tt.date, got, tt.want)
		}
	}
}
