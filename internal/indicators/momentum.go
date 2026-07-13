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

// MACD returns the MACD line, signal line, and histogram.
// MACD = EMA12 - EMA26, Signal = EMA9 of MACD, Histogram = MACD - Signal.
// All values exclude bar i (no lookahead).
func MACD(closes []float64) (macd, signal, histogram []float64) {
	ema12 := EMA(closes, 12)
	ema26 := EMA(closes, 26)
	
	macd = make([]float64, len(closes))
	for i := range closes {
		if math.IsNaN(ema12[i]) || math.IsNaN(ema26[i]) {
			macd[i] = math.NaN()
		} else {
			macd[i] = ema12[i] - ema26[i]
		}
	}
	
	// Signal line is EMA9 of MACD
	signal = EMA(macd, 9)
	
	// Histogram is MACD - Signal
	histogram = make([]float64, len(closes))
	for i := range closes {
		if math.IsNaN(macd[i]) || math.IsNaN(signal[i]) {
			histogram[i] = math.NaN()
		} else {
			histogram[i] = macd[i] - signal[i]
		}
	}
	
	return macd, signal, histogram
}

// Stochastic returns %K and %D for the Stochastic oscillator.
// %K = (close - lowest_low) / (highest_high - lowest_low) * 100
// %D = SMA3 of %K
// All values exclude bar i (no lookahead).
func Stochastic(highs, lows, closes []float64, kPeriod, dPeriod, smooth int) (k, d []float64) {
	n := len(closes)
	k = make([]float64, n)
	d = make([]float64, n)
	
	// Calculate raw %K
	for i := range closes {
		if i < kPeriod {
			k[i] = math.NaN()
			continue
		}
		
		// Find highest high and lowest low in lookback period (excluding current bar)
		highestHigh := highs[i-kPeriod]
		lowestLow := lows[i-kPeriod]
		for j := i - kPeriod + 1; j < i; j++ {
			if highs[j] > highestHigh {
				highestHigh = highs[j]
			}
			if lows[j] < lowestLow {
				lowestLow = lows[j]
			}
		}
		
		if highestHigh == lowestLow {
			k[i] = 50 // Avoid division by zero
		} else {
			k[i] = ((closes[i-1] - lowestLow) / (highestHigh - lowestLow)) * 100
		}
	}
	
	// Smooth %K and calculate %D
	rawK := make([]float64, n)
	copy(rawK, k)
	
	// Apply smoothing to %K
	for i := range k {
		if i < kPeriod+smooth-1 {
			k[i] = math.NaN()
			continue
		}
		var sum float64
		for j := i - smooth; j < i; j++ {
			sum += rawK[j]
		}
		k[i] = sum / float64(smooth)
	}
	
	// %D is SMA of smoothed %K
	d = SMA(k, dPeriod)
	
	return k, d
}

// WilliamsR returns Williams %R indicator.
// %R = (highest_high - close) / (highest_high - lowest_low) * -100
// Values exclude bar i (no lookahead).
func WilliamsR(highs, lows, closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if i < period {
			out[i] = math.NaN()
			continue
		}
		
		// Find highest high and lowest low in lookback period (excluding current bar)
		highestHigh := highs[i-period]
		lowestLow := lows[i-period]
		for j := i - period + 1; j < i; j++ {
			if highs[j] > highestHigh {
				highestHigh = highs[j]
			}
			if lows[j] < lowestLow {
				lowestLow = lows[j]
			}
		}
		
		if highestHigh == lowestLow {
			out[i] = -50 // Avoid division by zero
		} else {
			out[i] = ((highestHigh - closes[i-1]) / (highestHigh - lowestLow)) * -100
		}
	}
	return out
}

// CCI returns the Commodity Channel Index.
// CCI = (typical_price - SMA_of_typical_price) / (0.015 * mean_deviation)
// Values exclude bar i (no lookahead).
func CCI(highs, lows, closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	
	// Calculate typical prices
	tp := make([]float64, len(closes))
	for i := range closes {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
	}
	
	for i := range closes {
		if i < period {
			out[i] = math.NaN()
			continue
		}
		
		// Calculate SMA of typical price (excluding current bar)
		var sum float64
		for j := i - period; j < i; j++ {
			sum += tp[j]
		}
		sma := sum / float64(period)
		
		// Calculate mean deviation (excluding current bar)
		var meanDev float64
		for j := i - period; j < i; j++ {
			meanDev += math.Abs(tp[j] - sma)
		}
		meanDev /= float64(period)
		
		if meanDev == 0 {
			out[i] = 0
		} else {
			out[i] = (tp[i-1] - sma) / (0.015 * meanDev)
		}
	}
	
	return out
}

// ROC returns the Rate of Change indicator.
// ROC = (close[i-1] / close[i-1-period] - 1) * 100
// Values exclude bar i (no lookahead).
func ROC(closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if i <= period {
			out[i] = math.NaN()
			continue
		}
		if closes[i-1-period] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (closes[i-1] / closes[i-1-period] - 1) * 100
	}
	return out
}

