package scanner

import (
	"testing"

	"cetus-marketdata-scanner/internal/model"
)

// bar is a tiny constructor for readable test fixtures.
func bar(ts int64, o, h, l, c float64, v int64) model.Bar {
	return model.Bar{Symbol: "T", Timestamp: ts, Open: o, High: h, Low: l, Close: c, Volume: v}
}

func has(sigs []model.Signal, typ string) bool {
	for _, s := range sigs {
		if s.Type == typ {
			return true
		}
	}
	return false
}

func TestScan(t *testing.T) {
	cfg := Config{Lookback: 3, VolumeMult: 2.0, GapPct: 0.05}

	t.Run("volume breakout", func(t *testing.T) {
		bars := []model.Bar{
			bar(1, 10, 10, 10, 10, 100), bar(2, 10, 10, 10, 10, 100),
			bar(3, 10, 10, 10, 10, 100), bar(4, 10, 10, 10, 10, 500), // 5× avg
		}
		if !has(Scan("T", bars, cfg), model.SignalVolumeBreakout) {
			t.Error("expected volume_breakout")
		}
	})

	t.Run("price breakout", func(t *testing.T) {
		bars := []model.Bar{
			bar(1, 10, 12, 9, 11, 100), bar(2, 11, 12, 10, 11, 100),
			bar(3, 11, 12, 10, 11, 100), bar(4, 11, 20, 11, 15, 100), // close 15 > trailing high 12
		}
		if !has(Scan("T", bars, cfg), model.SignalPriceBreakout) {
			t.Error("expected price_breakout")
		}
	})

	t.Run("gap up / down", func(t *testing.T) {
		up := []model.Bar{
			bar(1, 10, 10, 10, 10, 100), bar(2, 10, 10, 10, 10, 100),
			bar(3, 10, 10, 10, 10, 100), bar(4, 11, 11, 11, 11, 100), // open 11 vs prev close 10 = +10%
		}
		if !has(Scan("T", up, cfg), model.SignalGapUp) {
			t.Error("expected gap_up")
		}
		down := []model.Bar{
			bar(1, 10, 10, 10, 10, 100), bar(2, 10, 10, 10, 10, 100),
			bar(3, 10, 10, 10, 10, 100), bar(4, 9, 9, 9, 9, 100), // open 9 vs prev close 10 = -10%
		}
		if !has(Scan("T", down, cfg), model.SignalGapDown) {
			t.Error("expected gap_down")
		}
	})

	t.Run("quiet bar → no signals", func(t *testing.T) {
		bars := []model.Bar{
			bar(1, 10, 10, 10, 10, 100), bar(2, 10, 10, 10, 10, 100),
			bar(3, 10, 10, 10, 10, 100), bar(4, 10, 10, 10, 10, 105),
		}
		if s := Scan("T", bars, cfg); len(s) != 0 {
			t.Errorf("expected no signals, got %v", s)
		}
	})

	t.Run("too few bars → nil", func(t *testing.T) {
		if s := Scan("T", []model.Bar{bar(1, 10, 10, 10, 10, 100)}, cfg); s != nil {
			t.Errorf("expected nil for short window, got %v", s)
		}
	})
}
