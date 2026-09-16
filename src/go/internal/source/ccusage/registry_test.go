package ccusage

import (
	"reflect"
	"testing"
)

func TestToolsOrderAndShape(t *testing.T) {
	want := []Tool{
		{Key: "cc", Name: "Claude Code", PrefixArgs: []string{"claude"}, LabelKey: "date"},
		{Key: "codex", Name: "Codex", PrefixArgs: []string{"codex"}, LabelKey: "date"},
		{Key: "oc", Name: "OpenCode", PrefixArgs: []string{"opencode"}, LabelKey: "date"},
		{Key: "gemini", Name: "Gemini", PrefixArgs: []string{"gemini"}, LabelKey: "date"},
		{Key: "copilot", Name: "Copilot", PrefixArgs: []string{"copilot"}, LabelKey: "date"},
		{Key: "kimi", Name: "Kimi", PrefixArgs: []string{"kimi"}, LabelKey: "date"},
	}
	if !reflect.DeepEqual(Tools, want) {
		t.Errorf("Tools = %+v, want %+v", Tools, want)
	}
}

func TestLookup(t *testing.T) {
	tool, ok := Lookup("gemini")
	if !ok {
		t.Fatal("Lookup(gemini) missed")
	}
	if tool.Name != "Gemini" || !reflect.DeepEqual(tool.PrefixArgs, []string{"gemini"}) || tool.LabelKey != "date" {
		t.Errorf("Lookup(gemini) = %+v", tool)
	}

	for _, alias := range []string{"gem", "co", "cop", "ki", ""} {
		if _, ok := Lookup(alias); ok {
			t.Errorf("Lookup(%q) should miss (aliases are not in the registry)", alias)
		}
	}
}

func TestArgvComposition(t *testing.T) {
	cc := Tools[0]
	if got := argv(cc, PeriodDaily, nil); !reflect.DeepEqual(got, []string{"claude", "daily", "--json"}) {
		t.Errorf("argv(cc, daily, nil) = %v", got)
	}
	if got := argv(cc, "monthly", []string{"--since", "20260101"}); !reflect.DeepEqual(got, []string{"claude", "monthly", "--json", "--since", "20260101"}) {
		t.Errorf("argv with extra args = %v", got)
	}
	// The returned slice must not alias the registry's PrefixArgs.
	got := argv(cc, PeriodDaily, nil)
	got[0] = "mutated"
	if Tools[0].PrefixArgs[0] != "claude" {
		t.Error("argv aliased tool.PrefixArgs")
	}
}
