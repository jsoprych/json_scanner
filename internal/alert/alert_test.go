package alert

import (
	"testing"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

func TestDetectEntries(t *testing.T) {
	// Create in-memory snapshot DB
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	// Load two snapshots with different data
	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 40},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, RSI14: 25}, // Still matches
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, RSI14: 20}, // New entry
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector(snap)

	// Study: RSI < 35
	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	entries, err := detector.DetectEntries(s, 1700086400, 1700000000)
	if err != nil {
		t.Fatal(err)
	}

	// GOOG should be an entry (new in snapshot 2)
	// AAPL should NOT be an entry (was already in snapshot 1)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if len(entries) > 0 && entries[0].Symbol != "GOOG" {
		t.Errorf("expected GOOG entry, got %s", entries[0].Symbol)
	}
}

func TestDetectExits(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 25},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, RSI14: 25}, // Still matches
		// MSFT is gone (exit)
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector(snap)

	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	exits, err := detector.DetectExits(s, 1700086400, 1700000000)
	if err != nil {
		t.Fatal(err)
	}

	// MSFT should be an exit
	if len(exits) != 1 {
		t.Errorf("expected 1 exit, got %d", len(exits))
	}
	if len(exits) > 0 && exits[0].Symbol != "MSFT" {
		t.Errorf("expected MSFT exit, got %s", exits[0].Symbol)
	}
}

func TestDetectChanges(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	rows1 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
		{Symbol: "MSFT", Close: 300, DollarVol: 2e9, RSI14: 25},
	}
	rows2 := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 155, DollarVol: 1.1e9, RSI14: 25}, // Still matches
		{Symbol: "GOOG", Close: 2800, DollarVol: 3e9, RSI14: 20}, // New entry
		// MSFT is gone (exit)
	}

	if err := snap.LoadHistory(rows1, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := snap.LoadHistory(rows2, 1700086400, 1700086400); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector(snap)

	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	entries, exits, err := detector.DetectChanges(s, 1700086400, 1700000000)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if len(exits) != 1 {
		t.Errorf("expected 1 exit, got %d", len(exits))
	}
}

func TestDetectEntriesNoPrevDate(t *testing.T) {
	snap, err := snapshot.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()

	rows := []screen.SnapshotRow{
		{Symbol: "AAPL", Close: 150, DollarVol: 1e9, RSI14: 30},
	}

	if err := snap.LoadHistory(rows, 1700000000, 1700000000); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector(snap)

	s := study.Study{
		Key:   "oversold",
		Where: "rsi14 < 35",
	}

	// Try to detect entries with a non-existent prev date
	entries, err := detector.DetectEntries(s, 1700000000, 1699913600)
	if err != nil {
		t.Fatal(err)
	}

	// All current matches should be entries when prev date doesn't exist
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}
