package indicators

import "math"

// DollarVol returns the dollar volume (close * volume).
// Values exclude bar i (no lookahead).
func DollarVol(closes []float64, volumes []int64) []float64 {
	out := make([]float64, len(closes))
	for i := range closes {
		if i < 1 {
			out[i] = math.NaN()
			continue
		}
		out[i] = closes[i-1] * float64(volumes[i-1])
	}
	return out
}

// AvgDollarVol returns the average dollar volume over the trailing period.
// Values exclude bar i (no lookahead).
func AvgDollarVol(closes []float64, volumes []int64, period int) []float64 {
	dv := DollarVol(closes, volumes)
	out := make([]float64, len(closes))
	
	for i := range closes {
		if i < period {
			out[i] = math.NaN()
			continue
		}
		
		var sum float64
		for j := i - period; j < i; j++ {
			sum += dv[j]
		}
		out[i] = sum / float64(period)
	}
	
	return out
}

// RelVolume returns the relative volume (current volume / average volume).
// Values exclude bar i (no lookahead).
func RelVolume(volumes []int64, period int) []float64 {
	out := make([]float64, len(volumes))
	
	for i := range volumes {
		if i < period+1 {
			out[i] = math.NaN()
			continue
		}
		
		// Calculate average volume (excluding current bar)
		var sum float64
		for j := i - period; j < i; j++ {
			sum += float64(volumes[j])
		}
		avgVol := sum / float64(period)
		
		if avgVol == 0 {
			out[i] = math.NaN()
		} else {
			out[i] = float64(volumes[i-1]) / avgVol
		}
	}
	
	return out
}

// OBV returns the On-Balance Volume indicator.
// OBV is a cumulative indicator that adds volume on up days and subtracts on down days.
// Values exclude bar i (no lookahead).
func OBV(closes []float64, volumes []int64) []float64 {
	out := make([]float64, len(closes))
	if len(closes) < 2 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	
	out[1] = 0 // Starting value
	
	for i := 2; i < len(closes); i++ {
		if closes[i-1] > closes[i-2] {
			out[i] = out[i-1] + float64(volumes[i-1])
		} else if closes[i-1] < closes[i-2] {
			out[i] = out[i-1] - float64(volumes[i-1])
		} else {
			out[i] = out[i-1]
		}
	}
	
	return out
}

// VWAP returns the Volume Weighted Average Price.
// VWAP = cumulative(typical_price * volume) / cumulative(volume)
// For daily data, this is calculated over a rolling period.
// Values exclude bar i (no lookahead).
func VWAP(highs, lows, closes []float64, volumes []int64, period int) []float64 {
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
		
		var sumTPV, sumV float64
		for j := i - period; j < i; j++ {
			sumTPV += tp[j] * float64(volumes[j])
			sumV += float64(volumes[j])
		}
		
		if sumV == 0 {
			out[i] = math.NaN()
		} else {
			out[i] = sumTPV / sumV
		}
	}
	
	return out
}

// VWAPDist returns the distance from VWAP as a percentage.
// Dist = (close / vwap - 1) * 100
// Values exclude bar i (no lookahead).
func VWAPDist(highs, lows, closes []float64, volumes []int64, period int) []float64 {
	vwap := VWAP(highs, lows, closes, volumes, period)
	out := make([]float64, len(closes))
	
	for i := range closes {
		if math.IsNaN(vwap[i]) || vwap[i] == 0 {
			out[i] = math.NaN()
			continue
		}
		out[i] = (closes[i-1] / vwap[i] - 1) * 100
	}
	
	return out
}

// MFI returns the Money Flow Index.
// MFI is a volume-weighted RSI.
// Values exclude bar i (no lookahead).
func MFI(highs, lows, closes []float64, volumes []int64, period int) []float64 {
	out := make([]float64, len(closes))
	if len(closes) < period+1 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}
	
	// Calculate typical prices
	tp := make([]float64, len(closes))
	for i := range closes {
		tp[i] = (highs[i] + lows[i] + closes[i]) / 3
	}
	
	// Calculate raw money flow
	rawMF := make([]float64, len(closes))
	for i := range closes {
		rawMF[i] = tp[i] * float64(volumes[i])
	}
	
	for i := period + 1; i < len(closes); i++ {
		var posFlow, negFlow float64

		for j := i - period; j < i; j++ {
			if tp[j] > tp[j-1] {
				posFlow += rawMF[j]
			} else if tp[j] < tp[j-1] {
				negFlow += rawMF[j]
			}
		}
		
		if negFlow == 0 {
			out[i] = 100
		} else {
			mfRatio := posFlow / negFlow
			out[i] = 100 - (100 / (1 + mfRatio))
		}
	}
	
	return out
}
