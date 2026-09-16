package fact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAddIsPureAndFieldWise(t *testing.T) {
	a := Totals{TotalCost: 1, InputTokens: 2}
	b := Totals{TotalCost: 0.5, OutputTokens: 3}
	c := a.Add(b)

	want := Totals{TotalCost: 1.5, InputTokens: 2, OutputTokens: 3}
	if c != want {
		t.Errorf("a.Add(b) = %+v, want %+v", c, want)
	}
	if a != (Totals{TotalCost: 1, InputTokens: 2}) {
		t.Errorf("receiver mutated: %+v", a)
	}
	if b != (Totals{TotalCost: 0.5, OutputTokens: 3}) {
		t.Errorf("argument mutated: %+v", b)
	}

	full := Totals{0.1, 1, 2, 3, 4, 10}
	sum := full.Add(full)
	if sum != (Totals{0.2, 2, 4, 6, 8, 20}) {
		t.Errorf("full.Add(full) = %+v", sum)
	}
}

func TestIsZero(t *testing.T) {
	if !((Totals{}).IsZero()) {
		t.Error("Totals{}.IsZero() = false")
	}
	nonZero := []Totals{
		{TotalCost: 0.1},
		{InputTokens: 1},
		{OutputTokens: 1},
		{CacheCreationTokens: 1},
		{CacheReadTokens: 1},
		{TotalTokens: 1},
	}
	for _, tt := range nonZero {
		if tt.IsZero() {
			t.Errorf("%+v.IsZero() = true", tt)
		}
	}
	if (Totals{TotalCost: 1.5, InputTokens: 2, OutputTokens: 3}).IsZero() {
		t.Error("non-zero totals reported zero")
	}
}

func TestTotalsJSONTagsArePinned(t *testing.T) {
	raw, err := json.Marshal(Totals{TotalCost: 0.5, InputTokens: 3000})
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	for _, key := range []string{
		`"totalCost":0.5`,
		`"inputTokens":3000`,
		`"outputTokens":0`,
		`"cacheCreationTokens":0`,
		`"cacheReadTokens":0`,
		`"totalTokens":0`,
	} {
		if !strings.Contains(out, key) {
			t.Errorf("marshalled Totals %s missing %s", out, key)
		}
	}

	var round Totals
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatal(err)
	}
	if round != (Totals{TotalCost: 0.5, InputTokens: 3000}) {
		t.Errorf("round-trip = %+v", round)
	}
}
