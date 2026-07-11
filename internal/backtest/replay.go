package backtest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cetus-marketdata-scanner/internal/model"
	"cetus-marketdata-scanner/internal/scanner"
	"cetus-marketdata-scanner/internal/snapshot"
)

// HistoricalSignal is a scan signal with P/L calculated to a reference date.
type HistoricalSignal struct {
	Symbol    string  `json:"symbol"`
	Type      string  `json:"type"`
	Date      int64   `json:"date"`
	Value     float64 `json:"value"`
	EntryPx   float64 `json:"entry_px"`
	CurrentPx float64 `json:"current_px"`
	Return    float64 `json:"return"`
	MaxProfit float64 `json:"max_profit"` // Max gain % from entry to highest price
	MaxLoss   float64 `json:"max_loss"`   // Max drawdown % from entry to lowest price
}

// HistoricalScanResult is the output of a historical scan replay.
type HistoricalScanResult struct {
	ScanDate    time.Time          `json:"scan_date"`
	CurrentDate time.Time          `json:"current_date"`
	Signals     []HistoricalSignal `json:"signals"`
	Summary     ScanSummary        `json:"summary"`
}

// ScanSummary aggregates signal performance.
type ScanSummary struct {
	TotalSignals int     `json:"total_signals"`
	Winners      int     `json:"winners"`
	Losers       int     `json:"losers"`
	WinRate      float64 `json:"win_rate"`
	AvgReturn    float64 `json:"avg_return"`
	TotalReturn  float64 `json:"total_return"`
}

// BarLoader supplies bars for a symbol.
type BarLoader interface {
	LoadAdjustedBars(ctx context.Context, symbol string, since int64) ([]model.Bar, error)
}

// ReplayHistoricalScan runs a scan as of a historical date and calculates P/L to current.
// Uses the snapshot DB for P/L calculation (no bar loading needed).
func ReplayHistoricalScan(
	ctx context.Context,
	loader BarLoader,
	snap *snapshot.DB,
	symbols []string,
	scanDate time.Time,
	cfg scanner.Config,
	log *slog.Logger,
) (*HistoricalScanResult, error) {
	// Get current snapshot date for P/L calculation
	dates, err := snap.ListSnapshots()
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	if len(dates) == 0 {
		return nil, fmt.Errorf("no snapshots available")
	}
	currentDate := time.Unix(dates[0], 0).UTC()
	currentDateUnix := dates[0]

	since := scanDate.AddDate(0, 0, -400).Unix() // Load enough history for indicators
	scanDateUnix := scanDate.Unix()

	var signals []HistoricalSignal
	var totalReturn float64
	var winners, losers int

	for _, sym := range symbols {
		if ctx.Err() != nil {
			break
		}

		// Load bars up to scan date
		bars, err := loader.LoadAdjustedBars(ctx, sym, since)
		if err != nil {
			log.Warn("load bars failed", "symbol", sym, "error", err)
			continue
		}

		// Filter bars to only include up to scan date
		var historicalBars []model.Bar
		for _, b := range bars {
			if b.Timestamp <= scanDateUnix {
				historicalBars = append(historicalBars, b)
			}
		}

		if len(historicalBars) == 0 {
			continue
		}

		// Run scan on historical bars
		scanSignals := scanner.Scan(sym, historicalBars, cfg)

		// For each signal, calculate P/L using snapshot DB
		for _, sig := range scanSignals {
			entryPx := historicalBars[len(historicalBars)-1].Close

			// Get current price from snapshot (parameterized query, indexed)
			currentPx, err := snap.SymbolClose(sym, currentDateUnix)
			if err != nil {
				continue // Skip if no current price
			}

			ret := (currentPx - entryPx) / entryPx
			totalReturn += ret

			if ret > 0 {
				winners++
			} else {
				losers++
			}

			// Calculate max profit and max loss from signal date to current
			maxProfit := 0.0
			maxLoss := 0.0
			if len(bars) > 0 {
				// Find bars after signal date
				var futureBars []model.Bar
				for _, b := range bars {
					if b.Timestamp > scanDateUnix {
						futureBars = append(futureBars, b)
					}
				}
				
				// Find max high and min low in future bars
				if len(futureBars) > 0 {
					maxHigh := futureBars[0].High
					minLow := futureBars[0].Low
					for _, b := range futureBars {
						if b.High > maxHigh {
							maxHigh = b.High
						}
						if b.Low < minLow {
							minLow = b.Low
						}
					}
					maxProfit = (maxHigh - entryPx) / entryPx
					maxLoss = (minLow - entryPx) / entryPx
				}
			}

			signals = append(signals, HistoricalSignal{
				Symbol:    sig.Symbol,
				Type:      sig.Type,
				Date:      sig.Date,
				Value:     sig.Value,
				EntryPx:   entryPx,
				CurrentPx: currentPx,
				Return:    ret,
				MaxProfit: maxProfit,
				MaxLoss:   maxLoss,
			})
		}
	}

	total := len(signals)
	avgReturn := 0.0
	winRate := 0.0
	if total > 0 {
		avgReturn = totalReturn / float64(total)
		winRate = float64(winners) / float64(total)
	}

	return &HistoricalScanResult{
		ScanDate:    scanDate,
		CurrentDate: currentDate,
		Signals:     signals,
		Summary: ScanSummary{
			TotalSignals: total,
			Winners:      winners,
			Losers:       losers,
			WinRate:      winRate,
			AvgReturn:    avgReturn,
			TotalReturn:  totalReturn,
		},
	}, nil
}
