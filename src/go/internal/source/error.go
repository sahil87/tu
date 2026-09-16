// Package source holds the typed per-source error shared by every data
// source adapter. Sources return errors as values beside their (zero)
// records — they never panic, and never write to stdout/stderr: the warning
// line reaches an io.Writer only through WriteWarnings, called by the command
// edge.
package source

import (
	"fmt"
	"io"
)

// Kind classifies a source failure.
type Kind int

const (
	// KindExec: binary missing, spawn failure, non-zero exit, or signal.
	KindExec Kind = iota
	// KindTimeout: the caller's context deadline fired.
	KindTimeout
	// KindParse: stdout was not the expected JSON document.
	KindParse
)

// Error is one source's failure, returned beside the (zero) records.
type Error struct {
	Tool   string // registry key
	Name   string // display name, for the warning line
	Kind   Kind
	Detail string // pre-rendered human detail (see Warning)
	Err    error  // wrapped cause, may be nil
}

// Error returns a readable form including Name and Detail. Only Warning is
// byte-pinned; this form is free.
func (e *Error) Error() string {
	return e.Name + ": " + e.Detail
}

// Unwrap returns the wrapped cause, which may be nil.
func (e *Error) Unwrap() error {
	return e.Err
}

// Warns reports whether this error produces a stderr line in the TS binary:
// Exec and Timeout do; Parse does not (the TS returns [] silently on bad
// JSON).
func (e *Error) Warns() bool {
	return e.Kind == KindExec || e.Kind == KindTimeout
}

// Warning is the byte-exact TS line (no trailing newline):
//
//	warning: {Name} fetch failed ({Detail}), showing zero data
func (e *Error) Warning() string {
	return "warning: " + e.Name + " fetch failed (" + e.Detail + "), showing zero data"
}

// WriteWarnings writes Warning()+"\n" for every error with Warns() true, in
// the order given, and nothing for the others. It is the only function in the
// source tree that touches an io.Writer, and it is called by the command
// edge, never by a source.
func WriteWarnings(w io.Writer, errs []*Error) {
	for _, e := range errs {
		if e == nil || !e.Warns() {
			continue
		}
		fmt.Fprintln(w, e.Warning())
	}
}
