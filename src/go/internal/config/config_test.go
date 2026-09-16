package config

import (
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

func TestDetectMode(t *testing.T) {
	t.Run("org.conf only", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/org.conf", "metrics_repo = x\n")
		p, _ := ResolvePaths(home)
		if got := DetectMode(p, emptyEnv); got != Multi {
			t.Errorf("DetectMode = %v, want Multi", got)
		}
	})
	t.Run("user conf empty value overrides org", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".config/tu/org.conf", "metrics_repo = x\n")
		writeConf(t, home, ".config/tu/tu.conf", "metrics_repo =\n")
		p, _ := ResolvePaths(home)
		if got := DetectMode(p, emptyEnv); got != Single {
			t.Errorf("DetectMode = %v, want Single", got)
		}
	})
	t.Run("env var wins", func(t *testing.T) {
		home := t.TempDir()
		p, _ := ResolvePaths(home)
		getenv := func(k string) string {
			if k == "TU_METRICS_REPO" {
				return "y"
			}
			return ""
		}
		if got := DetectMode(p, getenv); got != Multi {
			t.Errorf("DetectMode = %v, want Multi", got)
		}
	})
	t.Run("legacy fallback", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "metrics_repo = x\n")
		p, _ := ResolvePaths(home)
		if got := DetectMode(p, emptyEnv); got != Multi {
			t.Errorf("DetectMode = %v, want Multi", got)
		}
	})
	t.Run("tu.conf wins over legacy", func(t *testing.T) {
		home := t.TempDir()
		writeConf(t, home, ".tu.conf", "metrics_repo = x\n")
		writeConf(t, home, ".config/tu/tu.conf", "metrics_repo =\n")
		p, _ := ResolvePaths(home)
		if got := DetectMode(p, emptyEnv); got != Single {
			t.Errorf("DetectMode = %v, want Single (tu.conf shadows legacy)", got)
		}
	})
	t.Run("nothing", func(t *testing.T) {
		p, _ := ResolvePaths(t.TempDir())
		if got := DetectMode(p, emptyEnv); got != Single {
			t.Errorf("DetectMode = %v, want Single", got)
		}
	})
}
