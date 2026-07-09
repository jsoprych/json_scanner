package indicators

import (
	"math"
	"testing"
)

const eps = 1e-9

// eqRow compares got against want, treating NaN==NaN as equal (want uses NaN for
// warm-up positions) and using an epsilon for real values.
func eqRow(t *testing.T, name string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length got %d want %d", name, len(got), len(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if math.IsNaN(w) {
			if !math.IsNaN(g) {
				t.Errorf("%s[%d]: got %v want NaN", name, i, g)
			}
			continue
		}
		if math.IsNaN(g) || math.Abs(g-w) > eps {
			t.Errorf("%s[%d]: got %v want %v", name, i, g, w)
		}
	}
}

func TestSMA(t *testing.T) {
	nan := math.NaN()
	// No lookahead: indicator at index i excludes bar i
	// SMA([1,2,3,4,5], 3): index 3 = (1+2+3)/3=2, index 4 = (2+3+4)/3=3
	eqRow(t, "sma3", SMA([]float64{1, 2, 3, 4, 5}, 3), []float64{nan, nan, nan, 2, 3})
	eqRow(t, "sma1", SMA([]float64{7, 8, 9}, 1), []float64{nan, 7, 8})
	// period longer than series → all NaN
	eqRow(t, "sma-long", SMA([]float64{1, 2}, 5), []float64{nan, nan})
}

func TestRollingHighLow(t *testing.T) {
	nan := math.NaN()
	vals := []float64{1, 3, 2, 5, 4}
	// No lookahead: indicator at index i excludes bar i
	// high3: index 3 = max(1,3,2)=3, index 4 = max(3,2,5)=5
	eqRow(t, "high3", RollingHigh(vals, 3), []float64{nan, nan, nan, 3, 5})
	// low3: index 3 = min(1,3,2)=1, index 4 = min(3,2,5)=2
	eqRow(t, "low3", RollingLow(vals, 3), []float64{nan, nan, nan, 1, 2})
}

func TestReturn(t *testing.T) {
	nan := math.NaN()
	// No lookahead: return at index i uses bars up to i-1
	// k=2: idx3 11/10-1, idx4 12/10-1
	got := Return([]float64{10, 10, 11, 12, 13}, 2)
	eqRow(t, "ret2", got, []float64{nan, nan, nan, 0.1, 0.2})
}

func TestRSI(t *testing.T) {
	nan := math.NaN()

	// No lookahead: indicator at index i excludes bar i
	// All gains → RSI 100 once warmed; first value at index=period+1.
	eqRow(t, "rsi-up", RSI([]float64{1, 2, 3, 4, 5, 6}, 3), []float64{nan, nan, nan, nan, 100, 100})

	// All losses → RSI 0.
	eqRow(t, "rsi-down", RSI([]float64{6, 5, 4, 3, 2, 1}, 3), []float64{nan, nan, nan, nan, 0, 0})

	// Hand-computed period-2 case:
	//   closes [10,11,10,11,10], seed avgGain=avgLoss=0.5 → RSI[3]=50;
	//   step i=3: avgGain=0.75, avgLoss=0.25 → rs=3 → RSI[4]=75.
	eqRow(t, "rsi-2", RSI([]float64{10, 11, 10, 11, 10}, 2), []float64{nan, nan, nan, 50, 75})
}
