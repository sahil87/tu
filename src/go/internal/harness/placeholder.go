package harness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// PlaceholderVersion is the ccusage version the placeholder shapes were
// derived from (the version live-probed on dev-ws-sahil02 on 2026-09-16).
const PlaceholderVersion = "20.0.19"

// PlaceholderDerivedFrom records where the placeholder schemas come from. Real
// captures carry the capturing user's spend and are never committed (they stay
// under a gitignored alias directory), so the committed corpus is placeholder-
// only and this string is provenance, not a path into the repository.
const PlaceholderDerivedFrom = "live ccusage 20.0.19 `<agent> daily --json` output on dev-ws-sahil02, 2026-09-16 (real captures are local-only, never committed)"

// PlaceholderDates are the fixed synthetic days: obviously fake, and sorting
// before any real 2026-08+ data.
var PlaceholderDates = []string{"2026-01-05", "2026-01-06", "2026-01-07"}

// The observed ccusage v20 per-agent claude-style daily shape (claude, gemini,
// kimi, and the empty opencode/copilot totals). Fields are declared
// alphabetically so encoding/json emits keys in the same order the ccusage
// serializer uses.
type (
	// ModelBreakdown is one model's contribution to a day.
	ModelBreakdown struct {
		CacheCreationTokens int     `json:"cacheCreationTokens"`
		CacheReadTokens     int     `json:"cacheReadTokens"`
		Cost                float64 `json:"cost"`
		InputTokens         int     `json:"inputTokens"`
		ModelName           string  `json:"modelName"`
		OutputTokens        int     `json:"outputTokens"`
	}
	// DailyEntry is one day of per-agent usage.
	DailyEntry struct {
		CacheCreationTokens int              `json:"cacheCreationTokens"`
		CacheReadTokens     int              `json:"cacheReadTokens"`
		Date                string           `json:"date"`
		InputTokens         int              `json:"inputTokens"`
		ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
		ModelsUsed          []string         `json:"modelsUsed"`
		OutputTokens        int              `json:"outputTokens"`
		TotalCost           float64          `json:"totalCost"`
		TotalTokens         int              `json:"totalTokens"`
	}
	// Totals is the whole-report sum block.
	Totals struct {
		CacheCreationTokens int     `json:"cacheCreationTokens"`
		CacheReadTokens     int     `json:"cacheReadTokens"`
		InputTokens         int     `json:"inputTokens"`
		OutputTokens        int     `json:"outputTokens"`
		TotalCost           float64 `json:"totalCost"`
		TotalTokens         int     `json:"totalTokens"`
	}
	// DailyReport is the top-level ccusage daily --json document.
	DailyReport struct {
		Daily  []DailyEntry `json:"daily"`
		Totals Totals       `json:"totals"`
	}
)

// The observed ccusage v20 codex daily shape — the one outlier: cost is
// `costUSD`, per-model usage is a `models` map keyed by model name, and both
// entries and totals carry `reasoningOutputTokens`.
type (
	// CodexModelUsage is one model's contribution to a codex day.
	CodexModelUsage struct {
		CacheCreationTokens   int  `json:"cacheCreationTokens"`
		CacheReadTokens       int  `json:"cacheReadTokens"`
		InputTokens           int  `json:"inputTokens"`
		IsFallback            bool `json:"isFallback"`
		OutputTokens          int  `json:"outputTokens"`
		ReasoningOutputTokens int  `json:"reasoningOutputTokens"`
		TotalTokens           int  `json:"totalTokens"`
	}
	// CodexDailyEntry is one day of codex usage.
	CodexDailyEntry struct {
		CacheCreationTokens   int                        `json:"cacheCreationTokens"`
		CacheReadTokens       int                        `json:"cacheReadTokens"`
		CostUSD               float64                    `json:"costUSD"`
		Date                  string                     `json:"date"`
		InputTokens           int                        `json:"inputTokens"`
		Models                map[string]CodexModelUsage `json:"models"`
		OutputTokens          int                        `json:"outputTokens"`
		ReasoningOutputTokens int                        `json:"reasoningOutputTokens"`
		TotalTokens           int                        `json:"totalTokens"`
	}
	// CodexTotals is the codex whole-report sum block.
	CodexTotals struct {
		CacheCreationTokens   int     `json:"cacheCreationTokens"`
		CacheReadTokens       int     `json:"cacheReadTokens"`
		CostUSD               float64 `json:"costUSD"`
		InputTokens           int     `json:"inputTokens"`
		OutputTokens          int     `json:"outputTokens"`
		ReasoningOutputTokens int     `json:"reasoningOutputTokens"`
		TotalTokens           int     `json:"totalTokens"`
	}
	// CodexDailyReport is the top-level `ccusage codex daily --json` document.
	CodexDailyReport struct {
		Daily  []CodexDailyEntry `json:"daily"`
		Totals CodexTotals       `json:"totals"`
	}
)

