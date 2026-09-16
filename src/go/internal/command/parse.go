package command

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/sahil87/tu/internal/query"
)

// boolFlags are stripped from the positional list (the TS parseGlobalFlags
// skip list).
var boolFlags = map[string]bool{
	"--json": true, "-j": true, "--csv": true, "--md": true,
	"--sync": true, "--dry-run": true, "--fresh": true, "-f": true,
	"--watch": true, "-w": true, "--no-color": true, "--no-rain": true,
	"--by-machine": true, "--full": true, "--skip-brew-update": true, "-t": true,
}

// digitsOnly matches the value shapes --interval consumes.
var digitsOnly = regexp.MustCompile(`^\d+$`)

// dashedDate / compactDate are the --since/--until value shapes
// (normalizeDateFlag). Validation is shape-only — a well-shaped impossible
// date parses and yields an empty window.
var (
	dashedDate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	compactDate = regexp.MustCompile(`^\d{8}$`)
)

// sourceAliases map the short source tokens to registry keys.
var sourceAliases = map[string]string{"co": "codex", "gem": "gemini", "cop": "copilot", "ki": "kimi"}

// knownSources are the first-positional source tokens.
var knownSources = map[string]bool{
	"cc": true, "codex": true, "co": true, "oc": true,
	"gemini": true, "gem": true, "copilot": true, "cop": true,
	"kimi": true, "ki": true, "all": true,
}

// nonDataCommands are the first-positional non-data tokens (dispatched before
// grammar parsing in the TS).
var nonDataCommands = map[string]bool{
	"init-conf": true, "init-metrics": true, "sync": true, "status": true,
	"update": true, "shell-init": true, "help-dump": true, "skill": true,
}

// helpCommands are the help tokens; the TS checks them before the --dry-run
// guard, so they are dispatched first.
var helpCommands = map[string]bool{"help": true, "-h": true, "--help": true}

// flagScan is the raw result of the flag pass: booleans by raw-argv
// membership, value flags by presence + raw value, everything else positional.
type flagScan struct {
	json, csv, md, sync, dryRun, fresh, watch, noColor, noRain, byMachine, full, skipBrew, tokensShort bool
	hasInterval                                                                                        bool
	rawInterval                                                                                        *string
	hasUser                                                                                            bool
	userVal                                                                                            *string
	hasSince                                                                                           bool
	rawSince                                                                                           *string
	hasUntil                                                                                           bool
	rawUntil                                                                                           *string
	hasMetric                                                                                          bool
	rawMetric                                                                                          *string
	hasTop                                                                                             bool
	rawTop                                                                                             *string
	positionals                                                                                        []string
}

