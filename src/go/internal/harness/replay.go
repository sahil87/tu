package harness

import (
	"os"
	"path/filepath"
)

// Corpus is an ordered list of fixture alias directories (first hit wins),
// the lookup structure behind the fake ccusage.
type Corpus struct {
	aliases []aliasCorpus
}

type aliasCorpus struct {
	name     string
	dir      string
	manifest *Manifest
}

// Hit is a resolved fixture: the recorded stdout/stderr bytes, the recorded
// exit code, and the "<alias>/<file>" label for the call log.
type Hit struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Matched  string
}

// LoadCorpus reads the manifest of each dir in order. A directory without a
// manifest is skipped (it is not an alias dir); a manifest that exists but
// fails to parse is an error.
func LoadCorpus(dirs []string) (*Corpus, error) {
	c := &Corpus{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		m, err := ReadManifest(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		c.aliases = append(c.aliases, aliasCorpus{name: filepath.Base(dir), dir: dir, manifest: m})
	}
	return c, nil
}

// Lookup resolves (source, period, flags) against the corpus in order; flags
// are order-insensitive (sorted inside Key). A manifest entry matches only on
// exact key equality — a miss is deliberately loud in the caller.
func (c *Corpus) Lookup(source, period string, flags []string) (Hit, bool) {
	key := Key(source, period, flags)
	for _, a := range c.aliases {
		for _, fx := range a.manifest.Fixtures {
			if Key(fx.Source, fx.Period, fx.Args) != key {
				continue
			}
			hit := Hit{ExitCode: fx.ExitCode, Matched: a.name + "/" + fx.File}
			raw, err := os.ReadFile(filepath.Join(a.dir, filepath.FromSlash(fx.File)))
			if err != nil {
				continue // manifest/file mismatch — try the next alias
			}
			hit.Stdout = raw
			if fx.StderrFile != "" {
				sraw, err := os.ReadFile(filepath.Join(a.dir, filepath.FromSlash(fx.StderrFile)))
				if err != nil {
					continue // stderr sidecar missing — manifest/fixture mismatch, try the next alias
				}
				hit.Stderr = sraw
			}
			return hit, true
		}
	}
	return Hit{}, false
}

// Version is the ccusage_version recorded in the first manifest found, for
// the fake's `--version`.
func (c *Corpus) Version() string {
	if len(c.aliases) == 0 {
		return ""
	}
	return c.aliases[0].manifest.CcusageVersion
}
