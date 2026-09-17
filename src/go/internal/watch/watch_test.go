package watch

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sahil87/tu/internal/render/ansi"
)

// --- Manual time harness: no wall-clock sleeps anywhere ---

// manualClock is the injected Now seam.
type manualClock struct{ t time.Time }

func (c *manualClock) now() time.Time          { return c.t }
func (c *manualClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// manualTimer is a one-shot step the test fires through the hub.
type manualTimer struct {
	c       chan time.Time
	stopped bool
	fired   bool
}

func (m *manualTimer) Chan() <-chan time.Time { return m.c }
func (m *manualTimer) Stop()                  { m.stopped = true }

// manualTicker is the rain ticker's channel form.
type manualTicker struct{ c chan time.Time }

func (m *manualTicker) Chan() <-chan time.Time { return m.c }
func (m *manualTicker) Stop()                  {}

// timerHub hands the loop manual timers/tickers; the test fires them.
type timerHub struct {
	mu      sync.Mutex
	timers  []*manualTimer
	rainC   chan time.Time
	stopped bool
}

func newTimerHub() *timerHub { return &timerHub{rainC: make(chan time.Time, 64)} }

func (h *timerHub) newTimer(_ time.Duration) Timer {
	h.mu.Lock()
	defer h.mu.Unlock()
	t := &manualTimer{c: make(chan time.Time, 1)}
	h.timers = append(h.timers, t)
	return t
}

func (h *timerHub) newTicker(_ time.Duration) Ticker { return &manualTicker{c: h.rainC} }

// tick fires every live countdown step once.
func (h *timerHub) tick() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, t := range h.timers {
		if !t.stopped && !t.fired {
			t.fired = true
			t.c <- time.Time{}
		}
	}
}

// rainTick fires the rain ticker once.
func (h *timerHub) rainTick() { h.rainC <- time.Time{} }

// scriptedPoll answers poll calls under test control: each call announces
// itself on entered, then blocks until release receives, returning the next
// scripted outcome (or the last one when the script runs out).
type scriptedPoll struct {
	mu      sync.Mutex
	calls   int
	frames  []Frame
	entered chan struct{}
	release chan struct{}
	results []pollResult
}

func (p *scriptedPoll) poll(_ context.Context, f Frame) ([]string, Stats, error) {
	p.mu.Lock()
	p.calls++
	p.frames = append(p.frames, f)
	p.mu.Unlock()
	p.entered <- struct{}{}
	<-p.release
	p.mu.Lock()
	r := p.results[min(p.calls-1, len(p.results)-1)]
	p.mu.Unlock()
	return r.lines, r.stats, r.err
}

func (p *scriptedPoll) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// frameSeen returns the Frame the nth call (1-based) received.
func (p *scriptedPoll) frameSeen(n int) Frame {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.frames[n-1]
}

// loopHarness bundles the fakes for one Run session.
type loopHarness struct {
	term   *fakeTerminal
	clock  *manualClock
	hub    *timerHub
	poll   *scriptedPoll
	stderr *strings.Builder
	done   chan []string
}

// startWatch launches Run in a goroutine with the manual seams.
func startWatch(t *testing.T, cols, rows int, results []pollResult, noRain bool) *loopHarness {
	t.Helper()
	term := newFakeTerminal(cols, rows, true)
	clock := &manualClock{t: time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)}
	hub := newTimerHub()
	poll := &scriptedPoll{
		entered: make(chan struct{}, 16),
		release: make(chan struct{}, 16),
		results: results,
	}
	var stderr strings.Builder
	h := &loopHarness{term: term, clock: clock, hub: hub, poll: poll, stderr: &stderr, done: make(chan []string, 1)}
	go func() {
		h.done <- Run(context.Background(), Options{
			Interval:  10,
			NoRain:    noRain,
			Poll:      poll.poll,
			Term:      term,
			Stderr:    &stderr,
			Colors:    ansi.Colors{Enabled: true},
			Now:       clock.now,
			Rand:      rand.New(rand.NewPCG(99, 7)),
			NewTicker: hub.newTicker,
			NewTimer:  hub.newTimer,
		})
	}()
	return h
}

// waitFor blocks until the terminal output contains substr.
func (h *loopHarness) waitFor(t *testing.T, substr string) {
	t.Helper()
	h.term.waitFor(t, substr)
}

// waitForAfter is waitFor on output past mark (for substrings already
// present earlier in the stream).
func (h *loopHarness) waitForAfter(t *testing.T, mark int, substr string) {
	t.Helper()
	h.term.waitForAfter(t, mark, substr)
}

