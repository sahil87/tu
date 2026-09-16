package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePaths(t *testing.T) {
	p, err := ResolvePaths("/home/u")
	if err != nil {
		t.Fatal(err)
	}
	want := Paths{
		Home:       "/home/u",
		ConfigDir:  "/home/u/.config/tu",
		UserConf:   "/home/u/.config/tu/tu.conf",
		OrgConf:    "/home/u/.config/tu/org.conf",
		LegacyConf: "/home/u/.tu.conf",
	}
	if p != want {
		t.Errorf("ResolvePaths = %+v, want %+v", p, want)
	}
}

func TestResolvePathsNoHome(t *testing.T) {
	_, err := ResolvePaths("")
	if err != ErrNoHome {
		t.Fatalf("err = %v, want ErrNoHome", err)
	}
	if got := err.Error(); got != "tu: $HOME is not set; cannot locate config" {
		t.Errorf("ErrNoHome text = %q", got)
	}
}

func TestParseConf(t *testing.T) {
	raw := "# comment\n\n version = 2 \nmetrics_repo = git@x:y.git\nno-equals-line\nkey = a=b\ndup = first\ndup = second\n"
	got := ParseConf(raw)
	want := map[string]string{
		"version":      "2",
		"metrics_repo": "git@x:y.git",
		"key":          "a=b",
		"dup":          "second",
	}
	if len(got) != len(want) {
		t.Fatalf("ParseConf = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("ParseConf[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestStateDir(t *testing.T) {
	if got := StateDir("/home/u"); got != "/home/u/.tu" {
		t.Errorf("StateDir = %q", got)
	}
}

func TestTildefy(t *testing.T) {
	cases := []struct{ p, home, want string }{
		{"/home/u/.config/tu/tu.conf", "/home/u", "~/.config/tu/tu.conf"},
		{"/home/u", "/home/u", "~"},
		{"/home/u2/x", "/home/u", "~2/x"}, // the TS prefix match has no path-boundary check
		{"/other/x", "/home/u", "/other/x"},
		{"/home/u/x", "", "/home/u/x"}, // empty home is a no-op
	}
	for _, c := range cases {
		if got := Tildefy(c.p, c.home); got != c.want {
			t.Errorf("Tildefy(%q, %q) = %q, want %q", c.p, c.home, got, c.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	cases := []struct{ p, home, want string }{
		{"~/.tu/metrics_repo", "/home/u", "/home/u/.tu/metrics_repo"},
		{"~/", "/home/u", "/home/u"},
		{"~", "/home/u", "/home/u"},
		{"/abs/x", "/home/u", "/abs/x"},
		{"~x/y", "/home/u", "~x/y"}, // only "~/" and a bare "~" expand
	}
	for _, c := range cases {
		if got := ExpandHome(c.p, c.home); got != c.want {
			t.Errorf("ExpandHome(%q, %q) = %q, want %q", c.p, c.home, got, c.want)
		}
	}
}

func TestParseIntJS(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"2", 2, true},
		{"  2", 2, true},
		{"\t-3", -3, true},
		{"+4", 4, true},
		{"2abc", 2, true},
		{"9 is newer", 9, true},
		{"x", 0, false},
		{"", 0, false},
		{"-x", 0, false},
		{"  ", 0, false},
	}
	for _, c := range cases {
		got, ok := parseIntJS(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseIntJS(%q) = %d, %v, want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// writeConf writes a conf file under a temp HOME, creating parents.
func writeConf(t *testing.T, home, rel, content string) {
	t.Helper()
	path := filepath.Join(home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func emptyEnv(string) string { return "" }

// testEnv pins hostname/username and serves env values from the given map.
func testEnv(vars map[string]string) Env {
	return Env{
		Getenv:   func(k string) string { return vars[k] },
		Hostname: func() (string, error) { return "test-host", nil },
		Username: func() (string, error) { return "test-user", nil },
	}
}

// R1: Load merges defaults ∪ org ∪ user ∪ env ∪ override, later wins per key.
func TestLoadCascade(t *testing.T) {
	home := t.TempDir()
	writeConf(t, home, ".config/tu/org.conf", "metrics_repo = A\nmachine = org-machine\n")
	writeConf(t, home, ".config/tu/tu.conf", "metrics_repo = B\n")
	p, _ := ResolvePaths(home)

	env := testEnv(map[string]string{"TU_METRICS_REPO": "C"})
	override := "D"
	cfg, warnings := Load(p, env, Overrides{MetricsRepo: &override})
	if cfg.MetricsRepo != "D" || cfg.Mode != Multi {
		t.Errorf("override: MetricsRepo = %q, Mode = %v, want D/Multi", cfg.MetricsRepo, cfg.Mode)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if !cfg.OrgRead || cfg.UserConfRead != p.UserConf {
		t.Errorf("OrgRead = %v, UserConfRead = %q", cfg.OrgRead, cfg.UserConfRead)
	}

	cfg, _ = Load(p, env, Overrides{})
	if cfg.MetricsRepo != "C" {
		t.Errorf("env: MetricsRepo = %q, want C", cfg.MetricsRepo)
	}

	cfg, _ = Load(p, testEnv(nil), Overrides{})
	if cfg.MetricsRepo != "B" || cfg.Machine != "org-machine" {
		t.Errorf("user conf: MetricsRepo = %q, Machine = %q, want B/org-machine", cfg.MetricsRepo, cfg.Machine)
	}

	if err := os.Remove(p.UserConf); err != nil {
		t.Fatal(err)
	}
	cfg, _ = Load(p, testEnv(nil), Overrides{})
	if cfg.MetricsRepo != "A" || cfg.UserConfRead != "" {
		t.Errorf("org only: MetricsRepo = %q, UserConfRead = %q, want A/\"\"", cfg.MetricsRepo, cfg.UserConfRead)
	}
}

// R1: an empty TU_METRICS_REPO is unset; a CLI override beats the env even
// when it points at the empty string.
func TestLoadMetricsRepoLayers(t *testing.T) {
	home := t.TempDir()
	p, _ := ResolvePaths(home)

	cfg, _ := Load(p, testEnv(map[string]string{"TU_METRICS_REPO": ""}), Overrides{})
	if cfg.Mode != Single {
		t.Errorf("empty env: Mode = %v, want Single", cfg.Mode)
	}

	cfg, _ = Load(p, testEnv(map[string]string{"TU_METRICS_REPO": "E"}), Overrides{})
	if cfg.MetricsRepo != "E" || cfg.Mode != Multi {
		t.Errorf("env: MetricsRepo = %q, Mode = %v", cfg.MetricsRepo, cfg.Mode)
	}

	empty := ""
	cfg, _ = Load(p, testEnv(map[string]string{"TU_METRICS_REPO": "E"}), Overrides{MetricsRepo: &empty})
	if cfg.MetricsRepo != "" || cfg.Mode != Single {
		t.Errorf("empty override beats env: MetricsRepo = %q, Mode = %v, want \"\"/Single", cfg.MetricsRepo, cfg.Mode)
	}
}

// R2: legacy fallback reads ~/.tu.conf and appends the byte-exact deprecation
// line; a readable tu.conf silences it; an unreadable tu.conf falls back.
func TestLoadLegacyFallback(t *testing.T) {
	t.Run("legacy only", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "metrics_repo = x\n")
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Mode != Multi || cfg.UserConfRead != p.LegacyConf {
			t.Errorf("Mode = %v, UserConfRead = %q", cfg.Mode, cfg.UserConfRead)
		}
		want := []string{"tu: ~/.tu.conf is deprecated; move it to ~/.config/tu/tu.conf"}
		if len(warnings) != 1 || warnings[0] != want[0] {
			t.Errorf("warnings = %v, want %v", warnings, want)
		}
	})
	t.Run("tu.conf silences legacy", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "metrics_repo = x\n")
		writeConf(t, home, ".config/tu/tu.conf", "metrics_repo =\n")
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Mode != Single || cfg.UserConfRead != p.UserConf {
			t.Errorf("Mode = %v, UserConfRead = %q", cfg.Mode, cfg.UserConfRead)
		}
		if len(warnings) != 0 {
			t.Errorf("warnings = %v, want none", warnings)
		}
	})
	t.Run("unreadable tu.conf falls back", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "metrics_repo = x\n")
		// A directory at the tu.conf path makes os.ReadFile fail.
		if err := os.MkdirAll(filepath.Join(home, ".config", "tu", "tu.conf"), 0o755); err != nil {
			t.Fatal(err)
		}
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Mode != Multi || cfg.UserConfRead != p.LegacyConf {
			t.Errorf("Mode = %v, UserConfRead = %q", cfg.Mode, cfg.UserConfRead)
		}
		if len(warnings) != 1 {
			t.Errorf("warnings = %v, want the deprecation line", warnings)
		}
	})
}

// R3: version parsing with JavaScript parseInt semantics and the
// newer-version warning's source attribution.
func TestLoadVersion(t *testing.T) {
	t.Run("version 9 from user conf", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "version = 9\n")
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Version != 9 {
			t.Errorf("Version = %d, want 9", cfg.Version)
		}
		want := "Warning: " + p.UserConf + " version 9 is newer than tu supports (2). Please update tu."
		if len(warnings) != 1 || warnings[0] != want {
			t.Errorf("warnings = %v, want [%q]", warnings, want)
		}
	})
	t.Run("version 9 from org conf", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/org.conf", "version = 9\n")
		p, _ := ResolvePaths(home)
		_, warnings := Load(p, testEnv(nil), Overrides{})
		want := "Warning: " + p.OrgConf + " version 9 is newer than tu supports (2). Please update tu."
		if len(warnings) != 1 || warnings[0] != want {
			t.Errorf("warnings = %v, want [%q]", warnings, want)
		}
	})
	t.Run("deprecation precedes version warning", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "version = 9\n")
		p, _ := ResolvePaths(home)
		_, warnings := Load(p, testEnv(nil), Overrides{})
		if len(warnings) != 2 || warnings[0] != legacyWarning {
			t.Errorf("warnings = %v, want deprecation first", warnings)
		}
	})
	t.Run("2abc parses as 2 without warning", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "version = 2abc\n")
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Version != 2 || len(warnings) != 0 {
			t.Errorf("Version = %d, warnings = %v", cfg.Version, warnings)
		}
	})
	t.Run("x parses as 1 without warning", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "version = x\n")
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Version != 1 || len(warnings) != 0 {
			t.Errorf("Version = %d, warnings = %v", cfg.Version, warnings)
		}
	})
}

