package ccusage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/cache"
)

// fakeBinary is the fake ccusage built once per test binary by TestMain; it
// replays the placeholder fixture corpus, which TUDIFF_FIXTURES points at —
// the _placeholder alias only, never a local capture, so the tests are
// deterministic in CI and on dev machines.
var fakeBinary string

func TestMain(m *testing.M) {
	fixtures, err := filepath.Abs(placeholderRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve placeholder corpus: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(fixtures); err != nil {
		fmt.Fprintf(os.Stderr, "placeholder corpus missing at %s: %v\n", fixtures, err)
		os.Exit(1)
	}

	tmp, err := os.MkdirTemp("", "fakeccusage")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mktemp: %v\n", err)
		os.Exit(1)
	}
	fakeBinary = filepath.Join(tmp, "ccusage")
	build := exec.Command("go", "build", "-o", fakeBinary, "../../../cmd/fakeccusage")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build fakeccusage: %v\n%s", err, out)
		os.RemoveAll(tmp)
		os.Exit(1)
	}

	os.Setenv("TUDIFF_FIXTURES", fixtures)
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// callLog is one line of the fake's $TUDIFF_CALL_LOG.
type callLog struct {
	Tool    string   `json:"tool"`
	Argv    []string `json:"argv"`
	Cwd     string   `json:"cwd"`
	Matched string   `json:"matched,omitempty"`
}

// readCallLog parses the ccusage lines of the call log at path.
func readCallLog(t *testing.T, path string) []callLog {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read call log: %v", err)
	}
	var lines []callLog
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var cl callLog
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			t.Fatalf("call log line not JSON: %q: %v", line, err)
		}
		if cl.Tool == "ccusage" {
			lines = append(lines, cl)
		}
	}
	return lines
}

// newCallLog points TUDIFF_CALL_LOG at a fresh temp file.
func newCallLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("TUDIFF_CALL_LOG", path)
	return path
}

func lookupTool(t *testing.T, key string) Tool {
	t.Helper()
	tool, ok := Lookup(key)
	if !ok {
		t.Fatalf("Lookup(%q) missed", key)
	}
	return tool
}

func TestFetchEachTool(t *testing.T) {
	src := &Source{Binary: fakeBinary, User: "alice", Machine: "ws-1"}
	for _, tool := range Tools {
		t.Run(tool.Key, func(t *testing.T) {
			records, serr := src.Fetch(context.Background(), tool, PeriodDaily, nil, false)
			if serr != nil {
				t.Fatalf("Fetch error: %v", serr)
			}
			if len(records) != 3 {
				t.Fatalf("got %d records, want 3", len(records))
			}
			for i, r := range records {
				if r.Date != placeholderDates[i] || r.Tool != tool.Key || r.Totals != placeholderTotals {
					t.Errorf("record %d = %+v", i, r)
				}
				if r.User != "alice" || r.Machine != "ws-1" {
					t.Errorf("record %d User/Machine = %q/%q, want alice/ws-1", i, r.User, r.Machine)
				}
			}
		})
	}
}

func TestFetchArgvComposition(t *testing.T) {
	logPath := newCallLog(t)
	src := &Source{Binary: fakeBinary}

	cc := lookupTool(t, "cc")
	if _, serr := src.Fetch(context.Background(), cc, PeriodDaily, nil, false); serr != nil {
		t.Fatal(serr)
	}
	lines := readCallLog(t, logPath)
	if len(lines) != 1 {
		t.Fatalf("call log has %d lines, want 1", len(lines))
	}
	if want := []string{"claude", "daily", "--json"}; !reflect.DeepEqual(lines[0].Argv, want) {
		t.Errorf("argv = %v, want %v", lines[0].Argv, want)
	}
}

func TestFetchFixtureMiss(t *testing.T) {
	src := &Source{Binary: fakeBinary}
	kimi := lookupTool(t, "kimi")

	_, serr := src.Fetch(context.Background(), kimi, "weekly", nil, false)
	if serr == nil {
		t.Fatal("expected an error for a fixture miss")
	}
	if serr.Kind != source.KindExec {
		t.Errorf("Kind = %v, want KindExec", serr.Kind)
	}
	prefix := "Command failed: " + fakeBinary + " kimi weekly --json\nfakeccusage: no fixture for argv"
	if !strings.HasPrefix(serr.Detail, prefix) {
		t.Errorf("Detail = %q, want prefix %q", serr.Detail, prefix)
	}
	if !strings.HasSuffix(serr.Detail, "\n") {
		t.Errorf("Detail should end with the captured stderr's trailing newline: %q", serr.Detail)
	}
	if serr.Tool != "kimi" || serr.Name != "Kimi" {
		t.Errorf("Tool/Name = %q/%q", serr.Tool, serr.Name)
	}
}

