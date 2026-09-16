package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveFixtures implements the D7 alias resolution under
// <root>/harness/fixtures/: an explicit alias list wins (each alias must hold
// a manifest.json); placeholderOnly forces the placeholder corpus alone;
// otherwise the machine's own capture precedes _placeholder when its manifest
// exists. Returns the alias directories in search order (first hit wins).
func ResolveFixtures(root string, explicit []string, placeholderOnly bool, hostname string) ([]string, error) {
	fixturesRoot := filepath.Join(root, "harness", "fixtures")
	placeholder := filepath.Join(fixturesRoot, PlaceholderAlias)
	if len(explicit) > 0 {
		out := make([]string, 0, len(explicit))
		for _, alias := range explicit {
			if err := pathComponent("fixtures alias", alias); err != nil {
				return nil, err
			}
			dir := filepath.Join(fixturesRoot, alias)
			if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
				return nil, fmt.Errorf("tudiff: fixtures alias %q has no manifest.json (%s)", alias, dir)
			}
			out = append(out, dir)
		}
		return out, nil
	}
	if placeholderOnly {
		return []string{placeholder}, nil
	}
	hostDir := filepath.Join(fixturesRoot, hostname)
	if hostname != "" {
		if _, err := os.Stat(filepath.Join(hostDir, "manifest.json")); err == nil {
			return []string{hostDir, placeholder}, nil
		}
	}
	return []string{placeholder}, nil
}

// UnconfirmedReplays reports whether any line of the JSON-lines call log at
// callLogPath replayed a fixture whose manifest entry carries
// unconfirmed: true. matched is "<alias>/<file>"; the alias is resolved
// against the base names of the given alias directories. A missing log means
// the side never called a fake — nothing replayed.
func UnconfirmedReplays(callLogPath string, aliases []string) (bool, error) {
	manifests := map[string]*Manifest{}
	found := false
	err := readCallLog(callLogPath, func(cl callLogLine) error {
		if cl.Matched == "" {
			return nil
		}
		alias, file, ok := strings.Cut(cl.Matched, "/")
		if !ok {
			return nil
		}
		m, err := aliasManifest(manifests, aliases, alias)
		if err != nil {
			return err // manifest unreadable/malformed — not an alias miss
		}
		if m == nil {
			return nil // alias not in this run's list — nothing to look up
		}
		for _, fx := range m.Fixtures {
			if fx.File == file && fx.Unconfirmed {
				found = true
			}
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

// aliasManifest returns the manifest of the alias directory whose base name
// is alias, cached per call; nil when the alias is not among dirs.
func aliasManifest(cache map[string]*Manifest, dirs []string, alias string) (*Manifest, error) {
	if m, ok := cache[alias]; ok {
		return m, nil
	}
	for _, dir := range dirs {
		if filepath.Base(dir) == alias {
			m, err := ReadManifest(dir)
			if err != nil {
				return nil, err
			}
			cache[alias] = m
			return m, nil
		}
	}
	cache[alias] = nil
	return nil, nil
}
