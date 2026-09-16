package source

import "time"

// PeriodDaily is the one period every adapter is asked for: tu fetches daily
// records only and rolls up client-side (query.RollUp).
const PeriodDaily = "daily"

// DefaultTimeout is the per-invocation deadline the command edge puts on the
// context it hands to a Fetcher. Adapters impose no deadline of their own;
// they honor the given context only.
const DefaultTimeout = 120 * time.Second
