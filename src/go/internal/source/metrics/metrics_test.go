package metrics

import (
	"os"
	"path/filepath"
	"reflect"
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
// located by walking up to package.json (the defaults_test.go convention).
func seedDir(t *testing.T) string {
	t.Helper()
	root := findPackageRoot(t)
	dst := filepath.Join(t.TempDir(), "metrics_repo")
	copyTree(t, filepath.Join(root, "harness", "metrics-repo"), dst)
	return dst
}

// findPackageRoot walks up from the package directory (the test's cwd) to the
// first directory containing package.json — the repo root.
func findPackageRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no package.json found walking up from the package directory")
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
