package backtest

import (
	"testing"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

func TestRunBacktest(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Load multiple snapshots with different data
	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 25},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, RSI14: 25},
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, RSI14: 20},
	}
	rows3 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 160, DollarVol: 1.2e9, RSI14: 35},
		{Symbol: "GOOG", Close: 2850, DollarVol: 3.1e9, RSI14: 22},
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows3, 1700172800, 1700172800); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(snap)

	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	summary, err := engine.RunBacktest(s, 1700000000, 1700172800, 1)
	if err != nil {
		t.Fatal(err)
	}

	if summary.TotalTrades == 0 {
		t.Error("expected some trades")
	}

	if len(summary.Results) != summary.TotalTrades {
		t.Errorf("expected %d results, got %d", summary.TotalTrades, len(summary.Results))
	}
}

func TestRunPointInTime(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	rows := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 25},
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, RSI14: 40},
	}

	if err := snap.LoadHistory(rows, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(snap)

	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	matches, err := engine.RunPointInTime(s, 1700000000)
	if err != nil {
		t.Fatal(err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestCalculateReturn(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 100, DollarVol: 1e9},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 110, DollarVol: 1e9},
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(snap)

	result, err := engine.calculateReturn("AAPL", 1700000000, 1)
	if err != nil {
		t.Fatal(err)
	}

	if result.EntryPx != 100 {
		t.Errorf("expected entry price 100, got %f", result.EntryPx)
	}

	if result.ExitPx != 110 {
		t.Errorf("expected exit price 110, got %f", result.ExitPx)
	}

	expectedReturn := 0.10 // 10%
	if result.Return != expectedReturn {
		t.Errorf("expected return %f, got %f", expectedReturn, result.Return)
	}
}
