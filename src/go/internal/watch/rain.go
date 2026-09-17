package watch

import (
	"math"
	"math/rand/v2"
	"strings"

	"github.com/sahil87/tu/internal/render"
	"github.com/sahil87/tu/internal/render/ansi"
)

// The rain character pool (the TS rain.ts): half-width katakana + digits +
// A–Za–z. Every character is BMP, so rune indexing matches the TS code-unit
// indexing.
var rainPool = []rune("ｦｱｲｳｴｵｶｷｸｹｺｻｼｽｾｿﾀﾁﾂﾃﾄﾅﾆﾇﾈﾉﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ" +
	"0123456789" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")

// The rain tuning constants (the TS rain.ts): the density calibrated at 20
// rows, the 3× scale cap, drop speed 0.3–1.0 rows/tick, trail length 3–8,
// respawn delay 0–5 ticks, shimmer rate 0.05.
const (
	rainDensity         = 0.3
	rainDensityRefRows  = 20
	rainMaxDensityScale = 3
	rainMinSpeed        = 0.3
	rainMaxSpeed        = 1.0
	rainMinLength       = 3
	rainMaxLength       = 8
	rainMaxRespawnDelay = 5
	rainShimmerRate     = 0.05
)

// ActiveDropCount is the TS computeActiveDropCount: the count scales with the
// zone HEIGHT (density × heightScale, clamped 1× at/below 20 rows, capped 3×
// at 60+), rounded with the JS Math.round rule.
func ActiveDropCount(cols, rows int) int {
	heightScale := math.Min(rainMaxDensityScale, math.Max(1, float64(rows)/rainDensityRefRows))
	return int(render.JSRound(float64(cols) * rainDensity * heightScale))
}

// rainDrop is one falling character trail (the TS Drop).
type rainDrop struct {
	col    int
	row    float64 // fractional position
	speed  float64 // rows per tick
	length int     // trail length
	delay  int     // ticks before starting
	chars  []rune  // pre-generated
}

// RainState is the rain layer's state machine (the TS RainState) with an
// injected RNG and Colors. The previous frame's occupied cells are kept in
// INSERTION order (the JS Set's iteration order) so the vacated-cell clears
// emit in a deterministic order.
type RainState struct {
	cols, rows, startCol int // startCol is the 0-based column offset (right-margin mode)
	rng                  *rand.Rand
	colors               ansi.Colors
	drops                []rainDrop
	prev                 [][2]int // occupied (row, col) of the previous render, insertion order
}

// NewRainState builds the state and scatters the initial drops (the TS
// constructor + initDrops): a Fisher–Yates shuffle of the column indices,
// then drop i on columns[i % cols] (round-robin; the shipped constants never
// exceed one drop per column).
func NewRainState(cols, rows, startCol int, rng *rand.Rand, c ansi.Colors) *RainState {
	r := &RainState{cols: cols, rows: rows, startCol: startCol, rng: rng, colors: c}
	r.initDrops()
	return r
}

// Resize keeps the drops when (cols, rows, startCol) are unchanged and
// re-initialises otherwise (the TS resize; startRow is a render-time input,
// not part of the geometry).
func (r *RainState) Resize(cols, rows, startCol int) {
	if cols == r.cols && rows == r.rows && startCol == r.startCol {
		return
	}
	r.cols, r.rows, r.startCol = cols, rows, startCol
	r.initDrops()
}

func (r *RainState) initDrops() {
	r.drops = nil
	r.prev = nil
	count := ActiveDropCount(r.cols, r.rows)
	columns := make([]int, r.cols)
	for i := range columns {
		columns[i] = i
	}
	for i := len(columns) - 1; i > 0; i-- {
		j := int(r.rng.Float64() * float64(i+1))
		columns[i], columns[j] = columns[j], columns[i]
	}
	for i := 0; i < count; i++ {
		r.drops = append(r.drops, r.createDrop(columns[i%r.cols], true))
	}
}

// createDrop is the TS createDrop: the RNG draw order is length, the chars
// (length draws), the scattered row (scatter only), speed, and the respawn
// delay (respawn only) — the seeded golden depends on it.
func (r *RainState) createDrop(col int, scatter bool) rainDrop {
	length := r.randInt(rainMinLength, rainMaxLength)
	chars := make([]rune, length)
	for i := range chars {
		chars[i] = r.randChar()
	}
	d := rainDrop{col: col, length: length, chars: chars}
	if scatter {
		d.row = r.randFloat(-float64(length), float64(r.rows))
	} else {
		d.row = -float64(length)
	}
	d.speed = r.randFloat(rainMinSpeed, rainMaxSpeed)
	if !scatter {
		d.delay = r.randInt(0, rainMaxRespawnDelay)
	}
	return d
}

// Tick advances every drop (the TS tick): a delayed drop counts down; others
// move by their speed, shimmer (each trail char replaced with probability
// 0.05), and respawn in place when fully off-screen (row − length > rows:
// back to −length with new speed, length, delay and chars).
func (r *RainState) Tick() {
	for i := range r.drops {
		d := &r.drops[i]
		if d.delay > 0 {
			d.delay--
			continue
		}
		d.row += d.speed
		for j := range d.chars {
			if r.rng.Float64() < rainShimmerRate {
				d.chars[j] = r.randChar()
			}
		}
		if math.Floor(d.row)-float64(d.length) > float64(r.rows) {
			nd := r.createDrop(d.col, false)
			d.row, d.speed, d.length, d.delay, d.chars = nd.row, nd.speed, nd.length, nd.delay, nd.chars
		}
	}
}

// Render emits the cursor-positioned frame (the TS render): for each active
// drop and trail index, `\x1b[{startRow+r};{startCol+col+1}H` + the colored
// char (BrightGreen head, Green body, DimGreen the last two), then a
// `\x1b[{r};{c}H ` clear for every previously occupied cell not occupied now.
// "" when rows <= 0.
func (r *RainState) Render(startRow int) string {
	if r.rows <= 0 {
		return ""
	}
	var b strings.Builder
	cur := make([][2]int, 0, len(r.prev))
	curSet := make(map[[2]int]bool, len(r.prev))

	for _, d := range r.drops {
		if d.delay > 0 {
			continue
		}
		head := int(math.Floor(d.row))
		for i := 0; i < d.length; i++ {
			row := head - i
			if row < 0 || row >= r.rows {
				continue
			}
			key := [2]int{row, d.col}
			if !curSet[key] {
				curSet[key] = true
				cur = append(cur, key)
			}
			ch := d.chars[i%d.length]
			var colored string
			switch {
			case i == 0:
				colored = r.colors.BrightGreen(string(ch))
			case i < d.length-2:
				colored = r.colors.Green(string(ch))
			default:
				colored = r.colors.DimGreen(string(ch))
			}
			b.WriteString("\x1b[" + itoa(startRow+row) + ";" + itoa(r.startCol+d.col+1) + "H" + colored)
		}
	}

	for _, key := range r.prev {
		if !curSet[key] {
			row, col := key[0], key[1]
			if row >= 0 && row < r.rows {
				b.WriteString("\x1b[" + itoa(startRow+row) + ";" + itoa(r.startCol+col+1) + "H ")
			}
		}
	}

	r.prev = cur
	return b.String()
}

// randFloat is the TS randFloat: min + random() × (max − min).
func (r *RainState) randFloat(minV, maxV float64) float64 {
	return minV + r.rng.Float64()*(maxV-minV)
}

// randInt is the TS randInt: min + floor(random() × (max − min + 1)).
func (r *RainState) randInt(minV, maxV int) int {
	return minV + int(r.rng.Float64()*float64(maxV-minV+1))
}

// randChar is the TS randChar: a uniform pool character.
func (r *RainState) randChar() rune {
	return rainPool[int(r.rng.Float64()*float64(len(rainPool)))]
}
