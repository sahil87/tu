package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReportHeader carries the run metadata printed at the top of report.txt and
// embedded in report.json.
type ReportHeader struct {
	Timestamp   time.Time // UTC
	NodePath    string    // as given on the command line
	NodeVersion string    // `node --version` output
	GoPath      string    // as given on the command line
	GoVersion   string    // `<go> --version` first line
	Fixtures    []string  // resolved fixture alias names, search order
	Script      string    // script(1) flavour: util-linux, bsd, or "n/a"
	MatrixPath  string    // as given on the command line
	Cases       int       // expanded, filtered case count
	Filter      string    // --filter substring, empty when unset
}

// AxisStat is the green/total pair for one axis value in the summary block.
type AxisStat struct {
	Value string
	Green int
	Total int
}

// Summary is the burndown block at the end of the report.
type Summary struct {
	Total       int
	Green       int
	Red         int
	Timeout     int
	Unconfirmed int
	ByConf      []AxisStat
	ByEnv       []AxisStat
	ByIO        []AxisStat
	ByTZ        []AxisStat
}

// Summarize tallies results; timeouts are listed separately from reds (but
// both make the run exit 1 — see runRun).
func SummarizeResults(results []Result) Summary {
	s := Summary{Total: len(results)}
	axes := map[string]map[string]*AxisStat{
		"conf": newAxisStats(axisValues["conf"]),
		"env":  newAxisStats(axisValues["env"]),
		"io":   newAxisStats(axisValues["io"]),
		"tz":   newAxisStats(axisValues["tz"]),
	}
	for _, r := range results {
		switch r.Status {
		case StatusGreen:
			s.Green++
		case StatusTimeout:
			s.Timeout++
		default:
			s.Red++
		}
		if r.Unconfirmed {
			s.Unconfirmed++
		}
		bump := func(axis, value string) {
			st := axes[axis][value]
			st.Total++
			if r.Status == StatusGreen {
				st.Green++
			}
		}
		bump("conf", r.Case.Conf)
		bump("env", r.Case.Env)
		bump("io", r.Case.IO)
		bump("tz", r.Case.TZ)
	}
	s.ByConf = axisStatList(axes["conf"], axisValues["conf"])
	s.ByEnv = axisStatList(axes["env"], axisValues["env"])
	s.ByIO = axisStatList(axes["io"], axisValues["io"])
	s.ByTZ = axisStatList(axes["tz"], axisValues["tz"])
	return s
}

func newAxisStats(values []string) map[string]*AxisStat {
	m := map[string]*AxisStat{}
	for _, v := range values {
		m[v] = &AxisStat{Value: v}
	}
	return m
}

func axisStatList(m map[string]*AxisStat, order []string) []AxisStat {
	out := make([]AxisStat, 0, len(order))
	for _, v := range order {
		out = append(out, *m[v])
	}
	return out
}

// RenderHeader renders the report header lines.
func RenderHeader(h ReportHeader) []string {
	script := h.Script
	if script == "" {
		script = "n/a"
	}
	matrix := fmt.Sprintf("%s (%d cases", h.MatrixPath, h.Cases)
	if h.Filter != "" {
		matrix += fmt.Sprintf(", filter %q", h.Filter)
	}
	matrix += ")"
	return []string{
		"tudiff run  " + h.Timestamp.UTC().Format(time.RFC3339),
		fmt.Sprintf("node: %s (%s)", h.NodePath, h.NodeVersion),
		fmt.Sprintf("go: %s (%s)", h.GoPath, h.GoVersion),
		"fixtures: " + strings.Join(h.Fixtures, ", "),
		"script: " + script,
		"matrix: " + matrix,
	}
}