// ADX returns the Average Directional Index, +DI, and -DI.
// All values exclude bar i (no lookahead).
func ADX(highs, lows, closes []float64, period int) (adx, diPlus, diMinus []float64) {
	n := len(closes)
	adx = make([]float64, n)
	diPlus = make([]float64, n)
	diMinus = make([]float64, n)
	
	if n < period+1 {
		for i := range adx {
			adx[i] = math.NaN()
			diPlus[i] = math.NaN()
			diMinus[i] = math.NaN()
		}
		return adx, diPlus, diMinus
	}
	
	// Calculate True Range, +DM, -DM
	tr := make([]float64, n)
	pdm := make([]float64, n)
	ndm := make([]float64, n)
	
	for i := 1; i < n; i++ {
		// True Range
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(hl, math.Max(hc, lc))
		
		// Directional Movement
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]
		
		if upMove > downMove && upMove > 0 {
			pdm[i] = upMove
		} else {
			pdm[i] = 0
		}
		
		if downMove > upMove && downMove > 0 {
			ndm[i] = downMove
		} else {
			ndm[i] = 0
		}
	}
	
	// Smooth with Wilder's method
	trSmooth := make([]float64, n)
	pdmSmooth := make([]float64, n)
	ndmSmooth := make([]float64, n)
	
	// Initial values (sum of first `period` values)
	for i := 1; i <= period; i++ {
		trSmooth[period] += tr[i]
		pdmSmooth[period] += pdm[i]
		ndmSmooth[period] += ndm[i]
	}
	
	// Smooth remaining values
	for i := period + 1; i < n; i++ {
		trSmooth[i] = trSmooth[i-1] - trSmooth[i-1]/float64(period) + tr[i]
		pdmSmooth[i] = pdmSmooth[i-1] - pdmSmooth[i-1]/float64(period) + pdm[i]
		ndmSmooth[i] = ndmSmooth[i-1] - ndmSmooth[i-1]/float64(period) + ndm[i]
	}
	
	// Calculate +DI and -DI
	for i := period; i < n; i++ {
		if trSmooth[i] == 0 {
			diPlus[i] = 0
			diMinus[i] = 0
		} else {
			diPlus[i] = (pdmSmooth[i] / trSmooth[i]) * 100
			diMinus[i] = (ndmSmooth[i] / trSmooth[i]) * 100
		}
	}
	
	// Calculate DX and ADX
	dx := make([]float64, n)
	for i := period; i < n; i++ {
		sum := diPlus[i] + diMinus[i]
		if sum == 0 {
			dx[i] = 0
		} else {
			dx[i] = (math.Abs(diPlus[i]-diMinus[i]) / sum) * 100
		}
	}
	
	// ADX is smoothed DX
	for i := period * 2; i < n; i++ {
		if i == period*2 {
			// Initial ADX value
			var sum float64
			for j := period; j < period*2; j++ {
				sum += dx[j]
			}
			adx[i] = sum / float64(period)
		} else {
			adx[i] = (adx[i-1]*float64(period-1) + dx[i-1]) / float64(period)
		}
	}
	
	return adx, diPlus, diMinus
}

// CMF returns Chaikin Money Flow over period (typically 20 or 21).
// CMF = sum(Money Flow Volume) / sum(Volume) for the period, where
// Money Flow Volume = volume * ((close - low) - (high - close)) / (high - low)
func CMF(highs, lows, closes []float64, volumes []int64, period int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	for i := range out { out[i] = math.NaN() }
	for i := period; i < n; i++ {
		var mfSum, volSum float64
		for j := i - period + 1; j <= i; j++ {
			mf := 0.0
			if highs[j] != lows[j] {
				mf = float64(volumes[j]) * ((closes[j]-float64(lows[j])) - (float64(highs[j])-closes[j])) / (float64(highs[j]) - float64(lows[j]))
			}
			mfSum += mf
			volSum += float64(volumes[j])
		}
		if volSum > 0 { out[i] = mfSum / volSum }
	}
	return out
}

// UltimateOsc returns the Ultimate Oscillator (period1=7, period2=14, period3=28 typical).
func UltimateOsc(highs, lows, closes []float64, p1, p2, p3 int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	for i := range out { out[i] = math.NaN() }
	if n < p3+2 { return out }

	bp := make([]float64, n) // buying pressure
	tr := make([]float64, n) // true range
	for i := 0; i < n; i++ {
		bp[i] = closes[i] - math.Min(lows[i], closes[i])
		tr[i] = math.Max(highs[i], closes[i]) - math.Min(lows[i], closes[i])
	}

	for i := p3; i < n; i++ {
		var sum1, sum2, sum3, tr1, tr2, tr3 float64
		for j := i - p1 + 1; j <= i; j++ { sum1 += bp[j]; tr1 += tr[j] }
		for j := i - p2 + 1; j <= i; j++ { sum2 += bp[j]; tr2 += tr[j] }
		for j := i - p3 + 1; j <= i; j++ { sum3 += bp[j]; tr3 += tr[j] }
		avg1 := 4.0; avg2 := 2.0; avg3 := 1.0
		if tr1 > 0 { avg1 = sum1 / tr1 }
		if tr2 > 0 { avg2 = sum2 / tr2 }
		if tr3 > 0 { avg3 = sum3 / tr3 }
		out[i] = 100 * (4*avg1 + 2*avg2 + avg3) / 7.0
	}
	return out
}
