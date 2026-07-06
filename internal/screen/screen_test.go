package screen

import (
	"math"
	"testing"

	"cetus-marketdata-scanner/internal/model"
)

func TestPredicates(t *testing.T) {
	nan := math.NaN()

	golden := SnapshotRow{SMA50: 11, SMA200: 10, PrevSMA50: 9, PrevSMA200: 10}
	if !golden.GoldenCross() {
		t.Error("expected golden cross (50 crossing above 200)")
	}
	stillBelow := SnapshotRow{SMA50: 9, SMA200: 10, PrevSMA50: 8, PrevSMA200: 10}
	if stillBelow.GoldenCross() {
		t.Error("no cross: 50 still below 200")
	}
	// Under-warm prev must not fire.
	if (SnapshotRow{SMA50: 11, SMA200: 10, PrevSMA50: nan, PrevSMA200: nan}).GoldenCross() {
		t.Error("NaN prev must not fire a cross")
	}

	bounce := SnapshotRow{RSI14: 33, PrevRSI14: 28}
	if !bounce.OversoldBounce() {
		t.Error("expected oversold bounce (RSI crossing above 30)")
	}
	if (SnapshotRow{RSI14: 33, PrevRSI14: 31}).OversoldBounce() {
		t.Error("no bounce: RSI was already above 30")
	}

	hi := SnapshotRow{High: 50, High52w: 50}
	if !hi.Is52wHigh() {
		t.Error("expected 52w high when today's high equals the window max")
	}
	if (SnapshotRow{High: 49, High52w: 50}).Is52wHigh() {
		t.Error("not a 52w high when below window max")
	}
}

func TestBuildAndMomentum(t *testing.T) {
	// 260 ascending bars → warmed SMA200/RSI/52w-high; strictly rising close.
	var bars []model.Bar
	for i := 0; i < 260; i++ {
		p := 100.0 + float64(i) // 100,101,...,359
		bars = append(bars, model.Bar{Symbol: "UP", Timestamp: int64(i), Open: p, High: p, Low: p, Close: p, Volume: 1000})
	}
	row, ok := Build("UP", bars)
	if !ok {
		t.Fatal("Build returned ok=false for 260 bars")
	}
	if math.IsNaN(row.SMA200) || math.IsNaN(row.RSI14) || math.IsNaN(row.Ret3m) {
		t.Fatalf("expected warmed features, got %+v", row)
	}
	if !row.AboveSMA200() {
		t.Error("rising series should be above its 200-DMA")
	}
	if !row.Is52wHigh() {
		t.Error("last bar of a strictly rising series is a 52w high")
	}
	if row.RSI14 != 100 {
		t.Errorf("all-gains series should have RSI 100, got %v", row.RSI14)
	}

	// Momentum preset ranks by 3-mo return, descending.
	flat := SnapshotRow{Symbol: "FLAT", Ret3m: 0.0}
	rows := []SnapshotRow{flat, row}
	got := MVPPresets(8, 10)[3].Run(rows) // momentum_leaders
	if len(got) != 2 || got[0].Symbol != "UP" {
		t.Errorf("momentum leader should be UP first, got %+v", got)
	}
}

func TestComputeBreadth(t *testing.T) {
	rows := []SnapshotRow{
		{Close: 11, SMA200: 10, SMA50: 10, High: 5, High52w: 5, Low52w: math.NaN()},        // above both, new high
		{Close: 9, SMA200: 10, SMA50: 10, High52w: math.NaN(), Low52w: math.NaN()},         // below both
		{Close: 11, SMA200: math.NaN(), SMA50: 10, High52w: math.NaN(), Low52w: math.NaN()}, // no 200-DMA yet
	}
	b := ComputeBreadth(rows)
	if b.Total != 3 || b.WithSMA200 != 2 || b.AboveSMA200 != 1 {
		t.Errorf("breadth 200: %+v", b)
	}
	if b.WithSMA50 != 3 || b.AboveSMA50 != 2 {
		t.Errorf("breadth 50: %+v", b)
	}
	if b.New52wHigh != 1 {
		t.Errorf("expected 1 new high, got %d", b.New52wHigh)
	}
	if got := b.PctAbove200(); math.Abs(got-50) > 1e-9 {
		t.Errorf("pct above 200 = %v, want 50", got)
	}
}
