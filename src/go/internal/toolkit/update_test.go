package toolkit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"testing"
	"time"
)

// fakeBrew records calls and replays scripted results — the seam that makes
// the Homebrew sequence unit-testable without brew (the test binary never
// lives under /Cellar/tu/).
type fakeBrew struct {
	calls      []string
	updateErr  error
	infoOut    []byte
	infoErr    error
	upgradeErr error
	upgradeOut string // written to the passed stdout, as a real brew would
}

func (f *fakeBrew) Update(ctx context.Context) error {
	f.calls = append(f.calls, "update")
	return f.updateErr
}

func (f *fakeBrew) Info(ctx context.Context) ([]byte, error) {
	f.calls = append(f.calls, "info")
	return f.infoOut, f.infoErr
}

func (f *fakeBrew) Upgrade(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	f.calls = append(f.calls, "upgrade")
	if f.upgradeOut != "" {
		io.WriteString(stdout, f.upgradeOut)
	}
	return f.upgradeErr
}

func infoJSON(stable string) []byte {
	return []byte(`{"formulae":[{"versions":{"stable":"` + stable + `"}}]}`)
}

func TestCheckLatestSequence(t *testing.T) {
	ctx := context.Background()

	t.Run("skip flag skips Update", func(t *testing.T) {
		f := &fakeBrew{infoOut: infoJSON("0.11.6")}
		latest, uerr := CheckLatest(ctx, f, true)
		if uerr != nil {
			t.Fatalf("CheckLatest = %v", uerr)
		}
		if latest != "0.11.6" {
			t.Errorf("latest = %q, want 0.11.6", latest)
		}
		if !slices.Equal(f.calls, []string{"info"}) {
			t.Errorf("calls = %v, want [info] (Update skipped)", f.calls)
		}
	})

	t.Run("Update precedes Info without the flag", func(t *testing.T) {
		f := &fakeBrew{infoOut: infoJSON("0.11.6")}
		if _, uerr := CheckLatest(ctx, f, false); uerr != nil {
			t.Fatalf("CheckLatest = %v", uerr)
		}
		if !slices.Equal(f.calls, []string{"update", "info"}) {
			t.Errorf("calls = %v, want [update info]", f.calls)
		}
	})

	t.Run("Update error maps to its line", func(t *testing.T) {
		f := &fakeBrew{updateErr: errors.New("boom")}
		_, uerr := CheckLatest(ctx, f, false)
		if uerr == nil || uerr.Message != "Error: could not check for updates (brew update failed). Check your network connection." {
			t.Errorf("CheckLatest = %v", uerr)
		}
		if !slices.Equal(f.calls, []string{"update"}) {
			t.Errorf("calls = %v, want [update] (Info never runs)", f.calls)
		}
	})

	t.Run("Info failures map to could-not-determine", func(t *testing.T) {
		cases := []struct {
			name string
			out  []byte
			err  error
		}{
			{"exec error", nil, errors.New("boom")},
			{"bad JSON", []byte("not json"), nil},
			{"no formulae", []byte(`{"formulae":[]}`), nil},
			{"empty stable", infoJSON(""), nil},
			{"whitespace stable", infoJSON("  "), nil},
			{"non-string stable", []byte(`{"formulae":[{"versions":{"stable":5}}]}`), nil},
			{"null stable", []byte(`{"formulae":[{"versions":{"stable":null}}]}`), nil},
		}
		for _, c := range cases {
			f := &fakeBrew{infoOut: c.out, infoErr: c.err}
			_, uerr := CheckLatest(ctx, f, true)
			if uerr == nil || uerr.Message != "Error: could not determine latest version." {
				t.Errorf("%s: CheckLatest = %v", c.name, uerr)
			}
		}
	})
}

