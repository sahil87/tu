// Package toolkit implements the shll toolkit contracts for the Go port —
// the version line, the help-dump envelope, shell-init completions, the skill
// bundle, and the Homebrew self-update sequence — reproducing the shipped
// TypeScript bytes. Audited against the shll standards help-dump, update,
// shell-init, skill, and version at shll v0.1.32 (2026-09-17). The package
// never writes to stdout/stderr itself; cmd/tu is the only writer (brew
// subprocess streams are passed through the injected driver).
package toolkit

import "strings"

// BareVersion strips one leading "v" ("v0.11.5" → "0.11.5"; "dev" → "dev") —
// the form help-dump's "version" field and brew's versions.stable report use.
func BareVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// DisplayVersion renders the user-facing form: "v" + bare when the bare form
// starts with an ASCII digit, else the input unchanged (an unstamped "dev"
// stays "dev"). Both -X main.version=0.11.5 and -X main.version=v0.11.5 print
// identically.
func DisplayVersion(v string) string {
	if bare := BareVersion(v); len(bare) > 0 && bare[0] >= '0' && bare[0] <= '9' {
		return "v" + bare
	}
	return v
}

// VersionLine renders the toolkit version standard's canonical shape,
// `<tool> version vX.Y.Z` ("tu version v0.11.5").
func VersionLine(v string) string {
	return "tu version " + DisplayVersion(v)
}
