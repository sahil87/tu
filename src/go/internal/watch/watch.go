package watch

import (
	"context"
	"io"
	"maps"
	"math/rand/v2"
	"time"

	"github.com/sahil87/tu/internal/render/ansi"
)

// Poll runs one refresh: the caller composes command.Run with the per-poll
// render options and the live width. It is the only thing watch knows about
// command — watch imports no source, query, view or render encoder package.
type Poll func(ctx context.Context, f Frame) (Lines []string, Stats Stats, err error)

// Frame is one poll's render inputs (the TS FormatOptions the watch loop
// builds): the previous poll's per-item values (nil on the first poll and
// whenever the previous map was empty — the TS `size > 0` guard), the compact
// switch, the constant row budget, and the live width.
type Frame struct {
	Prev    map[string]float64
	Compact bool
	MaxRows int
	Width   int
}

// Stats are one poll's watch-mode numbers: the session totals and the
// per-item cost map the next poll's deltas compare against.
type Stats struct {
	TotalCost   float64
	TotalTokens int64
	CostByItem  map[string]float64
}

// Options are Run's inputs; the timer seams are nil at the edge (real time)
// and injected in tests — no test sleeps on wall-clock time.
type Options struct {
	Interval int // seconds, already validated by Parse (5–3600, default 10)
	NoRain   bool
	Poll     Poll
	Term     Terminal
	Stderr   io.Writer
	Colors   ansi.Colors
	Now      func() time.Time
	Rand     *rand.Rand // math/rand/v2; seeded in tests

	// NewTicker / NewTimer are the timer seams (the rain ticker and the 1 s
	// countdown steps); nil installs the real-time constructors.
	NewTicker func(d time.Duration) Ticker
	NewTimer  func(d time.Duration) Timer
}

// Ticker / Timer are the loop's timer seam: a fire channel plus Stop. The
// countdown re-arms by constructing a fresh Timer per step (the TS
// setTimeout chain).
type Ticker interface {
	Chan() <-chan time.Time
	Stop()
}

// Timer is a one-shot countdown step.
type Timer interface {
	Chan() <-chan time.Time
	Stop()
}

type realTicker struct{ t *time.Ticker }

func (r realTicker) Chan() <-chan time.Time { return r.t.C }
func (r realTicker) Stop()                  { r.t.Stop() }

type realTimer struct{ t *time.Timer }

func (r realTimer) Chan() <-chan time.Time { return r.t.C }
func (r realTimer) Stop()                  { r.t.Stop() }

// pollResult is the poll worker's delivery to the loop.
type pollResult struct {
	lines []string
	stats Stats
	err   error
}

