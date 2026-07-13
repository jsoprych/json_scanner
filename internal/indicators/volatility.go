package indicators

import "math"

// ATR returns the Average True Range indicator.
// ATR is the smoothed average of True Range values.
// Values exclude bar i (no lookahead).
func ATR(highs, lows, closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	if len(closes) < period+1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	
	// Calculate True Range
	tr := make([]float64, len(closes))
	for i := 1; i < len(closes); i++ {
		hl := highs[i] - lows[i]
		hc := math.Abs(highs[i] - closes[i-1])
		lc := math.Abs(lows[i] - closes[i-1])
		tr[i] = math.Max(hl, math.Max(hc, lc))
	}
	
	// Initial ATR is average of first `period` TR values
	var sum float64
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	out[period] = sum / float64(period)
	
	// Smooth remaining values using Wilder's method
	for i := period + 1; i < len(closes); i++ {
		out[i] = (out[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	
	return out
}

// ATRPct returns ATR as a percentage of close.
// Values exclude bar i (no lookahead).
func ATRPct(highs, lows, closes []float64, period int) []float64 {
	atr := ATR(highs, lows, closes, period)
	out := make([]float64, len(closes))
	
	for i := range closes {
		if i < 1 || math.IsNaN(atr[i]) || closes[i-1] == 0 {
			out[i] = math.NaN()
		} else {
			out[i] = (atr[i] / closes[i-1]) * 100
		}
	}
	
	return out
}

// BollingerBands returns upper, middle, and lower Bollinger Bands.
// Middle = SMA(period), Upper = Middle + mult*StdDev, Lower = Middle - mult*StdDev
// Values exclude bar i (no lookahead).
func BollingerBands(closes []float64, period int, mult float64) (upper, middle, lower []float64) {
	n := len(closes)
	upper = make([]float64, n)
	middle = make([]float64, n)
	lower = make([]float64, n)
	
	for i := range closes {
		if i < period {
			upper[i] = math.NaN()
			middle[i] = math.NaN()
			lower[i] = math.NaN()
			continue
		}
		
		// Calculate SMA (excluding current bar)
		var sum float64
		for j := i - period; j < i; j++ {
			sum += closes[j]
		}
		sma := sum / float64(period)
		middle[i] = sma
		
		// Calculate standard deviation (excluding current bar)
		var variance float64
		for j := i - period; j < i; j++ {
			diff := closes[j] - sma
			variance += diff * diff
		}
		stdDev := math.Sqrt(variance / float64(period))
		
		upper[i] = sma + mult*stdDev
		lower[i] = sma - mult*stdDev
	}
	
	return upper, middle, lower
}

// BBWidth returns the Bollinger Band width as a percentage.
// Width = (Upper - Lower) / Middle * 100
// Values exclude bar i (no lookahead).
func BBWidth(closes []float64, period int, mult float64) []float64 {
	upper, middle, lower := BollingerBands(closes, period, mult)
	out := make([]float64, len(closes))
	
	for i := range closes {
		if math.IsNaN(upper[i]) || math.IsNaN(middle[i]) || middle[i] == 0 {
			out[i] = math.NaN()
		} else {
			out[i] = ((upper[i] - lower[i]) / middle[i]) * 100
		}
	}
	
	return out
}

// BBPctB returns the %B indicator for Bollinger Bands.
// %B = (Close - Lower) / (Upper - Lower)
// Values exclude bar i (no lookahead).
func BBPctB(closes []float64, period int, mult float64) []float64 {
	upper, _, lower := BollingerBands(closes, period, mult)
	out := make([]float64, len(closes))
	
	for i := range closes {
		if math.IsNaN(upper[i]) || math.IsNaN(lower[i]) {
			out[i] = math.NaN()
			continue
		}
		
		bandWidth := upper[i] - lower[i]
		if bandWidth == 0 {
			out[i] = 0.5
		} else {
			out[i] = (closes[i-1] - lower[i]) / bandWidth
		}
	}
	
	return out
}

// HistoricalVol returns the annualized historical volatility.
// Vol = StdDev(log_returns) * sqrt(252)
// Values exclude bar i (no lookahead).
func HistoricalVol(closes []float64, period int) []float64 {
	out := make([]float64, len(closes))
	
	// Calculate log returns
	logReturns := make([]float64, len(closes))
	for i := 1; i < len(closes); i++ {
		if closes[i-1] == 0 {
			logReturns[i] = math.NaN()
		} else {
			logReturns[i] = math.Log(closes[i] / closes[i-1])
		}
	}
	
	for i := range closes {
		if i <= period {
			out[i] = math.NaN()
			continue
		}
		
		// Calculate mean of log returns (excluding current bar)
		var sum float64
		count := 0
		for j := i - period; j < i; j++ {
			if !math.IsNaN(logReturns[j]) {
				sum += logReturns[j]
				count++
			}
		}
		
		if count == 0 {
			out[i] = math.NaN()
			continue
		}
		
		mean := sum / float64(count)
		
		// Calculate variance
		var variance float64
		for j := i - period; j < i; j++ {
			if !math.IsNaN(logReturns[j]) {
				diff := logReturns[j] - mean
				variance += diff * diff
			}
		}
		variance /= float64(count)
		
		// Annualize
		out[i] = math.Sqrt(variance) * math.Sqrt(252) * 100
	}
	
	return out
}

// KeltnerChannels returns upper, middle, and lower Keltner Channel bands.
// Middle = EMA(period), Upper = middle + mult * ATR(atrPeriod), Lower = middle - mult * ATR(atrPeriod).
func KeltnerChannels(closes, highs, lows []float64, period, atrPeriod int, mult float64) (upper, mid, lower []float64) {
	n := len(closes)
	mid = EMA(closes, period)
	atr := ATR(highs, lows, closes, atrPeriod)
	upper = make([]float64, n)
	lower = make([]float64, n)
	for i := range closes {
		if math.IsNaN(mid[i]) || math.IsNaN(atr[i]) {
			upper[i] = math.NaN()
			lower[i] = math.NaN()
		} else {
			upper[i] = mid[i] + mult*atr[i]
			lower[i] = mid[i] - mult*atr[i]
		}
	}
	return
}
