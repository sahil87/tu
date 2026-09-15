package harness

import (
	"encoding/json"
	"os"
)

// CallLogEnv names the environment variable both fakes append their
// invocation log to; each line lets P4 byte-compare the *sequence* of
// ccusage/git calls between the TypeScript and Go binaries.
const CallLogEnv = "TUDIFF_CALL_LOG"

// callLogLine is the JSON-lines shape appended to $TUDIFF_CALL_LOG. Matched
// is present only for fake-ccusage hits.
type callLogLine struct {
	Tool    string   `json:"tool"`
	Argv    []string `json:"argv"`
	Cwd     string   `json:"cwd"`
	Matched string   `json:"matched,omitempty"`
}

// LogCall appends one JSON line describing an invocation to the file named by
// $TUDIFF_CALL_LOG (created if absent, appended otherwise). It is a no-op when
// the variable is unset, and all errors are swallowed: the fakes impersonate
// tools whose stderr the harness byte-compares, so logging must never change
// a fake's exit code or output.
func LogCall(tool string, argv []string, matched string) {
	path := os.Getenv(CallLogEnv)
	if path == "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	line := callLogLine{Tool: tool, Argv: argv, Cwd: cwd, Matched: matched}
	raw, err := json.Marshal(line)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(raw, '\n'))
}
