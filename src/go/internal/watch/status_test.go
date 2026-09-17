package watch

import (
	"strings"
	"testing"

	"github.com/sahil87/tu/internal/render/ansi"
)

func TestFooter(t *testing.T) {
	on := ansi.Colors{Enabled: true}
	off := ansi.Colors{}

	cases := []struct {
		name       string
		countdown  int
		refreshing bool
		width      int
		want       string // visible (stripped) text
	}{
		{"countdown wide", 10, false, 80, "Next refresh: 10s · ↵ refresh · q quit"},
		{"countdown one digit", 9, false, 80, "Next refresh: 9s · ↵ refresh · q quit"},
		{"refreshing", 0, true, 80, "Refreshing... · ↵ refresh · q quit"},
		// "Next refresh: 45s · ↵ refresh · q quit" is 39 visible — over 30, drop controls.
		{"width 30 drops controls", 45, false, 30, "Next refresh: 45s"},
		// Still fits at 39.
		{"width 39 keeps controls", 45, false, 39, "Next refresh: 45s · ↵ refresh · q quit"},
		// "Refreshing..." is 13 — over 10: only the status text remains.
		{"width 20 refreshing drops controls", 0, true, 20, "Refreshing..."}, // 13+3+17=33 > 20 → drop
		{"width 10 keeps status alone", 45, false, 10, "Next refresh: 45s"},  // 17 > 10 but one part left
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ansi.StripANSI(Footer(tc.countdown, tc.refreshing, tc.width, on))
			if got != tc.want {
				t.Errorf("visible = %q, want %q", got, tc.want)
			}
			plain := Footer(tc.countdown, tc.refreshing, tc.width, off)
			if plain != tc.want {
				t.Errorf("no-color = %q, want %q", plain, tc.want)
			}
		})
	}
}

// The colored form: every part is dim-wrapped, the joiner included.
func TestFooterColoredBytes(t *testing.T) {
	on := ansi.Colors{Enabled: true}
	got := Footer(10, false, 80, on)
	want := "\x1b[2mNext refresh: 10s\x1b[0m\x1b[2m · \x1b[0m\x1b[2m↵ refresh · q quit\x1b[0m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !strings.HasPrefix(Footer(0, true, 80, on), "\x1b[2mRefreshing...\x1b[0m") {
		t.Errorf("refreshing form wrong: %q", Footer(0, true, 80, on))
	}
}