// awaitPoll waits for the next poll call to be in flight, then releases it.
func (h *loopHarness) awaitPoll(t *testing.T) {
	t.Helper()
	select {
	case <-h.poll.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("poll never started")
	}
}

func (h *loopHarness) releasePoll() { h.poll.release <- struct{}{} }

// tickDown drives the countdown from `from` to zero deterministically: each
// step waits for the footer's rewrite — which is written only AFTER the next
// step's timer exists (the loop's body order) — so no tick can be lost
// against a not-yet-created timer. The final tick lands on 0 and starts the
// next poll.
func (h *loopHarness) tickDown(t *testing.T, mark, from int) {
	t.Helper()
	for n := from - 1; n >= 1; n-- {
		h.hub.tick()
		h.waitForAfter(t, mark, "Next refresh: "+itoa(n)+"s")
	}
	h.hub.tick()
}

// quit sends q and returns Run's result.
func (h *loopHarness) quit(t *testing.T) []string {
	t.Helper()
	h.term.key("q")
	select {
	case last := <-h.done:
		return last
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after q")
		return nil
	}
}

// cellRe matches one cursor-position sequence for the right-margin check.
var cellRe = regexp.MustCompile(`\x1b\[(\d+);(\d+)H`)

var pollLines = []string{"", "📊 Combined Usage (daily)", "", "Claude Code  |       24,400 |        3,000 |          400 |       21,000 |        $0.50", ""}

func pollOutcome(cost float64, tokens int64, items map[string]float64) pollResult {
	return pollResult{lines: pollLines, stats: Stats{TotalCost: cost, TotalTokens: tokens, CostByItem: items}}
}

// R1/R4: startup bytes (alt screen, cursor hide, skeleton), first poll →
// frame; q exits with the cursor/alt-screen restore and returns the poll's
// lines.
func TestLoopFirstPollThenQuit(t *testing.T) {
	h := startWatch(t, 100, 30, []pollResult{pollOutcome(0.50, 24400, map[string]float64{"Claude Code": 0.5})}, false)

	h.awaitPoll(t)
	out := h.term.output()
	if !strings.HasPrefix(out, "\x1b[?1049h\x1b[?25l\x1b[H") {
		t.Fatalf("stream prefix = %q", out[:min(40, len(out))])
	}
	if !strings.Contains(out, "Loading...") {
		t.Errorf("skeleton missing: %q", out)
	}
	if !strings.Contains(out, "\x1b[30;1H\x1b[K\x1b[2mRefreshing...\x1b[0m") {
		t.Errorf("refreshing footer missing: %q", out)
	}
	// The refreshing footer appears BEFORE the first poll completes.
	before := len(out)
	h.releasePoll()
	h.waitFor(t, "Next refresh: 10s")
	out = h.term.output()
	if !strings.Contains(out[before:], "$0.50") {
		t.Errorf("frame lacks the table lines: %q", out[before:])
	}
	if !strings.Contains(out[before:], "\x1b[J") {
		t.Errorf("frame lacks the clear: %q", out[before:])
	}
	if f := h.poll.frameSeen(1); f.Prev != nil || f.Compact || f.MaxRows != 15 || f.Width != 100 {
		t.Errorf("first frame = %+v", f)
	}

	last := h.quit(t)
	if strings.Join(last, "\n") != strings.Join(pollLines, "\n") {
		t.Errorf("Run returned %q, want the poll's lines", last)
	}
	if !strings.HasSuffix(h.term.output(), "\x1b[?25h\x1b[?1049l") {
		t.Errorf("stream tail = %q", h.term.output()[max(0, len(h.term.output())-40):])
	}
	if !h.term.isClosed() {
		t.Errorf("terminal not closed on exit")
	}
}

