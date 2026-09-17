package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Axis values of the differential matrix (plan row P4). Each value drives one
// dimension of a case: the staged $HOME skeleton (conf), extra child
// environment (env), pipe vs. pty capture (io), and the child's timezone (tz).
const (
	ConfSingle = "single"
	ConfMulti  = "multi"
	ConfOrg    = "org"
	ConfLegacy = "legacy"

	EnvDefault  = "default"
	EnvNoColor  = "nocolor"
	EnvEnvrepo  = "envrepo"
	EnvPullfail = "pullfail"
	EnvPushfail = "pushfail"
	EnvDirty    = "dirty"

	IOPipe = "pipe"
	IOTTY  = "tty"

	TZFixed = "fixed"
	TZAlt   = "alt"
)

// IANA zone names behind the tz axis values: a fixed zone plus one
// non-integer-offset alternate (the zone the local captures were bucketed in).
const (
	TZFixedName = "UTC"
	TZAltName   = "Asia/Kolkata"
)

// TZName maps a tz axis value to its IANA zone name.
func TZName(axis string) string {
	if axis == TZAlt {
		return TZAltName
	}
	return TZFixedName
}

// Matrix is the argument matrix file (harness/matrix.json): case groups, each
// one tu argv crossed with the axes it lists. Omitted axes take the single
// base value, so the file controls the expanded case count explicitly.
type Matrix struct {
	Schema int         `json:"schema"`
	Cases  []CaseGroup `json:"cases"`
}

// CaseGroup is one matrix entry. Args is the tu argv (an empty array is
// valid; a missing key is not). Conf/Env/IO/TZ list the axis values to cross;
// nil means the base value only.
type CaseGroup struct {
	ID   string   `json:"id"`
	Args []string `json:"args"`
	Conf []string `json:"conf,omitempty"`
	Env  []string `json:"env,omitempty"`
	IO   []string `json:"io,omitempty"`
	TZ   []string `json:"tz,omitempty"`
}

// Case is one expanded matrix cell: a group crossed with concrete axis
// values. ID is "<group>/<conf>/<env>/<io>/<tz>" — every axis always appears,
// so IDs stay stable when a group later gains an axis.
type Case struct {
	ID    string
	Group string
	Args  []string
	Conf  string
	Env   string
	IO    string
	TZ    string
}

var axisValues = map[string][]string{
	"conf": {ConfSingle, ConfMulti, ConfOrg, ConfLegacy},
	"env":  {EnvDefault, EnvNoColor, EnvEnvrepo, EnvPullfail, EnvPushfail, EnvDirty},
	"io":   {IOPipe, IOTTY},
	"tz":   {TZFixed, TZAlt},
}

// LoadMatrix reads and validates a matrix file. Every validation error names
// the offending group id and field.
func LoadMatrix(path string) (*Matrix, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Matrix
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	switch err := dec.Decode(&extra); err {
	case io.EOF:
	case nil:
		return nil, fmt.Errorf("trailing JSON value after matrix document")
	default:
		return nil, fmt.Errorf("trailing content after matrix document: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Matrix) validate() error {
	if m.Schema != 1 {
		return fmt.Errorf("schema must be 1, got %d", m.Schema)
	}
	seen := map[string]bool{}
	for i, g := range m.Cases {
		if g.ID == "" {
			return fmt.Errorf("case %d: empty id", i)
		}
		if err := pathComponent("case id", g.ID); err != nil {
			return fmt.Errorf("case %q: %s", g.ID, strings.TrimPrefix(err.Error(), "tudiff: "))
		}
		if seen[g.ID] {
			return fmt.Errorf("duplicate case id %q", g.ID)
		}
		seen[g.ID] = true
		if g.Args == nil {
			return fmt.Errorf("case %q: missing args", g.ID)
		}
		axes := []struct {
			name  string
			value []string
		}{
			{"conf", g.Conf}, {"env", g.Env}, {"io", g.IO}, {"tz", g.TZ},
		}
		for _, a := range axes {
			if err := validateAxis(g.ID, a.name, a.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAxis(id, name string, values []string) error {
	if values != nil && len(values) == 0 {
		return fmt.Errorf("case %q: %s axis is an empty array (omit the key for the base value)", id, name)
	}
	seen := map[string]bool{}
	for _, v := range values {
		valid := false
		for _, allowed := range axisValues[name] {
			if v == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("case %q: invalid %s value %q", id, name, v)
		}
		if seen[v] {
			return fmt.Errorf("case %q: duplicate %s value %q", id, name, v)
		}
		seen[v] = true
	}
	return nil
}

// Expand crosses every group with its axes in the nested order
// conf → env → io → tz, substituting the base value for each omitted axis.
func Expand(m *Matrix) []Case {
	var out []Case
	for _, g := range m.Cases {
		for _, conf := range orBase(g.Conf, ConfSingle) {
			for _, env := range orBase(g.Env, EnvDefault) {
				for _, io := range orBase(g.IO, IOPipe) {
					for _, tz := range orBase(g.TZ, TZFixed) {
						out = append(out, Case{
							ID:    g.ID + "/" + conf + "/" + env + "/" + io + "/" + tz,
							Group: g.ID,
							Args:  g.Args,
							Conf:  conf,
							Env:   env,
							IO:    io,
							TZ:    tz,
						})
					}
				}
			}
		}
	}
	return out
}

func orBase(values []string, base string) []string {
	if len(values) == 0 {
		return []string{base}
	}
	return values
}

// Filter keeps the cases whose expanded ID contains substr; an empty
// substring keeps all cases.
func Filter(cases []Case, substr string) []Case {
	if substr == "" {
		return cases
	}
	var out []Case
	for _, c := range cases {
		if strings.Contains(c.ID, substr) {
			out = append(out, c)
		}
	}
	return out
}
