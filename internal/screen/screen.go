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

// SnapshotRow is one symbol's latest cross-sectional state: the display fields plus
// every feature the MVP presets reference. NaN marks a not-yet-warmed feature;
// comparisons against NaN are false in Go, so an under-warm symbol naturally fails
// a filter instead of matching on partial data.
type SnapshotRow struct {
	Symbol string  `json:"symbol"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Open   float64 `json:"open"`

	// Trend indicators
	SMA5   float64 `json:"sma5"`
	SMA10  float64 `json:"sma10"`
	SMA20  float64 `json:"sma20"`
	SMA30  float64 `json:"sma30"`
	SMA50  float64 `json:"sma50"`
	SMA100 float64 `json:"sma100"`
	SMA200 float64 `json:"sma200"`
	EMA10  float64 `json:"ema10"`
	EMA21  float64 `json:"ema21"`
	EMA50  float64 `json:"ema50"`
	EMA100 float64 `json:"ema100"`
	EMA200 float64 `json:"ema200"`
	PctFromSMA50  float64 `json:"pct_from_sma50"`
	PctFromSMA200 float64 `json:"pct_from_sma200"`
	MAStack       bool    `json:"ma_stack"`

	// Momentum indicators
	RSI14         float64 `json:"rsi14"`
	MACD          float64 `json:"macd"`
	MACDSignal    float64 `json:"macd_signal"`
	MACDHist      float64 `json:"macd_hist"`
	StochK        float64 `json:"stoch_k"`
	StochD        float64 `json:"stoch_d"`
	WilliamsR     float64 `json:"willr14"`
	CCI20         float64 `json:"cci20"`
	ROC10         float64 `json:"roc10"`
	ROC20         float64 `json:"roc20"`
	ADX14         float64 `json:"adx14"`
	DIPlus        float64 `json:"di_plus"`
	DIMinus       float64 `json:"di_minus"`

	// Volatility indicators
	ATR14         float64 `json:"atr14"`
	ATRPct        float64 `json:"atr_pct"`
	BBUpper       float64 `json:"bb_upper"`
	BBMiddle      float64 `json:"bb_mid"`
	BBLower       float64 `json:"bb_lower"`
	BBWidth       float64 `json:"bb_bandwidth"`
	BBPctB        float64 `json:"bb_pct_b"`
	HistVol20     float64 `json:"hist_vol20"`

	// Price structure
	High52w       float64 `json:"high_52w"`
	Low52w        float64 `json:"low_52w"`
	Is52wHigh     bool    `json:"is_52w_high"`
	Is52wLow      bool    `json:"is_52w_low"`
	GapPct        float64 `json:"gap_pct"`
	TrueRange     float64 `json:"true_range"`
	PctOff52wHigh float64 `json:"pct_off_52w_high"`
	PctAbove52wLow float64 `json:"pct_above_52w_low"`

	// Returns
	Ret1d  float64 `json:"ret_1d"`
	Ret5d  float64 `json:"ret_5d"`
	Ret1m  float64 `json:"ret_1m"`
	Ret3m  float64 `json:"ret_3m"`
	Ret6m  float64 `json:"ret_6m"`
	Ret1y  float64 `json:"ret_1y"`

	// Volume indicators
	DollarVol      float64 `json:"dollar_vol"`
	AvgDollarVol20 float64 `json:"avg_dollar_vol20"`
	RelVolume      float64 `json:"rel_volume"`
	OBV            float64 `json:"obv"`
	VWAPDist       float64 `json:"vwap_dist"`
	MFI14          float64 `json:"mfi14"`

	// Cross-detection booleans
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
	opens := make([]float64, n)
	volumes := make([]int64, n)
	for i, b := range bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
		opens[i] = b.Open
		volumes[i] = b.Volume
	}

	// Trend indicators
	sma5 := indicators.SMA(closes, 5)
	sma10 := indicators.SMA(closes, 10)
	sma20 := indicators.SMA(closes, 20)
	sma30 := indicators.SMA(closes, 30)
	sma50 := indicators.SMA(closes, 50)
	sma100 := indicators.SMA(closes, 100)
	sma200 := indicators.SMA(closes, 200)
	ema10 := indicators.EMA(closes, 10)
	ema21 := indicators.EMA(closes, 21)
	ema50 := indicators.EMA(closes, 50)
	ema100 := indicators.EMA(closes, 100)
	ema200 := indicators.EMA(closes, 200)
	pctFromSMA50 := indicators.PctFromSMA(closes, sma50)
	pctFromSMA200 := indicators.PctFromSMA(closes, sma200)
	maStack := indicators.MAStack(ema10, ema21, ema50, ema200)

	// Momentum indicators
	rsi := indicators.RSI(closes, 14)
	macd, macdSignal, macdHist := indicators.MACD(closes)
	stochK, stochD := indicators.Stochastic(highs, lows, closes, 14, 3, 3)
	williamsR := indicators.WilliamsR(highs, lows, closes, 14)
	cci20 := indicators.CCI(highs, lows, closes, 20)
	roc10 := indicators.ROC(closes, 10)
	roc20 := indicators.ROC(closes, 20)
	adx14, diPlus, diMinus := indicators.ADX(highs, lows, closes, 14)

	// Volatility indicators
	atr14 := indicators.ATR(highs, lows, closes, 14)
	atrPct := indicators.ATRPct(highs, lows, closes, 14)
	bbUpper, bbMiddle, bbLower := indicators.BollingerBands(closes, 20, 2.0)
	bbWidth := indicators.BBWidth(closes, 20, 2.0)
	bbPctB := indicators.BBPctB(closes, 20, 2.0)
	histVol20 := indicators.HistoricalVol(closes, 20)

	// Price structure
	hi52 := indicators.RollingHigh(highs, 252)
	lo52 := indicators.RollingLow(lows, 252)
	is52wHigh := indicators.Is52wHigh(closes, hi52)
	is52wLow := indicators.Is52wLow(closes, lo52)
	gapPct := indicators.GapPct(opens, closes)
	trueRange := indicators.TrueRange(highs, lows, closes)
	pctOff52wHigh := indicators.PctOff52wHigh(closes, hi52)
	pctAbove52wLow := indicators.PctAbove52wLow(closes, lo52)

	// Returns
	ret1d := indicators.Return(closes, 1)
	ret5d := indicators.Return(closes, 5)
	ret1m := indicators.Return(closes, 21)
	ret3m := indicators.Return(closes, 63)
	ret6m := indicators.Return(closes, 126)
	ret1y := indicators.Return(closes, 252)

	// Volume indicators
	dollarVol := indicators.DollarVol(closes, volumes)
	avgDollarVol20 := indicators.AvgDollarVol(closes, volumes, 20)
	relVolume := indicators.RelVolume(volumes, 20)
	obv := indicators.OBV(closes, volumes)
	vwapDist := indicators.VWAPDist(highs, lows, closes, volumes, 20)
	mfi14 := indicators.MFI(highs, lows, closes, volumes, 14)

	last := bars[n-1]
	row := SnapshotRow{
		Symbol: symbol,
		Close:  last.Close,
		High:   last.High,
		Low:    last.Low,
		Open:   last.Open,

		// Trend
		SMA5:          sma5[n-1],
		SMA10:         sma10[n-1],
		SMA20:         sma20[n-1],
		SMA30:         sma30[n-1],
		SMA50:         sma50[n-1],
		SMA100:        sma100[n-1],
		SMA200:        sma200[n-1],
		EMA10:         ema10[n-1],
		EMA21:         ema21[n-1],
		EMA50:         ema50[n-1],
		EMA100:        ema100[n-1],
		EMA200:        ema200[n-1],
		PctFromSMA50:  pctFromSMA50[n-1],
		PctFromSMA200: pctFromSMA200[n-1],
		MAStack:       maStack[n-1],

		// Momentum
		RSI14:      rsi[n-1],
		MACD:       macd[n-1],
		MACDSignal: macdSignal[n-1],
		MACDHist:   macdHist[n-1],
		StochK:     stochK[n-1],
		StochD:     stochD[n-1],
		WilliamsR:  williamsR[n-1],
		CCI20:      cci20[n-1],
		ROC10:      roc10[n-1],
		ROC20:      roc20[n-1],
		ADX14:      adx14[n-1],
		DIPlus:     diPlus[n-1],
		DIMinus:    diMinus[n-1],

		// Volatility
		ATR14:     atr14[n-1],
		ATRPct:    atrPct[n-1],
		BBUpper:   bbUpper[n-1],
		BBMiddle:  bbMiddle[n-1],
		BBLower:   bbLower[n-1],
		BBWidth:   bbWidth[n-1],
		BBPctB:    bbPctB[n-1],
		HistVol20: histVol20[n-1],

		// Price structure
		High52w:        hi52[n-1],
		Low52w:         lo52[n-1],
		Is52wHigh:      is52wHigh[n-1],
		Is52wLow:       is52wLow[n-1],
		GapPct:         gapPct[n-1],
		TrueRange:      trueRange[n-1],
		PctOff52wHigh:  pctOff52wHigh[n-1],
		PctAbove52wLow: pctAbove52wLow[n-1],

		// Returns
		Ret1d: ret1d[n-1],
		Ret5d: ret5d[n-1],
		Ret1m: ret1m[n-1],
		Ret3m: ret3m[n-1],
		Ret6m: ret6m[n-1],
		Ret1y: ret1y[n-1],

		// Volume
		DollarVol:      dollarVol[n-1],
		AvgDollarVol20: avgDollarVol20[n-1],
		RelVolume:      relVolume[n-1],
		OBV:            obv[n-1],
		VWAPDist:       vwapDist[n-1],
		MFI14:          mfi14[n-1],
	}

	// Compute crosses directly with shifted indicators (no prev_* fields needed)
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

// At52wHigh reports today's high being the highest in the trailing 52 weeks.
func (r SnapshotRow) At52wHigh() bool {
	if !math.IsNaN(r.High52w) && r.High52w > 0 && r.Close >= r.High52w {
		return true
	}
	return r.Is52wHigh
}

func (r SnapshotRow) At52wLow() bool {
	if !math.IsNaN(r.Low52w) && r.Low52w > 0 && r.Close <= r.Low52w {
		return true
	}
	return r.Is52wLow
}

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
			Match: SnapshotRow.At52wHigh, Metric: dollarVol, Limit: topN},
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
		if r.At52wHigh() {
			b.New52wHigh++
		}
		if r.At52wLow() {
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
