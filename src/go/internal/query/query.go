package query

import (
	"sort"
	"strings"

	"github.com/sahil87/tu/internal/fact"
)

// Window keeps records with since <= Date <= until by lexicographic comparison
// on the ISO labels, inclusive on both ends; an empty bound is open on that
// side. Pure: the input is never mutated and never returned.
func Window(recs []fact.Record, since, until string) []fact.Record {
	var out []fact.Record
	for _, r := range recs {
		if since != "" && r.Date < since {
			continue
		}
		if until != "" && r.Date > until {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ByTool keeps records whose Tool equals key. Pure.
func ByTool(recs []fact.Record, key string) []fact.Record {
	var out []fact.Record
	for _, r := range recs {
		if r.Tool == key {
			out = append(out, r)
		}
	}
	return out
}

// ByUser keeps records whose User equals user. Pure.
func ByUser(recs []fact.Record, user string) []fact.Record {
	var out []fact.Record
	for _, r := range recs {
		if r.User == user {
			out = append(out, r)
		}
	}
	return out
}

// RollUp re-labels each record to its period bucket (daily: identity; weekly:
// WeekLabel(Date); monthly: Date[:7]) and sums Totals over records sharing
// (Date', Tool, User, Machine). Output is ascending by Date (byte order equals
// the TS localeCompare on ISO labels), first-seen order within a label. Pure:
// the input is never mutated, and daily returns a copy, never the input slice.
func RollUp(recs []fact.Record, p Period) []fact.Record {
	type groupKey struct {
		date, tool, user, machine string
	}
	index := make(map[groupKey]int)
	var out []fact.Record
	for _, r := range recs {
		r.Date = relabel(r.Date, p)
		k := groupKey{r.Date, r.Tool, r.User, r.Machine}
		if i, ok := index[k]; ok {
			out[i].Totals = out[i].Totals.Add(r.Totals)
			continue
		}
		index[k] = len(out)
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// relabel maps a daily ISO label to its period bucket.
func relabel(date string, p Period) string {
	switch p {
	case Weekly:
		return WeekLabel(date)
	case Monthly:
		if len(date) >= 7 {
			return date[:7]
		}
		return date
	default:
		return date
	}
}

// Dim is a record dimension GroupBy can group on.
type Dim int

const (
	Date Dim = iota
	Tool
	User
	Machine
)

// Group is one bucket of GroupBy: Key carries only the grouped dims (other
// string fields empty); Totals is the field-wise sum.
type Group struct {
	Key fact.Record
	fact.Totals
}

// GroupBy is the one group-by: it sums Totals per distinct tuple of the
// requested dims and emits groups in first-seen input order, so a
// registry-ordered input yields registry-ordered groups. Pure.
func GroupBy(recs []fact.Record, dims ...Dim) []Group {
	index := make(map[string]int)
	var out []Group
	for _, r := range recs {
		key, s := groupKeyFor(r, dims)
		if i, ok := index[s]; ok {
			out[i].Totals = out[i].Totals.Add(r.Totals)
			continue
		}
		index[s] = len(out)
		out = append(out, Group{Key: key, Totals: r.Totals})
	}
	return out
}

// groupKeyFor builds the group key record (only the grouped dims set) and its
// string form for the index map.
func groupKeyFor(r fact.Record, dims []Dim) (fact.Record, string) {
	var key fact.Record
	var parts []string
	for _, d := range dims {
		var v string
		switch d {
		case Tool:
			v = r.Tool
			key.Tool = v
		case User:
			v = r.User
			key.User = v
		case Machine:
			v = r.Machine
			key.Machine = v
		default:
			v = r.Date
			key.Date = v
		}
		parts = append(parts, v)
	}
	return key, strings.Join(parts, "\x00")
}
