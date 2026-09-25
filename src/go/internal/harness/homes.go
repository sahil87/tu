package harness

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// HomeConfBody is the conf content shared byte-for-byte by the multi, org,
// and legacy $HOME variants — only the file's location varies between them.
// machine/user are pinned so the $HOSTNAME/$USER sentinels never make
// multi-mode output machine-specific.
const HomeConfBody = `version = 2
metrics_repo = git@example.invalid:harness/tu-metrics.git
metrics_dir = ~/.tu/metrics_repo
machine = harness-machine
user = harness-user
auto_sync = true
`

// MetricsRepoURL is the pinned metrics remote of the seeded homes and the
// envrepo env axis; the .invalid TLD can never resolve.
const MetricsRepoURL = "git@example.invalid:harness/tu-metrics.git"

// StageHome creates dir as a fresh $HOME for one conf variant: single gets an
// empty directory; multi writes .config/tu/tu.conf; org writes
// .config/tu/org.conf (no tu.conf); legacy writes .tu.conf (no .config/). The
// three multi variants also receive a recursive copy of the seed tree (the
// committed harness/metrics-repo mini-clone) at .tu/metrics_repo/.
func StageHome(dir, variant, seedDir string) error {
	switch variant {
	case ConfSingle:
		return os.MkdirAll(dir, 0o755)
	case ConfMulti:
		if err := writeHomeConf(filepath.Join(dir, ".config", "tu", "tu.conf")); err != nil {
			return err
		}
	case ConfOrg:
		if err := writeHomeConf(filepath.Join(dir, ".config", "tu", "org.conf")); err != nil {
			return err
		}
	case ConfLegacy:
		if err := writeHomeConf(filepath.Join(dir, ".tu.conf")); err != nil {
			return err
		}
	default:
		return fmt.Errorf("tudiff: unknown home variant %q", variant)
	}
	return CopyTree(seedDir, filepath.Join(dir, ".tu", "metrics_repo"))
}

func writeHomeConf(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(HomeConfBody), 0o644)
}

// CopyTree recursively copies src to dst (files 0644, dirs 0755). No .git/ is
// created — the metrics-dir guard is an existence check and the fake git
// answers every call.
func CopyTree(src, dst string) error {
	return CopyTreeExcept(src, dst, nil)
}

// CopyTreeExcept is CopyTree with an exclusion predicate over slash-separated
// relpaths; excluding a directory prunes its subtree.
func CopyTreeExcept(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if skip != nil && rel != "." && skip(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
}
