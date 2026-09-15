package harness

import (
	"testing"
)

// The R7 scenario: home-rooted and generic-root paths are replaced with
// stable first-seen numbering; the encoded `-home-…` form and non-path
// strings are untouched.
func TestRedactScenario(t *testing.T) {
	in := `{"a":"/home/sahil/code/x","b":"/home/sahil/code/x","c":"/Users/bob/work/y","d":"-home-sahil-code-x","e":"gpt-5"}`
	want := `{"a":"~/redacted-1","b":"~/redacted-1","c":"~/redacted-2","d":"-home-sahil-code-x","e":"gpt-5"}`
	out, n := Redact([]byte(in), "/home/sahil")
	if string(out) != want {
		t.Errorf("Redact output:\n got %s\nwant %s", out, want)
	}
	if n != 3 {
		t.Errorf("Redact count = %d, want 3", n)
	}
}

// Numbering is per distinct path in first-seen order within one call, and the
// remainder of a path runs to the closing quote.
func TestRedactDistinctPaths(t *testing.T) {
	in := `{"a":"/Users/x/work/secret","b":"/Users/x/work/other"}`
	out, n := Redact([]byte(in), "/home/nobody")
	want := `{"a":"~/redacted-1","b":"~/redacted-2"}`
	if string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

// The capturing user's own home is a redaction root even when it lies outside
// /home and /Users (e.g. /root or a custom prefix).
func TestRedactCustomHome(t *testing.T) {
	in := `{"a":"/srv/home/u/proj","b":"/home/other/proj"}`
	out, n := Redact([]byte(in), "/srv/home/u")
	want := `{"a":"~/redacted-1","b":"~/redacted-2"}`
	if string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestRedactNoOp(t *testing.T) {
	in := `{"daily":[{"date":"2026-09-08","modelBreakdowns":[{"modelName":"claude-fable-5-1"}]}],"host":"dev-ws-sahil02"}`
	out, n := Redact([]byte(in), "/home/sahil")
	if string(out) != in {
		t.Errorf("no-path input changed:\n got %s\nwant %s", out, in)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}

// home="" must not turn the empty string into a redaction root (which would
// match everywhere); the generic roots still apply.
func TestRedactEmptyHome(t *testing.T) {
	in := `{"a":"/home/sahil/code/x"}`
	out, n := Redact([]byte(in), "")
	want := `{"a":"~/redacted-1"}`
	if string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
	if n != 1 {
		t.Errorf("count = %d, want 1", n)
	}

	plain := `{"a":"nothing to do"}`
	out, n = Redact([]byte(plain), "")
	if string(out) != plain || n != 0 {
		t.Errorf("empty home altered plain input: got %s (%d)", out, n)
	}
}

// The custom home is a fixed prefix, so it must bind at a path-component
// boundary: /srv/home/u redacts itself and its subtree but must not touch
// the longer sibling /srv/home/user.
func TestRedactCustomHomeBoundary(t *testing.T) {
	in := `{"a":"/srv/home/u","b":"/srv/home/u/proj","c":"/srv/home/user/secret"}`
	out, n := Redact([]byte(in), "/srv/home/u")
	want := `{"a":"~/redacted-1","b":"~/redacted-2","c":"/srv/home/user/secret"}`
	if string(out) != want {
		t.Errorf("got %s, want %s", out, want)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}
