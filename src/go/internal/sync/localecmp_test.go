package sync

import (
	"slices"
	"testing"
)

// Every table in this file was verified against Node v24 (2026-09-17) with
// String.prototype.localeCompare (ICU root collation) and the default
// Array.prototype.sort; run `node -e` on the same inputs to re-verify.

// R11: the full repertoire sort order, Node-verified:
// ["_","-",".","/","0","9","a","A","b","B","m","M","z","Z"] is already sorted
// under localeCompare.
func TestLocaleCompareRepertoireOrder(t *testing.T) {
	sorted := []string{"_", "-", ".", "/", "0", "9", "a", "A", "b", "B", "m", "M", "z", "Z"}
	shuffled := []string{"b", "B", "a", "A", "0", "9", "_", "-", ".", "/", "m", "M", "z", "Z"}
	slices.SortFunc(shuffled, localeCompare)
	if !equalStrings(shuffled, sorted) {
		t.Errorf("sorted = %v, want %v", shuffled, sorted)
	}
}

// R11: pairwise orders, each Node-verified (a localeCompare b < 0).
func TestLocaleComparePairs(t *testing.T) {
	pairs := []struct{ a, b string }{
		// Punctuation before digits before letters.
		{"_", "-"}, {"-", "."}, {".", "/"}, {"/", "0"}, {".", "9"}, {"9", "a"}, {"0", "A"},
		// Letters case-insensitively at the primary level.
		{"a", "Z"}, {"z", "Z"}, {"a", "b"}, {"A", "B"},
		// Tertiary: lowercase before uppercase.
		{"a", "A"}, {"b", "B"}, {"mac", "Mac"}, {"sahil", "Sahil"}, {"aB", "Ab"},
		// Primary differences anywhere outrank an earlier tertiary one.
		{"aa", "Az"}, {"aB", "Az"},
		// Punctuation/digits before letters mid-string.
		{"a-a", "aa"}, {"a_a", "a-a"}, {"a-a", "a.a"}, {"a.a", "a/a"},
		{"a1", "aa"}, {"a-1", "aa"}, {"a0", "a9"},
		// No numeric collation: '1' < '9' at the first digit.
		{"10", "9"},
		// Prefix sorts first.
		{"cc-2026-01-01.jsonl", "cc-2026-01-01b.jsonl"},
		{"u/2026/m/cc-2026-01-01.jsonl", "u/2026/m/cc-2026-01-02.jsonl"},
		{"dev-ws", "devws"},
	}
	for _, p := range pairs {
		if got := localeCompare(p.a, p.b); got >= 0 {
			t.Errorf("localeCompare(%q, %q) = %d, want < 0", p.a, p.b, got)
		}
		if got := localeCompare(p.b, p.a); got <= 0 {
			t.Errorf("localeCompare(%q, %q) = %d, want > 0", p.b, p.a, got)
		}
	}
	if got := localeCompare("same", "same"); got != 0 {
		t.Errorf("localeCompare(same, same) = %d, want 0", got)
	}
}

// R11: the observable real-repo case — ICU sorts Sahils-Mac-mini.local AFTER
// dev-ws-sahil02; byte order would put it first.
func TestLocaleCompareRealRepoPaths(t *testing.T) {
	got := []string{
		"sahil/2026/Sahils-Mac-mini.local/cc-2026-04-26.jsonl",
		"sahil/2026/devws/cc-2026-05-01.jsonl",
		"sahil/2026/dev-ws-sahil02/cc-2026-04-24.jsonl",
		"bob/2026/laptop/cc-2026-04-24.jsonl",
	}
	slices.SortFunc(got, localeCompare)
	want := []string{
		"bob/2026/laptop/cc-2026-04-24.jsonl",
		"sahil/2026/dev-ws-sahil02/cc-2026-04-24.jsonl",
		"sahil/2026/devws/cc-2026-05-01.jsonl",
		"sahil/2026/Sahils-Mac-mini.local/cc-2026-04-26.jsonl",
	}
	if !equalStrings(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
	if slices.IsSorted(got) { // plain byte order would sort Sahils first
		t.Error("byte order also sorts these — the fixture stopped distinguishing ICU from bytes")
	}
}

// R11: the per-user block is [...byUser.entries()].sort() — the default JS
// sort on "{user},[object Object]". Node-verified: ["Zed","alice","bob",
// "sahil"] (byte order) and "a!" before "a" (the ',' in the decoration sorts
// after '!').
func TestUserEntryCompare(t *testing.T) {
	users := []string{"sahil", "bob", "Zed", "alice"}
	slices.SortFunc(users, userEntryCompare)
	if want := []string{"Zed", "alice", "bob", "sahil"}; !equalStrings(users, want) {
		t.Errorf("sorted = %v, want %v", users, want)
	}
	if got := userEntryCompare("a!", "a"); got >= 0 {
		t.Errorf("userEntryCompare(a!, a) = %d, want < 0 (the \",...\" decoration decides)", got)
	}
}
