// Package indicators holds the pure technical-indicator functions the scanner
// materializes. Every function is PURE and RIGHT-ALIGNED: given a series indexed
// oldest→newest, the value at index i is computed from bars ≤ i (zero lookahead).
// Warm-up positions (before the window is full) are math.NaN, so a half-warmed
// symbol simply fails any comparison rather than matching on a partial value.
//
// This is the Phase-1 subset (see docs/PHASE1_MVP.md) — the seven features the
// daily digest needs. The full catalog (docs/INDICATORS.md) grows from here.
package indicators

import "math"

// SMA returns the simple moving average of vals over the trailing period.
// out[i] = mean(vals[i-period+1 : i+1]); positions before the first full window
// are NaN. A rolling sum keeps it a single pass.
func SMA(vals []float64, period int) []float64 {
	out := make([]float64, len(vals))
	if period < 1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	var sum float64
	for i, v := range vals {
		sum += v
		if i >= period {
			sum -= vals[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}

// RSI returns Wilder's Relative Strength Index over the trailing period.
// The first defined value is at index `period` (needs `period` deltas); earlier
// positions are NaN. A zero average loss yields RSI = 100.
func RSI(closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	for i := range out {
		out[i] = math.NaN()
	}
	if period < 1 || len(closes) <= period {
		return out
	}

	// Seed: average gain/loss over the first `period` deltas (closes[0..period]).
	var gain, loss float64
	for i := 1; i <= period; i++ {
		d := closes[i] - closes[i-1]
		if d >= 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgGain := gain / float64(period)
	avgLoss := loss / float64(period)
	out[period] = rsiFrom(avgGain, avgLoss)

	// Wilder smoothing forward.
	for i := period + 1; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		var g, l float64
		if d >= 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out[i] = rsiFrom(avgGain, avgLoss)
	}
	return out
}

func rsiFrom(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // flat: no gains, no losses
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs)
}

// RollingHigh returns the highest value over the trailing period (inclusive).
// Positions before the first full window are NaN.
func RollingHigh(vals []float64, period int) []float64 {
	return rollingExtreme(vals, period, true)
}

// RollingLow returns the lowest value over the trailing period (inclusive).
// Positions before the first full window are NaN.
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
		if i < period-1 {
			out[i] = math.NaN()
			continue
		}
		ext := vals[i-period+1]
		for _, v := range vals[i-period+2 : i+1] {
			if high && v > ext || !high && v < ext {
				ext = v
			}
		}
		out[i] = ext
	}
	return out
}

// Return returns the k-bar fractional change: out[i] = vals[i]/vals[i-k] - 1.
// Positions before index k (or where the base is non-positive) are NaN.
func Return(vals []float64, k int) []float64 {
	out := make([]float64, len(vals))
	for i := range vals {
		if i < k || k < 1 || vals[i-k] <= 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = vals[i]/vals[i-k] - 1
	}
	return out
}
