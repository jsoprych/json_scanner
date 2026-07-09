package indicators

import "math"

// RollingHigh returns the highest value over the trailing period (exclusive of current bar).
// Positions before the first full window are NaN. The indicator at index i excludes
// bar i (no lookahead).
func RollingHigh(vals []float64, period int) []float64 {
	return rollingExtreme(vals, period, true)
}

// RollingLow returns the lowest value over the trailing period (exclusive of current bar).
// Positions before the first full window are NaN. The indicator at index i excludes
// bar i (no lookahead).
func RollingLow(vals []float64, period int) []float64 {
	return rollingExtreme(vals, period, false)
}

func rollingExtreme(vals []float64, period int, high bool) []float64 {
	out := make([]float64, len(vals))
	if period < 1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	for i := range vals {
		// Need period bars before index i (indices i-period to i-1)
		if i < period {
			out[i] = math.NaN()
			continue
		}
		ext := vals[i-period]
		for _, v := range vals[i-period+1 : i] {
			if high && v > ext || !high && v < ext {
				ext = v
			}
		}
		out[i] = ext
	}
	return out
}

// GapPct returns the overnight gap percentage.
// Gap = (open[i-1] / close[i-2] - 1) * 100
// Values exclude bar i (no lookahead).
func GapPct(opens, closes []float64) []float64 {
	out := make([]float64, len(opens))
	for i := range opens {
		if i < 2 || closes[i-2] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (opens[i-1] / closes[i-2] - 1) * 100
	}
	return out
}

// TrueRange returns the True Range for each bar.
// TR = max(high-low, |high-prev_close|, |low-prev_close|)
// Values exclude bar i (no lookahead).
func TrueRange(highs, lows, closes []float64) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if i < 2 {
			out[i] = math.NaN()
			continue
		}
		hl := highs[i-1] - lows[i-1]
		hc := math.Abs(highs[i-1] - closes[i-2])
		lc := math.Abs(lows[i-1] - closes[i-2])
		out[i] = math.Max(hl, math.Max(hc, lc))
	}
	return out
}

// Is52wHigh returns true if close is at or above the 52-week high.
// Values exclude bar i (no lookahead).
func Is52wHigh(closes, high52w []float64) []bool {
	out := make([]bool, len(closes))
	for i := range closes {
		if math.IsNaN(high52w[i]) {
			out[i] = false
			continue
		}
		out[i] = closes[i-1] >= high52w[i]
	}
	return out
}

// Is52wLow returns true if close is at or below the 52-week low.
// Values exclude bar i (no lookahead).
func Is52wLow(closes, low52w []float64) []bool {
	out := make([]bool, len(closes))
	for i := range closes {
		if math.IsNaN(low52w[i]) {
			out[i] = false
			continue
		}
		out[i] = closes[i-1] <= low52w[i]
	}
	return out
}

// PctOff52wHigh returns the percentage distance from the 52-week high.
// Pct = (close / high52w - 1) * 100
// Values exclude bar i (no lookahead).
func PctOff52wHigh(closes, high52w []float64) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if math.IsNaN(high52w[i]) || high52w[i] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (closes[i-1] / high52w[i] - 1) * 100
	}
	return out
}

// PctAbove52wLow returns the percentage distance from the 52-week low.
// Pct = (close / low52w - 1) * 100
// Values exclude bar i (no lookahead).
func PctAbove52wLow(closes, low52w []float64) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if math.IsNaN(low52w[i]) || low52w[i] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (closes[i-1] / low52w[i] - 1) * 100
	}
	return out
}
