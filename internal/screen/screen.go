// Package screen builds the cross-sectional snapshot and runs the Phase-1 preset
// studies + market breadth over it. It is PURE and standalone: given a symbol's
// split-adjusted bars it derives one SnapshotRow (latest features + the 1-bar
// mirror needed for crosses); given the whole universe of rows it filters/ranks.
//
// See docs/PHASE1_MVP.md for the fixed lineup and docs/SCANNER_DESIGN.md for why
// the scan surface is a cross-sectional snapshot rather than a per-symbol grid.
package screen

import (
	"math"
	"sort"

	"cetus-marketdata-scanner/internal/indicators"
	"cetus-marketdata-scanner/internal/model"
)

// Fixed Phase-1 windows (data-driven config comes with the fuller catalog).
const (
	smaFast  = 50
	smaSlow  = 200
	rsiLen   = 14
	weeks52  = 252
	ret3mLen = 63
)

// SnapshotRow is one symbol's latest cross-sectional state: the display fields plus
// every feature the MVP presets reference. NaN marks a not-yet-warmed feature;
// comparisons against NaN are false in Go, so an under-warm symbol naturally fails
// a filter instead of matching on partial data.
type SnapshotRow struct {
	Symbol string  `json:"symbol"`
	Close  float64 `json:"close"`

	DollarVol float64 `json:"dollar_vol"` // close × volume (feed-safe ranking metric)

	SMA50  float64 `json:"sma50"`
	SMA200 float64 `json:"sma200"`

	RSI14 float64 `json:"rsi14"`

	High    float64 `json:"-"` // today's H/L, for the 52-week flags
	Low     float64 `json:"-"`
	High52w float64 `json:"-"`
	Low52w  float64 `json:"-"`
	Ret3m   float64 `json:"ret_3m"`

	// Cross-detection booleans (computed directly, no prev_* fields needed)
	IsGoldenCross    bool `json:"golden_cross"`
	IsOversoldBounce bool `json:"oversold_bounce"`
}

// Build derives the latest SnapshotRow from a symbol's ascending, split-adjusted
// bars. ok is false only when there are no bars at all; short histories still
// return a row whose deep-window features are NaN.
func Build(symbol string, bars []model.Bar) (SnapshotRow, bool) {
	n := len(bars)
	if n == 0 {
		return SnapshotRow{}, false
	}

	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	for i, b := range bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
	}

	sma50 := indicators.SMA(closes, smaFast)
	sma200 := indicators.SMA(closes, smaSlow)
	rsi := indicators.RSI(closes, rsiLen)
	hi52 := indicators.RollingHigh(highs, weeks52)
	lo52 := indicators.RollingLow(lows, weeks52)
	ret3 := indicators.Return(closes, ret3mLen)

	last := bars[n-1]
	row := SnapshotRow{
		Symbol:    symbol,
		Close:     last.Close,
		DollarVol: last.Close * float64(last.Volume),
		High:      last.High,
		Low:       last.Low,
		SMA50:     sma50[n-1],
		SMA200:    sma200[n-1],
		RSI14:     rsi[n-1],
		High52w:   hi52[n-1],
		Low52w:    lo52[n-1],
		Ret3m:     ret3[n-1],
	}

	// Compute crosses directly with shifted indicators (no prev_* fields needed)
	// With shifted indicators, the value at n-1 is computed from bars ≤ n-2.
	// The value at n-2 is computed from bars ≤ n-3.
	// A golden cross at the latest bar means:
	//   sma50[n-1] > sma200[n-1] AND sma50[n-2] <= sma200[n-2]
	if n >= 2 {
		row.IsGoldenCross = sma50[n-1] > sma200[n-1] && sma50[n-2] <= sma200[n-2]
		row.IsOversoldBounce = rsi[n-1] > 30 && rsi[n-2] <= 30
	}

	return row, true
}

// --- predicates (NaN-safe by construction: any comparison with NaN is false) ---

// AboveSMA200 reports a long-term uptrend.
func (r SnapshotRow) AboveSMA200() bool { return r.Close > r.SMA200 }

// AboveSMA50 reports a medium-term uptrend.
func (r SnapshotRow) AboveSMA50() bool { return r.Close > r.SMA50 }

// GoldenCross reports SMA50 crossing above SMA200 on the latest bar.
func (r SnapshotRow) GoldenCross() bool {
	return r.IsGoldenCross
}

