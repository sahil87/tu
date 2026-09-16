package fact

import (
	"reflect"
	"testing"
)

func TestToolsOrderAndShape(t *testing.T) {
	want := []Tool{
		{Key: "cc", Name: "Claude Code"},
		{Key: "codex", Name: "Codex"},
		{Key: "oc", Name: "OpenCode"},
		{Key: "gemini", Name: "Gemini"},
		{Key: "copilot", Name: "Copilot"},
		{Key: "kimi", Name: "Kimi"},
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
	if tool != (Tool{Key: "gemini", Name: "Gemini"}) {
		t.Errorf("Lookup(gemini) = %+v", tool)
	}

	for _, alias := range []string{"gem", "co", "cop", "ki", ""} {
		if _, ok := Lookup(alias); ok {
			t.Errorf("Lookup(%q) should miss (aliases are not in the registry)", alias)
		}
	}
}
