// Package watch is the live-polling TUI (ported from the retired TypeScript
// implementation's watch/compositor/panel/rain quartet): a hand-rolled loop
// on x/term +
// os/signal + time (plan decision D12 — no TUI framework). One goroutine owns
// every terminal write; the compositor is a pure function from (session,
// table lines, terminal size, clock) to frame bytes, golden-testable without
// a terminal.
//
// The package imports no source, query, view or render encoder package — the
// per-poll table lines arrive through the Poll callback (composed by cmd/tu
// over command.Run); render/ansi supplies Colors/StripANSI/the pad helpers
// and render the JS rounding/formatting twins.
package watch

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// Terminal is the loop's I/O seam (intake §2). The real implementation is
// x/term + os/signal over os.Stdout/os.Stdin; tests use a fake with a
// scripted key sequence, a settable size and a capturing buffer.
type Terminal interface {
	// Size reports the terminal geometry: 80×24 when stdout is not a TTY or
	// the probe fails (the TS process.stdout.columns ?? 80 / rows ?? 24).
	Size() (cols, rows int)
	// Write appends to stdout.
	Write(p []byte) (int, error)
	// Keys delivers raw stdin chunks; nil when stdin is not a TTY (the TS
	// process.stdin.isTTY guard — SIGINT is then the only exit).
	Keys() <-chan []byte
	// Resize fires on SIGWINCH.
	Resize() <-chan struct{}
	// Interrupt fires on SIGINT.
	Interrupt() <-chan struct{}
	// Close restores raw mode and stops signal delivery. Idempotent.
	Close() error
}

// The non-TTY / probe-error fallbacks (the TS ?? 80 / ?? 24).
const (
	fallbackCols = 80
	fallbackRows = 24
)

// terminal is the real Terminal over x/term + os/signal. The release targets
// are linux and darwin only (D8), so syscall.SIGWINCH needs no build tags.
type terminal struct {
	stdout    *os.File
	stdin     *os.File
	rawState  *term.State
	keys      chan []byte
	resize    chan struct{}
	interrupt chan struct{}
	sigWinch  chan os.Signal
	sigInt    chan os.Signal
	closed    chan struct{}
	closeOnce bool
}

// NewTerminal builds the real terminal seam. Stdin enters raw mode (and the
// key reader starts) only when it is a TTY; otherwise Keys() is nil and
// SIGINT is the only exit, exactly as the TS process.stdin.isTTY guard.
func NewTerminal(stdout, stdin *os.File) Terminal {
	t := &terminal{
		stdout:    stdout,
		stdin:     stdin,
		resize:    make(chan struct{}, 1),
		interrupt: make(chan struct{}, 1),
		closed:    make(chan struct{}),
	}
	if term.IsTerminal(int(stdin.Fd())) {
		if st, err := term.MakeRaw(int(stdin.Fd())); err == nil {
			t.rawState = st
			// Raw INPUT, but output post-processing stays on for libuv
			// parity: x/term's MakeRaw clears OPOST (and with it ONLCR), so
			// the "\n" the compositor writes would no longer become "\r\n"
			// at the tty driver (stair-stepping frames). libuv's
			// uv_tty_set_mode(UV_TTY_MODE_RAW) only strips iflag/lflag bits
			// and keeps c_oflag |= ONLCR — the TS session's bytes assume it,
			// the mid-session "Warning: fetch failed" stderr lines included.
			keepOutputPostProcessing(int(stdin.Fd()))
			t.keys = make(chan []byte, 8)
			go t.readKeys()
		}
	}
	t.sigWinch = make(chan os.Signal, 1)
	signal.Notify(t.sigWinch, syscall.SIGWINCH)
	go t.forward(t.sigWinch, t.resize)
	t.sigInt = make(chan os.Signal, 1)
	signal.Notify(t.sigInt, os.Interrupt)
	go t.forward(t.sigInt, t.interrupt)
	return t
}

// keepOutputPostProcessing re-arms OPOST|ONLCR on the tty after MakeRaw
// stripped them (see the MakeRaw call site for the libuv-parity rationale).
// Close restores the pre-MakeRaw state, which already has OPOST on. A probe
// failure is deliberately silent: the watch session still works, only the
// line endings degrade.
func keepOutputPostProcessing(fd int) {
	tio, err := getTermios(fd)
	if err != nil {
		return
	}
	tio.Oflag |= unix.OPOST | unix.ONLCR
	_ = setTermios(fd, tio)
}

// Size probes stdout; each axis falls back independently (columns ?? 80,
// rows ?? 24).
func (t *terminal) Size() (cols, rows int) {
	cols, rows = fallbackCols, fallbackRows
	if !term.IsTerminal(int(t.stdout.Fd())) {
		return cols, rows
	}
	w, h, err := term.GetSize(int(t.stdout.Fd()))
	if err != nil {
		return cols, rows
	}
	if w > 0 {
		cols = w
	}
	if h > 0 {
		rows = h
	}
	return cols, rows
}

func (t *terminal) Write(p []byte) (int, error) { return t.stdout.Write(p) }

func (t *terminal) Keys() <-chan []byte        { return t.keys }
func (t *terminal) Resize() <-chan struct{}    { return t.resize }
func (t *terminal) Interrupt() <-chan struct{} { return t.interrupt }

// Close restores raw mode and stops signal delivery. A key read already
// blocked on stdin is left to the process exit (the TS likewise just pauses
// stdin and exits).
func (t *terminal) Close() error {
	if t.closeOnce {
		return nil
	}
	t.closeOnce = true
	close(t.closed)
	if t.rawState != nil {
		_ = term.Restore(int(t.stdin.Fd()), t.rawState)
	}
	signal.Stop(t.sigWinch)
	signal.Stop(t.sigInt)
	return nil
}

// readKeys copies stdin chunks onto the keys channel. Each Read is one chunk,
// matching the TS stdin "data" handler's per-chunk comparison.
func (t *terminal) readKeys() {
	buf := make([]byte, 64)
	for {
		n, err := t.stdin.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case t.keys <- chunk:
			case <-t.closed:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// forward translates an os.Signal channel into the struct{} form the loop
// selects on.
func (t *terminal) forward(sig chan os.Signal, out chan<- struct{}) {
	for {
		select {
		case <-sig:
			select {
			case out <- struct{}{}:
			default: // coalesce: one pending event is enough
			}
		case <-t.closed:
			return
		}
	}
}
