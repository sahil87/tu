// Package command parses the tu positional grammar into a Request and
// composes the pipeline — source → query → view → render — into a Result.
// Nothing here writes to stdout/stderr or exits; cmd/tu is the only writer.
package command

import (
	"github.com/sahil87/tu/internal/query"
)

// Display is the requested display.
type Display int

const (
	Snapshot Display = iota
	History
	Leaderboard
	LeaderboardHistory
)

// Format is the output format.
type Format int

const (
	Table Format = iota
	JSON
	CSV
	Markdown
)

// Metric is the display unit (cost or tokens).
type Metric int

const (
	Cost Metric = iota
	Tokens
)

// Flags carries every parsed global flag. Interval defaults to 10 and is
// validated only when Watch. Top == 0 means unset.
type Flags struct {
	Fresh, NoColor, Watch, Sync, DryRun, ByMachine, Full, NoRain, SkipBrewUpdate bool
	Interval                                                                     int
	User, Since, Until                                                           string
	Metric                                                                       Metric
	Top                                                                          int
}

// Request is the parsed invocation. Source "" means all tools (aliases
// resolved); Command holds the first positional when it is a non-data command
// ("" for data commands); Version is set when a version flag appears anywhere.
type Request struct {
	Source  string
	Period  query.Period
	Display Display
	Format  Format
	Flags   Flags
	Command string
	Version bool
}

// UsageError is a grammar rejection: stderr message, exit 2. ShowUsage
// appends ShortUsage on stderr (the TS prints two console.error calls).
type UsageError struct {
	Message   string
	ShowUsage bool
}

// ShortUsage is the byte-exact TS SHORT_USAGE constant.
const ShortUsage = "Usage: tu [source] [period] [display]\n\n  tu                Today's cost, all tools\n  tu cc             Today's cost, Claude Code\n  tu mh             Monthly cost history, all tools\n  tu -h             Show full help\n\nRun 'tu help' for all commands."
