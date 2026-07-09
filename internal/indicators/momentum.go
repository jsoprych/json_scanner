package indicators

import "math"

// RSI returns Wilder's Relative Strength Index over the trailing period.
// The first defined value is at index `period+1` (needs `period` deltas before
// the current bar); earlier positions are NaN. The indicator at index i excludes
// bar i (no lookahead). A zero average loss yields RSI = 100.
func RSI(closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	for i := range out {
		out[i] = math.NaN()
	}
	if period < 1 || len(closes) <= period+1 {
		return out
	}

	// Seed: average gain/loss over the first `period` deltas (closes[0..period]).
	// This gives us the RSI value for index `period` (which will be shifted to index `period+1`).
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
	out[period+1] = rsiFrom(avgGain, avgLoss)

	// Wilder smoothing forward, shifted by 1 for no lookahead.
	for i := period + 1; i < len(closes)-1; i++ {
		d := closes[i] - closes[i-1]
		var g, l float64
		if d >= 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out[i+1] = rsiFrom(avgGain, avgLoss)
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