// Fixed synthetic counters shared by every placeholder day.
const (
	placeholderCacheCreation = 1000
	placeholderCacheRead     = 20000
	placeholderInput         = 3000
	placeholderOutput        = 400
	placeholderReasoning     = 100 // codex only; counted inside outputTokens upstream, so not added to totalTokens
	placeholderCost          = 0.5
	placeholderTotalTokens   = placeholderCacheCreation + placeholderCacheRead + placeholderInput + placeholderOutput
)

// PlaceholderFor returns the deterministic placeholder report for a source in
// that source's observed shape: codex-style for `codex`, claude-style for
// everything else.
func PlaceholderFor(source string) any {
	if source == "codex" {
		return PlaceholderCodex()
	}
	return Placeholder(source)
}

// Placeholder builds the deterministic claude-style placeholder report for a
// source: the three fixed days with fixed counters, totals equal to the column
// sums, and totalTokens equal to the sum of the four token counters in every
// entry and in the totals — internally consistent so tu's aggregation can be
// checked against it.
func Placeholder(source string) DailyReport {
	model := "placeholder-" + source + "-model"
	report := DailyReport{}
	for _, date := range PlaceholderDates {
		report.Daily = append(report.Daily, DailyEntry{
			CacheCreationTokens: placeholderCacheCreation,
			CacheReadTokens:     placeholderCacheRead,
			Date:                date,
			InputTokens:         placeholderInput,
			ModelBreakdowns: []ModelBreakdown{{
				CacheCreationTokens: placeholderCacheCreation,
				CacheReadTokens:     placeholderCacheRead,
				Cost:                placeholderCost,
				InputTokens:         placeholderInput,
				ModelName:           model,
				OutputTokens:        placeholderOutput,
			}},
			ModelsUsed:   []string{model},
			OutputTokens: placeholderOutput,
			TotalCost:    placeholderCost,
			TotalTokens:  placeholderTotalTokens,
		})
	}
	for _, e := range report.Daily {
		report.Totals.CacheCreationTokens += e.CacheCreationTokens
		report.Totals.CacheReadTokens += e.CacheReadTokens
		report.Totals.InputTokens += e.InputTokens
		report.Totals.OutputTokens += e.OutputTokens
		report.Totals.TotalCost += e.TotalCost
		report.Totals.TotalTokens += e.TotalTokens
	}
	return report
}

// PlaceholderCodex builds the deterministic codex-style placeholder report,
// with the same consistency guarantees as Placeholder.
func PlaceholderCodex() CodexDailyReport {
	const model = "placeholder-codex-model"
	report := CodexDailyReport{}
	for _, date := range PlaceholderDates {
		report.Daily = append(report.Daily, CodexDailyEntry{
			CacheCreationTokens: placeholderCacheCreation,
			CacheReadTokens:     placeholderCacheRead,
			CostUSD:             placeholderCost,
			Date:                date,
			InputTokens:         placeholderInput,
			Models: map[string]CodexModelUsage{model: {
				CacheCreationTokens:   placeholderCacheCreation,
				CacheReadTokens:       placeholderCacheRead,
				InputTokens:           placeholderInput,
				IsFallback:            false,
				OutputTokens:          placeholderOutput,
				ReasoningOutputTokens: placeholderReasoning,
				TotalTokens:           placeholderTotalTokens,
			}},
			OutputTokens:          placeholderOutput,
			ReasoningOutputTokens: placeholderReasoning,
			TotalTokens:           placeholderTotalTokens,
		})
	}
	for _, e := range report.Daily {
		report.Totals.CacheCreationTokens += e.CacheCreationTokens
		report.Totals.CacheReadTokens += e.CacheReadTokens
		report.Totals.CostUSD += e.CostUSD
		report.Totals.InputTokens += e.InputTokens
		report.Totals.OutputTokens += e.OutputTokens
		report.Totals.ReasoningOutputTokens += e.ReasoningOutputTokens
		report.Totals.TotalTokens += e.TotalTokens
	}
	return report
}

