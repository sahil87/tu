package view

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestGlyphs(t *testing.T) {
	cases := []struct {
		name       string
		value, max float64
		width      int
		want       string
	}{
		{"zero value", 0, 100, 10, ""},
		{"zero max", 100, 0, 10, ""},
		{"full", 100, 100, 10, strings.Repeat("█", 10)},
		{"half", 50, 100, 10, strings.Repeat("█", 5)},
		{"exact full block", 25, 100, 4, "█"},            // scaled 1.0 → 1 full, 0 eighths
		{"half-eighth", 3, 16, 8, "█▌"},                  // scaled 1.5 → eighths 4 → "█▌"
		{"sliver floors to min bar", 0.01, 100, 10, "▏"}, // scaled 0.001 → "" → ▏
	}
	for _, c := range cases {
		if got := Glyphs(c.value, c.max, c.width); got != c.want {
			t.Errorf("%s: Glyphs(%v, %v, %d) = %q, want %q", c.name, c.value, c.max, c.width, got, c.want)
		}
	}
	// eighths == 8 carry: scaled = 0.999 × 4 = 3.996 → full 3, eighths round(0.996×8)=8 → 4 full blocks
	if got := Glyphs(0.999, 1, 4); got != strings.Repeat("█", 4) {
		t.Errorf("eighths carry: Glyphs(0.999, 1, 4) = %q, want %q", got, strings.Repeat("█", 4))
	}
}

func TestPercentile(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		p    float64
		want float64
	}{
		{"empty", nil, 95, 0},
		{"single", []float64{42}, 95, 42},
		{"p0", []float64{10, 20, 30}, 0, 10},
		{"p100", []float64{10, 20, 30}, 100, 30},
		{"interpolation", []float64{10, 20, 30}, 50, 20},
		{"fractional index", []float64{100, 300, 1091.67, 4031.61}, 95, 1091.67 + (4031.61-1091.67)*0.85},
	}
	for _, c := range cases {
		if got := Percentile(c.in, c.p); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%s: Percentile(%v, %v) = %v, want %v", c.name, c.in, c.p, got, c.want)
		}
	}
}

// twoZoneValues is the TS test's canonical outlier window: 21 well-behaved
// rows (100…300) plus 1,091.67 and 4,031.61 → p95 = 1012.503.
func twoZoneValues() []float64 {
	values := make([]float64, 0, 23)
	for i := 0; i < 21; i++ {
		values = append(values, 100+float64(i)*10)
	}
	return append(values, 1091.67, 4031.61)
}

func TestComputeScale(t *testing.T) {
	t.Run("no nonzero values is single-zone", func(t *testing.T) {
		s := ComputeScale([]float64{0, 0, 0}, 30)
		if s.TwoZone || s.Max != 0 {
			t.Errorf("scale = %+v, want single-zone max 0", s)
		}
	})
	t.Run("max == 1.5 × p95 stays single-zone", func(t *testing.T) {
		// p95 of [100] is 100; 150 == 1.5 × 100 is NOT > the trigger.
		s := ComputeScale([]float64{100, 150}, 30)
		if s.TwoZone {
			t.Errorf("scale = %+v, want single-zone at the boundary", s)
		}
	})
	t.Run("two-zone geometry at width 30", func(t *testing.T) {
		s := ComputeScale(twoZoneValues(), 30)
		if !s.TwoZone {
			t.Fatalf("scale = %+v, want two-zone", s)
		}
		if math.Abs(s.P95-1012.503) > 1e-9 {
			t.Errorf("P95 = %v, want 1012.503", s.P95)
		}
		if s.Max != 4031.61 || s.OverflowZone != 8 || s.MainZone != 21 {
			t.Errorf("scale = %+v, want max 4031.61, overflow 8, main 21", s)
		}
	})
	t.Run("two-zone geometry at width 10", func(t *testing.T) {
		s := ComputeScale(twoZoneValues(), 10)
		if !s.TwoZone || s.OverflowZone != 4 || s.MainZone != 5 {
			t.Errorf("scale = %+v, want two-zone overflow 4, main 5", s)
		}
	})
	t.Run("row at exactly p95 has empty overflow", func(t *testing.T) {
		s := ComputeScale(twoZoneValues(), 30)
		b := rowBar(s.P95, s, nil)
		if b.Overflow != "" {
			t.Errorf("Overflow = %q, want empty at exactly p95", b.Overflow)
		}
		if b.Main != strings.Repeat("█", s.MainZone) {
			t.Errorf("Main = %q, want a full main zone at p95", b.Main)
		}
	})
	t.Run("outlier row carries overflow", func(t *testing.T) {
		s := ComputeScale(twoZoneValues(), 30)
		b := rowBar(4031.61, s, nil)
		if b.Main != strings.Repeat("█", s.MainZone) || b.Overflow != strings.Repeat("█", s.OverflowZone) {
			t.Errorf("bar = %+v, want both zones full at max", b)
		}
	})
	t.Run("zero row in a two-zone window", func(t *testing.T) {
		s := ComputeScale(twoZoneValues(), 30)
		b := rowBar(0, s, nil)
		if b.Main != "" || b.Overflow != "" {
			t.Errorf("bar = %+v, want empty glyphs (the encoder pads and rules)", b)
		}
	})
}

func TestApportion(t *testing.T) {
	cases := []struct {
		name   string
		shares []float64
		total  int
		want   []int
	}{
		{"exact thirds", []float64{1, 1, 1}, 6, []int{2, 2, 2}},
		{"ties to the earlier index", []float64{2, 1, 1}, 5, []int{3, 1, 1}},
		{"zero share gets zero", []float64{0, 1}, 3, []int{0, 3}},
		{"zero total", []float64{1, 1}, 0, []int{0, 0}},
		{"all zero shares", []float64{0, 0}, 4, []int{0, 0}},
		{"fractional carry", []float64{1, 1, 1}, 10, []int{4, 3, 3}},
	}
	for _, c := range cases {
		got := Apportion(c.shares, c.total)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Apportion(%v, %d) = %v, want %v", c.name, c.shares, c.total, got, c.want)
		}
		sum := 0
		for _, n := range got {
			sum += n
		}
		shareSum := 0.0
		for _, s := range c.shares {
			shareSum += s
		}
		if c.total > 0 && shareSum > 0 && sum != c.total {
			t.Errorf("%s: segments sum to %d, want %d", c.name, sum, c.total)
		}
	}
}
