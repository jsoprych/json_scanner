package digest

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/screen"
)

func TestRender(t *testing.T) {
	rows := []screen.SnapshotRow{
		{Symbol: "NVDA", Close: 132.4, RSI14: 68, Ret3m: 0.21, DollarVol: 5.2e9,
			High: 132.4, High52w: 132.4, SMA200: 100, SMA50: 120, PrevClose: 130, PrevSMA200: 99, PrevSMA50: 119},
		{Symbol: "TINY", Close: 3.1, RSI14: 33, Ret3m: -0.05, DollarVol: 4e5,
			High52w: 9, PrevRSI14: 28, PrevClose: 3.0, SMA50: 3.2, SMA200: 3.5, PrevSMA50: 3.2, PrevSMA200: 3.5},
	}
	presets := screen.MVPPresets(8, 10)
	d := Build(time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC), 2110, rows, presets)

	if d.SymbolsScanned != 2110 {
		t.Errorf("scanned = %d", d.SymbolsScanned)
	}
	if len(d.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(d.Sections))
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
