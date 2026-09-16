package toolkit

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Brew is the driver seam for the Homebrew subprocesses `tu update` runs (the
// internal/sync Exec precedent: an injected driver, streams passed through,
// nothing printed by this package). BrewExec is the real driver; tests use a
// fake. The argv is verbatim from the TS runUpdate.
type Brew interface {
	// Update runs `brew update --quiet`, streams captured (the TS stdio "pipe").
	Update(ctx context.Context) error
	// Info runs `brew info --json=v2 tu` and returns its stdout.
	Info(ctx context.Context) ([]byte, error)
	// Upgrade runs `brew upgrade tu` with the given streams attached (the TS
	// stdio "inherit" — the call is interactive).
	Upgrade(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error
}

// BrewExec drives the real brew found on PATH.
type BrewExec struct{}

// Bounds for the brew metadata calls (the TS timeout: 600_000 / 60_000),
// sized for a network transfer per the toolkit update standard. `brew
// upgrade` is deliberately unbounded: a kill landing mid-transaction corrupts
// the keg, and the interactive call's escape hatch is Ctrl-C.
const (
	brewUpdateTimeout = 600 * time.Second
	brewInfoTimeout   = 60 * time.Second
	// brewGraceDelay is how long a bounded brew subprocess gets to unwind
	// after the graceful SIGTERM before the runtime's forced kill.
	brewGraceDelay = 10 * time.Second
)

// Update runs `brew update --quiet`, bounded at 600 s and SIGTERM-graceful.
func (BrewExec) Update(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, brewUpdateTimeout)
	defer cancel()
	return boundedBrewCmd(ctx, "update", "--quiet").Run()
}

// Info runs `brew info --json=v2 tu`, bounded at 60 s and SIGTERM-graceful,
// and returns its stdout.
func (BrewExec) Info(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, brewInfoTimeout)
	defer cancel()
	return boundedBrewCmd(ctx, "info", "--json=v2", "tu").Output()
}

// Upgrade runs `brew upgrade tu` with NO deadline and NO Cancel override:
// context.Background() semantics, the streams passed through, and
// HOMEBREW_NO_ASK=1 appended to the child environment (Homebrew 6 ask-mode
// suppression — the env var, not --no-ask, so Homebrew < 6 is unaffected).
func (BrewExec) Upgrade(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := upgradeBrewCmd()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// boundedBrewCmd builds a context-bounded brew command that terminates
// GRACEFULLY on expiry: ctx cancellation sends SIGTERM (trappable — brew can
// finish or roll back its transaction) instead of exec.CommandContext's
// default SIGKILL, and WaitDelay grants a grace period before the runtime's
// forced kill (the wt newBoundedBrewCmd helper, copied). Every bounded brew
// call goes through here so no code path can SIGKILL a package-manager
// subprocess mid-transaction.
func boundedBrewCmd(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "brew", args...)
	cmd.Cancel = func() error {
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	cmd.WaitDelay = brewGraceDelay
	return cmd
}

// upgradeBrewCmd builds the unbounded interactive upgrade command: no
// deadline, no Cancel (exec.Command, not CommandContext — the default Cancel
// kills on ctx completion, and the upgrade must never be killed on a timer),
// HOMEBREW_NO_ASK=1 in the environment.
func upgradeBrewCmd() *exec.Cmd {
	cmd := exec.Command("brew", "upgrade", "tu")
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_ASK=1")
	return cmd
}

// IsBrewInstall reports whether the symlink-resolved executable path lives
// under the formula's Cellar — the TS `__cli_dirname.includes("/Cellar/tu/")`
// gate, narrower than the siblings' "/Cellar/".
func IsBrewInstall(resolvedExe string) bool {
	return strings.Contains(resolvedExe, "/Cellar/tu/")
}

// NotBrewInstallLines is the two stdout lines of the off-Homebrew path
// (exit 0 — every dev build and the `go test` binary take it).
func NotBrewInstallLines(version string) []string {
	return []string{
		"tu " + DisplayVersion(version) + " was not installed via Homebrew.",
		"Update manually, or reinstall with: brew install sahil87/tap/tu",
	}
}

// The wrapper lines around the brew calls and the three brew-failure stderr
// lines, byte-exact to the TS runUpdate. Version strings use DisplayVersion.
func CurrentVersionLine(version string) string {
	return "Current version: " + DisplayVersion(version)
}

func AlreadyUpToDateLine(version string) string {
	return "Already up to date (" + DisplayVersion(version) + ")."
}

func UpdatingLine(version, latest string) string {
	return "Updating " + DisplayVersion(version) + " → " + DisplayVersion(latest) + "..."
}

func UpdatedLine(latest string) string {
	return "Updated to " + DisplayVersion(latest) + "."
}

// UpdateError carries the exact stderr line of a brew failure; cmd/tu prints
// Message and exits 1.
type UpdateError struct{ Message string }

func (e *UpdateError) Error() string { return e.Message }

// Brew-failure messages, byte-exact to the TS runUpdate.
const (
	msgUpdateFailed  = "Error: could not check for updates (brew update failed). Check your network connection."
	msgLatestUnknown = "Error: could not determine latest version."
	msgUpgradeFailed = "Error: brew upgrade failed."
)

// CheckLatest runs the update-unless-skip then info+parse steps and returns
// the latest stable version brew reports (the bare form, e.g. "0.11.6").
// skipBrewUpdate gates ONLY the `brew update` tap-metadata refresh. Any
// update error maps to msgUpdateFailed; any exec/parse error or a
// missing/empty/non-string stable maps to msgLatestUnknown.
func CheckLatest(ctx context.Context, brew Brew, skipBrewUpdate bool) (string, *UpdateError) {
	if !skipBrewUpdate {
		if err := brew.Update(ctx); err != nil {
			return "", &UpdateError{Message: msgUpdateFailed}
		}
	}
	raw, err := brew.Info(ctx)
	if err != nil {
		return "", &UpdateError{Message: msgLatestUnknown}
	}
	// A non-string `stable` fails this typed unmarshal, matching the TS
	// `typeof stable !== "string"` guard.
	var info struct {
		Formulae []struct {
			Versions struct {
				Stable string `json:"stable"`
			} `json:"versions"`
		} `json:"formulae"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", &UpdateError{Message: msgLatestUnknown}
	}
	if len(info.Formulae) == 0 || strings.TrimSpace(info.Formulae[0].Versions.Stable) == "" {
		return "", &UpdateError{Message: msgLatestUnknown}
	}
	return info.Formulae[0].Versions.Stable, nil
}

// UpToDate reports whether brew's latest equals the binary's version (brew
// reports the bare form, so the binary's leading "v" is stripped first).
func UpToDate(version, latest string) bool {
	return BareVersion(version) == latest
}

// Upgrade runs the interactive `brew upgrade` through the driver with the
// streams passed through; any failure maps to msgUpgradeFailed. No code path
// reads stdin for a confirmation — HOMEBREW_NO_ASK=1 (set by BrewExec)
// suppresses Homebrew 6's ask prompt.
func Upgrade(ctx context.Context, brew Brew, stdin io.Reader, stdout, stderr io.Writer) *UpdateError {
	if err := brew.Upgrade(ctx, stdin, stdout, stderr); err != nil {
		return &UpdateError{Message: msgUpgradeFailed}
	}
	return nil
}
