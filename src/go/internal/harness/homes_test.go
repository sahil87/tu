package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// syntheticSeed builds a two-file seed tree in a temp dir.
func syntheticSeed(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"harness-user/2026/harness-machine/cc-2026-01-06.jsonl": "{\"label\":\"2026-01-06\"}\n",
		"docs/README.md": "seed\n",
	}
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// listFiles returns the slash-separated relative paths of every file under dir.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func TestStageHomeVariants(t *testing.T) {
	seed := syntheticSeed(t)
	seedFiles := []string{
		".tu/metrics_repo/docs/README.md",
		".tu/metrics_repo/harness-user/2026/harness-machine/cc-2026-01-06.jsonl",
	}
	tests := []struct {
		variant string
		want    []string
	}{
		{ConfSingle, nil},
		{ConfMulti, append([]string{".config/tu/tu.conf"}, seedFiles...)},
		{ConfOrg, append([]string{".config/tu/org.conf"}, seedFiles...)},
		{ConfLegacy, append([]string{".tu.conf"}, seedFiles...)},
	}
	for _, tt := range tests {
		t.Run(tt.variant, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "home")
			if err := StageHome(dir, tt.variant, seed); err != nil {
				t.Fatalf("StageHome: %v", err)
			}
			got := listFiles(t, dir)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("files =\n%s\nwant =\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

// R6: the three conf files are byte-identical; only the location varies.
func TestStageHomeConfBytesIdentical(t *testing.T) {
	seed := syntheticSeed(t)
	root := t.TempDir()
	variants := map[string]string{
		ConfMulti:  ".config/tu/tu.conf",
		ConfOrg:    ".config/tu/org.conf",
		ConfLegacy: ".tu.conf",
	}
	var ref []byte
	for variant, rel := range variants {
		dir := filepath.Join(root, variant)
		if err := StageHome(dir, variant, seed); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != HomeConfBody {
			t.Errorf("%s conf bytes differ from HomeConfBody", variant)
		}
		if ref == nil {
			ref = raw
		} else if string(raw) != string(ref) {
			t.Errorf("%s conf bytes differ across variants", variant)
		}
	}
	// legacy has no .config/, org has no tu.conf, single has nothing.
	legacy := filepath.Join(root, ConfLegacy)
	if _, err := os.Stat(filepath.Join(legacy, ".config")); !os.IsNotExist(err) {
		t.Errorf("legacy home has .config/")
	}
	org := filepath.Join(root, ConfOrg)
	if _, err := os.Stat(filepath.Join(org, ".config", "tu", "tu.conf")); !os.IsNotExist(err) {
		t.Errorf("org home has tu.conf")
	}
}

// R6: two stagings of one case (node/go) share no path.
func TestStageHomeTwoSides(t *testing.T) {
	seed := syntheticSeed(t)
	caseDir := t.TempDir()
	nodeHome := filepath.Join(caseDir, "node", "home")
	goHome := filepath.Join(caseDir, "go", "home")
	if err := StageHome(nodeHome, ConfMulti, seed); err != nil {
		t.Fatal(err)
	}
	if err := StageHome(goHome, ConfMulti, seed); err != nil {
		t.Fatal(err)
	}
	if nodeHome == goHome || strings.HasPrefix(nodeHome, goHome) || strings.HasPrefix(goHome, nodeHome) {
		t.Errorf("homes share a path: %q vs %q", nodeHome, goHome)
	}
	// Mutating one side must not affect the other.
	victim := filepath.Join(goHome, ".config", "tu", "tu.conf")
	if err := os.WriteFile(victim, []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(nodeHome, ".config", "tu", "tu.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != HomeConfBody {
		t.Errorf("node home conf changed when go home was mutated")
	}
}

func TestStageHomeUnknownVariant(t *testing.T) {
	if err := StageHome(t.TempDir(), "weird", t.TempDir()); err == nil {
		t.Fatal("StageHome accepted unknown variant")
	}
}

// R7: the committed seed holds exactly the listed files, and every .jsonl
// line decodes into the seven UsageEntry fields with label == filename date.
func TestMetricsRepoSeed(t *testing.T) {
	root := "../../../../harness/metrics-repo"
	want := []string{
		"docs/README.md",
		"harness-user/2026/harness-machine/cc-2026-01-05.jsonl",
		"harness-user/2026/harness-machine/cc-2026-01-06.jsonl",
		"harness-user/2026/other-box/cc-2026-01-06.jsonl",
		"harness-user/2026/other-box/codex-2026-01-07.jsonl",
		"other-user/2026/laptop/cc-2026-01-05.jsonl",
		"other-user/2026/laptop/gemini-2026-01-06.jsonl",
	}
	got := listFiles(t, root)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("seed files =\n%s\nwant =\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	fields := []string{"label", "totalCost", "inputTokens", "outputTokens", "cacheCreationTokens", "cacheReadTokens", "totalTokens"}
	for _, rel := range want[1:] {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("%s has %d lines, want 1", rel, len(lines))
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
			t.Errorf("%s: %v", rel, err)
			continue
		}
		if len(entry) != len(fields) {
			t.Errorf("%s has %d fields, want %d", rel, len(entry), len(fields))
		}
		for _, f := range fields {
			if _, ok := entry[f]; !ok {
				t.Errorf("%s lacks field %q", rel, f)
			}
		}
		base := filepath.Base(rel)
		wantLabel := strings.TrimSuffix(strings.TrimPrefix(base, strings.Split(base, "-")[0]+"-"), ".jsonl")
		if entry["label"] != wantLabel {
			t.Errorf("%s label = %v, want %q", rel, entry["label"], wantLabel)
		}
	}
}
