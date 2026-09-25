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

// Exit codes — the shll toolkit convention (spec § Exit Codes): 0 = success,
// 1 = operational failure (well-formed invocation that could not complete;
// also the placeholder), 2 = usage error (the invocation itself was wrong).
// cmd/tu returns only these values.
const (
	ExitOK          = 0
	ExitOperational = 1
	ExitUsage       = 2
)

// Request is the parsed invocation. Source "" means all tools (aliases
// resolved); Command holds the first positional when it is a non-data command
// ("" for data commands); Args carries the positionals after Command (nil for
// data commands — B8 reads shell-init's arg from here); Version is set when a
// version flag appears anywhere.
type Request struct {
	Source  string
	Period  query.Period
	Display Display
	Format  Format
	Flags   Flags
	Command string
	Args    []string
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

// FullHelp is the byte-exact FULL_HELP constant of the retired TypeScript
// implementation —
// the full help text `tu help`/`tu -h`/`tu --help` print. `help-dump` is
// hidden and appears nowhere in it. cmd/tu prints it with Fprintln (the TS
// console.log appends exactly one newline).
const FullHelp = `Usage: tu [source] [period] [display]

Sources: cc (Claude Code), codex/co (Codex), oc (OpenCode), gemini/gem (Gemini), copilot/cop (Copilot), kimi/ki (Kimi), all (default)
Periods: d/daily (default), w/weekly, m/monthly
Display: (bare) = snapshot, h/history = history, lb = leaderboard, lbh = leaderboard history
Combined: dh (daily history), wh (weekly history), mh (monthly history)

Examples:
  tu                   Today's cost, all tools (snapshot)
  tu cc                Today's cost, Claude Code
  tu h                 Daily cost history, all tools (pivot)
  tu cc mh             Monthly cost history, Claude Code
  tu wh                Weekly cost history, all tools
  tu m                 This month's cost, all tools
  tu m lb              This month's leaderboard — users ranked by cost (multi mode)
  tu lbh               Daily leaderboard history — rows x user columns (multi mode)

Setup:
  tu init-conf         Scaffold ~/.config/tu/tu.conf
  tu init-metrics [url] Clone metrics repo (url also sets metrics_repo)
  tu sync              Push/pull metrics manually
  tu status            Show config and sync state
  tu update            Update tu to latest version
  tu shell-init <sh>   Emit shell init script (bash/zsh/fish)
  tu skill             Print agent usage bundle (markdown)

Help: tu help | tu -h | tu --help

Flags:
  --json / -j          Output data as JSON (data commands only)
  --csv                Output data as CSV (data commands only)
  --md                 Output data as Markdown (data commands only)
  --since / -s <date>  Only include entries on/after date (YYYY-MM-DD or YYYYMMDD, history display)
  --until <date>       Only include entries on/before date (YYYY-MM-DD or YYYYMMDD, history display)
  --full               Show full history (default: last 3 months for daily/weekly history)
  --metric <m>         Show 'cost' (default) or 'tokens' in table cells, bars and footer stats (snapshot keeps its Cost column in dollars)
  -t                   Shorthand for --metric tokens
  --top <n>            Show only the top N rows/columns on the lb/lbh leaderboard
  --sync               Sync metrics before fetching (multi mode)
  --dry-run            Preview sync without writing (tu sync only)
  --fresh / -f         Bypass cache, fetch fresh data (data commands only)
  --watch / -w         Persistent polling mode with live display (data commands only)
  --interval / -i <s>  Poll interval in seconds (default: 10, range: 5-3600)
  --user / -u <user>   Show usage for a specific user, or 'all' for every user
                       in the metrics repo (multi mode only; repo data — sync for today)
  --by-machine         Show per-machine cost breakdown (data commands only)
  --skip-brew-update   Skip 'brew update' tap refresh during 'tu update'
  --no-color           Disable ANSI color output
  --no-rain            Disable matrix rain animation in watch mode`
