package toolkit

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSkillDriftGuard pins the embedded copy to the canonical
// docs/site/skill.md: editing one byte of either copy fails this test (the
// internal/config/defaults_test.go walk-up precedent).
func TestSkillDriftGuard(t *testing.T) {
	root, ok := findPackageRoot(t)
	if !ok {
		t.Fatal("no package.json found walking up from the package directory")
	}
	raw, err := os.ReadFile(filepath.Join(root, "docs", "site", "skill.md"))
	if err != nil {
		t.Fatalf("read docs/site/skill.md: %v", err)
	}
	if !bytes.Equal(Skill, raw) {
		t.Errorf("internal/toolkit/skill.md drifted from docs/site/skill.md — run scripts/sync-skill.sh")
	}
}

// TestSkillShape pins the toolkit skill standard's invariants: the bundle is
// within the 150-line hard budget and ends with a newline.
func TestSkillShape(t *testing.T) {
	if lines := bytes.Count(Skill, []byte("\n")); lines > 150 {
		t.Errorf("skill bundle is %d lines, over the 150-line budget", lines)
	}
	if len(Skill) == 0 || Skill[len(Skill)-1] != '\n' {
		t.Error("skill bundle does not end with a newline")
	}
}

func TestWriteSkill(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteSkill(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), Skill) {
		t.Error("WriteSkill output != embedded Skill")
	}
}
