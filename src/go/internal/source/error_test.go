package source

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWarningByteExact(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "exec with stderr",
			err: &Error{
				Tool: "kimi", Name: "Kimi", Kind: KindExec,
				Detail: "Command failed: /x/ccusage kimi daily --json\nboom\n",
			},
			want: "warning: Kimi fetch failed (Command failed: /x/ccusage kimi daily --json\nboom\n), showing zero data",
		},
		{
			name: "exec with empty stderr keeps the newline",
			err: &Error{
				Tool: "cc", Name: "Claude Code", Kind: KindExec,
				Detail: "Command failed: /x/ccusage claude daily --json\n",
			},
			want: "warning: Claude Code fetch failed (Command failed: /x/ccusage claude daily --json\n), showing zero data",
		},
		{
			name: "spawn ENOENT",
			err: &Error{
				Tool: "codex", Name: "Codex", Kind: KindExec,
				Detail: "spawn ccusage ENOENT",
			},
			want: "warning: Codex fetch failed (spawn ccusage ENOENT), showing zero data",
		},
		{
			name: "timeout",
			err: &Error{
				Tool: "oc", Name: "OpenCode", Kind: KindTimeout,
				Detail: "timeout after 200ms",
			},
			want: "warning: OpenCode fetch failed (timeout after 200ms), showing zero data",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Warning(); got != tt.want {
				t.Errorf("Warning() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWarnsPerKind(t *testing.T) {
	if !(&Error{Kind: KindExec}).Warns() {
		t.Error("KindExec should warn")
	}
	if !(&Error{Kind: KindTimeout}).Warns() {
		t.Error("KindTimeout should warn")
	}
	if (&Error{Kind: KindParse}).Warns() {
		t.Error("KindParse must not warn")
	}
}

func TestWriteWarningsOrderAndFiltering(t *testing.T) {
	execErr := &Error{Name: "Claude Code", Kind: KindExec, Detail: "spawn ccusage ENOENT"}
	parseErr := &Error{Name: "Codex", Kind: KindParse, Detail: "invalid JSON"}
	timeoutErr := &Error{Name: "Kimi", Kind: KindTimeout, Detail: "timeout after 200ms"}

	var buf bytes.Buffer
	WriteWarnings(&buf, []*Error{execErr, parseErr, timeoutErr, nil})

	want := "warning: Claude Code fetch failed (spawn ccusage ENOENT), showing zero data\n" +
		"warning: Kimi fetch failed (timeout after 200ms), showing zero data\n"
	if buf.String() != want {
		t.Errorf("WriteWarnings wrote %q, want %q", buf.String(), want)
	}
	if got := strings.Count(buf.String(), "\n"); got != 2 {
		t.Errorf("WriteWarnings wrote %d lines, want 2", got)
	}
}

func TestErrorImplementsErrorAndUnwrap(t *testing.T) {
	cause := errors.New("exit status 2")
	err := &Error{Name: "Kimi", Kind: KindExec, Detail: "Command failed: boom", Err: cause}

	msg := err.Error()
	if !strings.Contains(msg, "Kimi") || !strings.Contains(msg, "Command failed: boom") {
		t.Errorf("Error() = %q, want it to include Name and Detail", msg)
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false")
	}
	var nilCause *Error
	nilCause = &Error{Name: "Codex"}
	if nilCause.Unwrap() != nil {
		t.Error("Unwrap() with nil Err should return nil")
	}
}