// RenderCaseLine renders one case's report line: the padded verdict and ID,
// the first-divergence detail for red/timeout, then the informational
// unconfirmed/calls-differ markers.
func RenderCaseLine(r Result) string {
	line := fmt.Sprintf("%-7s %s", strings.ToUpper(r.Status), r.Case.ID)
	switch r.Status {
	case StatusTimeout:
		line += fmt.Sprintf("  timeout: node=%t go=%t", r.NodeTimeout, r.GoTimeout)
	case StatusRed:
		if r.Channel == "exit" {
			line += fmt.Sprintf("  exit: node=%d go=%d", r.NodeExit, r.GoExit)
		} else {
			line += fmt.Sprintf("  %s @%d (line %d): node=%s go=%s",
				r.Channel, r.Offset, r.Line, r.NodeExcerpt, r.GoExcerpt)
		}
	}
	if r.Unconfirmed {
		line += " [unconfirmed]"
	}
	if r.CallsDiffer {
		line += fmt.Sprintf(" [calls differ: node=%d go=%d]", r.NodeCalls, r.GoCalls)
	}
	return line
}

// RenderSummary renders the burndown block.
func RenderSummary(s Summary, fixtures []string) []string {
	axis := func(label string, stats []AxisStat) string {
		var pairs []string
		for _, st := range stats {
			pairs = append(pairs, fmt.Sprintf("%s %d/%d", st.Value, st.Green, st.Total))
		}
		return fmt.Sprintf("  %-8s  %s", label, strings.Join(pairs, "  "))
	}
	return []string{
		fmt.Sprintf("tudiff: %d cases — %d green, %d red, %d timeout   (fixtures: %s; %d cases replayed unconfirmed fixtures)",
			s.Total, s.Green, s.Red, s.Timeout, strings.Join(fixtures, ", "), s.Unconfirmed),
		axis("by conf:", s.ByConf),
		axis("by env:", s.ByEnv),
		axis("by io:", s.ByIO),
		axis("by tz:", s.ByTZ),
	}
}

// RenderReport renders the full report.txt body (trailing newline included).
func RenderReport(h ReportHeader, results []Result) string {
	var b strings.Builder
	for _, line := range RenderHeader(h) {
		b.WriteString(line + "\n")
	}
	for _, r := range results {
		b.WriteString(RenderCaseLine(r) + "\n")
	}
	for _, line := range RenderSummary(SummarizeResults(results), h.Fixtures) {
		b.WriteString(line + "\n")
	}
	return b.String()
}

// reportJSON is the serialized report.json shape (schema 1); field order is
// the key order.
type reportJSON struct {
	Schema  int            `json:"schema"`
	Header  reportHeaderJ  `json:"header"`
	Summary reportSummaryJ `json:"summary"`
	Cases   []reportCaseJ  `json:"cases"`
}

type reportHeaderJ struct {
	Timestamp   string   `json:"timestamp"`
	Node        string   `json:"node"`
	NodeVersion string   `json:"node_version"`
	Go          string   `json:"go"`
	GoVersion   string   `json:"go_version"`
	Fixtures    []string `json:"fixtures"`
	Script      string   `json:"script"`
	Matrix      string   `json:"matrix"`
	Cases       int      `json:"cases"`
	Filter      string   `json:"filter"`
}

type reportSummaryJ struct {
	Total       int `json:"total"`
	Green       int `json:"green"`
	Red         int `json:"red"`
	Timeout     int `json:"timeout"`
	Unconfirmed int `json:"unconfirmed"`
}

type reportCaseJ struct {
	ID          string   `json:"id"`
	Group       string   `json:"group"`
	Args        []string `json:"args"`
	Conf        string   `json:"conf"`
	Env         string   `json:"env"`
	IO          string   `json:"io"`
	TZ          string   `json:"tz"`
	Status      string   `json:"status"`
	Channel     string   `json:"channel"`
	Offset      int      `json:"offset"`
	Line        int      `json:"line"`
	NodeExcerpt string   `json:"node_excerpt"`
	GoExcerpt   string   `json:"go_excerpt"`
	NodeExit    int      `json:"node_exit"`
	GoExit      int      `json:"go_exit"`
	NodeMs      int64    `json:"node_ms"`
	GoMs        int64    `json:"go_ms"`
	Unconfirmed bool     `json:"unconfirmed"`
	CallsDiffer bool     `json:"calls_differ"`
	NodeCalls   int      `json:"node_calls"`
	GoCalls     int      `json:"go_calls"`
	Rerun       bool     `json:"rerun"`
}

