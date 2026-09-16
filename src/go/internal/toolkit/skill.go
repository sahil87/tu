package toolkit

import (
	_ "embed"
	"io"
)

//go:generate ../../../../scripts/sync-skill.sh

// Skill is the agent usage bundle (`tu skill`), byte-identical to the
// canonical docs/site/skill.md. The Go module root is src/go/, so //go:embed
// cannot reach docs/site/ directly — scripts/sync-skill.sh maintains this
// committed copy, the drift-guard test in skill_test.go pins it on every
// `go test`, and the cmp guard in `just go-build` fails the build on drift.
// Raw markdown, printed verbatim with no rendering, no pager, no framing
// (the toolkit skill standard); any arguments are ignored, as the TS does.
//
//go:embed skill.md
var Skill []byte

// WriteSkill writes the bundle verbatim to w.
func WriteSkill(w io.Writer) error {
	_, err := w.Write(Skill)
	return err
}
