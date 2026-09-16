// Package config is the full config cascade (the TS config.ts): path
// resolution from $HOME, conf-file parsing, the defaults ∪ org ∪ user ∪ env ∪
// CLI merge, sentinel and ~ expansion, the legacy-deprecation and
// newer-version warnings, and the setup-command data (init-conf, init-metrics,
// status). The package writes nothing to any stream and execs nothing.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNoHome is returned by ResolvePaths when $HOME is unset or empty. The
// message is byte-exact (the TS ConfigHomeError text): the edge prints it
// verbatim, exit 1.
var ErrNoHome = errors.New("tu: $HOME is not set; cannot locate config")

// CurrentConfigVersion is the config schema version this binary supports (the
// TS CURRENT_CONFIG_VERSION); a newer version in any read layer warns.
const CurrentConfigVersion = 2

// legacyWarning is the byte-exact deprecation line for the legacy ~/.tu.conf
// fallback (literal text in the TS, not built from the path).
const legacyWarning = "tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf"

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

// StateDir is the runtime-state root, the TS TU_HOME: $HOME/.tu (cache,
// metrics repo clone, .last-sync, .clone-failed). Config lives under
// $HOME/.config/tu — the two roots are intentionally separate
// (config-system memory).
func StateDir(home string) string {
	return filepath.Join(home, ".tu")
}