// TestHomebrewSequence composes the toolkit pieces exactly as cmd/tu's
// runUpdate Homebrew branch does, against the fake driver (R7's fake-Brew
// scenario).
func TestHomebrewSequence(t *testing.T) {
	ctx := context.Background()

	t.Run("up to date short-circuits before Upgrade", func(t *testing.T) {
		f := &fakeBrew{infoOut: infoJSON("0.11.5")}
		lines := []string{CurrentVersionLine("v0.11.5")}
		latest, uerr := CheckLatest(ctx, f, true)
		if uerr != nil {
			t.Fatalf("CheckLatest = %v", uerr)
		}
		if !UpToDate("v0.11.5", latest) {
			t.Fatal("UpToDate = false, want true")
		}
		lines = append(lines, AlreadyUpToDateLine("v0.11.5"))
		if !slices.Equal(f.calls, []string{"info"}) {
			t.Errorf("calls = %v, want [info] (no Update with the skip flag, no Upgrade)", f.calls)
		}
		want := []string{"Current version: v0.11.5", "Already up to date (v0.11.5)."}
		if !slices.Equal(lines, want) {
			t.Errorf("lines = %v, want %v", lines, want)
		}
	})

	t.Run("upgrade success passes streams through", func(t *testing.T) {
		f := &fakeBrew{infoOut: infoJSON("0.11.6"), upgradeOut: "==> Upgrading tu\n"}
		latest, uerr := CheckLatest(ctx, f, true)
		if uerr != nil {
			t.Fatalf("CheckLatest = %v", uerr)
		}
		if UpToDate("v0.11.5", latest) {
			t.Fatal("UpToDate = true, want false")
		}
		if got, want := UpdatingLine("v0.11.5", latest), "Updating v0.11.5 → v0.11.6..."; got != want {
			t.Errorf("UpdatingLine = %q, want %q", got, want)
		}
		var stdout, stderr bytes.Buffer
		if uerr := Upgrade(ctx, f, nil, &stdout, &stderr); uerr != nil {
			t.Fatalf("Upgrade = %v", uerr)
		}
		if stdout.String() != "==> Upgrading tu\n" {
			t.Errorf("passed stdout writer did not receive the fake's output: %q", stdout.String())
		}
		if got, want := UpdatedLine(latest), "Updated to v0.11.6."; got != want {
			t.Errorf("UpdatedLine = %q, want %q", got, want)
		}
		if !slices.Equal(f.calls, []string{"info", "upgrade"}) {
			t.Errorf("calls = %v, want [info upgrade]", f.calls)
		}
	})

	t.Run("upgrade failure maps to its line", func(t *testing.T) {
		f := &fakeBrew{upgradeErr: errors.New("boom")}
		uerr := Upgrade(ctx, f, nil, io.Discard, io.Discard)
		if uerr == nil || uerr.Message != "Error: brew upgrade failed." {
			t.Errorf("Upgrade = %v", uerr)
		}
	})
}

func TestUpToDate(t *testing.T) {
	if !UpToDate("v0.11.5", "0.11.5") {
		t.Error(`UpToDate("v0.11.5", "0.11.5") = false`)
	}
	if UpToDate("v0.11.5", "0.11.6") {
		t.Error(`UpToDate("v0.11.5", "0.11.6") = true`)
	}
}

func TestIsBrewInstall(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/opt/homebrew/Cellar/tu/0.11.5/bin/tu", true},
		{"/usr/local/Cellar/tu/0.11.5/bin/tu", true},
		{"/usr/local/bin/tu", false},
		{"/home/user/.local/bin/tu", false},
		{"/opt/homebrew/Cellar/other/1.0/bin/other", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsBrewInstall(c.path); got != c.want {
			t.Errorf("IsBrewInstall(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestNotBrewInstallLines(t *testing.T) {
	want := []string{
		"tu v0.11.5 was not installed via Homebrew.",
		"Update manually, or reinstall with: brew install sahil87/tap/tu",
	}
	if got := NotBrewInstallLines("v0.11.5"); !slices.Equal(got, want) {
		t.Errorf("NotBrewInstallLines = %v, want %v", got, want)
	}
	// An unstamped dev build prints the bare form (A-018).
	if got := NotBrewInstallLines("dev"); got[0] != "tu dev was not installed via Homebrew." {
		t.Errorf("NotBrewInstallLines(dev) = %v", got)
	}
}

// TestBrewCommandShapes pins the R8 safety contract on the command
// constructors: the bounded metadata commands terminate gracefully (SIGTERM +
// 10 s WaitDelay), the upgrade command has no deadline machinery and carries
// HOMEBREW_NO_ASK=1.
func TestBrewCommandShapes(t *testing.T) {
	bounded := boundedBrewCmd(context.Background(), "update", "--quiet")
	if !slices.Equal(bounded.Args, []string{"brew", "update", "--quiet"}) {
		t.Errorf("bounded Args = %v", bounded.Args)
	}
	if bounded.Cancel == nil {
		t.Error("bounded Cancel = nil, want the SIGTERM override")
	}
	if bounded.WaitDelay != 10*time.Second {
		t.Errorf("bounded WaitDelay = %v, want 10s", bounded.WaitDelay)
	}

	info := boundedBrewCmd(context.Background(), "info", "--json=v2", "tu")
	if !slices.Equal(info.Args, []string{"brew", "info", "--json=v2", "tu"}) {
		t.Errorf("info Args = %v", info.Args)
	}

	upgrade := upgradeBrewCmd()
	if !slices.Equal(upgrade.Args, []string{"brew", "upgrade", "tu"}) {
		t.Errorf("upgrade Args = %v, want [brew upgrade tu]", upgrade.Args)
	}
	if !slices.Contains(upgrade.Env, "HOMEBREW_NO_ASK=1") {
		t.Error("upgrade Env lacks HOMEBREW_NO_ASK=1")
	}
	if upgrade.Cancel != nil {
		t.Error("upgrade Cancel != nil, want nil (no graceful-kill override, no deadline)")
	}
	if upgrade.WaitDelay != 0 {
		t.Errorf("upgrade WaitDelay = %v, want 0", upgrade.WaitDelay)
	}
}
