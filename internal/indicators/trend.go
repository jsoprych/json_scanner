package indicators

import "math"

// SMA returns the simple moving average of vals over the trailing period.
// out[i] = mean(vals[i-period : i]); positions before the first full window
// are NaN. The indicator at index i excludes bar i (no lookahead).
func SMA(vals []float64, period int) []float64 {
	out := make([]float64, len(vals))
	if period < 1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	
	// Calculate SMA using a simple approach: for each index i, average the previous `period` values
	for i := range vals {
		if i < period {
			out[i] = math.NaN()
			continue
		}
		var sum float64
		for j := i - period; j < i; j++ {
			sum += vals[j]
		}
		out[i] = sum / float64(period)
	}
	return out
}
