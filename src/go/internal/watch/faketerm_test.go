package watch

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeTerminal is the test Terminal: a settable size, a scripted key channel,
// manual resize/interrupt channels, and a buffer capturing every write.
type fakeTerminal struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	cols      int
	rows      int
	keys      chan []byte
	resize    chan struct{}
	interrupt chan struct{}
	closed    bool
	notify    chan struct{} // closed+recreated on every Write (waitFor's seam)
}

func newFakeTerminal(cols, rows int, withKeys bool) *fakeTerminal {
	f := &fakeTerminal{
		cols:      cols,
		rows:      rows,
		resize:    make(chan struct{}),
		interrupt: make(chan struct{}),
		notify:    make(chan struct{}),
	}
	if withKeys {
		f.keys = make(chan []byte)
	}
	return f
}

func (f *fakeTerminal) Size() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cols, f.rows
}

func (f *fakeTerminal) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.buf.Write(p)
	close(f.notify)
	f.notify = make(chan struct{})
	return n, err
}

// waitFor blocks until the captured output contains substr (the harness
// synchronization point — no wall-clock sleeps; the 5 s bound is a deadlock
// guard only).
func (f *fakeTerminal) waitFor(t *testing.T, substr string) {
	t.Helper()
	f.waitForAfter(t, 0, substr)
}

// waitForAfter is waitFor on output past mark — for substrings that already
// appear earlier in the stream.
func (f *fakeTerminal) waitForAfter(t *testing.T, mark int, substr string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		f.mu.Lock()
		ok := strings.Contains(f.buf.String()[min(mark, f.buf.Len()):], substr)
		ch := f.notify
		f.mu.Unlock()
		if ok {
			return
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("output never contained %q; tail: %q", substr, f.output()[max(0, len(f.output())-200):])
		}
		select {
		case <-ch:
		case <-time.After(remaining):
			t.Fatalf("output never contained %q; tail: %q", substr, f.output()[max(0, len(f.output())-200):])
		}
	}
}

func (f *fakeTerminal) Keys() <-chan []byte        { return f.keys }
func (f *fakeTerminal) Resize() <-chan struct{}    { return f.resize }
func (f *fakeTerminal) Interrupt() <-chan struct{} { return f.interrupt }

func (f *fakeTerminal) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// output is the captured byte stream so far.
func (f *fakeTerminal) output() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

// take drains and returns the captured byte stream.
func (f *fakeTerminal) take() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.buf.String()
	f.buf.Reset()
	return s
}

func (f *fakeTerminal) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// key scripts one key chunk (as one Read would deliver it). The channel is
// unbuffered: when key returns, the loop has RECEIVED the chunk, and its
// (sequential) case body runs before the loop selects again — event order
// is deterministic without wall-clock sleeps.
func (f *fakeTerminal) key(s string) { f.keys <- []byte(s) }

// setSize changes the geometry and fires SIGWINCH; the unbuffered channel
// makes it return once the loop has received the event.
func (f *fakeTerminal) setSize(cols, rows int) {
	f.mu.Lock()
	f.cols, f.rows = cols, rows
	f.mu.Unlock()
	f.resize <- struct{}{}
}

func (f *fakeTerminal) sigint() { f.interrupt <- struct{}{} }

func TestFakeTerminalCapture(t *testing.T) {
	f := newFakeTerminal(100, 30, true)
	if _, err := f.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if f.output() != "hi" {
		t.Errorf("output = %q", f.output())
	}
	if got := f.take(); got != "hi" || f.output() != "" {
		t.Errorf("take = %q then %q", got, f.output())
	}
	if c, r := f.Size(); c != 100 || r != 30 {
		t.Errorf("Size = %d×%d", c, r)
	}
	// The key channel is unbuffered (loop ordering); deliver from a goroutine.
	go f.key("q")
	if got := <-f.Keys(); string(got) != "q" {
		t.Errorf("key = %q", got)
	}
}

// The real terminal falls back to 80×24 when the probe fails (a pipe in
// tests): Size on non-TTY files, no key channel on a non-TTY stdin.
func TestTerminalSizeFallback(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	term := NewTerminal(w, r)
	if c, rw := term.Size(); c != fallbackCols || rw != fallbackRows {
		t.Errorf("Size = %d×%d, want %d×%d", c, rw, fallbackCols, fallbackRows)
	}
	if term.Keys() != nil {
		t.Errorf("Keys() non-nil for a non-TTY stdin")
	}
	if err := term.Close(); err != nil {
		t.Fatal(err)
	}
	if err := term.Close(); err != nil {
		t.Errorf("second Close = %v, want nil (idempotent)", err)
	}
}
