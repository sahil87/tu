package toolkit

import (
	"embed"
)

// completions holds the three static shell completion scripts, byte-identical
// to the retired TypeScript implementation's
// BASH_COMPLETION/ZSH_COMPLETION/FISH_COMPLETION constants with the
// template-literal escapes resolved.
// They live as plain files — not Go strings — because the zsh script contains
// backticks a Go raw string cannot hold, and hand-un-escaping is the
// transcription error the harness exists to catch. The frozen bytes are the
// reference (a change is an output change); compare with
// `bin/tu shell-init <shell>`.
//
//go:embed completions/tu.bash completions/tu.zsh completions/tu.fish
var completions embed.FS

// Shells is the supported-shell list as it appears in the unknown-shell
// message ("bash, zsh, fish").
const Shells = "bash, zsh, fish"

// ShellInitUsage is the 6-line block `tu shell-init` with no argument writes
// to stderr (exit 2, stdout empty — stdout may be eval'd, so usage text must
// never reach it). cmd/tu prints it with Fprintln, adding the one trailing
// newline.
const ShellInitUsage = `Usage: tu shell-init <bash|zsh|fish>

Install:
  bash: echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc
  zsh:  echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
  fish: tu shell-init fish > ~/.config/fish/completions/tu.fish`

// Completion returns the embedded completion script for bash/zsh/fish, or
// false for any other shell name.
func Completion(shell string) ([]byte, bool) {
	switch shell {
	case "bash", "zsh", "fish":
	default:
		return nil, false
	}
	// The embed patterns above guarantee these reads; a failure would be a
	// build-integrity bug, not user error.
	b, err := completions.ReadFile("completions/tu." + shell)
	if err != nil {
		return nil, false
	}
	return b, true
}

// UnknownShellMessage is the stderr line for an unsupported shell argument
// (exit 2, stdout empty).
func UnknownShellMessage(shell string) string {
	return "Unknown shell: " + shell + ". Supported: " + Shells
}