// Parse implements the complete TS grammar: parseGlobalFlags (flag pass +
// validation), the version check, the non-data command tokens, and
// parseDataArgs (positionals), in the TS order so the same argv produces the
// same first error.
func Parse(args []string) (Request, *UsageError) {
	scan := scanFlags(args)

	if err := validate(scan); err != nil {
		return Request{}, err
	}

	req := Request{Flags: Flags{
		Fresh: scan.fresh, NoColor: scan.noColor, Watch: scan.watch,
		Sync: scan.sync, DryRun: scan.dryRun, ByMachine: scan.byMachine,
		Full: scan.full, NoRain: scan.noRain, SkipBrewUpdate: scan.skipBrew,
		Interval: 10,
	}}
	if scan.watch && scan.rawInterval != nil {
		req.Flags.Interval, _ = strconv.Atoi(*scan.rawInterval)
	}
	if scan.userVal != nil {
		req.Flags.User = *scan.userVal
	}
	if scan.rawSince != nil {
		req.Flags.Since = normalizeDateFlag(*scan.rawSince)
	}
	if scan.rawUntil != nil {
		req.Flags.Until = normalizeDateFlag(*scan.rawUntil)
	}
	if scan.hasMetric && *scan.rawMetric == "tokens" {
		req.Flags.Metric = Tokens
	}
	if scan.tokensShort {
		req.Flags.Metric = Tokens
	}
	if scan.hasTop {
		req.Flags.Top, _ = strconv.Atoi(*scan.rawTop)
	}
	switch {
	case scan.json:
		req.Format = JSON
	case scan.csv:
		req.Format = CSV
	case scan.md:
		req.Format = Markdown
	}

	// Version runs AFTER flag validation, as in the TS main():
	// `tu --json --csv --version` is exit 2, not a version line.
	for _, a := range args {
		if a == "--version" || a == "-V" || a == "-v" {
			req.Version = true
			return req, nil
		}
	}

	// Help first (the TS help check precedes the --dry-run guard):
	// `tu help --dry-run` prints help.
	if len(scan.positionals) > 0 && helpCommands[scan.positionals[0]] {
		req.Command = scan.positionals[0]
		return req, nil
	}

	// The --dry-run misuse guard (TS main(), after help): honored only by
	// `tu sync`; anything else carrying it is a usage error, exit 2, no usage
	// block. `tu sync --dry-run` parses to Command "sync" with DryRun set and
	// stays on the placeholder (B6).
	if req.Flags.DryRun && (len(scan.positionals) == 0 || scan.positionals[0] != "sync") {
		return Request{}, &UsageError{
			Message:   "Error: --dry-run is supported only with 'tu sync' — run 'tu sync --dry-run' to preview a sync.",
			ShowUsage: false,
		}
	}

	// Non-data commands: dispatched before grammar parsing; the positionals
	// after the command become Args (the TS dispatches on filteredArgs[0]).
	// The init-metrics arity check fires here — before $HOME is consulted,
	// matching the TS.
	if len(scan.positionals) > 0 && nonDataCommands[scan.positionals[0]] {
		req.Command = scan.positionals[0]
		if len(scan.positionals) > 1 {
			req.Args = scan.positionals[1:]
		}
		if req.Command == "init-metrics" && len(req.Args) > 1 {
			return Request{}, &UsageError{
				Message:   "Error: init-metrics takes at most one argument (repo-url)",
				ShowUsage: true,
			}
		}
		return req, nil
	}

	return parsePositionals(req, scan.positionals)
}

