package sync

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sahil87/tu/internal/config"
	"github.com/sahil87/tu/internal/fact"
	"github.com/sahil87/tu/internal/render"
)

// ToolReport is the TS ToolWriteReport: one tool's write decisions.
type ToolReport struct {
	Tool      fact.Tool
	Decisions []Decision
}

// Report is the TS DrySyncReport: the structured preview a dry-run FullSync
// returns instead of mutating anything. Writes come from Write's dry-run
// decisions; the commit decision from would-be writes plus a read-only
// `status --porcelain`. pull/push are reported as the operations that WOULD
// follow, never executed or probed.
type Report struct {
	MetricsDir    string
	User          string
	Machine       string
	Tools         []ToolReport // registry order
	WouldCommit   bool         // any would-write or the user dir already dirty
	CommitMessage string       // the same string a live commit uses
}

// Format is the TS formatDrySyncReport (src/node/core/cli.ts), returned as
// stdout lines (the edge Fprintln's them). home tildefies the user prefix.
func (r Report) Format(home string) []string {
	// fmtCost is the TS fmt: "$" + toFixed(2) — NO thousands separators in
	// either block (the TS uses the same fmt for both, DC-22).
	fmtCost := func(x float64) string { return "$" + render.FixedHalfUp(x, 2) }
	userPrefix := filepath.Join(r.MetricsDir, r.User)
	dir := config.Tildefy(userPrefix, home) + "/"

	var writes, skips []string
	for _, tool := range r.Tools {
		for _, d := range tool.Decisions {
			name := d.Path
			if strings.HasPrefix(name, userPrefix+"/") {
				name = name[len(userPrefix)+1:]
			}
			if d.Action == ActionWrite {
				note := "(new)"
				if d.ExistingCost != nil {
					note = "(update: " + fmtCost(*d.ExistingCost) + " → " + fmtCost(d.IncomingCost) + ")"
				}
				writes = append(writes, "  "+name+"  "+fmtCost(d.IncomingCost)+"  "+note)
			} else {
				existing := 0.0
				if d.ExistingCost != nil {
					existing = *d.ExistingCost
				}
				skips = append(skips, "  "+name+"  incoming "+fmtCost(d.IncomingCost)+" < existing "+fmtCost(existing))
			}
		}
	}

	var lines []string
	if len(writes) > 0 {
		lines = append(lines, "Would write "+strconv.Itoa(len(writes))+" day-file(s) under "+dir+":")
		lines = append(lines, writes...)
	} else {
		lines = append(lines, "Would write 0 day-file(s) under "+dir+".")
	}
	if len(skips) > 0 {
		lines = append(lines, "Would skip "+strconv.Itoa(len(skips))+" file(s) (never-shrink guard):")
		lines = append(lines, skips...)
	}
	if r.WouldCommit {
		lines = append(lines, "Would commit: \""+r.CommitMessage+"\", then pull --rebase origin main, then push")
	} else {
		lines = append(lines, "Would commit: nothing (no changes), then pull --rebase origin main, then push")
	}
	lines = append(lines, "Dry run — nothing written, committed, or pushed.")
	return lines
}
