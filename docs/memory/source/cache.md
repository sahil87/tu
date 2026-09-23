---
type: memory
description: The on-disk fetch cache — cache.Key over tool+period+args hashed into {tool}-{period}-{sha256[:16]}.json, a verified version-1 envelope, the 60-second TTL checked by file mtime, the $HOME/.tu/cache location under the state dir, and the --fresh bypass semantics.
---

# Fetch Cache

**Domain**: source

## Overview

`internal/source/cache` is the on-disk JSON cache behind live source fetches: one file per `(tool, period, args)` key, named by a truncated sha256 of the key, with a 60-second TTL checked via file mtime. `cmd/tu` builds the store with `cache.Default()`; the ccusage adapter consults it inside `Fetch` (see [ccusage-adapter](/source/ccusage-adapter.md)).

## Requirements

### Requirement: Key and filename
`cache.Key{Tool, Period, Args}` identifies one cached fetch. `Key.Filename()` MUST be `{tool}-{period}-{h}.json` where `h` is the first 16 hex characters of `sha256(tool + "\x00" + period + "\x00" + strings.Join(args, "\x00"))`. The hash alone is the key; the readable prefix is a courtesy for inspecting the cache directory.

#### Scenario: Filename stability and distinctness
- GIVEN `Key{Tool: "cc", Period: "daily"}`
- WHEN `Filename()` is computed
- THEN it is stable across calls, has the prefix `cc-daily-` and suffix `.json`, and differs from the same key with extra args, another tool's key, and another period's key

### Requirement: Envelope schema and read verification
`Store.Put` SHALL write the envelope `{v: 1, tool, period, args, records}` — empty `args` encodes as `[]`, not `null` — creating `Dir` with mode 0755 and writing the file with mode 0644; records are stored unstamped (empty `User`/`Machine`). `Store.Get` MUST verify the envelope's `v`, `tool`, `period`, and `args` against the key: a hash collision or stale schema is a miss, never a wrong answer.

#### Scenario: Envelope mismatch misses
- GIVEN a file whose envelope says `tool: codex` written under the `cc` key's filename
- WHEN `Get` runs for the `cc` key
- THEN it misses

### Requirement: TTL by modification time
`cache.TTL` SHALL be `60 * time.Second`. `Get` MUST miss when the file's age — `Now() − mtime` — exceeds the TTL. `Store{Dir, TTL, Now}` injects clock and lifetime: a zero `Store.TTL` means the package `TTL`, a nil `Now` means `time.Now`.

#### Scenario: Expiry
- GIVEN a stored entry and a controllable `Now`
- WHEN `Now` advances by 61 s
- THEN `Get` misses

### Requirement: Location under the state dir
`cache.Default()` MUST return the store at `$HOME/.tu/cache` — under the state dir `config.StateDir(home)` = `$HOME/.tu` (see [cascade](/config/cascade.md)) — with the package TTL and the real clock, or an error when `$HOME` is empty (the config-home failure is the edge's to report; a nil store means no caching).

### Requirement: Miss conditions
`Get` MUST return `(nil, false)` when the file is absent, older than the TTL, undecodable, or fails envelope verification. None of these is an error.

### Requirement: --fresh bypass semantics
`ccusage.Source.Fetch(ctx, tool, period, extraArgs, fresh)` with `fresh = true` SHALL skip the cache read but still write the new result. The command edge passes `req.Flags.Fresh` from `--fresh`/`-f` (see [run-and-result](/command/run-and-result.md)); the watch loop forces `fresh` on every poll.

#### Scenario: Fresh re-invokes and rewrites
- GIVEN a warm cache entry
- WHEN `Fetch` runs with `fresh = true`
- THEN the binary is invoked again and the cache file's mtime is refreshed

## Design Decisions

### 60-second TTL checked by mtime
**Decision**: the cache lifetime is 60 s, checked against the file's modification time rather than a timestamp inside the envelope.
**Why**: re-fetching means re-scanning hundreds of MB of transcript files through ccusage; 60 s balances freshness against that cost, and mtime keeps the envelope free of a redundant clock field.
**Rejected**: no cache (every invocation re-scans); an in-envelope timestamp (a second source of truth for expiry).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Hash-keyed filename with a readable prefix
**Decision**: the filename is `{tool}-{period}-{sha256[:16]}.json` over the NUL-joined key.
**Why**: the prefix keeps `ls ~/.tu/cache` inspectable while the truncated hash guarantees distinct keys; the envelope is verified on read, so the prefix is never load-bearing.
**Rejected**: a hash-only filename (opaque when debugging the cache); a fully readable filename without a hash (args would need path escaping and could collide).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Envelope verified on read
**Decision**: `Get` re-checks `v`, `tool`, `period`, and `args` inside the file against the key.
**Why**: a hash collision or a stale schema version must be a miss, never a wrong answer.
**Rejected**: trusting the filename (a collision would serve the wrong data silently).
*Introduced by*: 260916-v0as-fact-source-ccusage

### Records stored unstamped
**Decision**: `Put` receives records with empty `User`/`Machine`; the adapter stamps on every return path.
**Why**: the key excludes identity, so a config change inside the TTL window must not serve a stale identity.
**Rejected**: including user/machine in the key (invalidates the cache on every identity change for data that did not change).
*Introduced by*: 260916-v0as-fact-source-ccusage

### --fresh skips the read but still writes
**Decision**: a fresh fetch bypasses the lookup and rewrites the entry.
**Why**: `--fresh` asks for new data, not for a cold cache; writing keeps the next non-fresh invocation warm.
**Rejected**: skipping the write too (every `--fresh` run would leave the cache stale for the next run).
*Introduced by*: 260916-v0as-fact-source-ccusage