// R3: the countdown decrements once a second, rewriting only the footer; at 0
// the next poll fires. \r cancels the countdown and polls immediately; a
// second \r mid-poll is dropped (the re-entrancy guard).
func TestLoopCountdownAndRefreshKey(t *testing.T) {
	h := startWatch(t, 100, 30, []pollResult{
		pollOutcome(0.50, 24400, map[string]float64{"Claude Code": 0.5}),
		pollOutcome(0.50, 24400, map[string]float64{"Claude Code": 0.5}),
	}, true) // no-rain: the stream is footer/table only

	h.awaitPoll(t)
	h.releasePoll()
	h.waitFor(t, "Next refresh: 10s")
	mark := len(h.term.output())

	// Three 1 s steps: three footer-only rewrites, no full frame.
	h.hub.tick()
	h.waitFor(t, "Next refresh: 9s")
	h.hub.tick()
	h.waitFor(t, "Next refresh: 8s")
	h.hub.tick()
	h.waitFor(t, "Next refresh: 7s")
	out := h.term.output()[mark:]
	if strings.Contains(out, "\x1b[H") {
		t.Errorf("countdown rewrote the frame: %q", out)
	}
	if n := strings.Count(out, "\x1b[30;1H"); n != 3 {
		t.Errorf("footer rewrites = %d, want 3: %q", n, out)
	}

	// \r cancels the countdown and polls immediately.
	mark2 := len(h.term.output())
	h.term.key("\r")
	h.awaitPoll(t)
	if got := h.poll.callCount(); got != 2 {
		t.Fatalf("poll calls = %d, want 2", got)
	}
	h.waitForAfter(t, mark2, "Refreshing...")
	// A second \r and a Space mid-poll are dropped by the re-entrancy guard.
	// The unbuffered key channel makes this deterministic: each key() returns
	// once the loop has received the chunk, and the (sequential) loop runs
	// the case body — the guard, while polling — before it can process the
	// released poll's result.
	h.term.key("\r")
	h.term.key(" ")
	h.releasePoll()
	h.waitForAfter(t, mark2, "Next refresh: 10s")
	if got := h.poll.callCount(); got != 2 {
		t.Errorf("poll calls = %d, want 2 (mid-poll refresh dropped)", got)
	}

	// The countdown reaching 0 fires the next poll (started at 10, the tenth
	// step lands on 0).
	mark3 := len(h.term.output())
	h.tickDown(t, mark3, 10)
	h.awaitPoll(t)
	if got := h.poll.callCount(); got != 3 {
		t.Errorf("poll calls after countdown expiry = %d, want 3", got)
	}
	h.releasePoll()
	h.waitForAfter(t, mark3, "Next refresh: 10s")
	h.quit(t)
}

// R5: SIGWINCH before the first successful poll writes nothing; after it, a
// full frame is re-laid-out (compact at 59 cols: no stats grid, no rain).
func TestLoopResize(t *testing.T) {
	h := startWatch(t, 100, 30, []pollResult{pollOutcome(0.50, 24400, map[string]float64{"Claude Code": 0.5})}, false)

	// Early resize: the skeleton's rain zone is not re-laid-out — zero bytes.
	// setSize returns once the loop has received the event; the poll is still
	// in flight, so nothing else can write.
	h.awaitPoll(t)
	mark := len(h.term.output())
	h.term.setSize(90, 28)
	if out := h.term.output()[mark:]; out != "" {
		t.Errorf("early resize wrote %q", out)
	}

	h.releasePoll()
	h.waitForAfter(t, mark, "\x1b[H") // the first frame
	h.waitForAfter(t, mark, "Next refresh: 10s")

	// Post-poll resize into compact: full frame, no stats grid, no rain.
	mark2 := len(h.term.output())
	h.term.setSize(59, 20)
	h.waitForAfter(t, mark2, "\x1b[H") // the resize flush
	out := h.term.output()
	tail := out[strings.LastIndex(out, "\x1b[H"):]
	if strings.Contains(tail, "Elapsed") {
		t.Errorf("compact frame carries the stats grid: %q", tail)
	}
	h.quit(t)
}

// R2: a Poll error writes the warning to stderr, leaves the frame unchanged,
// and restarts the countdown.
func TestLoopPollError(t *testing.T) {
	h := startWatch(t, 100, 30, []pollResult{
		pollOutcome(0.50, 24400, map[string]float64{"Claude Code": 0.5}),
		{err: errors.New("boom")},
	}, true)
	h.awaitPoll(t)
	h.releasePoll()
	h.waitFor(t, "Next refresh: 10s")
	mark := len(h.term.output())

	// Expire the countdown into the failing poll (the tenth step lands on 0).
	h.tickDown(t, mark, 10)
	h.awaitPoll(t)
	h.waitForAfter(t, mark, "Refreshing...")
	h.releasePoll()
	h.waitForAfter(t, mark, "Next refresh: 10s") // restarted after the failure

	if got := h.stderr.String(); got != "Warning: fetch failed, retrying next cycle\n" {
		t.Errorf("stderr = %q", got)
	}
	// No frame bytes after the mark other than the two footer writes.
	out := h.term.output()[mark:]
	if strings.Contains(out, "\x1b[H") {
		t.Errorf("the frame changed on a poll error: %q", out)
	}
	last := h.quit(t)
	if strings.Join(last, "\n") != strings.Join(pollLines, "\n") {
		t.Errorf("last = %q, want the first poll's lines", last)
	}
}

