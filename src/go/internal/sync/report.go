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

// Format renders the dry-run report as stdout lines (the edge Fprintln's
// them). home tildefies the user prefix.
func (r Report) Format(home string) []string {
	userPrefix := filepath.Join(r.MetricsDir, r.User)
	dir := config.Tildefy(userPrefix, home) + "/"

	var writes, skips []string
	for _, tool := range r.Tools {
		for _, d := range tool.Decisions {
			if d.Action == ActionUnchanged {
				continue // a byte-identical rewrite is not a write
			}
			name := d.Path
			if strings.HasPrefix(name, userPrefix+"/") {
				name = name[len(userPrefix)+1:]
			}
			if d.Action == ActionWrite {
				note := "(new)"
				if d.ExistingCost != nil {
					note = "(update: " + render.FormatCost(*d.ExistingCost) + " → " + render.FormatCost(d.IncomingCost) + ")"
				}
				writes = append(writes, "  "+name+"  "+render.FormatCost(d.IncomingCost)+"  "+note)
			} else {
				existing := 0.0
				if d.ExistingCost != nil {
					existing = *d.ExistingCost
				}
				skips = append(skips, "  "+name+"  incoming "+render.FormatCost(d.IncomingCost)+" < existing "+render.FormatCost(existing))
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
		lines = append(lines, "Would commit: \""+r.CommitMessage+"\", then pull --rebase, then push")
	} else {
		lines = append(lines, "Would commit: nothing (no changes), then pull --rebase, then push")
	}
	lines = append(lines, "Dry run — nothing written, committed, or pushed.")
	return lines
}