// writeStub writes an executable sh stub to a temp dir and returns its path.
func writeStub(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ccusage")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFetchNonexistentBinary(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "ccusage")
	src := &Source{Binary: missing}

	_, serr := src.Fetch(context.Background(), lookupTool(t, "cc"), PeriodDaily, nil, false)
	if serr == nil {
		t.Fatal("expected an error for a nonexistent binary")
	}
	if serr.Kind != source.KindExec {
		t.Errorf("Kind = %v, want KindExec", serr.Kind)
	}
	if want := "spawn " + missing + " ENOENT"; serr.Detail != want {
		t.Errorf("Detail = %q, want %q", serr.Detail, want)
	}
}

func TestFetchEmptyStderrKeepsNewline(t *testing.T) {
	stub := writeStub(t, "#!/bin/sh\nexit 1\n")
	src := &Source{Binary: stub}

	_, serr := src.Fetch(context.Background(), lookupTool(t, "cc"), PeriodDaily, nil, false)
	if serr == nil {
		t.Fatal("expected an error")
	}
	want := "Command failed: " + stub + " claude daily --json\n"
	if serr.Detail != want {
		t.Errorf("Detail = %q, want %q (empty stderr leaves the trailing newline)", serr.Detail, want)
	}
}

func TestFetchTimeout(t *testing.T) {
	stub := writeStub(t, "#!/bin/sh\nsleep 5\n")
	src := &Source{Binary: stub}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, serr := src.Fetch(ctx, lookupTool(t, "cc"), PeriodDaily, nil, false)
	elapsed := time.Since(start)

	if serr == nil {
		t.Fatal("expected a timeout error")
	}
	if serr.Kind != source.KindTimeout {
		t.Errorf("Kind = %v, want KindTimeout", serr.Kind)
	}
	if !strings.HasPrefix(serr.Detail, "timeout after ") {
		t.Errorf("Detail = %q, want a 'timeout after ' prefix", serr.Detail)
	}
	if elapsed > 2*time.Second {
		t.Errorf("Fetch took %v, want a prompt return after the 200ms deadline", elapsed)
	}
}

func TestResolveBinaryMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no ccusage anywhere; no vendor sibling beside the test binary
	src := &Source{}              // Binary == "" → ResolveBinary()

	_, serr := src.Fetch(context.Background(), lookupTool(t, "cc"), PeriodDaily, nil, false)
	if serr == nil {
		t.Fatal("expected an error when no ccusage binary exists")
	}
	if serr.Kind != source.KindExec {
		t.Errorf("Kind = %v, want KindExec", serr.Kind)
	}
	if serr.Detail != "spawn ccusage ENOENT" {
		t.Errorf("Detail = %q, want %q", serr.Detail, "spawn ccusage ENOENT")
	}
}

func TestResolveBinaryPathFallback(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "ccusage")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	got, err := ResolveBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got != stub {
		t.Errorf("ResolveBinary() = %q, want %q", got, stub)
	}
}

func TestFetchAllDaily(t *testing.T) {
	src := &Source{Binary: fakeBinary, User: "alice", Machine: "ws-1"}
	records, errs := src.FetchAll(context.Background(), PeriodDaily, nil, false)

	if len(errs) != 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(records) != 18 {
		t.Fatalf("got %d records, want 18", len(records))
	}
	for i, r := range records {
		wantTool := Tools[i/3]
		if r.Tool != wantTool.Key {
			t.Errorf("record %d Tool = %q, want %q (registry order)", i, r.Tool, wantTool.Key)
		}
		if r.Date != placeholderDates[i%3] {
			t.Errorf("record %d Date = %q, want %q", i, r.Date, placeholderDates[i%3])
		}
		if r.User != "alice" || r.Machine != "ws-1" {
			t.Errorf("record %d User/Machine = %q/%q", i, r.User, r.Machine)
		}
	}
}

