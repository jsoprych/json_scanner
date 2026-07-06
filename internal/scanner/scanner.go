// Package scanner holds the pure scan logic: given a symbol's split-adjusted
// bars, detect breakout/anomaly signals on the most recent bar. No I/O, no state
// — the seam a back-test/strategy engine calls bar-by-bar.
package scanner

import "cetus-marketdata-scanner/internal/model"

// Config holds the (data-driven) scan thresholds. Tuning never touches logic.
type Config struct {
	Lookback   int     // trailing bars forming the baseline
	VolumeMult float64 // volume-breakout threshold (× trailing average)
	GapPct     float64 // gap threshold (fraction vs the prior close)
}

// Scan is a PURE function. bars must be ascending by timestamp and split-adjusted
// (as read from the cetus adjusted_bars view). It evaluates the most recent bar
// against the trailing Lookback window and returns any signals fired.
func Scan(symbol string, bars []model.Bar, cfg Config) []model.Signal {
	if cfg.Lookback < 1 || len(bars) < cfg.Lookback+1 {
		return nil
	}
	last := bars[len(bars)-1]
	prev := bars[len(bars)-2]
	window := bars[len(bars)-1-cfg.Lookback : len(bars)-1] // trailing baseline, excludes last

	var out []model.Signal

	// Volume breakout: last volume >= mult × trailing average.
	// (Note: on the free IEX feed volume is a fraction of consolidated — treat as
	// provisional until a SIP/Yahoo source exists. See the data dictionary.)
	var sum float64
	for _, b := range window {
		sum += float64(b.Volume)
	}
	if avg := sum / float64(len(window)); avg > 0 && float64(last.Volume) >= cfg.VolumeMult*avg {
		out = append(out, model.Signal{Symbol: symbol, Type: model.SignalVolumeBreakout, Date: last.Timestamp, Value: float64(last.Volume) / avg})
	}

	// Price breakout: last close above the trailing high.
	hi := window[0].High
	for _, b := range window {
		if b.High > hi {
			hi = b.High
		}
	}
	if hi > 0 && last.Close > hi {
		out = append(out, model.Signal{Symbol: symbol, Type: model.SignalPriceBreakout, Date: last.Timestamp, Value: last.Close/hi - 1})
	}

	// Gap up/down vs the prior close.
	if prev.Close > 0 {
		switch gap := last.Open/prev.Close - 1; {
		case gap >= cfg.GapPct:
			out = append(out, model.Signal{Symbol: symbol, Type: model.SignalGapUp, Date: last.Timestamp, Value: gap})
		case gap <= -cfg.GapPct:
			out = append(out, model.Signal{Symbol: symbol, Type: model.SignalGapDown, Date: last.Timestamp, Value: gap})
		}
	}
	return out
}