// scanFlags is the TS parseGlobalFlags flag pass: booleans are stripped;
// value flags consume the next token only when it qualifies (digits for
// --interval/-i; not-dash-prefixed for the others) and are remembered as
// present either way; everything else lands in the positional list —
// including --help when not first, and unknown flags like --bogus.
func scanFlags(args []string) flagScan {
	var s flagScan
	for _, a := range args {
		switch a {
		case "--json", "-j":
			s.json = true
		case "--csv":
			s.csv = true
		case "--md":
			s.md = true
		case "--sync":
			s.sync = true
		case "--dry-run":
			s.dryRun = true
		case "--fresh", "-f":
			s.fresh = true
		case "--watch", "-w":
			s.watch = true
		case "--no-color":
			s.noColor = true
		case "--no-rain":
			s.noRain = true
		case "--by-machine":
			s.byMachine = true
		case "--full":
			s.full = true
		case "--skip-brew-update":
			s.skipBrew = true
		case "-t":
			s.tokensShort = true
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if boolFlags[a] {
			continue
		}
		next := func() *string {
			if i+1 < len(args) {
				return &args[i+1]
			}
			return nil
		}
		switch a {
		case "--interval", "-i":
			s.hasInterval = true
			if v := next(); v != nil && digitsOnly.MatchString(*v) {
				s.rawInterval = v
				i++
			}
		case "--user", "-u":
			s.hasUser = true
			if v := next(); v != nil && !strings.HasPrefix(*v, "-") {
				s.userVal = v
				i++
			}
		case "--since", "-s":
			s.hasSince = true
			if v := next(); v != nil && !strings.HasPrefix(*v, "-") {
				s.rawSince = v
				i++
			}
		case "--until":
			s.hasUntil = true
			if v := next(); v != nil && !strings.HasPrefix(*v, "-") {
				s.rawUntil = v
				i++
			}
		case "--metric":
			s.hasMetric = true
			if v := next(); v != nil && !strings.HasPrefix(*v, "-") {
				s.rawMetric = v
				i++
			}
		case "--top":
			s.hasTop = true
			if v := next(); v != nil && !strings.HasPrefix(*v, "-") {
				s.rawTop = v
				i++
			}
		default:
			s.positionals = append(s.positionals, a)
		}
	}
	return s
}

// validate is the TS validation block, in the TS order, first failure wins.
// Every message is byte-exact; none shows the usage block.
func validate(s flagScan) *UsageError {
	err := func(msg string) *UsageError { return &UsageError{Message: msg} }

	if s.watch && s.hasInterval {
		if s.rawInterval == nil {
			return err("Error: --interval requires a numeric value")
		}
		num, _ := strconv.Atoi(*s.rawInterval)
		if num < 5 {
			return err("Error: --interval minimum is 5 seconds")
		}
		if num > 3600 {
			return err("Error: --interval maximum is 3600 seconds")
		}
	}

	// Output-format flags are mutually exclusive, in the TS check order.
	if s.watch && s.json {
		return err("Error: --watch and --json are incompatible")
	}
	if s.json && s.csv {
		return err("Error: --json and --csv are incompatible")
	}
	if s.json && s.md {
		return err("Error: --json and --md are incompatible")
	}
	if s.csv && s.md {
		return err("Error: --csv and --md are incompatible")
	}
	if s.watch && s.csv {
		return err("Error: --watch and --csv are incompatible")
	}
	if s.watch && s.md {
		return err("Error: --watch and --md are incompatible")
	}

	if s.hasUser && s.userVal == nil {
		return err("Error: -u requires a username")
	}

	if s.hasSince && (s.rawSince == nil || normalizeDateFlag(*s.rawSince) == "") {
		return err("Error: --since requires a date (YYYY-MM-DD or YYYYMMDD)")
	}
	if s.hasUntil && (s.rawUntil == nil || normalizeDateFlag(*s.rawUntil) == "") {
		return err("Error: --until requires a date (YYYY-MM-DD or YYYYMMDD)")
	}
	if s.hasSince && s.hasUntil && s.rawSince != nil && s.rawUntil != nil {
		if normalizeDateFlag(*s.rawSince) > normalizeDateFlag(*s.rawUntil) {
			return err("Error: --since must be on or before --until")
		}
	}

	if s.hasMetric && (s.rawMetric == nil || (*s.rawMetric != "tokens" && *s.rawMetric != "cost")) {
		return err("Error: --metric requires 'tokens' or 'cost'")
	}
	// -t is boolean sugar for --metric tokens: redundant with --metric tokens
	// (accepted silently), contradictory with an explicit --metric cost.
	if s.tokensShort && s.hasMetric && *s.rawMetric == "cost" {
		return err("Error: -t and --metric cost are incompatible")
	}

	if s.hasTop {
		if s.rawTop == nil || !digitsOnly.MatchString(*s.rawTop) {
			return err("Error: --top requires a positive integer")
		}
		if num, _ := strconv.Atoi(*s.rawTop); num < 1 {
			return err("Error: --top requires a positive integer")
		}
	}

	return nil
}

// normalizeDateFlag is the TS normalizeDateFlag: YYYY-MM-DD passes through,
// YYYYMMDD is dashed, anything else yields "" (the invalid marker).
func normalizeDateFlag(value string) string {
	if dashedDate.MatchString(value) {
		return value
	}
	if compactDate.MatchString(value) {
		return value[:4] + "-" + value[4:6] + "-" + value[6:8]
	}
	return ""
}

// parsePositionals is the TS parseDataArgs: an optional leading source token,
// then period/display tokens; anything else is an unknown argument.
func parsePositionals(req Request, positionals []string) (Request, *UsageError) {
	rest := positionals
	if len(rest) > 0 && knownSources[rest[0]] {
		tok := rest[0]
		if tok == "all" {
			req.Source = ""
		} else if alias, ok := sourceAliases[tok]; ok {
			req.Source = alias
		} else {
			req.Source = tok
		}
		rest = rest[1:]
	}
	for _, tok := range rest {
		switch tok {
		case "d", "daily":
			req.Period = query.Daily
		case "w", "weekly":
			req.Period = query.Weekly
		case "m", "monthly":
			req.Period = query.Monthly
		case "h", "history":
			req.Display = History
		case "lb":
			req.Display = Leaderboard
		case "lbh":
			req.Display = LeaderboardHistory
		case "dh":
			req.Period, req.Display = query.Daily, History
		case "wh":
			req.Period, req.Display = query.Weekly, History
		case "mh":
			req.Period, req.Display = query.Monthly, History
		default:
			return Request{}, &UsageError{Message: "Unknown argument: " + tok, ShowUsage: true}
		}
	}
	return req, nil
}
