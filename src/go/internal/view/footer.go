package view

import (
	"strings"
	"time"

	"github.com/sahil87/tu/internal/query"
)

// periodUnitSuffix is the footer avg unit per period (the TS
// PERIOD_UNIT_SUFFIX).
func periodUnitSuffix(p query.Period) string {
	switch p {
	case query.Weekly:
		return "/week"
	case query.Monthly:
		return "/month"
	default:
		return "/day"
	}
}

// footerText is the history summary line (the TS renderHistoryFooter minus
// the legend, which the encoder appends outside the dim span):
//
//	avg {v}{/day|/week|/month}[ · this month {v}] · peak {v} ({label})[ · ┊ = {p95} (p95)]
//
// `this month` is daily-only over rows prefixed by CurrentLabel(Monthly, now),
// omitted when none match; peak is the FIRST strict maximum starting from 0,
// so an all-zero window prints `peak $0.00` with no parentheses; the p95 term
// appears only under the two-zone scale. Parts join with " · ".
func footerText(labels []string, values []float64, p query.Period, s Scale, now time.Time, m Metric) string {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	parts := []string{"avg " + fmtMetric(sum/float64(len(values)), m) + periodUnitSuffix(p)}

	if p == query.Daily {
		monthPrefix := query.CurrentLabel(query.Monthly, now)
		monthSum := 0.0
		hasMonthRows := false
		for i, label := range labels {
			if strings.HasPrefix(label, monthPrefix) {
				monthSum += values[i]
				hasMonthRows = true
			}
		}
		if hasMonthRows {
			parts = append(parts, "this month "+fmtMetric(monthSum, m))
		}
	}

	peak := 0.0
	peakLabel := ""
	for i, label := range labels {
		if values[i] > peak {
			peak = values[i]
			peakLabel = label
		}
	}
	if peakLabel == "" {
		parts = append(parts, "peak "+fmtMetric(peak, m))
	} else {
		parts = append(parts, "peak "+fmtMetric(peak, m)+" ("+peakLabel+")")
	}

	if s.TwoZone {
		parts = append(parts, scaleRule+" = "+fmtMetric(s.P95, m)+" (p95)")
	}
	return strings.Join(parts, " · ")
}
