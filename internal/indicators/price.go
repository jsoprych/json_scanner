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
