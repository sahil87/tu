// Package cache is the on-disk JSON cache behind source fetches: one file
// per (tool, period, args) key, named by a truncated sha256 of the key, with
// a 60-second TTL checked via file mtime.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sahil87/tu/internal/fact"
)

// TTL is the default cache lifetime.
const TTL = 60 * time.Second

// envelopeVersion is the on-disk envelope schema version.
const envelopeVersion = 1

// hashLen is the number of hex characters of the sha256 key digest used in
// the filename.
const hashLen = 16

// Key identifies one cached fetch.
type Key struct {
	Tool   string
	Period string
	Args   []string
}

// Filename is "{tool}-{period}-{h}.json" where h is the first 16 hex
// characters of sha256(tool + "\x00" + period + "\x00" +
// strings.Join(args, "\x00")). The hash alone is the key; the readable prefix
// is a courtesy for `ls ~/.tu/cache`.
func (k Key) Filename() string {
	sum := sha256.Sum256([]byte(k.Tool + "\x00" + k.Period + "\x00" + strings.Join(k.Args, "\x00")))
	return k.Tool + "-" + k.Period + "-" + hex.EncodeToString(sum[:])[:hashLen] + ".json"
}

// Store is a cache rooted at Dir. TTL zero means the package TTL; Now nil
// means time.Now (tests inject both).
type Store struct {
	Dir string
	TTL time.Duration
	Now func() time.Time
}

// Default returns the store at $HOME/.tu/cache, or an error when $HOME is
// empty (the config-home exit-1 case is the edge's to report).
func Default() (*Store, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return nil, fmt.Errorf("cache: $HOME is not set")
	}
	return &Store{Dir: filepath.Join(home, ".tu", "cache"), TTL: TTL, Now: time.Now}, nil
}

// envelope is the on-disk document; tool/period/args are verified against
// the key on read so a hash collision or stale schema is a miss, never a
// wrong answer.
type envelope struct {
	V       int           `json:"v"`
	Tool    string        `json:"tool"`
	Period  string        `json:"period"`
	Args    []string      `json:"args"`
	Records []fact.Record `json:"records"`
}

func (s *Store) ttl() time.Duration {
	if s.TTL == 0 {
		return TTL
	}
	return s.TTL
}

func (s *Store) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// Get returns the cached records for k, or (nil, false) when the file is
// absent, older than the TTL (by mtime), undecodable, or its envelope does
// not match the key.
func (s *Store) Get(k Key) ([]fact.Record, bool) {
	path := filepath.Join(s.Dir, k.Filename())
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if s.now().Sub(info.ModTime()) > s.ttl() {
		return nil, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, false
	}
	if env.V != envelopeVersion || env.Tool != k.Tool || env.Period != k.Period || !slices.Equal(env.Args, k.Args) {
		return nil, false
	}
	return env.Records, true
}

// Put writes recs under k, creating Dir as needed. Records are stored
// unstamped (empty User/Machine): the key excludes identity, so Source stamps
// on the way out.
func (s *Store) Put(k Key, recs []fact.Record) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	env := envelope{V: envelopeVersion, Tool: k.Tool, Period: k.Period, Args: k.Args, Records: recs}
	if env.Args == nil {
		env.Args = []string{} // encode as [] not null
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, k.Filename()), raw, 0o644)
}
