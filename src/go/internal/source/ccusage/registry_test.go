package ccusage

import (
	"reflect"
	"testing"

	"github.com/sahil87/tu/internal/fact"
)

func TestInvocationsMatchRegistry(t *testing.T) {
	if len(invocations) != len(fact.Tools) {
		t.Errorf("invocations has %d entries, fact.Tools has %d", len(invocations), len(fact.Tools))
	}
	for _, tool := range fact.Tools {
		if _, ok := invocations[tool.Key]; !ok {
			t.Errorf("invocations missing registry key %q", tool.Key)
		}
	}
	for key := range invocations {
		if _, ok := fact.Lookup(key); !ok {
			t.Errorf("invocations has orphan key %q (not in fact.Tools)", key)
		}
	}
	if invocations["cc"].prefixArgs[0] != "claude" {
		t.Errorf(`invocations["cc"].prefixArgs[0] = %q, want "claude"`, invocations["cc"].prefixArgs[0])
	}
}

func TestArgvComposition(t *testing.T) {
	cc, ok := fact.Lookup("cc")
	if !ok {
		t.Fatal(`fact.Lookup("cc") missed`)
	}
	if got := argv(cc, "daily", nil); !reflect.DeepEqual(got, []string{"claude", "daily", "--json"}) {
		t.Errorf("argv(cc, daily, nil) = %v", got)
	}
	if got := argv(cc, "monthly", []string{"--since", "20260101"}); !reflect.DeepEqual(got, []string{"claude", "monthly", "--json", "--since", "20260101"}) {
		t.Errorf("argv with extra args = %v", got)
	}
	// The returned slice must not alias the invocation's prefixArgs.
	got := argv(cc, "daily", nil)
	got[0] = "mutated"
	if invocations["cc"].prefixArgs[0] != "claude" {
		t.Error("argv aliased the invocation's prefixArgs")
	}
}
