// Package model holds neutral domain types shared across the scanner.
package model

// Bar is a split-adjusted daily OHLCV bar, as read from the cetus warehouse's
// `adjusted_bars` view. Prices are split-adjusted; volume is split-adjusted and
// feed-limited (IEX-only on the free tier — see the data dictionary). Timestamp
// is Unix epoch seconds, UTC.
type Bar struct {
	Symbol    string
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	VWAP      float64
	Source    string
}

// Signal is a detected scan event on a symbol's most recent bar.
type Signal struct {
	Symbol string  `json:"symbol"`
	Type   string  `json:"type"`  // one of the Signal* constants
	Date   int64   `json:"date"`  // Unix seconds of the triggering bar
	Value  float64 `json:"value"` // magnitude: volume multiple, gap fraction, breakout %
}

// Signal types.
const (
	SignalVolumeBreakout = "volume_breakout"
	SignalPriceBreakout  = "price_breakout"
	SignalGapUp          = "gap_up"
	SignalGapDown        = "gap_down"
)