// R2: two polls 11 s apart — the second poll's Frame.Prev is the first poll's
// CostByItem, and the stats grid shows the session delta.
func TestLoopSecondPollDeltas(t *testing.T) {
	h := startWatch(t, 100, 30, []pollResult{
		pollOutcome(31.79, 64946471, map[string]float64{"Claude Code": 31.79}),
		pollOutcome(31.86, 73409000, map[string]float64{"Claude Code": 31.86}),
	}, true)
	h.awaitPoll(t)
	h.releasePoll()
	h.waitFor(t, "Next refresh: 10s")

	h.clock.advance(11 * time.Second)
	h.term.key(" ")
	h.awaitPoll(t)
	f2 := h.poll.frameSeen(2)
	if len(f2.Prev) != 1 || f2.Prev["Claude Code"] != 31.79 {
		t.Errorf("second frame Prev = %+v", f2.Prev)
	}
	h.releasePoll()
	h.waitFor(t, "+$0.07")
	if out := h.term.output(); !strings.Contains(out, "~$22.91/hr") {
		t.Errorf("second frame lacks the burn rate: %q", out[strings.LastIndex(out, "\x1b[H"):])
	}
	h.quit(t)
}

// R4: Ctrl-C (the \x03 chunk) and SIGINT both exit with the restore bytes.
func TestLoopExitPaths(t *testing.T) {
	newSession := func(t *testing.T) *loopHarness {
		h := startWatch(t, 100, 30, []pollResult{pollOutcome(0.5, 24400, map[string]float64{"Claude Code": 0.5})}, true)
		h.awaitPoll(t)
		h.releasePoll()
		h.waitFor(t, "Next refresh: 10s")
		return h
	}

	t.Run("ctrl-c byte", func(t *testing.T) {
		h := newSession(t)
		h.term.key("\x03")
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return after \\x03")
		}
		if !strings.HasSuffix(h.term.output(), "\x1b[?25h\x1b[?1049l") {
			t.Errorf("stream tail wrong")
		}
	})

	t.Run("SIGINT", func(t *testing.T) {
		h := newSession(t)
		h.term.sigint()
		select {
		case <-h.done:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return after SIGINT")
		}
		if !strings.HasSuffix(h.term.output(), "\x1b[?25h\x1b[?1049l") {
			t.Errorf("stream tail wrong")
		}
	})

	t.Run("unknown keys ignored", func(t *testing.T) {
		h := newSession(t)
		// Unbuffered keys: each key() returns once the loop has received the
		// chunk; the ignored-chunk case body is a no-op, so after both
		// receipts the session state is settled — a buggy exit would fire
		// done, which the short drain below observes.
		h.term.key("\x1b[A") // arrow-up escape chunk
		h.term.key("x")
		if got := h.poll.callCount(); got != 1 {
			t.Errorf("poll calls = %d, want 1", got)
		}
		select {
		case <-h.done:
			t.Fatal("unknown key exited the loop")
		case <-time.After(50 * time.Millisecond):
		}
		h.quit(t)
	})
}

// Non-TTY stdin: no key channel — SIGINT is the only exit.
func TestLoopNoKeys(t *testing.T) {
	term := newFakeTerminal(80, 24, false)
	clock := &manualClock{t: time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)}
	hub := newTimerHub()
	poll := &scriptedPoll{
		entered: make(chan struct{}, 4),
		release: make(chan struct{}, 4),
		results: []pollResult{pollOutcome(0.5, 24400, map[string]float64{"Claude Code": 0.5})},
	}
	var stderr strings.Builder
	done := make(chan []string, 1)
	go func() {
		done <- Run(context.Background(), Options{
			Interval: 10, Poll: poll.poll, Term: term, Stderr: &stderr,
			Colors: ansi.Colors{}, Now: clock.now,
			Rand: rand.New(rand.NewPCG(1, 2)), NewTicker: hub.newTicker, NewTimer: hub.newTimer,
		})
	}()
	select {
	case <-poll.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("poll never started")
	}
	poll.release <- struct{}{}
	term.waitFor(t, "Next refresh: 10s")
	term.sigint()
	select {
	case last := <-done:
		if len(last) == 0 {
			t.Error("last lines empty after a successful poll")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after SIGINT")
	}
}

