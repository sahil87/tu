package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testHeader() ReportHeader {
	return ReportHeader{
		Timestamp:       time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC),
		NodePath:        "dist/tu.mjs",
		NodeVersion:     "v24.15.0",
		GoPath:          "bin/tu",
		GoVersion:       "tu version v0.11.5",
		Fixtures:        []string{"dev-ws-sahil02", PlaceholderAlias},
		Script:          "util-linux",
		MatrixPath:      "harness/matrix.json",
		Cases:           3,
		Filter:          "snap",
		ExpectedPath:    "harness/expected-diffs.json",
		ExpectedEntries: 0,
	}
}

func TestRenderHeader(t *testing.T) {
	got := strings.Join(RenderHeader(testHeader()), "\n")
	want := `tudiff run  2026-09-16T12:00:00Z
node: dist/tu.mjs (v24.15.0)
go: bin/tu (tu version v0.11.5)
fixtures: dev-ws-sahil02, _placeholder
script: util-linux
matrix: harness/matrix.json (3 cases, filter "snap")
expected: harness/expected-diffs.json (0 entries)`
	if got != want {
		t.Errorf("header =\n%s\nwant =\n%s", got, want)
	}
}

func TestRenderCaseLines(t *testing.T) {
	mk := func(status, id string) Result {
		return Result{Case: Case{ID: id}, Status: status}
	}
	green := RenderCaseLine(mk(StatusGreen, "version/single/default/pipe/fixed"))
	if green != "GREEN   version/single/default/pipe/fixed" {
		t.Errorf("green line = %q", green)
	}

	redExit := mk(StatusRed, "bogus/single/default/pipe/fixed")
	redExit.Channel = "exit"
	redExit.NodeExit, redExit.GoExit = 2, 1
	if got := RenderCaseLine(redExit); got != "RED     bogus/single/default/pipe/fixed  exit: node=2 go=1" {
		t.Errorf("red exit line = %q", got)
	}

	redOut := mk(StatusRed, "help/single/default/pipe/fixed")
	redOut.Channel, redOut.Offset, redOut.Line = "stdout", 0, 1
	redOut.NodeExcerpt, redOut.GoExcerpt = `"Usage: tu\n"`, `""`
	if got := RenderCaseLine(redOut); got != `RED     help/single/default/pipe/fixed  stdout @0 (line 1): node="Usage: tu\n" go=""` {
		t.Errorf("red stdout line = %q", got)
	}

	to := mk(StatusTimeout, "h/single/default/pipe/fixed")
	to.Channel = "timeout"
	to.GoTimeout = true
	if got := RenderCaseLine(to); got != "TIMEOUT h/single/default/pipe/fixed  timeout: node=false go=true" {
		t.Errorf("timeout line = %q", got)
	}

	redTree := mk(StatusRed, "sync-cmd/multi/default/pipe/fixed")
	redTree.Channel = "tree"
	redTree.NodeExcerpt = `harness-user/x.jsonl: "1.00}\n"`
	redTree.GoExcerpt = `harness-user/x.jsonl: "2.00}\n"`
	wantTree := `RED     sync-cmd/multi/default/pipe/fixed  tree: node=harness-user/x.jsonl: "1.00}\n" go=harness-user/x.jsonl: "2.00}\n"`
	if got := RenderCaseLine(redTree); got != wantTree {
		t.Errorf("red tree line = %q", got)
	}

	marked := mk(StatusRed, "x/single/default/pipe/fixed")
	marked.Channel = "exit"
	marked.Unconfirmed = true
	marked.CallsDiffer = true
	marked.NodeCalls, marked.GoCalls = 6, 0
	got := RenderCaseLine(marked)
	if !strings.HasSuffix(got, " [unconfirmed] [calls differ: node=6 go=0]") {
		t.Errorf("markers = %q", got)
	}

	// R6: the expected marker sits after the divergence detail and before
	// [unconfirmed] / [calls differ …].
	expected := mk(StatusRed, "y/single/default/pipe/fixed")
	expected.Channel = "stdout"
	expected.NodeExcerpt, expected.GoExcerpt = `"q"`, `"p"`
	expected.Expected = "DC-05"
	expected.Unconfirmed = true
	expected.CallsDiffer = true
	expected.NodeCalls, expected.GoCalls = 6, 0
	got = RenderCaseLine(expected)
	if !strings.HasSuffix(got, `node="q" go="p" [expected DC-05] [unconfirmed] [calls differ: node=6 go=0]`) {
		t.Errorf("expected marker placement = %q", got)
	}
}