// EncodePretty renders v as 2-space-indented JSON with a trailing newline —
// the same layout ccusage's serializer uses.
func EncodePretty(v any) ([]byte, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// ConfirmedLedgerFile is the per-alias ledger of human-confirmed placeholder
// shapes (plan row P3b), read by WritePlaceholders from <out>/confirmed.json
// and committed alongside the placeholder corpus. It is the only hand-written
// file under _placeholder/: manifest.json is regenerated from the fixtures
// plus this ledger, so a confirmation recorded here survives regeneration
// where a hand-edited manifest flag would not.
const ConfirmedLedgerFile = "confirmed.json"

// ledgerDate is the accepted shape of a ConfirmedBy.Date: provenance, so a
// plain YYYY-MM-DD pattern rather than a calendar parse.
var ledgerDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ReadConfirmed loads <dir>/confirmed.json: a JSON object keyed by ccusage
// source name whose values carry machine, date, and ccusage_version. A
// missing file is an empty ledger, not an error. Malformed JSON, a key that
// is not one of DefaultSources, a missing or malformed field, or an unknown
// field is an error — the ledger is hand-edited and a typo must fail loudly
// rather than leave a source silently unconfirmed.
func ReadConfirmed(dir string) (map[string]ConfirmedBy, error) {
	path := filepath.Join(dir, ConfirmedLedgerFile)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]ConfirmedBy{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tudiff: %s: %w", ConfirmedLedgerFile, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	ledger := map[string]ConfirmedBy{}
	if err := dec.Decode(&ledger); err != nil {
		return nil, fmt.Errorf("tudiff: %s: %w", ConfirmedLedgerFile, err)
	}
	for source, cb := range ledger {
		if !slices.Contains(DefaultSources, source) {
			return nil, fmt.Errorf("tudiff: %s: unknown source %q (known: %s)", ConfirmedLedgerFile, source, strings.Join(DefaultSources, ", "))
		}
		switch {
		case cb.Machine == "":
			return nil, fmt.Errorf("tudiff: %s: %s: machine is required", ConfirmedLedgerFile, source)
		case cb.Date == "":
			return nil, fmt.Errorf("tudiff: %s: %s: date is required", ConfirmedLedgerFile, source)
		case !ledgerDate.MatchString(cb.Date):
			return nil, fmt.Errorf("tudiff: %s: %s: date %q must be YYYY-MM-DD", ConfirmedLedgerFile, source, cb.Date)
		case cb.CcusageVersion == "":
			return nil, fmt.Errorf("tudiff: %s: %s: ccusage_version is required", ConfirmedLedgerFile, source)
		}
	}
	return ledger, nil
}

// WritePlaceholders writes <out>/<source>/daily.json for each source plus a
// regenerated <out>/manifest.json. Every entry is unconfirmed unless its
// source is listed in <out>/confirmed.json (ReadConfirmed), in which case it
// is unconfirmed: false with that ledger entry as confirmed_by. A ledger
// error aborts before any file is written. Real captures (any alias other
// than PlaceholderAlias) are local-only and may coexist with the placeholder
// corpus; the replayer's ordered TUDIFF_FIXTURES list decides which wins at
// replay time.
func WritePlaceholders(out string, sources []string) error {
	// Refuse to overwrite a real alias's corpus: a mistaken --out pointing at
	// a capture directory would clobber its fixtures.
	if m, err := ReadManifest(out); err == nil {
		if m.Machine != PlaceholderAlias {
			return fmt.Errorf("tudiff: %s already holds the captured corpus of %s — refusing to overwrite it with placeholders", out, m.Machine)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	ledger, err := ReadConfirmed(out)
	if err != nil {
		return err
	}

	m := &Manifest{
		Schema:         SchemaVersion,
		Machine:        PlaceholderAlias,
		CapturedAt:     time.Now().UTC().Format(time.RFC3339),
		CcusageVersion: PlaceholderVersion,
		CcusagePath:    "",
		Platform:       "derived",
		Timezone:       "",
		DerivedFrom:    PlaceholderDerivedFrom,
	}
	for _, source := range sources {
		raw, err := EncodePretty(PlaceholderFor(source))
		if err != nil {
			return err
		}
		fx := Fixture{
			Source:      source,
			Period:      "daily",
			Args:        []string{"--json"},
			File:        FixturePath(source, "daily"),
			Unconfirmed: true,
		}
		if cb, ok := ledger[source]; ok {
			fx.Unconfirmed = false
			fx.ConfirmedBy = &cb
		}
		fx.Days, fx.FirstDate, fx.LastDate, fx.Empty = Summarize(raw)
		sum := sha256.Sum256(raw)
		fx.Sha256 = hex.EncodeToString(sum[:])

		path := filepath.Join(out, filepath.FromSlash(fx.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("tudiff: cannot create placeholder dir for %s: %w", path, err)
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return fmt.Errorf("tudiff: cannot write placeholder %s: %w", path, err)
		}
		m.Fixtures = append(m.Fixtures, fx)
	}
	return WriteManifest(out, m)
}
