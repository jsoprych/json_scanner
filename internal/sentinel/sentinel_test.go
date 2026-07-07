package sentinel

import (
	"math"
	"testing"

	"cetus-marketdata-scanner/internal/screen"
)

func TestTier0(t *testing.T) {
	rows := []screen.SnapshotRow{
		// AGL-like: +1040% on $1.1M — the classic thin/uncaught-split artifact.
		{Symbol: "AGL", Ret3m: 10.4, DollarVol: 1.07e6, Close: 111.16, SMA200: 34.0},
		// ALAB-like: +280% but $75.9M liquid — extreme, likely real → WATCH.
		{Symbol: "ALAB", Ret3m: 2.8, DollarVol: 75.9e6, Close: 406, SMA200: 195},
		// Normal mover: no flag.
		{Symbol: "MSFT", Ret3m: 0.12, DollarVol: 30e6, Close: 402, SMA200: 360},
		// No 3-mo return yet: skipped.
		{Symbol: "IPO", Ret3m: math.NaN(), DollarVol: 8e6},
	}

	flags := Tier0(rows, DefaultTier0())
	if len(flags) != 2 {
		t.Fatalf("expected 2 flags, got %d: %+v", len(flags), flags)
	}
	// Suspect ranks first.
	if flags[0].Symbol != "AGL" || flags[0].Severity != Suspect {
		t.Errorf("expected AGL suspect first, got %+v", flags[0])
	}
	if flags[1].Symbol != "ALAB" || flags[1].Severity != Watch {
		t.Errorf("expected ALAB watch second, got %+v", flags[1])
	}
	// Ratio evidence carried through (~3.27).
	if math.Abs(flags[0].Ratio200-111.16/34.0) > 1e-9 {
		t.Errorf("AGL ratio200 = %v", flags[0].Ratio200)
	}
	if s, w := Counts(flags); s != 1 || w != 1 {
		t.Errorf("counts: suspect=%d watch=%d", s, w)
	}
}
