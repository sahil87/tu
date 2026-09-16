// Package config is the minimal config cascade V2 needs: path resolution from
// $HOME, conf-file parsing, and single/multi mode detection. B1 grows it into
// the full cascade (defaults layer, sentinels, warnings). The package writes
// nothing to any stream.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoHome is returned by ResolvePaths when $HOME is unset or empty. The
// message is byte-exact (the TS ConfigHomeError text): the edge prints it
// verbatim, exit 1.
var ErrNoHome = errors.New("tu: $HOME is not set; cannot locate config")

// Paths locates the config files, built from $HOME and nothing else (no
// $XDG_CONFIG_HOME, no TU_* override).
type Paths struct {
	Home       string
	ConfigDir  string // $HOME/.config/tu
	UserConf   string // $HOME/.config/tu/tu.conf
	OrgConf    string // $HOME/.config/tu/org.conf
	LegacyConf string // $HOME/.tu.conf
}

// ResolvePaths mirrors the TS resolveConfigPaths.
func ResolvePaths(home string) (Paths, error) {
	if home == "" {
		return Paths{}, ErrNoHome
	}
	configDir := filepath.Join(home, ".config", "tu")
	return Paths{
		Home:       home,
		ConfigDir:  configDir,
		UserConf:   filepath.Join(configDir, "tu.conf"),
		OrgConf:    filepath.Join(configDir, "org.conf"),
		LegacyConf: filepath.Join(home, ".tu.conf"),
	}, nil
}

// ParseConf is the TS parseConf: per line trim; skip blank and '#' lines;
// split at the FIRST '='; trim key and value; later keys overwrite earlier
// ones.
func ParseConf(raw string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		fields[strings.TrimSpace(trimmed[:idx])] = strings.TrimSpace(trimmed[idx+1:])
	}
	return fields
}

// Mode is the operating mode derived from metrics_repo presence.
type Mode int

const (
	Single Mode = iota
	Multi
)

// DetectMode derives the mode exactly as the TS readConfig does:
// TU_METRICS_REPO non-empty → Multi; else merge org.conf then the user conf
// (tu.conf, falling back to legacy ~/.tu.conf when tu.conf is unreadable) and
// Multi iff the merged metrics_repo is non-empty; else Single. Unreadable
// files are empty maps.
func DetectMode(p Paths, getenv func(string) string) Mode {
	if getenv("TU_METRICS_REPO") != "" {
		return Multi
	}
	merged := readConf(p.OrgConf)
	for k, v := range readConf(userConfPath(p)) {
		merged[k] = v
	}
	if merged["metrics_repo"] != "" {
		return Multi
	}
	return Single
}

// userConfPath selects the user conf: tu.conf, falling back to the legacy
// file when tu.conf is unreadable.
func userConfPath(p Paths) string {
	if _, err := os.ReadFile(p.UserConf); err == nil {
		return p.UserConf
	}
	return p.LegacyConf
}

// readConf parses the file at path; an unreadable file reads as an empty map.
func readConf(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	return ParseConf(string(raw))
}
