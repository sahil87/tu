package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMatrix(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadMatrixValid(t *testing.T) {
	path := writeMatrix(t, `{"schema":1,"cases":[{"id":"x","args":[],"conf":["single","multi"]}]}`)
	m, err := LoadMatrix(path)
	if err != nil {
		t.Fatalf("LoadMatrix: %v", err)
	}
	if len(m.Cases) != 1 || m.Cases[0].ID != "x" {
		t.Errorf("unexpected matrix: %+v", m)
	}
}

// R13: the env axis accepts the scripted-git failure values.
func TestLoadMatrixEnvAxisValues(t *testing.T) {
	path := writeMatrix(t, `{"schema":1,"cases":[{"id":"sync","args":["sync"],"env":["default","pullfail","pushfail","dirty"]}]}`)
	m, err := LoadMatrix(path)
	if err != nil {
		t.Fatalf("LoadMatrix: %v", err)
	}
	cases := Expand(m)
	want := []string{
		"sync/single/default/pipe/fixed",
		"sync/single/pullfail/pipe/fixed",
		"sync/single/pushfail/pipe/fixed",
		"sync/single/dirty/pipe/fixed",
	}
	if len(cases) != len(want) {
		t.Fatalf("Expand produced %d cases, want %d", len(cases), len(want))
	}
	for i, c := range cases {
		if c.ID != want[i] {
			t.Errorf("cases[%d].ID = %q, want %q", i, c.ID, want[i])
		}
	}
}

func TestLoadMatrixValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string // substrings of the error
	}{
		{"bad schema", `{"schema":2,"cases":[]}`, []string{"schema"}},
		{"empty id", `{"schema":1,"cases":[{"id":"","args":[]}]}`, []string{"empty id"}},
		{"duplicate id", `{"schema":1,"cases":[{"id":"x","args":[]},{"id":"x","args":[]}]}`, []string{`duplicate case id "x"`}},
		{"id not a path component", `{"schema":1,"cases":[{"id":"a/b","args":[]}]}`, []string{`case "a/b"`, "single path component"}},
		{"missing args", `{"schema":1,"cases":[{"id":"x"}]}`, []string{`case "x"`, "args"}},
		{"unknown key", `{"schema":1,"cases":[{"id":"x","args":[],"colour":["red"]}]}`, []string{"colour"}},
		{"invalid axis value", `{"schema":1,"cases":[{"id":"snap","args":[],"env":["loud"]}]}`, []string{`"snap"`, "env", `"loud"`}},
		{"duplicate axis value", `{"schema":1,"cases":[{"id":"snap","args":[],"io":["tty","tty"]}]}`, []string{`"snap"`, "io", `"tty"`}},
		{"unreadable", ``, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.body
			if tt.name != "unreadable" {
				path = writeMatrix(t, tt.body)
			}
			_, err := LoadMatrix(path)
			if err == nil {
				t.Fatalf("LoadMatrix succeeded, want error")
			}
			for _, sub := range tt.want {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q lacks %q", err.Error(), sub)
				}
			}
		})
	}
}

// R4: expansion is nested conf → env → io → tz with base values for omitted
// axes, and IDs are "<id>/<conf>/<env>/<io>/<tz>".
func TestExpandOrderAndIDs(t *testing.T) {
	m := &Matrix{Schema: 1, Cases: []CaseGroup{
		{ID: "x", Args: []string{}, Conf: []string{"single", "multi"}, IO: []string{"pipe", "tty"}},
	}}
	cases := Expand(m)
	want := []string{
		"x/single/default/pipe/fixed",
		"x/single/default/tty/fixed",
		"x/multi/default/pipe/fixed",
		"x/multi/default/tty/fixed",
	}
	if len(cases) != len(want) {
		t.Fatalf("Expand produced %d cases, want %d", len(cases), len(want))
	}
	for i, c := range cases {
		if c.ID != want[i] {
			t.Errorf("cases[%d].ID = %q, want %q", i, c.ID, want[i])
		}
		if c.Group != "x" || c.Conf+"/"+c.Env+"/"+c.IO+"/"+c.TZ != strings.SplitN(c.ID, "/", 2)[1] {
			t.Errorf("cases[%d] axis fields disagree with ID %q", i, c.ID)
		}
	}
}

