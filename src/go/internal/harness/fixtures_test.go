package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureRoot builds <tmp>/harness/fixtures/ with the given aliases; each
// entry is alias -> unconfirmed flag of its single claude/daily.json fixture.
func fixtureRoot(t *testing.T, aliases map[string]bool) string {
	t.Helper()
	root := t.TempDir()
	for alias, unconfirmed := range aliases {
		dir := filepath.Join(root, "harness", "fixtures", alias)
		m := &Manifest{
			Schema:  SchemaVersion,
			Machine: alias,
			Fixtures: []Fixture{{
				Source:      "claude",
				Period:      "daily",
				Args:        []string{"--json"},
				File:        "claude/daily.json",
				Unconfirmed: unconfirmed,
			}},
		}
		if err := WriteManifest(dir, m); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestResolveFixtures(t *testing.T) {
	t.Run("hostname alias present", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{"myhost": false, PlaceholderAlias: true})
		got, err := ResolveFixtures(root, nil, false, "myhost")
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(root, "harness", "fixtures", "myhost") + ":" + filepath.Join(root, "harness", "fixtures", PlaceholderAlias)
		if strings.Join(got, ":") != want {
			t.Errorf("got %v", got)
		}
	})
	t.Run("hostname alias absent", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{PlaceholderAlias: true})
		got, err := ResolveFixtures(root, nil, false, "nohost")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{filepath.Join(root, "harness", "fixtures", PlaceholderAlias)}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("placeholder only", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{"myhost": false, PlaceholderAlias: true})
		got, err := ResolveFixtures(root, nil, true, "myhost")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || filepath.Base(got[0]) != PlaceholderAlias {
			t.Errorf("got %v", got)
		}
	})
	t.Run("explicit list", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{"a": false, "b": true, PlaceholderAlias: true})
		got, err := ResolveFixtures(root, []string{"b", "a"}, false, "myhost")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || filepath.Base(got[0]) != "b" || filepath.Base(got[1]) != "a" {
			t.Errorf("got %v", got)
		}
	})
	t.Run("explicit alias without manifest", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{PlaceholderAlias: true})
		if err := os.MkdirAll(filepath.Join(root, "harness", "fixtures", "bare"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := ResolveFixtures(root, []string{"bare"}, false, "")
		if err == nil || !strings.Contains(err.Error(), `"bare"`) || !strings.Contains(err.Error(), "manifest.json") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("explicit alias with separator", func(t *testing.T) {
		root := fixtureRoot(t, map[string]bool{PlaceholderAlias: true})
		if _, err := ResolveFixtures(root, []string{"../escape"}, false, ""); err == nil {
			t.Error("accepted alias with separator")
		}
	})
}

func TestUnconfirmedReplays(t *testing.T) {
	root := fixtureRoot(t, map[string]bool{PlaceholderAlias: true, "myhost": false})
	aliases, err := ResolveFixtures(root, nil, false, "myhost")
	if err != nil {
		t.Fatal(err)
	}

	write := func(lines ...string) string {
		path := filepath.Join(t.TempDir(), "calls.jsonl")
		if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("unconfirmed fixture replayed", func(t *testing.T) {
		log := write(
			`{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"/x","matched":"_placeholder/claude/daily.json"}`,
			`{"tool":"git","argv":["push"],"cwd":"/x"}`,
		)
		got, err := UnconfirmedReplays(log, aliases)
		if err != nil || !got {
			t.Errorf("got %v, %v; want true, nil", got, err)
		}
	})
	t.Run("confirmed fixture replayed", func(t *testing.T) {
		log := write(`{"tool":"ccusage","argv":["claude","daily","--json"],"cwd":"/x","matched":"myhost/claude/daily.json"}`)
		got, err := UnconfirmedReplays(log, aliases)
		if err != nil || got {
			t.Errorf("got %v, %v; want false, nil", got, err)
		}
	})
	t.Run("missing log", func(t *testing.T) {
		got, err := UnconfirmedReplays(filepath.Join(t.TempDir(), "nope.jsonl"), aliases)
		if err != nil || got {
			t.Errorf("got %v, %v; want false, nil", got, err)
		}
	})
	t.Run("empty matched", func(t *testing.T) {
		log := write(`{"tool":"git","argv":["status"],"cwd":"/x"}`)
		got, err := UnconfirmedReplays(log, aliases)
		if err != nil || got {
			t.Errorf("got %v, %v; want false, nil", got, err)
		}
	})
}
