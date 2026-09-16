package cache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/fact"
)

var testRecords = []fact.Record{
	{Date: "2026-01-05", Tool: "cc", Totals: fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}},
	{Date: "2026-01-06", Tool: "cc", Totals: fact.Totals{TotalCost: 0.5, InputTokens: 3000, OutputTokens: 400, CacheCreationTokens: 1000, CacheReadTokens: 20000, TotalTokens: 24400}},
}

func TestPutGetRoundTrip(t *testing.T) {
	s := &Store{Dir: t.TempDir(), TTL: TTL, Now: time.Now}
	k := Key{Tool: "cc", Period: "daily"}

	if _, ok := s.Get(k); ok {
		t.Fatal("Get before Put should miss")
	}
	if err := s.Put(k, testRecords); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get(k)
	if !ok {
		t.Fatal("Get after Put should hit")
	}
	if !reflect.DeepEqual(got, testRecords) {
		t.Errorf("round-trip = %+v, want %+v", got, testRecords)
	}
}

func TestTTLExpiry(t *testing.T) {
	now := time.Now()
	s := &Store{Dir: t.TempDir(), TTL: TTL, Now: func() time.Time { return now }}
	k := Key{Tool: "cc", Period: "daily"}

	if err := s.Put(k, testRecords); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(k); !ok {
		t.Fatal("fresh entry should hit")
	}
	now = now.Add(61 * time.Second)
	if _, ok := s.Get(k); ok {
		t.Error("entry older than TTL should miss")
	}
}

func TestZeroTTLUsesPackageTTL(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir} // zero TTL and nil Now
	k := Key{Tool: "cc", Period: "daily"}

	if err := s.Put(k, testRecords); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(k); !ok {
		t.Fatal("fresh entry should hit with zero Store.TTL")
	}
	old := time.Now().Add(-61 * time.Second)
	if err := os.Chtimes(filepath.Join(dir, k.Filename()), old, old); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(k); ok {
		t.Error("entry older than the package TTL should miss with zero Store.TTL")
	}
}

func TestEnvelopeMismatchMisses(t *testing.T) {
	s := &Store{Dir: t.TempDir(), TTL: TTL, Now: time.Now}
	k := Key{Tool: "cc", Period: "daily"}

	// An envelope that says tool codex under the cc filename.
	raw := `{"v":1,"tool":"codex","period":"daily","args":[],"records":[]}`
	if err := os.WriteFile(filepath.Join(s.Dir, k.Filename()), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(k); ok {
		t.Error("envelope tool mismatch should miss")
	}
}

func TestCorruptFileMisses(t *testing.T) {
	s := &Store{Dir: t.TempDir(), TTL: TTL, Now: time.Now}
	k := Key{Tool: "cc", Period: "daily"}
	if err := os.WriteFile(filepath.Join(s.Dir, k.Filename()), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(k); ok {
		t.Error("corrupt file should miss")
	}
}

func TestFilenameStabilityAndDistinctness(t *testing.T) {
	base := Key{Tool: "cc", Period: "daily"}
	if base.Filename() != base.Filename() {
		t.Error("Filename() not stable across calls")
	}
	if !strings.HasPrefix(base.Filename(), "cc-daily-") || !strings.HasSuffix(base.Filename(), ".json") {
		t.Errorf("Filename() = %q, want {tool}-{period}-{hash}.json", base.Filename())
	}
	distinct := []Key{
		{Tool: "cc", Period: "daily", Args: []string{"--x"}},
		{Tool: "codex", Period: "daily"},
		{Tool: "cc", Period: "monthly"},
	}
	for _, other := range distinct {
		if base.Filename() == other.Filename() {
			t.Errorf("Filename() collides for %+v and %+v", base, other)
		}
	}
}

func TestPutEncodesEmptyArgsAsArray(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	k := Key{Tool: "cc", Period: "daily"}
	if err := s.Put(k, testRecords); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, k.Filename()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"args":[]`) {
		t.Errorf("envelope = %s, want args encoded as []", raw)
	}
}

func TestDefault(t *testing.T) {
	t.Run("HOME unset", func(t *testing.T) {
		t.Setenv("HOME", "")
		if _, err := Default(); err == nil {
			t.Error("Default() with empty $HOME should error")
		}
	})
	t.Run("HOME set", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		s, err := Default()
		if err != nil {
			t.Fatal(err)
		}
		if s.Dir != filepath.Join(home, ".tu", "cache") {
			t.Errorf("Dir = %q", s.Dir)
		}
		if s.TTL != TTL {
			t.Errorf("TTL = %v, want %v", s.TTL, TTL)
		}
		if s.Now == nil {
			t.Error("Now is nil")
		}
	})
}