// WriteReport writes report.txt and report.json under dir (created as
// needed; the caller wipes the dir at the start of the run). Raw per-case
// captures under cases/ are written by WriteCaseCaptures as each case runs.
func WriteReport(dir string, h ReportHeader, results []Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.txt"), []byte(RenderReport(h, results)), 0o644); err != nil {
		return err
	}
	s := SummarizeResults(results)
	script := h.Script
	if script == "" {
		script = "n/a" // match RenderHeader's display of "no TTY cases selected"
	}
	doc := reportJSON{
		Schema: 1,
		Header: reportHeaderJ{
			Timestamp:   h.Timestamp.UTC().Format(time.RFC3339),
			Node:        h.NodePath,
			NodeVersion: h.NodeVersion,
			Go:          h.GoPath,
			GoVersion:   h.GoVersion,
			Fixtures:    h.Fixtures,
			Script:      script,
			Matrix:      h.MatrixPath,
			Cases:       h.Cases,
			Filter:      h.Filter,
		},
		Summary: reportSummaryJ{
			Total:       s.Total,
			Green:       s.Green,
			Red:         s.Red,
			Timeout:     s.Timeout,
			Unconfirmed: s.Unconfirmed,
		},
	}
	for _, r := range results {
		doc.Cases = append(doc.Cases, reportCaseJ{
			ID:          r.Case.ID,
			Group:       r.Case.Group,
			Args:        r.Case.Args,
			Conf:        r.Case.Conf,
			Env:         r.Case.Env,
			IO:          r.Case.IO,
			TZ:          r.Case.TZ,
			Status:      r.Status,
			Channel:     r.Channel,
			Offset:      r.Offset,
			Line:        r.Line,
			NodeExcerpt: r.NodeExcerpt,
			GoExcerpt:   r.GoExcerpt,
			NodeExit:    r.NodeExit,
			GoExit:      r.GoExit,
			NodeMs:      r.NodeMs,
			GoMs:        r.GoMs,
			Unconfirmed: r.Unconfirmed,
			CallsDiffer: r.CallsDiffer,
			NodeCalls:   r.NodeCalls,
			GoCalls:     r.GoCalls,
			Rerun:       r.Rerun,
		})
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "report.json"), append(raw, '\n'), 0o644)
}

// WriteCaseCaptures writes one case's per-side captures under
// <reportDir>/cases/<case ID as nested dirs>/: {node,go}.{stdout,stderr,exit}
// for pipe cases, {node,go}.{tty,exit} for tty cases. The byte channels are
// home-normalized with each side's staged Home, so the files hold exactly
// what Compare compared and `diff node.stdout go.stdout` stays useful. The
// call logs are written by the fakes themselves (TUDIFF_CALL_LOG points
// here).
func WriteCaseCaptures(reportDir string, r Result, node, goCap SideCapture) error {
	dir := filepath.Join(reportDir, "cases", filepath.FromSlash(r.Case.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(dir, name), data, 0o644)
	}
	sides := []struct {
		name string
		cap  SideCapture
	}{
		{string(SideNode), node},
		{string(SideGo), goCap},
	}
	for _, side := range sides {
		if r.Case.IO == IOTTY {
			if err := write(side.name+".tty", NormalizeHome(side.cap.TTY, side.cap.Home)); err != nil {
				return err
			}
		} else {
			if err := write(side.name+".stdout", NormalizeHome(side.cap.Stdout, side.cap.Home)); err != nil {
				return err
			}
			if err := write(side.name+".stderr", NormalizeHome(side.cap.Stderr, side.cap.Home)); err != nil {
				return err
			}
		}
		if err := write(side.name+".exit", []byte(fmt.Sprintf("%d\n", side.cap.Exit))); err != nil {
			return err
		}
	}
	return nil
}
