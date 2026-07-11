package snapshot

import (
	"math"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/study"
)

func TestLoadAndRun(t *testing.T) {
	rows := []screen.SnapshotRow{
		// golden cross: 50 just crossed above 200.
		{Symbol: "GX", Close: 11, High: 11, SMA50: 11, SMA200: 10, IsGoldenCross: true, DollarVol: 2e6, Ret3m: 0.1, RSI14: 60, High52w: 11},
		// no cross: 50 still below 200.
		{Symbol: "NO", Close: 9, High: 9, SMA50: 8, SMA200: 10, IsGoldenCross: false, DollarVol: 1e6, Ret3m: -0.2, RSI14: 40, High52w: 12},
		// under-warm: NaN features → NULL, must not match anything.
		{Symbol: "WARM", Close: 5, High: math.NaN(), SMA50: math.NaN(), SMA200: math.NaN(),
			IsGoldenCross: false, High52w: math.NaN()},
	}

	db, err := OpenTest("") // in-memory
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Load(rows, 123); err != nil {
		t.Fatal(err)
	}

	golden := study.Study{Key: "golden", Where: "golden_cross = 1", OrderBy: "dollar_vol DESC", Limit: 10}
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

func TestSnapshotHistory(t *testing.T) {
	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, Ret3m: 0.1, RSI14: 60, High52w: 155},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, Ret3m: 0.2, RSI14: 70, High52w: 310},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, Ret3m: 0.15, RSI14: 65, High52w: 155},
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, Ret3m: 0.3, RSI14: 75, High52w: 2850},
	}

	db, err := OpenTest("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Load two historical snapshots
	date1 := int64(1700000000) // 2023-11-14
	date2 := int64(1700086400) // 2023-11-15
	if err := db.LoadHistory(rows1, date1, date1); err != nil {
		t.Fatal(err)
	}
	if err := db.LoadHistory(rows2, date2, date2); err != nil {
		t.Fatal(err)
	}

	// List snapshots
	dates, err := db.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(dates) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(dates))
	}
	if dates[0] != date2 || dates[1] != date1 {
		t.Errorf("expected [%d, %d], got %v", date2, date1, dates)
	}

	// Active date should be the latest (date2)
	m, err := db.Run(study.Study{Where: "symbol = 'AAPL'"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].Close != 155 {
		t.Errorf("expected AAPL close 155 (date2), got %+v", m)
	}

	// Switch to date1
	if err := db.SetActive(date1); err != nil {
		t.Fatal(err)
	}
	m, err = db.Run(study.Study{Where: "symbol = 'AAPL'"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].Close != 150 {
		t.Errorf("expected AAPL close 150 (date1), got %+v", m)
	}

	// GOOG only exists in date2
	if err := db.SetActive(date2); err != nil {
		t.Fatal(err)
	}
	m, err = db.Run(study.Study{Where: "symbol = 'GOOG'"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[0].Close != 2800 {
		t.Errorf("expected GOOG close 2800 (date2), got %+v", m)
	}

	// SetActive with invalid date should fail
	if err := db.SetActive(9999999999); err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestCleanup(t *testing.T) {
	rows := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9},
	}

	db, err := OpenTest("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Load snapshots with different dates
	// Use relative dates: 10 days ago and today
	now := time.Now().UTC().Unix()
	oldDate := now - 10*86400 // 10 days ago
	newDate := now            // today
	if err := db.LoadHistory(rows, oldDate, oldDate); err != nil {
		t.Fatal(err)
	}
	if err := db.LoadHistory(rows, newDate, newDate); err != nil {
		t.Fatal(err)
	}

	// Verify both exist
	dates, _ := db.ListSnapshots()
	if len(dates) != 2 {
		t.Fatalf("expected 2 snapshots before cleanup, got %d", len(dates))
	}

	// Cleanup with keepDays=5 should delete the old one (10 days ago)
	deleted, err := db.Cleanup(5)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify only new one remains
	dates, _ = db.ListSnapshots()
	if len(dates) != 1 {
		t.Fatalf("expected 1 snapshot after cleanup, got %d", len(dates))
	}
	if dates[0] != newDate {
		t.Errorf("expected newDate %d to remain, got %d", newDate, dates[0])
	}
}
