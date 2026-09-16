package toolkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// completionInventory pins the spec's frozen token inventory per shell, in
// each script's own syntax (fish spells long flags `-l json`, not `--json`).
var completionInventory = map[string][]string{
	"bash": {
		"help init-conf init-metrics sync status update shell-init skill",
		"cc codex co oc gemini gem copilot cop kimi ki all",
		"d w m daily weekly monthly",
		"h history dh wh mh lb lbh",
		"--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help",
		"-f -w -i -u -s -j -t -v -V -h",
		"cost tokens",
		"bash zsh fish",
	},
	"zsh": {
		"help init-conf init-metrics sync status update shell-init skill",
		"cc codex co oc gemini gem copilot cop kimi ki all",
		"d w m daily weekly monthly",
		"h history dh wh mh lb lbh",
		"--json --csv --md --since --until --full --metric --top --sync --dry-run --fresh --watch --interval --user --by-machine --skip-brew-update --no-color --no-rain --version --help",
		"-f -w -i -u -s -j -t -v -V -h",
		"cost tokens",
		"bash zsh fish",
	},
	"fish": {
		"-a 'help'", "-a 'init-conf'", "-a 'init-metrics'", "-a 'sync'", "-a 'status'", "-a 'update'", "-a 'shell-init'", "-a 'skill'",
		"-a 'cc'", "-a 'gem'", "-a 'cop'", "-a 'kimi'", "-a 'ki'",
		"-a 'd w m daily weekly monthly h history dh wh mh lb lbh'",
		"-l json", "-l csv", "-l md", "-l since", "-l until", "-l full", "-l metric", "-l top", "-l sync", "-l dry-run", "-l fresh", "-l watch", "-l interval", "-l user", "-l by-machine", "-l skip-brew-update", "-l no-color", "-l no-rain", "-l version", "-l help",
		"-s f", "-s w", "-s i", "-s u", "-s s", "-s j", "-s t", "-s v", "-s V", "-s h",
		"cost tokens",
		"-a 'bash'", "-a 'zsh'", "-a 'fish'",
	},
}

func TestCompletionScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			script, ok := Completion(shell)
			if !ok {
				t.Fatalf("Completion(%q) = not found", shell)
			}
			if len(script) == 0 {
				t.Fatal("empty script")
			}
			first, _, _ := strings.Cut(string(script), "\n")
			if want := "# tu(1) " + shell + " completion"; first != want {
				t.Errorf("first line = %q, want %q", first, want)
			}
			// Ends with exactly one newline.
			if script[len(script)-1] != '\n' || (len(script) > 1 && script[len(script)-2] == '\n') {
				t.Error("script does not end with exactly one newline")
			}
			for _, tok := range completionInventory[shell] {
				if !strings.Contains(string(script), tok) {
					t.Errorf("script missing inventory token %q", tok)
				}
			}
			if strings.Contains(string(script), "help-dump") {
				t.Error("script must not mention the hidden help-dump command")
			}
		})
	}
}

// TestCompletionParse feeds each script to its shell's parser (the toolkit
// shell-init standard's recommended guard), skipping when the shell is not on
// PATH. The script is passed as a file argument, so nothing executes.
func TestCompletionParse(t *testing.T) {
	checks := map[string][]string{
		"bash": {"-n"},
		"zsh":  {"-n"},
		"fish": {"--no-execute"},
	}
	for shell, flags := range checks {
		t.Run(shell, func(t *testing.T) {
			bin, err := exec.LookPath(shell)
			if err != nil {
				t.Skipf("%s not on PATH", shell)
			}
			script, ok := Completion(shell)
			if !ok {
				t.Fatalf("Completion(%q) = not found", shell)
			}
			f := filepath.Join(t.TempDir(), "tu."+shell)
			if err := os.WriteFile(f, script, 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, append(flags, f)...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s %v rejected the script: %v\n%s", shell, flags, err, out)
			}
		})
	}
}

func TestCompletionUnknownShell(t *testing.T) {
	if _, ok := Completion("tcsh"); ok {
		t.Error(`Completion("tcsh") = found, want false`)
	}
	if got, want := UnknownShellMessage("tcsh"), "Unknown shell: tcsh. Supported: bash, zsh, fish"; got != want {
		t.Errorf("UnknownShellMessage = %q, want %q", got, want)
	}
}

// TestShellInitUsage pins the 6-line usage block; cmd/tu adds the one trailing
// newline (the node shell-init-missing capture is the block + "\n", 223 bytes).
func TestShellInitUsage(t *testing.T) {
	want := `Usage: tu shell-init <bash|zsh|fish>

Install:
  bash: echo 'eval "$(tu shell-init bash)"' >> ~/.bashrc
  zsh:  echo 'eval "$(tu shell-init zsh)"' >> ~/.zshrc
  fish: tu shell-init fish > ~/.config/fish/completions/tu.fish`
	if ShellInitUsage != want {
		t.Errorf("ShellInitUsage = %q, want %q", ShellInitUsage, want)
	}
}
