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

// ReturnPct returns the k-bar percentage change: out[i] = (vals[i-1]/vals[i-1-k] - 1) * 100.
// Positions before index k+1 (or where the base is non-positive) are NaN.
// The indicator at index i excludes bar i (no lookahead).
func ReturnPct(vals []float64, k int) []float64 {
	out := Return(vals, k)
	for i := range out {
		if !math.IsNaN(out[i]) {
			out[i] *= 100
		}
	}
	return out
}

// RelativeStrength returns the relative strength of symbol closes vs benchmark closes.
// RS = (symbol_return - benchmark_return) * 100
// Both returns are calculated over the same period k.
// Values exclude bar i (no lookahead).
func RelativeStrength(symbolCloses, benchmarkCloses []float64, k int) []float64 {
	if len(symbolCloses) != len(benchmarkCloses) {
		panic("symbol and benchmark closes must have same length")
	}
	
	symbolRet := ReturnPct(symbolCloses, k)
	benchmarkRet := ReturnPct(benchmarkCloses, k)
	
	out := make([]float64, len(symbolCloses))
	for i := range symbolCloses {
		if math.IsNaN(symbolRet[i]) || math.IsNaN(benchmarkRet[i]) {
			out[i] = math.NaN()
			continue
		}
		out[i] = symbolRet[i] - benchmarkRet[i]
	}
	
	return out
}