// R14/R6: the summary block format, with green/total per axis value.
func TestRenderSummary(t *testing.T) {
	results := []Result{
		{Case: Case{ID: "a/single/default/pipe/fixed", Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed}, Status: StatusGreen},
		{Case: Case{ID: "b/multi/default/tty/alt", Conf: ConfMulti, Env: EnvDefault, IO: IOTTY, TZ: TZAlt}, Status: StatusRed, Channel: "stdout"},
		{Case: Case{ID: "c/multi/nocolor/pipe/fixed", Conf: ConfMulti, Env: EnvNoColor, IO: IOPipe, TZ: TZFixed}, Status: StatusTimeout, Unconfirmed: true},
	}
	got := strings.Join(RenderSummary(SummarizeResults(nil, results), []string{PlaceholderAlias}), "\n")
	want := `tudiff: 3 cases — 1 green, 1 red (0 expected, 1 unexpected), 1 timeout   (fixtures: _placeholder; 1 cases replayed unconfirmed fixtures)
  by conf:  single 1/1  multi 0/2  org 0/0  legacy 0/0
  by env:   default 1/2  nocolor 0/1  envrepo 0/0  pullfail 0/0  pushfail 0/0  dirty 0/0
  by io:    pipe 1/2  tty 0/1
  by tz:    fixed 1/2  alt 0/1`
	if got != want {
		t.Errorf("summary =\n%s\nwant =\n%s", got, want)
	}
}

// R6: with a non-empty set one line per entry follows the axis lines, with
// the stale / no-executed-case annotations; the expected red count is split
// out on the first line.
func TestRenderSummaryWithEntries(t *testing.T) {
	exp := &Expected{Entries: []ExpectedEntry{
		{ID: "DC-05", Cases: []string{"b"}, Reason: "x"},
		{ID: "DC-06", Cases: []string{"a"}, Reason: "y"},
		{ID: "DC-07", Cases: []string{"zzz"}, Reason: "z"},
	}}
	red := Result{Case: Case{ID: "b/single/default/pipe/fixed", Group: "b", Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed}, Status: StatusRed, Channel: "stdout", Expected: "DC-05"}
	green := Result{Case: Case{ID: "a/single/default/pipe/fixed", Group: "a", Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed}, Status: StatusGreen}
	got := strings.Join(RenderSummary(SummarizeResults(exp, []Result{green, red}), []string{PlaceholderAlias}), "\n")
	want := `tudiff: 2 cases — 1 green, 1 red (1 expected, 0 unexpected), 0 timeout   (fixtures: _placeholder; 0 cases replayed unconfirmed fixtures)
  by conf:  single 1/2  multi 0/0  org 0/0  legacy 0/0
  by env:   default 1/2  nocolor 0/0  envrepo 0/0  pullfail 0/0  pushfail 0/0  dirty 0/0
  by io:    pipe 1/2  tty 0/0
  by tz:    fixed 1/2  alt 0/0
  expected: DC-05  1/1 red
  expected: DC-06  0/1 red (stale)
  expected: DC-07  0/0 red (no executed case)`
	if got != want {
		t.Errorf("summary =\n%s\nwant =\n%s", got, want)
	}
}

