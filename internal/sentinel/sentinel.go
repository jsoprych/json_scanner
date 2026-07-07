// Package sentinel is the data-integrity gate. Tier-0 is pure, deterministic, and
// LLM-agnostic: it flags rows whose numbers smell wrong (extreme move × thin
// liquidity × price far above its own 200-DMA — the signature of an uncaught split
// or an illiquid print) before they ever reach a digest or an email.
//
// Higher tiers (cross-source price recheck, news verification) are roadmapped and
// pluggable behind this same seam; they must never be a dependency — Tier-0 stands
// alone. See docs/SCANNER_DESIGN.md and the AI value-add notes.
package sentinel

import (
	"math"
	"sort"

	"cetus-marketdata-scanner/internal/screen"
)

// Severity ranks a flag.
type Severity string

const (
	Suspect Severity = "suspect" // almost certainly a data error — hold it back
	Watch   Severity = "watch"   // extreme but plausible — verify before trusting
)

// Flag is one abnormal row with the evidence behind the verdict.
type Flag struct {
	Symbol    string   `json:"symbol"`
	Severity  Severity `json:"severity"`
	Reason    string   `json:"reason"`
	Ret3m     float64  `json:"ret_3m"`
	DollarVol float64  `json:"dollar_vol"`
	Ratio200  float64  `json:"ratio_200dma"` // close / sma200
}

// Tier0Config holds the deterministic thresholds (data-driven, never hardcoded in
// logic).
type Tier0Config struct {
	ExtremeRet    float64 // 3-mo return that warrants a WATCH (e.g. 1.0 = +100%)
	SuspectRet    float64 // 3-mo return that, if thin, is SUSPECT (e.g. 2.0 = +200%)
	ThinDollarVol float64 // liquidity below which an extreme move is suspect
}

// DefaultTier0 is a sensible starting point; tune via config as data teaches us.
func DefaultTier0() Tier0Config {
	return Tier0Config{ExtremeRet: 1.0, SuspectRet: 2.0, ThinDollarVol: 5e6}
}

// Tier0 flags abnormal rows, most severe first. Rows without a 3-mo return are
// skipped (nothing to judge).
func Tier0(rows []screen.SnapshotRow, cfg Tier0Config) []Flag {
	var out []Flag
	for _, r := range rows {
		if math.IsNaN(r.Ret3m) {
			continue
		}
		ratio := math.NaN()
		if !math.IsNaN(r.SMA200) && r.SMA200 > 0 {
			ratio = r.Close / r.SMA200
		}

		var sev Severity
		var reason string
		switch {
		case r.Ret3m >= cfg.SuspectRet && r.DollarVol < cfg.ThinDollarVol:
			sev = Suspect
			reason = "extreme 3-mo move on thin liquidity — likely uncaught split or illiquid print"
		case r.Ret3m >= cfg.ExtremeRet:
			sev = Watch
			reason = "extreme 3-mo move — verify catalyst / corporate action"
		default:
			continue
		}
		out = append(out, Flag{
			Symbol: r.Symbol, Severity: sev, Reason: reason,
			Ret3m: r.Ret3m, DollarVol: r.DollarVol, Ratio200: ratio,
		})
	}

	// Suspect before Watch; within a severity, worst move first.
	sevRank := func(s Severity) int {
		if s == Suspect {
			return 0
		}
		return 1
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := sevRank(out[i].Severity), sevRank(out[j].Severity); a != b {
			return a < b
		}
		return out[i].Ret3m > out[j].Ret3m
	})
	return out
}

// Counts summarizes flags by severity.
func Counts(flags []Flag) (suspect, watch int) {
	for _, f := range flags {
		if f.Severity == Suspect {
			suspect++
		} else {
			watch++
		}
	}
	return
}
