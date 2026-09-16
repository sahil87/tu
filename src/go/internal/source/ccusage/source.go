package ccusage

import (
	"context"
	"sync"

	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/source"
	"github.com/sahil87/tu/internal/source/cache"
)

// Source fetches usage records from one ccusage binary.
type Source struct {
	Binary  string       // "" → ResolveBinary()
	User    string       // stamped on every returned record
	Machine string       // stamped on every returned record
	Cache   *cache.Store // nil → no caching
}

// Fetch returns tool's records for period. On any failure it returns
// (nil, err) — the caller decides what "zero data" means; nothing is printed
// or written anywhere by this package.
//
// Order of operations (mirroring the TS fetchHistory): cache read (skipped
// when fresh) → resolve binary → run → parse → cache write → stamp. No cache
// write happens on an exec failure, a parse failure, or an empty result —
// the TS returns before writeCache in all three cases. fresh skips the read
// but still writes.
func (s *Source) Fetch(ctx context.Context, tool fact.Tool, period string, extraArgs []string, fresh bool) ([]fact.Record, *source.Error) {
	key := cache.Key{Tool: tool.Key, Period: period, Args: extraArgs}
	if !fresh && s.Cache != nil {
		if records, ok := s.Cache.Get(key); ok {
			return stamp(records, s.User, s.Machine), nil
		}
	}

	binary := s.Binary
	if binary == "" {
		resolved, err := ResolveBinary()
		if err != nil {
			return nil, &source.Error{
				Tool: tool.Key, Name: tool.Name, Kind: source.KindExec,
				Detail: "spawn ccusage ENOENT",
				Err:    err,
			}
		}
		binary = resolved
	}

	raw, serr := run(ctx, tool, binary, argv(tool, period, extraArgs))
	if serr != nil {
		return nil, serr
	}
	records, serr := Parse(raw, tool)
	if serr != nil {
		return nil, serr
	}
	if len(records) == 0 {
		return records, nil
	}
	if s.Cache != nil {
		// Put errors are ignored by design: a cache write failure must not
		// fail an otherwise successful fetch (graceful degradation). Records
		// are cached unstamped; the key excludes identity.
		_ = s.Cache.Put(key, records)
	}
	return stamp(records, s.User, s.Machine), nil
}

// FetchAll fetches every tool in fact.Tools concurrently (one goroutine
// each) and returns the records concatenated in registry order followed by
// the non-nil errors, also in registry order. It never stops early: one
// failing tool does not cancel the others, and a failed tool contributes no
// records.
func (s *Source) FetchAll(ctx context.Context, period string, extraArgs []string, fresh bool) ([]fact.Record, []*source.Error) {
	recSlots := make([][]fact.Record, len(fact.Tools))
	errSlots := make([]*source.Error, len(fact.Tools))

	var wg sync.WaitGroup
	for i, tool := range fact.Tools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recSlots[i], errSlots[i] = s.Fetch(ctx, tool, period, extraArgs, fresh)
		}()
	}
	wg.Wait()

	var records []fact.Record
	var errs []*source.Error
	for i := range fact.Tools {
		records = append(records, recSlots[i]...)
		if errSlots[i] != nil {
			errs = append(errs, errSlots[i])
		}
	}
	return records, errs
}

// stamp sets User/Machine on every record in place and returns the slice.
// Cached records are stored unstamped, so stamping happens on every return
// path, after the cache write.
func stamp(records []fact.Record, user, machine string) []fact.Record {
	for i := range records {
		records[i].User = user
		records[i].Machine = machine
	}
	return records
}
