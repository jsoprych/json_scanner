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
	eqRow(t, "sma3", SMA([]float64{1, 2, 3, 4, 5}, 3), []float64{nan, nan, 2, 3, 4})
	eqRow(t, "sma1", SMA([]float64{7, 8, 9}, 1), []float64{7, 8, 9})
	// period longer than series → all NaN
	eqRow(t, "sma-long", SMA([]float64{1, 2}, 5), []float64{nan, nan})
}

func TestRollingHighLow(t *testing.T) {
	nan := math.NaN()
	vals := []float64{1, 3, 2, 5, 4}
	eqRow(t, "high3", RollingHigh(vals, 3), []float64{nan, nan, 3, 5, 5})
	eqRow(t, "low3", RollingLow(vals, 3), []float64{nan, nan, 1, 2, 2})
}

func TestReturn(t *testing.T) {
	nan := math.NaN()
	// k=2: idx2 11/10-1, idx3 12/10-1, idx4 13/11-1
	got := Return([]float64{10, 10, 11, 12, 13}, 2)
	eqRow(t, "ret2", got, []float64{nan, nan, 0.1, 0.2, 13.0/11.0 - 1})
}

func TestRSI(t *testing.T) {
	nan := math.NaN()

	// All gains → RSI 100 once warmed; first value at index=period.
	eqRow(t, "rsi-up", RSI([]float64{1, 2, 3, 4, 5}, 3), []float64{nan, nan, nan, 100, 100})

	// All losses → RSI 0.
	eqRow(t, "rsi-down", RSI([]float64{5, 4, 3, 2, 1}, 3), []float64{nan, nan, nan, 0, 0})

	// Hand-computed period-2 case:
	//   closes [10,11,10,11], seed avgGain=avgLoss=0.5 → RSI[2]=50;
	//   step i=3: avgGain=0.75, avgLoss=0.25 → rs=3 → RSI[3]=75.
	eqRow(t, "rsi-2", RSI([]float64{10, 11, 10, 11}, 2), []float64{nan, nan, 50, 75})
}
