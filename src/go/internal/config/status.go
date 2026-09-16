package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// StatusData is the TS runStatus's data: the layouts §13 block's fields.
// UserConfRead is "" when omitted (org-only); OrgConf is "" when org.conf
// does not exist; LastSync is "never" or "{relative} ({ISO})".
type StatusData struct {
	Mode          Mode
	NoConfig      bool // neither a readable user/legacy conf nor org.conf → the one-line form
	UserConfRead  string
	Version       int
	OrgConf       string
	User, Machine string
	MetricsDir    string
	MetricsFound  bool
	LastSync      string
	AutoSync      bool
	Home          string // for Tildefy in Lines()
}

// Status is the TS runStatus: reads config (via Load) and state; returns the
// data and the cascade warnings the edge prints on stderr. now is injected.
// Status never clones, syncs, or writes.
func Status(p Paths, env Env, now time.Time) (StatusData, []string) {
	orgExists := fileExists(p.OrgConf)
	// Mirror the Load selection rule exactly: a file counts as selected only
	// when it actually reads (an existing-but-unreadable file falls back).
	selected := selectUserConfPath(p)

	// No config file at all: the original no-config line. An org-only setup
	// falls through to the full layout (with the Config: line omitted). No
	// Load, no warnings.
	if selected == "" && !orgExists {
		return StatusData{Mode: Single, NoConfig: true, Home: p.Home}, nil
	}

	cfg, warnings := Load(p, env, Overrides{})
	d := StatusData{
		Mode:         cfg.Mode,
		UserConfRead: selected,
		Version:      cfg.Version,
		User:         cfg.User,
		Machine:      cfg.Machine,
		MetricsDir:   cfg.MetricsDir,
		AutoSync:     cfg.AutoSync,
		Home:         p.Home,
	}
	if orgExists {
		d.OrgConf = p.OrgConf
	}
	if cfg.Mode == Multi {
		d.MetricsFound = fileExists(cfg.MetricsDir)
		d.LastSync = lastSync(StateDir(p.Home), now)
	}
	return d, warnings
}

// Lines renders the layouts §13 block; the label column is exactly 13
// characters, reproduced as literal strings (not %-13s — the em dash and the
// apostrophe in the NOT FOUND suffix are copied verbatim).
func (s StatusData) Lines() []string {
	if s.NoConfig {
		return []string{"Mode:        single (no ~/.config/tu/tu.conf)"}
	}
	var lines []string
	configLine := ""
	if s.UserConfRead != "" {
		configLine = "Config:      " + Tildefy(s.UserConfRead, s.Home) + " (v" + strconv.Itoa(s.Version) + ")"
	}
	if s.Mode != Multi {
		lines = append(lines, "Mode:        single")
		if configLine != "" {
			lines = append(lines, configLine)
		}
		if s.OrgConf != "" {
			lines = append(lines, "Org config:  "+Tildefy(s.OrgConf, s.Home))
		}
		return lines
	}
	metricsLine := Tildefy(s.MetricsDir, s.Home)
	if !s.MetricsFound {
		metricsLine += " (NOT FOUND — run 'tu init-metrics')"
	}
	lines = append(lines,
		"Mode:        multi",
		"User:        "+s.User,
		"Machine:     "+s.Machine,
	)
	if configLine != "" {
		lines = append(lines, configLine)
	}
	if s.OrgConf != "" {
		lines = append(lines, "Org config:  "+Tildefy(s.OrgConf, s.Home))
	}
	return append(lines,
		"Metrics:     "+metricsLine,
		"Last sync:   "+s.LastSync,
		"Auto-sync:   "+autoSyncWord(s.AutoSync),
	)
}

func autoSyncWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// lastSync is the TS formatLastSync: stateDir/.last-sync absent, unreadable,
// or unparseable (after trimming, as RFC3339Nano — the file is written by
// `tu sync` as JavaScript toISOString()) → "never"; else
// "{RelativeTime(now − ts)} ({trimmed raw})".
func lastSync(stateDir string, now time.Time) string {
	raw, err := os.ReadFile(filepath.Join(stateDir, ".last-sync"))
	if err != nil {
		return "never"
	}
	trimmed := strings.TrimSpace(string(raw))
	ts, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return "never"
	}
	return RelativeTime(now.Sub(ts)) + " (" + trimmed + ")"
}

// RelativeTime is the TS relativeTime: floor to whole seconds (negatives
// clamp to 0); <60 s "<1m ago"; <60 min "{m}m ago"; <24 h "{h}h ago"; else
// "{d}d ago".
func RelativeTime(d time.Duration) string {
	seconds := int64(d / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	if seconds < 60 {
		return "<1m ago"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return strconv.FormatInt(minutes, 10) + "m ago"
	}
	hours := minutes / 60
	if hours < 24 {
		return strconv.FormatInt(hours, 10) + "h ago"
	}
	return strconv.FormatInt(hours/24, 10) + "d ago"
}