func TestExpandBaseDefaults(t *testing.T) {
	m := &Matrix{Schema: 1, Cases: []CaseGroup{{ID: "y", Args: []string{"h"}}}}
	cases := Expand(m)
	if len(cases) != 1 {
		t.Fatalf("Expand produced %d cases, want 1", len(cases))
	}
	c := cases[0]
	if c.ID != "y/single/default/pipe/fixed" {
		t.Errorf("ID = %q", c.ID)
	}
	if c.Conf != ConfSingle || c.Env != EnvDefault || c.IO != IOPipe || c.TZ != TZFixed {
		t.Errorf("axes = %+v", c)
	}
	if len(c.Args) != 1 || c.Args[0] != "h" {
		t.Errorf("Args = %v", c.Args)
	}
}

func TestFilter(t *testing.T) {
	m := &Matrix{Schema: 1, Cases: []CaseGroup{
		{ID: "snapshot-all", Args: []string{}, Conf: []string{"single", "multi"}},
		{ID: "version", Args: []string{"--version"}},
	}}
	cases := Expand(m)
	if got := Filter(cases, ""); len(got) != 3 {
		t.Errorf("empty filter kept %d cases, want 3", len(got))
	}
	got := Filter(cases, "snapshot-all/multi")
	if len(got) != 1 || got[0].ID != "snapshot-all/multi/default/pipe/fixed" {
		t.Errorf("filtered = %+v", got)
	}
	if got := Filter(cases, "zzz"); len(got) != 0 {
		t.Errorf("no-match filter kept %d cases", len(got))
	}
}

// R5: the committed matrix loads, expands into the agreed bounds (200..600 —
// the leaderboard row grew the matrix to 420; the rail still catches an
// accidental axis cross-product explosion), and covers every axis value at
// least once.
func TestCommittedMatrix(t *testing.T) {
	m, err := LoadMatrix("../../../../harness/matrix.json")
	if err != nil {
		t.Fatalf("committed matrix: %v", err)
	}
	cases := Expand(m)
	if len(cases) < 200 || len(cases) > 600 {
		t.Errorf("expanded to %d cases, want 200..600", len(cases))
	}
	present := map[string]bool{}
	axisSeen := map[string]bool{}
	for _, c := range cases {
		present[c.ID] = true
		axisSeen[c.Conf] = true
		axisSeen[c.Env] = true
		axisSeen[c.IO] = true
		axisSeen[c.TZ] = true
	}
	if !present["version/single/default/pipe/fixed"] {
		t.Errorf("version/single/default/pipe/fixed missing")
	}
	for _, v := range []string{
		ConfSingle, ConfMulti, ConfOrg, ConfLegacy,
		EnvDefault, EnvNoColor, EnvEnvrepo,
		IOPipe, IOTTY,
		TZFixed, TZAlt,
	} {
		if !axisSeen[v] {
			t.Errorf("axis value %q never occurs", v)
		}
	}
	// No update group, and --watch only in the deterministic watch-family
	// cases (the exit-2 incompatibilities and interval bounds — B7; no live
	// -w session case: a frame is time- and RNG-dependent, see the intake).
	watchGroups := map[string]bool{
		"watch-json": true, "watch-csv": true, "watch-md": true,
		"watch-interval-min": true, "watch-interval-max": true, "watch-interval-nan": true,
	}
	for _, g := range m.Cases {
		for _, a := range g.Args {
			if a == "update" {
				t.Errorf("group %q uses update", g.ID)
			}
			if (a == "--watch" || a == "-w") && !watchGroups[g.ID] {
				t.Errorf("group %q uses %s outside the deterministic watch-family cases", g.ID, a)
			}
		}
	}
}