// OversoldBounce reports RSI(14) crossing back above 30 on the latest bar.
func (r SnapshotRow) OversoldBounce() bool {
	return r.IsOversoldBounce
}

// Is52wHigh reports today's high being the highest in the trailing 52 weeks.
func (r SnapshotRow) Is52wHigh() bool { return r.High >= r.High52w && !math.IsNaN(r.High52w) }

// Is52wLow reports today's low being the lowest in the trailing 52 weeks.
func (r SnapshotRow) Is52wLow() bool { return r.Low <= r.Low52w && !math.IsNaN(r.Low52w) }

// --- presets ---

// Preset is one named study: a filter, a ranking metric (higher = better; NaN rows
// dropped), and a cap.
type Preset struct {
	Key    string
	Title  string
	Emoji  string
	Match  func(SnapshotRow) bool  // nil = whole universe (cross-sectional study)
	Metric func(SnapshotRow) float64
	Limit  int
}

// Run applies the preset to the snapshot: filter → drop NaN-metric rows → rank
// descending → cap.
func (p Preset) Run(rows []SnapshotRow) []SnapshotRow {
	var out []SnapshotRow
	for _, r := range rows {
		if p.Match != nil && !p.Match(r) {
			continue
		}
		if math.IsNaN(p.Metric(r)) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return p.Metric(out[i]) > p.Metric(out[j]) })
	if p.Limit > 0 && len(out) > p.Limit {
		out = out[:p.Limit]
	}
	return out
}

func dollarVol(r SnapshotRow) float64 { return r.DollarVol }
func ret3m(r SnapshotRow) float64     { return r.Ret3m }

// MVPPresets returns the Phase-1 digest lineup, in display order.
func MVPPresets(topN, momentumN int) []Preset {
	return []Preset{
		{Key: "new_52w_high", Title: "New 52-Week Highs", Emoji: "📈",
			Match: SnapshotRow.Is52wHigh, Metric: dollarVol, Limit: topN},
		{Key: "golden_cross", Title: "Golden Cross Today", Emoji: "⚡",
			Match: SnapshotRow.GoldenCross, Metric: dollarVol, Limit: topN},
		{Key: "oversold_bounce", Title: "Oversold Bounce", Emoji: "🔄",
			Match: SnapshotRow.OversoldBounce, Metric: dollarVol, Limit: topN},
		{Key: "momentum_leaders", Title: "Momentum Leaders (3-mo)", Emoji: "🚀",
			Match: nil, Metric: ret3m, Limit: momentumN},
	}
}

// --- market breadth ---

// Breadth is the cross-sectional health summary shown atop the digest.
type Breadth struct {
	Total       int `json:"total"`
	WithSMA200  int `json:"with_sma200"`
	AboveSMA200 int `json:"above_sma200"`
	WithSMA50   int `json:"with_sma50"`
	AboveSMA50  int `json:"above_sma50"`
	New52wHigh  int `json:"new_52w_high"`
	New52wLow   int `json:"new_52w_low"`
}

// ComputeBreadth aggregates the snapshot. Percentages are taken over symbols that
// actually have the moving average (a symbol lacking 200 bars isn't counted against
// the 200-DMA breadth).
func ComputeBreadth(rows []SnapshotRow) Breadth {
	var b Breadth
	b.Total = len(rows)
	for _, r := range rows {
		if !math.IsNaN(r.SMA200) {
			b.WithSMA200++
			if r.AboveSMA200() {
				b.AboveSMA200++
			}
		}
		if !math.IsNaN(r.SMA50) {
			b.WithSMA50++
			if r.AboveSMA50() {
				b.AboveSMA50++
			}
		}
		if r.Is52wHigh() {
			b.New52wHigh++
		}
		if r.Is52wLow() {
			b.New52wLow++
		}
	}
	return b
}

// PctAbove200 is the share of MA-eligible symbols above their 200-DMA (0 if none).
func (b Breadth) PctAbove200() float64 { return pct(b.AboveSMA200, b.WithSMA200) }

// PctAbove50 is the share of MA-eligible symbols above their 50-DMA (0 if none).
func (b Breadth) PctAbove50() float64 { return pct(b.AboveSMA50, b.WithSMA50) }

func pct(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return 100 * float64(num) / float64(den)
}