// --- Frame goldens (R19): the byte stream of scripted sessions under the
// fixed clock and seeded RNG, hand-verified against the TS byte rules ---

// goldenSession runs first poll → quit and returns the whole stream split at
// the first frame: [startup+refreshing, frame+countdown-footer, exit].
func goldenSession(t *testing.T, cols, rows int, lines []string, stats Stats, noRain bool) []string {
	t.Helper()
	h := startWatch(t, cols, rows, []pollResult{{lines: lines, stats: stats}}, noRain)
	h.awaitPoll(t)
	startup := h.term.output()
	h.releasePoll()
	h.waitForAfter(t, len(startup), "Next refresh: 10s")
	frame := h.term.output()[len(startup):]
	last := h.quit(t)
	if strings.Join(last, "\n") != strings.Join(lines, "\n") {
		t.Errorf("last lines = %q", last)
	}
	exit := h.term.output()[len(startup)+len(frame):]
	return []string{startup, frame, exit}
}

func TestLoopFrameGoldens(t *testing.T) {
	stats1 := Stats{TotalCost: 31.79, TotalTokens: 64946471, CostByItem: map[string]float64{"Claude Code": 31.79}}

	t.Run("first poll 100x30", func(t *testing.T) {
		parts := goldenSession(t, 100, 30, pollLines, stats1, false)
		goldenFile(t, "frame_first_poll_100x30.golden", parts[0]+"=== frame ===\n"+parts[1]+"=== exit ===\n"+parts[2])
	})

	t.Run("second poll 100x30", func(t *testing.T) {
		h := startWatch(t, 100, 30, []pollResult{
			{lines: pollLines, stats: stats1},
			{lines: pollLines, stats: Stats{TotalCost: 31.86, TotalTokens: 73409000, CostByItem: map[string]float64{"Claude Code": 31.86}}},
		}, false)
		h.awaitPoll(t)
		h.releasePoll()
		h.waitFor(t, "Next refresh: 10s")
		mark := len(h.term.output())
		h.clock.advance(11 * time.Second)
		h.term.key(" ")
		h.awaitPoll(t)
		h.releasePoll()
		h.waitForAfter(t, mark, "+$0.07")
		h.waitForAfter(t, mark, "Next refresh: 10s")
		goldenFile(t, "frame_second_poll_100x30.golden", h.term.output()[mark:])
		h.quit(t)
	})

	t.Run("compact 59x20", func(t *testing.T) {
		// The compact table lines themselves are command's (view goldens pin
		// them); watch's part is no stats grid and no rain.
		compact := []string{"", "📊 Combined Usage (daily)", "", "Claude Code          $31.79", ""}
		parts := goldenSession(t, 59, 20, compact, stats1, false)
		goldenFile(t, "frame_compact_59x20.golden", parts[0]+"=== frame ===\n"+parts[1]+"=== exit ===\n"+parts[2])
	})

	t.Run("no-rain 100x30", func(t *testing.T) {
		parts := goldenSession(t, 100, 30, pollLines, stats1, true)
		goldenFile(t, "frame_norain_100x30.golden", parts[0]+"=== frame ===\n"+parts[1]+"=== exit ===\n"+parts[2])
		if strings.Contains(parts[1], "\x1b[92m") {
			t.Errorf("no-rain frame carries rain bytes")
		}
	})

	t.Run("right margin 100x12", func(t *testing.T) {
		// 4 stats + 8 table lines = 12 content rows, widest exactly 87: no
		// room below, margin 100 − 87 − 2 = 11 ≥ 10 → zone {11, 11, 1, 89}.
		wide := []string{strings.Repeat("x", 87)}
		for len(wide) < 8 {
			wide = append(wide, "row")
		}
		parts := goldenSession(t, 100, 12, wide, stats1, false)
		goldenFile(t, "frame_rightmargin_100x12.golden", parts[0]+"=== frame ===\n"+parts[1]+"=== exit ===\n"+parts[2])
		// Rain cells sit at startCol 89 → 1-based columns ≥ 90.
		rainCol := false
		for _, m := range cellRe.FindAllStringSubmatch(parts[1], -1) {
			var r, c int
			fmt.Sscanf(m[1], "%d", &r)
			fmt.Sscanf(m[2], "%d", &c)
			if c >= 90 && r >= 1 && r <= 11 {
				rainCol = true
			}
		}
		if !rainCol {
			t.Errorf("no right-margin rain cell (col ≥ 90) in frame")
		}
	})
}