// Tildefy abbreviates a path under home to "~/…" (the TS tildefy: a prefix
// match on the home string, no trailing-slash handling); other paths are
// returned as-is.
func Tildefy(p, home string) string {
	if home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// ExpandHome resolves a leading "~/" or a bare "~" against home (the TS
// resolveHome).
func ExpandHome(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	if p == "~" {
		return home
	}
	return p
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

// Env is Load's view of the process environment, injected so tests pin
// hostname/username/env without touching the real process.
type Env struct {
	Getenv   func(string) string    // os.Getenv at the edge
	Hostname func() (string, error) // os.Hostname
	Username func() (string, error) // user.Current().Username
}

// Overrides is the CLI-argument layer (the fifth layer of the cascade); only
// metrics_repo can be overridden, exactly as in the TS.
type Overrides struct {
	MetricsRepo *string
}

// Config is the resolved configuration (the TS TuConfig) plus the read
// provenance the version warning and status need.
type Config struct {
	Version      int
	Mode         Mode
	MetricsRepo  string
	MetricsDir   string // sentinels expanded, then ~ expanded against Home
	Machine      string // sentinel expanded
	User         string // sentinel expanded
	AutoSync     bool
	UserConfRead string // the user-conf path actually read: UserConf, LegacyConf, or "" (neither readable)
	OrgRead      bool   // org.conf was readable and fed the merge
}

// Load is the TS readConfig. It reads files and returns values: the Config,
// the stderr warning lines the edge prints (in order), and never an error —
// unreadable files are empty layers, exactly as the TS readConfFile returns
// null.
func Load(p Paths, env Env, ov Overrides) (Config, []string) {
	var warnings []string

	// Cascade: tu.default.conf < org.conf < user conf < env < CLI overrides.
	merged := ParseConf(string(DefaultConf))
	orgFields, orgRead := readConf(p.OrgConf)
	for k, v := range orgFields {
		merged[k] = v
	}
	userFields, userPath, legacy := selectUserConf(p)
	if legacy {
		warnings = append(warnings, legacyWarning)
	}
	for k, v := range userFields {
		merged[k] = v
	}

	// TU_METRICS_REPO takes precedence over config files; a CLI-layer override
	// beats the env var even when it is the empty string (the TS
	// `overrides.metrics_repo ?? …` nullish test). TU_METRICS_REPO is the only
	// config-bearing environment variable.
	metricsRepo := merged["metrics_repo"]
	if v := env.Getenv("TU_METRICS_REPO"); v != "" {
		metricsRepo = v
	}
	if ov.MetricsRepo != nil {
		metricsRepo = *ov.MetricsRepo
	}

	version := 1
	if raw := merged["version"]; raw != "" {
		if n, ok := parseIntJS(raw); ok {
			version = n
		}
	}
	if version > CurrentConfigVersion {
		// Name the file the value came from: the user conf actually read, else
		// the org layer when it was actually read, else the shipped defaults —
		// an absolute path, never tildefied.
		source := userPath
		if source == "" {
			if orgRead {
				source = p.OrgConf
			} else {
				source = DefaultConfName
			}
		}
		warnings = append(warnings, "Warning: "+source+" version "+strconv.Itoa(version)+
			" is newer than tu supports ("+strconv.Itoa(CurrentConfigVersion)+"). Please update tu.")
	}

	mode := Single
	if metricsRepo != "" {
		mode = Multi
	}

	autoSync := merged["auto_sync"] != "false" && merged["auto_sync"] != "0"

	return Config{
		Version:      version,
		Mode:         mode,
		MetricsRepo:  metricsRepo,
		MetricsDir:   ExpandHome(expandSentinels(orDefault(merged["metrics_dir"], "~/.tu/metrics_repo"), env), p.Home),
		Machine:      expandSentinels(orDefault(merged["machine"], "$HOSTNAME"), env),
		User:         expandSentinels(orDefault(merged["user"], "$USER"), env),
		AutoSync:     autoSync,
		UserConfRead: userPath,
		OrgRead:      orgRead,
	}, warnings
}

// orDefault is the TS `value || fallback`: an empty value falls back.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// parseIntJS mirrors JavaScript parseInt(s, 10): optional leading whitespace
// and sign, then the longest run of ASCII digits ("2abc" → 2). ok is false
// when no digits follow (NaN in the TS, whose caller substitutes 1).
func parseIntJS(s string) (n int, ok bool) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' || s[i] == '\f' || s[i] == '\v') {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[start:i])
	if err != nil {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

// expandSentinels is the TS expandSentinels: a value that is exactly
// "$HOSTNAME" becomes the hostname (error → ""), exactly "$USER" becomes the
// username (error → "unknown", the TS safeUsername); anything else passes
// through. No substring expansion.
func expandSentinels(value string, env Env) string {
	switch value {
	case "$HOSTNAME":
		host, err := env.Hostname()
		if err != nil {
			return ""
		}
		return host
	case "$USER":
		user, err := env.Username()
		if err != nil {
			return "unknown"
		}
		return user
	}
	return value
}

// selectUserConf is the TS selectUserConf: UserConf readable → its fields and
// path (legacy ignored silently); else LegacyConf readable → its fields and
// path plus legacy=true (the caller emits the deprecation warning); else
// neither. "Readable" means os.ReadFile succeeds — an existing-but-unreadable
// file falls back, as the TS does. The TS once-per-process Set guard is
// unnecessary: Load is called once per invocation on every wired path.
func selectUserConf(p Paths) (fields map[string]string, path string, legacy bool) {
	if raw, err := os.ReadFile(p.UserConf); err == nil {
		return ParseConf(string(raw)), p.UserConf, false
	}
	if raw, err := os.ReadFile(p.LegacyConf); err == nil {
		return ParseConf(string(raw)), p.LegacyConf, true
	}
	return map[string]string{}, "", false
}

// selectUserConfPath is the TS selectUserConfPath: the user-conf path Load
// would read, "" when neither reads — status mirrors this selection (an
// exists-based check would misreport an existing-but-unreadable file as
// selected while Load falls back).
func selectUserConfPath(p Paths) string {
	_, path, _ := selectUserConf(p)
	return path
}

// readConf parses the file at path and reports whether it was readable; an
// unreadable file is an empty layer (the TS readConfFile → null).
func readConf(path string) (map[string]string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}, false
	}
	return ParseConf(string(raw)), true
}

// fileExists reports whether path exists (any file type).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
