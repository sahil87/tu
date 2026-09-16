package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfDriftGuard pins the embedded copy to the repo-root
// tu.default.conf (the file scripts/build.sh copies beside tu.mjs): editing
// one byte of either copy fails this test.
func TestDefaultConfDriftGuard(t *testing.T) {
	root := findPackageRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "tu.default.conf"))
	if err != nil {
		t.Fatalf("read repo-root tu.default.conf: %v", err)
	}
	if !bytes.Equal(DefaultConf, raw) {
		t.Errorf("internal/config/tu.default.conf drifted from the repo-root copy")
	}
}

// findPackageRoot walks up from the package directory (the test's cwd) to the
// first directory containing package.json — the repo root.
func findPackageRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no package.json found walking up from the package directory")
		}
		dir = parent
	}
}
