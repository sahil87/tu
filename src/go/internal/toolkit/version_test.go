package toolkit

import "testing"

// R3's table: the retired TypeScript implementation printed `v${VERSION}` from
// its package metadata's version field, which DisplayVersion reproduces for
// every stamped build; an unstamped "dev" stays bare.
func TestVersionHelpers(t *testing.T) {
	cases := []struct {
		in, bare, display, line string
	}{
		{"v0.11.5", "0.11.5", "v0.11.5", "tu version v0.11.5"},
		{"0.11.5", "0.11.5", "v0.11.5", "tu version v0.11.5"},
		{"dev", "dev", "dev", "tu version dev"},
		{"", "", "", "tu version "},
	}
	for _, c := range cases {
		if got := BareVersion(c.in); got != c.bare {
			t.Errorf("BareVersion(%q) = %q, want %q", c.in, got, c.bare)
		}
		if got := DisplayVersion(c.in); got != c.display {
			t.Errorf("DisplayVersion(%q) = %q, want %q", c.in, got, c.display)
		}
		if got := VersionLine(c.in); got != c.line {
			t.Errorf("VersionLine(%q) = %q, want %q", c.in, got, c.line)
		}
	}
}
