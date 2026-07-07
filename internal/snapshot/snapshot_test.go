package snapshot

import (
	"math"
	"testing"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/study"
)

func TestLoadAndRun(t *testing.T) {
	rows := []screen.SnapshotRow{
		// golden cross: 50 just crossed above 200.
		{Symbol: "GX", Close: 11, High: 11, SMA50: 11, SMA200: 10, PrevSMA50: 9, PrevSMA200: 10, DollarVol: 2e6, Ret3m: 0.1, RSI14: 60, High52w: 11},
		// no cross: 50 still below 200.
		{Symbol: "NO", Close: 9, High: 9, SMA50: 8, SMA200: 10, PrevSMA50: 8, PrevSMA200: 10, DollarVol: 1e6, Ret3m: -0.2, RSI14: 40, High52w: 12},
		// under-warm: NaN features → NULL, must not match anything.
		{Symbol: "WARM", Close: 5, High: math.NaN(), SMA50: math.NaN(), SMA200: math.NaN(),
			PrevSMA50: math.NaN(), PrevSMA200: math.NaN(), High52w: math.NaN()},
	}

	db, err := Open("") // in-memory
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Load(rows, 123); err != nil {
		t.Fatal(err)
	}

	golden := study.Study{Key: "golden", Where: "sma50 > sma200 AND prev_sma50 <= prev_sma200", OrderBy: "dollar_vol DESC", Limit: 10}
	m, err := db.Run(golden)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].Symbol != "GX" {
		t.Fatalf("golden cross matches = %+v", m)
	}
	if m[0].DollarVol != 2e6 {
		t.Errorf("dollar_vol = %v", m[0].DollarVol)
	}

	// 52-week high study: only GX (high >= high_52w).
	hi, err := db.Run(study.Study{Key: "hi", Where: "high >= high_52w"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hi) != 1 || hi[0].Symbol != "GX" {
		t.Errorf("52w-high matches = %+v", hi)
	}
}
