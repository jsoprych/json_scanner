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

// PSAR returns Parabolic SAR values.
// step=0.02, maxAF=0.20 are standard Wilder defaults.
func PSAR(highs, lows []float64, step, maxAF float64) []float64 {
	n := len(highs)
	out := make([]float64, n)
	for i := range out { out[i] = math.NaN() }
	if n < 2 { return out }

	af, ep := step, lows[0]
	isBullish := true
	sar := lows[0]
	if highs[1] > highs[0] && lows[1] > lows[0] {
		ep = highs[0]
	} else {
		isBullish = false; sar = highs[0]; ep = lows[0]
	}
	out[1] = sar

	for i := 1; i < n-1; i++ {
		if isBullish {
			sar = sar + af*(ep-sar)
			if sar > lows[i] { sar = lows[i] }
			if sar > lows[i+1] { sar = lows[i+1] }
			if highs[i] > ep { ep = highs[i]; af = math.Min(af+step, maxAF) }
			if i+1 < n && lows[i+1] < sar { isBullish = false; af = step; sar = ep; ep = lows[i] }
		} else {
			sar = sar - af*(sar-ep)
			if sar < highs[i] { sar = highs[i] }
			if sar < highs[i+1] { sar = highs[i+1] }
			if lows[i] < ep { ep = lows[i]; af = math.Min(af+step, maxAF) }
			if i+1 < n && highs[i+1] > sar { isBullish = true; af = step; sar = ep; ep = highs[i] }
		}
		out[i+1] = math.Max(sar, 0.01)
	}
	return out
}

// Aroon returns Aroon Up, Down, and Oscillator (period typically 25).
func Aroon(highs, lows []float64, period int) (up, down, osc []float64) {
	n := len(highs)
	up = make([]float64, n); down = make([]float64, n); osc = make([]float64, n)
	for i := range up { up[i] = math.NaN(); down[i] = math.NaN(); osc[i] = math.NaN() }
	for i := period; i < n; i++ {
		hi, lo := i, i
		for j := i - period; j < i; j++ {
			if highs[j] > highs[hi] { hi = j }
			if lows[j] < lows[lo] { lo = j }
		}
		up[i] = 100 * float64(period-(i-hi)) / float64(period)
		down[i] = 100 * float64(period-(i-lo)) / float64(period)
		osc[i] = up[i] - down[i]
	}
	return
}
