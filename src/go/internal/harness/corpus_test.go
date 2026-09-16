package harness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusRoot is the committed fixture corpus, resolved from this package's
// directory (Go tests run with cwd = package dir): src/go/internal/harness is
// four levels below the repo root.
const corpusRoot = "../../../../harness/fixtures"

// TestCorpus validates every manifest and fixture under harness/fixtures/ —
// the committed placeholder corpus plus any local (gitignored) real capture.
// It fails — never skips — when the corpus is absent, because the placeholder
// corpus is a committed deliverable of this repository.
func TestCorpus(t *testing.T) {
	if _, err := os.Stat(corpusRoot); err != nil {
		t.Fatalf("fixture corpus missing at %s: %v", corpusRoot, err)
	}
	manifestPaths, err := filepath.Glob(filepath.Join(corpusRoot, "*", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifestPaths) == 0 {
		t.Fatalf("no manifests under %s", corpusRoot)
	}

	for _, path := range manifestPaths {
		dir := filepath.Dir(path)
		alias := filepath.Base(dir)
		m, err := ReadManifest(dir)
		if err != nil {
			t.Errorf("%s: unreadable manifest: %v", alias, err)
			continue
		}

		if alias == PlaceholderAlias {
			assertLedgerAgreement(t, dir, m)
		}

		for _, fx := range m.Fixtures {
			prefix := alias + "/" + fx.File

			raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(fx.File)))
			if err != nil {
				t.Errorf("%s: fixture file missing: %v", prefix, err)
				continue
			}

			sum := sha256.Sum256(raw)
			if got := hex.EncodeToString(sum[:]); got != fx.Sha256 {
				t.Errorf("%s: sha256 mismatch (manifest %s, content %s) — regenerate the manifest", prefix, fx.Sha256, got)
			}

			if fx.ExitCode == 0 {
				assertCorpusContent(t, prefix, raw, fx)
			}

			assertNoLeakedPaths(t, prefix, raw)

			if fx.Unconfirmed && alias != PlaceholderAlias {
				t.Errorf("%s: unconfirmed: true outside %s/", prefix, PlaceholderAlias)
			}
			if fx.ConfirmedBy != nil && alias != PlaceholderAlias {
				t.Errorf("%s: confirmed_by outside %s/ — a real capture is confirmed by being real, not by the ledger", prefix, PlaceholderAlias)
			}
		}
	}
}

// assertLedgerAgreement checks that the placeholder manifest is what the
// generator would produce from the committed confirmed.json: every listed
// source is unconfirmed: false with that exact confirmed_by, every unlisted
// source is unconfirmed: true with none, and no ledger key is missing from
// the corpus. A mismatch means one side was edited without regenerating.
func assertLedgerAgreement(t *testing.T, dir string, m *Manifest) {
	t.Helper()
	ledger, err := ReadConfirmed(dir)
	if err != nil {
		t.Errorf("%s: %v", PlaceholderAlias, err)
		return
	}
	seen := map[string]bool{}
	for _, fx := range m.Fixtures {
		prefix := PlaceholderAlias + "/" + fx.File
		if fx.Period == "daily" {
			seen[fx.Source] = true
		}
		cb, listed := ledger[fx.Source]
		switch {
		case listed && (fx.Unconfirmed || fx.ConfirmedBy == nil || *fx.ConfirmedBy != cb):
			t.Errorf("%s: manifest disagrees with %s (unconfirmed=%v confirmed_by=%+v, ledger %+v) — re-run tudiff placeholder", prefix, ConfirmedLedgerFile, fx.Unconfirmed, fx.ConfirmedBy, cb)
		case !listed && (!fx.Unconfirmed || fx.ConfirmedBy != nil):
			t.Errorf("%s: confirmed in the manifest but not listed in %s — re-run tudiff placeholder", prefix, ConfirmedLedgerFile)
		}
	}
	for source := range ledger {
		if !seen[source] {
			t.Errorf("%s/%s: source %q is confirmed but has no daily fixture in the corpus — re-run tudiff placeholder", PlaceholderAlias, ConfirmedLedgerFile, source)
		}
	}
}

// assertCorpusContent checks that the manifest's derived fields agree with
// the recorded content of a successful capture.
func assertCorpusContent(t *testing.T, prefix string, raw []byte, fx Fixture) {
	t.Helper()
	var report struct {
		Daily []struct {
			Date string `json:"date"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Errorf("%s: exit_code 0 but content is not JSON: %v", prefix, err)
		return
	}
	days := len(report.Daily)
	if days != fx.Days {
		t.Errorf("%s: manifest days=%d, content has %d", prefix, fx.Days, days)
	}
	if days == 0 {
		if !fx.Empty {
			t.Errorf("%s: content has no days but manifest empty=false", prefix)
		}
		return
	}
	if fx.Empty {
		t.Errorf("%s: content has %d days but manifest empty=true", prefix, days)
	}
	if got := report.Daily[0].Date; got != fx.FirstDate {
		t.Errorf("%s: first_date=%s, content starts %s", prefix, fx.FirstDate, got)
	}
	if got := report.Daily[days-1].Date; got != fx.LastDate {
		t.Errorf("%s: last_date=%s, content ends %s", prefix, fx.LastDate, got)
	}
}

// assertNoLeakedPaths enforces the redaction contract on committed bytes: no
// absolute home-rooted path may survive into the corpus.
func assertNoLeakedPaths(t *testing.T, prefix string, raw []byte) {
	t.Helper()
	var walk func(v any)
	walk = func(v any) {
		switch vv := v.(type) {
		case map[string]any:
			for _, x := range vv {
				walk(x)
			}
		case []any:
			for _, x := range vv {
				walk(x)
			}
		case string:
			if strings.Contains(vv, "/home/") || strings.Contains(vv, "/Users/") {
				t.Errorf("%s: unredacted home path %q", prefix, vv)
			}
		}
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		// A failed capture is stored as non-JSON stdout; there are no string
		// values to walk, so scan the raw bytes — an unredacted home path
		// must not slip past the leak check on the error path.
		if bytes.Contains(raw, []byte("/home/")) || bytes.Contains(raw, []byte("/Users/")) {
			t.Errorf("%s: unredacted home path in non-JSON content", prefix)
		}
		return
	}
	walk(doc)
}