// Run blocks until q, Ctrl-C or SIGINT, then returns the last rendered table
// lines for cmd/tu to print on the normal screen. It never calls os.Exit.
//
// One goroutine (this one) owns every terminal write. Startup order (the TS
// runWatch): enter the alt screen (`\x1b[?1049h` then `\x1b[?25l`), write the
// skeleton, lay out the rain zone against the skeleton lines, start the rain
// ticker (whenever rain is on — the zone may enable on a later layout), then
// run the first poll — the key/resize/interrupt inputs are already being
// selected on. A poll request while one is in flight is dropped (the TS
// re-entrancy guard).
func Run(ctx context.Context, o Options) (last []string) {
	newTicker := o.NewTicker
	if newTicker == nil {
		newTicker = func(d time.Duration) Ticker { return realTicker{time.NewTicker(d)} }
	}
	newTimer := o.NewTimer
	if newTimer == nil {
		newTimer = func(d time.Duration) Timer { return realTimer{time.NewTimer(d)} }
	}

	// Alt screen, cursor hidden (the TS enterAltScreen).
	o.Term.Write([]byte("\x1b[?1049h"))
	o.Term.Write([]byte("\x1b[?25l"))

	// The loading skeleton, then the rain zone against its geometry so rain
	// animates from the first tick (the TS layoutForSkeleton).
	cols, rows := o.Term.Size()
	skeleton := Skeleton(cols, o.Now(), o.Colors)
	o.Term.Write(SkeletonFrame(skeleton))
	zone := LaySkeleton(skeleton, cols, rows, o.NoRain).Rain
	rain := setupRain(zone, o.Rand, o.Colors, nil)

	var rainTicker Ticker
	if !o.NoRain {
		rainTicker = newTicker(RainTick)
	}
	var rainC <-chan time.Time
	if rainTicker != nil {
		rainC = rainTicker.Chan()
	}

	// Session state (the TS WatchSession plus the loop's render cache).
	var session Session
	var prevCosts map[string]float64
	var todayCost float64
	var haveData bool // the TS rerender() no-op until the first successful poll
	var footer string // the current footer content (the TS StatusPanel.content)

	polling := false
	pollResults := make(chan pollResult, 1)

	// renderFooter is the TS renderStatus: push-driven on countdown state
	// changes only, positioned at the live row count, built against the live
	// width (the TS getTermWidth() reads at every render).
	renderFooter := func(count int, refreshing bool) {
		curCols, curRows := o.Term.Size()
		footer = Footer(count, refreshing, curCols, o.Colors)
		o.Term.Write([]byte(FooterLine(footer, curRows)))
	}

	// flush is the TS compositor.flush: the full frame (carrying the current
	// footer), then the rain frame re-emitted — the flush's clears erased
	// every drawn rain cell, and the renderer rewrites every occupied cell.
	flush := func(l Layout) {
		rainOut := ""
		if rain != nil {
			rainOut = rain.Render(l.Rain.StartRow)
		}
		o.Term.Write(l.Frame(footer, rainOut))
	}

	startPoll := func() {
		if polling {
			return // the TS re-entrancy guard
		}
		polling = true
		renderFooter(0, true) // "Refreshing..."
		curCols, _ := o.Term.Size()
		frame := Frame{Prev: prevCosts, Compact: curCols < CompactThreshold, MaxRows: MaxRows, Width: curCols}
		go func() {
			lines, stats, err := o.Poll(ctx, frame)
			pollResults <- pollResult{lines, stats, err}
		}()
	}

	var countdownTimer Timer
	var countdownC <-chan time.Time
	countdown := 0

	// startCountdown is the TS startCountdown: set the value, render the
	// footer, tick down once a second; at 0 the next poll fires (the footer
	// never renders "0s" — the poll's "Refreshing..." replaces it).
	startCountdown := func() {
		countdown = o.Interval
		renderFooter(countdown, false)
		countdownTimer = newTimer(CountdownTick)
		countdownC = countdownTimer.Chan()
	}
	cancelCountdown := func() {
		if countdownTimer != nil {
			countdownTimer.Stop()
			countdownTimer = nil
			countdownC = nil
		}
	}

	// cleanup is the TS cleanup: stop the timers, restore the terminal, show
	// the cursor, leave the alt screen. Run returns `last` afterwards;
	// cmd/tu prints it on the normal screen and exits 0.
	cleanup := func() {
		cancelCountdown()
		if rainTicker != nil {
			rainTicker.Stop()
		}
		o.Term.Close()
		o.Term.Write([]byte("\x1b[?25h"))
		o.Term.Write([]byte("\x1b[?1049l"))
	}

	startPoll() // the initial poll

	for {
		select {
		case key, ok := <-o.Term.Keys():
			if !ok {
				continue
			}
			switch string(key) {
			case "q", "\x03":
				cleanup()
				return last
			case "\r", "\n", " ":
				// Enter or Space — immediate refresh (the countdown restarts
				// when the poll completes).
				cancelCountdown()
				startPoll()
			default:
				// Arrow keys and other multi-byte chunks are ignored.
			}

		case <-o.Term.Interrupt():
			cleanup()
			return last

		case <-o.Term.Resize():
			// The TS rerender(): a no-op until the first successful poll (the
			// skeleton's rain zone is NOT re-laid-out on an early resize);
			// afterwards re-layout from the cached data and flush.
			if !haveData {
				continue
			}
			cols, rows = o.Term.Size()
			layout := Lay(StatsGrid(session, todayCost, o.Now(), o.Colors), last, cols, rows, o.NoRain)
			zone = layout.Rain
			rain = setupRain(zone, o.Rand, o.Colors, rain)
			flush(layout)

		case res := <-pollResults:
			polling = false
			if res.err != nil {
				// The TS catch: warn on stderr, restart the countdown, no
				// frame change.
				io.WriteString(o.Stderr, "Warning: fetch failed, retrying next cycle\n")
				startCountdown()
				continue
			}
			now := o.Now()
			todayCost = res.stats.TotalCost
			if len(session.Polls) == 0 {
				session.StartTime = now
				session.StartCost = todayCost
				session.StartTokens = res.stats.TotalTokens
			}
			session.Polls = append(session.Polls, PollPoint{At: now, Cost: todayCost})
			session.TotalTokens = res.stats.TotalTokens
			last = res.lines
			haveData = true

			cols, rows = o.Term.Size() // live geometry, as the TS layoutAndUpdate reads it
			layout := Lay(StatsGrid(session, todayCost, now, o.Colors), last, cols, rows, o.NoRain)
			zone = layout.Rain
			rain = setupRain(zone, o.Rand, o.Colors, rain)
			// The flush carries the current footer ("Refreshing..."); the
			// countdown's initial footer write follows (the TS flush →
			// startCountdown order).
			flush(layout)
			if len(res.stats.CostByItem) > 0 {
				prevCosts = maps.Clone(res.stats.CostByItem)
			} else {
				prevCosts = nil
			}
			startCountdown()

		case <-countdownC:
			countdownTimer = nil
			countdownC = nil
			countdown--
			if countdown <= 0 {
				startPoll()
			} else {
				renderFooter(countdown, false)
				countdownTimer = newTimer(CountdownTick)
				countdownC = countdownTimer.Chan()
			}

		case <-rainC:
			if rain == nil {
				continue
			}
			rain.Tick()
			if out := rain.Render(zone.StartRow); out != "" {
				o.Term.Write([]byte(out))
			}
		}
	}
}

// setupRain applies a zone to the rain state (the TS RainLayer.setup +
// resize): disabled or degenerate geometry drops the state; an existing state
// resizes (a no-op when the geometry is unchanged, so the first real poll
// after the skeleton keeps the drops when they match); otherwise a new state
// is created.
func setupRain(zone RainZone, rng *rand.Rand, c ansi.Colors, prev *RainState) *RainState {
	if !zone.Enabled || zone.Cols <= 0 || zone.Rows <= 0 {
		return nil
	}
	if prev == nil {
		return NewRainState(zone.Cols, zone.Rows, zone.StartCol, rng, c)
	}
	prev.Resize(zone.Cols, zone.Rows, zone.StartCol)
	return prev
}
