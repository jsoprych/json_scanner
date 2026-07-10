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

// EMA returns the exponential moving average of vals over the trailing period.
// Uses SMA of first `period` values as seed, then applies EMA formula.
// out[i] excludes bar i (no lookahead).
func EMA(vals []float64, period int) []float64 {
	out := make([]float64, len(vals))
	if period < 1 || len(vals) <= period {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	
	// Seed with SMA of first `period` values (bars 0 to period-1)
	var sum float64
	for i := 0; i < period; i++ {
		sum += vals[i]
	}
	sma := sum / float64(period)
	
	// EMA multiplier
	multiplier := 2.0 / float64(period+1)
	
	// First EMA value is at index `period` (uses bars 0 to period-1)
	out[period] = sma
	
	// Calculate EMA for remaining bars
	for i := period + 1; i < len(vals); i++ {
		// EMA at i uses vals[i-1] and EMA at i-1
		out[i] = (vals[i-1] - out[i-1]) * multiplier + out[i-1]
	}
	
	return out
}

// PctFromSMA returns the percentage distance of close from the SMA.
// out[i] = (close[i-1] / sma[i-1]) - 1 (no lookahead).
func PctFromSMA(close, sma []float64) []float64 {
	out := make([]float64, len(close))
	for i := range close {
		if i == 0 || math.IsNaN(sma[i]) || sma[i] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (close[i-1] / sma[i]) - 1
	}
	return out
}

// MAStack returns true when EMAs are in perfect bullish order:
// EMA10 > EMA21 > EMA50 > EMA200 (no lookahead).
func MAStack(ema10, ema21, ema50, ema200 []float64) []bool {
	out := make([]bool, len(ema10))
	for i := range ema10 {
		if math.IsNaN(ema10[i]) || math.IsNaN(ema21[i]) || 
		   math.IsNaN(ema50[i]) || math.IsNaN(ema200[i]) {
			out[i] = false
			continue
		}
		out[i] = ema10[i] > ema21[i] && ema21[i] > ema50[i] && ema50[i] > ema200[i]
	}
	return out
}