func TestFetchAllWeekly(t *testing.T) {
	src := &Source{Binary: fakeBinary}
	records, errs := src.FetchAll(context.Background(), "weekly", nil, false)

	if len(records) != 0 {
		t.Errorf("records = %v, want none", records)
	}
	if len(errs) != len(Tools) {
		t.Fatalf("got %d errors, want %d", len(errs), len(Tools))
	}
	for i, err := range errs {
		if err.Kind != source.KindExec {
			t.Errorf("error %d Kind = %v, want KindExec", i, err.Kind)
		}
		if err.Tool != Tools[i].Key {
			t.Errorf("error %d Tool = %q, want %q (registry order)", i, err.Tool, Tools[i].Key)
		}
	}
}

// --- Cache through Source (R11) ---

func TestFetchCacheHitSkipsBinary(t *testing.T) {
	logPath := newCallLog(t)
	store := &cache.Store{Dir: t.TempDir(), TTL: cache.TTL, Now: time.Now}
	src := &Source{Binary: fakeBinary, User: "alice", Machine: "ws-1", Cache: store}
	cc := lookupTool(t, "cc")

	first, serr := src.Fetch(context.Background(), cc, PeriodDaily, nil, false)
	if serr != nil {
		t.Fatal(serr)
	}
	second, serr := src.Fetch(context.Background(), cc, PeriodDaily, nil, false)
	if serr != nil {
		t.Fatal(serr)
	}

	lines := readCallLog(t, logPath)
	if len(lines) != 1 {
		t.Errorf("call log has %d ccusage lines, want exactly 1 (second fetch served from cache)", len(lines))
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("cached result differs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	for i, r := range second {
		if r.User != "alice" || r.Machine != "ws-1" {
			t.Errorf("cached record %d User/Machine = %q/%q, want stamped", i, r.User, r.Machine)
		}
	}
}

func TestFetchFreshReinvokesAndRewrites(t *testing.T) {
	logPath := newCallLog(t)
	dir := t.TempDir()
	store := &cache.Store{Dir: dir, TTL: cache.TTL, Now: time.Now}
	src := &Source{Binary: fakeBinary, Cache: store}
	cc := lookupTool(t, "cc")

	if _, serr := src.Fetch(context.Background(), cc, PeriodDaily, nil, false); serr != nil {
		t.Fatal(serr)
	}
	cacheFile := filepath.Join(dir, cache.Key{Tool: "cc", Period: "daily"}.Filename())
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(cacheFile, old, old); err != nil {
		t.Fatal(err)
	}

	if _, serr := src.Fetch(context.Background(), cc, PeriodDaily, nil, true); serr != nil {
		t.Fatal(serr)
	}
	lines := readCallLog(t, logPath)
	if len(lines) != 2 {
		t.Errorf("call log has %d ccusage lines, want 2 (fresh re-invokes the binary)", len(lines))
	}
	info, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().After(old) {
		t.Error("fresh fetch should have refreshed the cache file's mtime")
	}
}

// cacheFiles lists the regular files in a cache dir; a missing dir (Put never
// ran) reads as zero files.
func cacheFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	return entries
}

func TestFetchEmptyDailyWritesNoCache(t *testing.T) {
	stub := writeStub(t, "#!/bin/sh\necho '{\"daily\":[]}'\n")
	dir := t.TempDir()
	src := &Source{Binary: stub, Cache: &cache.Store{Dir: dir, TTL: cache.TTL, Now: time.Now}}

	records, serr := src.Fetch(context.Background(), lookupTool(t, "cc"), PeriodDaily, nil, false)
	if serr != nil {
		t.Fatalf("Fetch error: %v", serr)
	}
	if len(records) != 0 {
		t.Errorf("records = %v, want none", records)
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Errorf("cache dir holds %d files, want none (empty results are not cached)", len(files))
	}
}

func TestFetchFailureWritesNoCache(t *testing.T) {
	dir := t.TempDir()
	src := &Source{Binary: fakeBinary, Cache: &cache.Store{Dir: dir, TTL: cache.TTL, Now: time.Now}}

	_, serr := src.Fetch(context.Background(), lookupTool(t, "kimi"), "weekly", nil, false)
	if serr == nil {
		t.Fatal("expected a fixture-miss error")
	}
	if files := cacheFiles(t, dir); len(files) != 0 {
		t.Errorf("cache dir holds %d files, want none (failures are not cached)", len(files))
	}
}
