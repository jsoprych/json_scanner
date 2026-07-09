package indicators

import "math"

// Return returns the k-bar fractional change: out[i] = vals[i-1]/vals[i-1-k] - 1.
// Positions before index k+1 (or where the base is non-positive) are NaN.
// The indicator at index i excludes bar i (no lookahead).
func Return(vals []float64, k int) []float64 {
	out := make([]float64, len(vals))
	for i := range vals {
		if i < k+1 || k < 1 || vals[i-1-k] <= 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = vals[i-1]/vals[i-1-k] - 1
	}
	return out
}