// R14/R6/R7: report.txt and report.json land under the report dir with the
// specified shapes.
func TestWriteReport(t *testing.T) {
	dir := t.TempDir()
	h := testHeader()
	h.Filter = ""
	exp := &Expected{Entries: []ExpectedEntry{
		{ID: "DC-05", Cases: []string{"b"}, Reason: "x"},
	}}
	h.ExpectedEntries = len(exp.Entries)
	results := []Result{
		{Case: Case{ID: "a/single/default/pipe/fixed", Group: "a", Args: []string{}, Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed}, Status: StatusGreen},
		{Case: Case{ID: "b/single/default/pipe/fixed", Group: "b", Args: []string{"h"}, Conf: ConfSingle, Env: EnvDefault, IO: IOPipe, TZ: TZFixed},
			Status: StatusRed, Channel: "stdout", Offset: 0, Line: 1, NodeExcerpt: `"x"`, GoExcerpt: `""`, NodeExit: 0, GoExit: 0, NodeMs: 12, GoMs: 3, Expected: "DC-05"},
	}
	if err := WriteReport(dir, h, exp, results); err != nil {
		t.Fatal(err)
	}

	txt, err := os.ReadFile(filepath.Join(dir, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(txt)
	for _, sub := range []string{
		"tudiff run  2026-09-16T12:00:00Z\n",
		"matrix: harness/matrix.json (3 cases)\n",
		"expected: harness/expected-diffs.json (1 entries)\n",
		"GREEN   a/single/default/pipe/fixed\n",
		"RED     b/single/default/pipe/fixed  stdout @0 (line 1): node=\"x\" go=\"\" [expected DC-05]\n",
		"tudiff: 2 cases — 1 green, 1 red (1 expected, 0 unexpected), 0 timeout   (fixtures: dev-ws-sahil02, _placeholder; 0 cases replayed unconfirmed fixtures)\n",
		"  expected: DC-05  1/1 red\n",
	} {
		if !strings.Contains(text, sub) {
			t.Errorf("report.txt lacks %q:\n%s", sub, text)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Errorf("report.json lacks trailing newline")
	}
	var doc struct {
		Schema int `json:"schema"`
		Header struct {
			Node            string   `json:"node"`
			Script          string   `json:"script"`
			Fixtures        []string `json:"fixtures"`
			Cases           int      `json:"cases"`
			Expected        string   `json:"expected"`
			ExpectedEntries int      `json:"expected_entries"`
		} `json:"header"`
		Summary struct {
			Total      int      `json:"total"`
			Green      int      `json:"green"`
			Red        int      `json:"red"`
			Timeout    int      `json:"timeout"`
			Expected   int      `json:"expected"`
			Unexpected int      `json:"unexpected"`
			Stale      []string `json:"stale"`
		} `json:"summary"`
		Cases []struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			Channel     string `json:"channel"`
			NodeMs      int64  `json:"node_ms"`
			CallsDiffer bool   `json:"calls_differ"`
			Rerun       bool   `json:"rerun"`
			Expected    string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("report.json: %v", err)
	}
	if doc.Schema != 1 || doc.Summary.Total != 2 || doc.Summary.Red != 1 || doc.Header.Script != "util-linux" || doc.Header.Cases != 3 {
		t.Errorf("doc = %+v", doc)
	}
	if doc.Summary.Expected != 1 || doc.Summary.Unexpected != 0 || doc.Summary.Stale == nil || len(doc.Summary.Stale) != 0 {
		t.Errorf("summary = %+v (stale must be [], never null)", doc.Summary)
	}
	if doc.Header.Expected != "harness/expected-diffs.json" || doc.Header.ExpectedEntries != 1 {
		t.Errorf("header = %+v", doc.Header)
	}
	if len(doc.Cases) != 2 || doc.Cases[1].ID != "b/single/default/pipe/fixed" || doc.Cases[1].Channel != "stdout" || doc.Cases[1].NodeMs != 12 {
		t.Errorf("cases = %+v", doc.Cases)
	}
	if doc.Cases[0].Expected != "" || doc.Cases[1].Expected != "DC-05" {
		t.Errorf("case expected ids = %q, %q", doc.Cases[0].Expected, doc.Cases[1].Expected)
	}
}

// R14: raw captures land under cases/<id as nested dirs>/, io-appropriate
// files only.
func TestWriteCaseCaptures(t *testing.T) {
	dir := t.TempDir()
	node := SideCapture{Stdout: []byte("o"), Stderr: []byte("e"), Exit: 0}
	goCap := SideCapture{Stdout: []byte("o"), Stderr: []byte("x"), Exit: 1}
	r := Result{Case: Case{ID: "h/multi/default/pipe/fixed", IO: IOPipe}, Status: StatusRed}
	if err := WriteCaseCaptures(dir, r, node, goCap); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "cases", "h", "multi", "default", "pipe", "fixed")
	for _, f := range []string{"node.stdout", "node.stderr", "node.exit", "go.stdout", "go.stderr", "go.exit"} {
		if _, err := os.Stat(filepath.Join(base, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if raw, _ := os.ReadFile(filepath.Join(base, "go.exit")); string(raw) != "1\n" {
		t.Errorf("go.exit = %q", raw)
	}
	if _, err := os.Stat(filepath.Join(base, "node.tty")); !os.IsNotExist(err) {
		t.Errorf("pipe case has node.tty")
	}

	rt := Result{Case: Case{ID: "help/single/default/tty/fixed", IO: IOTTY}, Status: StatusGreen}
	ttyCap := SideCapture{TTY: []byte("x\r\n"), Exit: 0}
	if err := WriteCaseCaptures(dir, rt, ttyCap, ttyCap); err != nil {
		t.Fatal(err)
	}
	tbase := filepath.Join(dir, "cases", "help", "single", "default", "tty", "fixed")
	for _, f := range []string{"node.tty", "go.tty", "node.exit", "go.exit"} {
		if _, err := os.Stat(filepath.Join(tbase, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if _, err := os.Stat(filepath.Join(tbase, "node.stdout")); !os.IsNotExist(err) {
		t.Errorf("tty case has node.stdout")
	}
}
