package snapshot

import (
	"fmt"
	"math/rand"
	"testing"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/study"
)

// synthRows builds n realistic-ish snapshot rows for benchmarking study SELECTs.
func synthRows(n int) []screen.SnapshotRow {
	r := rand.New(rand.NewSource(1))
	rows := make([]screen.SnapshotRow, n)
	for i := range rows {
		close := 5 + r.Float64()*500
		rows[i] = screen.SnapshotRow{
			Symbol: fmt.Sprintf("SY%05d", i), Close: close, High: close * (1 + r.Float64()*0.02),
			DollarVol: r.Float64() * 5e8, SMA50: close * (0.9 + r.Float64()*0.2), SMA200: close * (0.8 + r.Float64()*0.3),
			RSI14: r.Float64() * 100, IsGoldenCross: r.Float64() > 0.9, IsOversoldBounce: r.Float64() > 0.95,
			Ret3m: (r.Float64() - 0.4), High52w: close * (1 + r.Float64()*0.1),
		}
	}
	return rows
}

// BenchmarkStudyRun measures one study SELECT over an in-memory snapshot of N rows —
// the free-tier "run a study on live tables" hot path. Run:
//
//	go test ./internal/snapshot/ -bench=StudyRun -benchmem
func BenchmarkStudyRun(b *testing.B) {
	for _, n := range []int{500, 3000, 14000} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			db, err := Open("")
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			if err := db.Load(synthRows(n), 0); err != nil {
				b.Fatal(err)
			}
			st := study.Study{Where: "close > sma200 AND rsi14 BETWEEN 55 AND 70 AND dollar_vol > 5e6",
				OrderBy: "ret_3m DESC", Limit: 25}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := db.Run(st); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoad measures materializing N rows into the in-memory table.
func BenchmarkLoad(b *testing.B) {
	rows := synthRows(3000)
	db, _ := Open("")
	defer db.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := db.Load(rows, 0); err != nil {
			b.Fatal(err)
		}
	}
}
