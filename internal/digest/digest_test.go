package digest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/screen"
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

func TestRenderFromStudies(t *testing.T) {
	rows := []screen.SnapshotRow{
		{Symbol: "NVDA", Close: 132.4, High: 132.4, RSI14: 68, Ret3m: 0.21, DollarVol: 5.2e9,
			SMA50: 120, SMA200: 100, IsGoldenCross: true, High52w: 132.4},
		{Symbol: "TINY", Close: 3.1, High: 3.1, RSI14: 33, Ret3m: -0.05, DollarVol: 4e5,
			SMA50: 3.2, SMA200: 3.5, IsOversoldBounce: true, High52w: 9},
	}

	snap, err := snapshot.OpenTest("")
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Close()
	if err := snap.Load(rows, 123); err != nil {
		t.Fatal(err)
	}

	studies := []study.Study{
		{Key: "momentum", Title: "Momentum Leaders", Emoji: "🚀", Where: "ret_3m IS NOT NULL", OrderBy: "ret_3m DESC", Limit: 10},
		{Key: "highs", Title: "New Highs", Emoji: "📈", Where: "high >= high_52w", OrderBy: "dollar_vol DESC", Limit: 8},
	}

	d, err := FromStudies(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), rows, snap, studies)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sections) != 2 {
		t.Fatalf("sections = %d", len(d.Sections))
	}
	if d.SymbolsScanned != 2 {
		t.Errorf("scanned = %d", d.SymbolsScanned)
	}

	var html bytes.Buffer
	if err := d.HTML(&html); err != nil {
		t.Fatalf("HTML render: %v", err)
	}
	h := html.String()
	for _, want := range []string{"Market Breadth", "Momentum Leaders", "NVDA", "$5.2B", "July 2, 2026"} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML missing %q", want)
		}
	}

	var text bytes.Buffer
	if err := d.Text(&text); err != nil {
		t.Fatalf("text render: %v", err)
	}
	if !strings.Contains(text.String(), "MARKET BREADTH") {
		t.Error("text missing breadth header")
	}
}