// R3: field derivation — defaults layer, sentinel and ~ expansion, auto_sync.
func TestLoadFields(t *testing.T) {
	t.Run("defaults only", func(t *testing.T) {
		home := t.TempDir()
		p, _ := ResolvePaths(home)
		cfg, warnings := Load(p, testEnv(nil), Overrides{})
		if cfg.Version != 2 || cfg.Mode != Single || cfg.MetricsRepo != "" {
			t.Errorf("cfg = %+v", cfg)
		}
		if cfg.MetricsDir != filepath.Join(home, ".tu", "metrics_repo") {
			t.Errorf("MetricsDir = %q", cfg.MetricsDir)
		}
		if cfg.Machine != "test-host" || cfg.User != "test-user" || !cfg.AutoSync {
			t.Errorf("Machine = %q, User = %q, AutoSync = %v", cfg.Machine, cfg.User, cfg.AutoSync)
		}
		if cfg.UserConfRead != "" || cfg.OrgRead || len(warnings) != 0 {
			t.Errorf("UserConfRead = %q, OrgRead = %v, warnings = %v", cfg.UserConfRead, cfg.OrgRead, warnings)
		}
	})
	t.Run("tilde expansion and empty-value fallback", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "metrics_dir = ~/x\nmachine =\n")
		p, _ := ResolvePaths(home)
		cfg, _ := Load(p, testEnv(nil), Overrides{})
		if cfg.MetricsDir != filepath.Join(home, "x") {
			t.Errorf("MetricsDir = %q", cfg.MetricsDir)
		}
		if cfg.Machine != "test-host" {
			t.Errorf("empty machine falls back to the sentinel: Machine = %q", cfg.Machine)
		}
	})
	t.Run("sentinels are exact-match only", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "machine = pre-$HOSTNAME\nuser = $USER-suffix\n")
		p, _ := ResolvePaths(home)
		cfg, _ := Load(p, testEnv(nil), Overrides{})
		if cfg.Machine != "pre-$HOSTNAME" || cfg.User != "$USER-suffix" {
			t.Errorf("Machine = %q, User = %q", cfg.Machine, cfg.User)
		}
	})
	t.Run("sentinel errors", func(t *testing.T) {
		home := t.TempDir()
		p, _ := ResolvePaths(home)
		env := Env{
			Getenv:   emptyEnv,
			Hostname: func() (string, error) { return "", errors.New("no host") },
			Username: func() (string, error) { return "", errors.New("no user") },
		}
		cfg, _ := Load(p, env, Overrides{})
		if cfg.Machine != "" || cfg.User != "unknown" {
			t.Errorf("Machine = %q, User = %q, want \"\"/unknown", cfg.Machine, cfg.User)
		}
	})
	t.Run("auto_sync parsing", func(t *testing.T) {
		for _, tc := range []struct {
			value string
			want  bool
		}{
			{"false", false},
			{"0", false},
			{"FALSE", true}, // exact-match only
			{"no", true},
			{"", true}, // empty falls back to the default true
		} {
			home := t.TempDir()
			writeConf(t, home, ".config/tu/tu.conf", "auto_sync = "+tc.value+"\n")
			p, _ := ResolvePaths(home)
			cfg, _ := Load(p, testEnv(nil), Overrides{})
			if cfg.AutoSync != tc.want {
				t.Errorf("auto_sync = %q: AutoSync = %v, want %v", tc.value, cfg.AutoSync, tc.want)
			}
		}
	})
	t.Run("mode key ignored", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/tu.conf", "mode = multi\n")
		p, _ := ResolvePaths(home)
		cfg, _ := Load(p, testEnv(nil), Overrides{})
		if cfg.Mode != Single {
			t.Errorf("Mode = %v, want Single (mode key is ignored)", cfg.Mode)
		}
	})
}
