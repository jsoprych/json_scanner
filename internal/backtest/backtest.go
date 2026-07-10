package backtest

import (
	"cetus-marketdata-scanner/internal/snapshot"
	"cetus-marketdata-scanner/internal/study"
)

// Result represents the outcome of a backtest for a single symbol.
type Result struct {
	Symbol   string  `json:"symbol"`
	Date     int64   `json:"date"`      // Entry date (Unix timestamp)
	EntryPx  float64 `json:"entry_px"`  // Entry price (close on entry date)
	ExitPx   float64 `json:"exit_px"`   // Exit price (close on exit date)
	Return   float64 `json:"return"`    // Return as decimal (e.g., 0.05 = 5%)
	HoldDays int     `json:"hold_days"` // Number of days held
}

// Summary represents the aggregate results of a backtest.
type Summary struct {
	StudyKey      string   `json:"study_key"`
	StartDate     int64    `json:"start_date"`
	EndDate       int64    `json:"end_date"`
	TotalTrades   int      `json:"total_trades"`
	WinningTrades int      `json:"winning_trades"`
	LosingTrades  int      `json:"losing_trades"`
	WinRate       float64  `json:"win_rate"`
	AvgReturn     float64  `json:"avg_return"`
	TotalReturn   float64  `json:"total_return"`
	Results       []Result `json:"results"`
}

// Engine runs backtests over historical snapshots.
type Engine struct {
	snap *snapshot.DB
}

// NewEngine creates a new backtest engine.
func NewEngine(snap *snapshot.DB) *Engine {
	return &Engine{snap: snap}
}

// RunBacktest runs a study over a range of historical dates and calculates returns.
func (e *Engine) RunBacktest(s study.Study, startDate, endDate int64, holdDays int) (*Summary, error) {
	dates, err := e.snap.ListSnapshots()
	if err != nil {
		return nil, err
	}

	summary := &Summary{
		StudyKey:  s.Key,
		StartDate: startDate,
		EndDate:   endDate,
		Results:   []Result{},
	}

	for _, date := range dates {
		if date < startDate || date > endDate {
			continue
		}

		if err := e.snap.SetActive(date); err != nil {
			continue
		}

		matches, err := e.snap.Run(s)
		if err != nil {
			continue
		}

		for _, match := range matches {
			result, err := e.calculateReturn(match.Symbol, date, holdDays)
			if err != nil || result == nil {
				continue
			}
			summary.Results = append(summary.Results, *result)
		}
	}

	e.calculateSummary(summary)
	return summary, nil
}

// calculateReturn calculates the return for a symbol over a hold period.
// Uses parameterized queries via SymbolClose and NearestDate — no string
// concatenation, no SQL injection.
func (e *Engine) calculateReturn(symbol string, entryDate int64, holdDays int) (*Result, error) {
	entryPx, err := e.snap.SymbolClose(symbol, entryDate)
	if err != nil {
		return nil, err
	}

	exitDate := entryDate + int64(holdDays*86400)
	actualExitDate, err := e.snap.NearestDate(exitDate)
	if err != nil {
		return nil, err
	}

	exitPx, err := e.snap.SymbolClose(symbol, actualExitDate)
	if err != nil {
		return nil, err
	}

	ret := (exitPx - entryPx) / entryPx
	holdDaysActual := int((actualExitDate - entryDate) / 86400)

	return &Result{
		Symbol:   symbol,
		Date:     entryDate,
		EntryPx:  entryPx,
		ExitPx:   exitPx,
		Return:   ret,
		HoldDays: holdDaysActual,
	}, nil
}

// calculateSummary calculates aggregate statistics for the backtest.
func (e *Engine) calculateSummary(s *Summary) {
	s.TotalTrades = len(s.Results)
	if s.TotalTrades == 0 {
		return
	}

	totalReturn := 0.0
	for _, r := range s.Results {
		totalReturn += r.Return
		if r.Return > 0 {
			s.WinningTrades++
		} else {
			s.LosingTrades++
		}
	}

	s.AvgReturn = totalReturn / float64(s.TotalTrades)
	s.WinRate = float64(s.WinningTrades) / float64(s.TotalTrades)
	s.TotalReturn = totalReturn
}

// RunPointInTime runs a study on a specific date and returns the matches.
func (e *Engine) RunPointInTime(s study.Study, date int64) ([]snapshot.Match, error) {
	if err := e.snap.SetActive(date); err != nil {
		return nil, err
	}
	return e.snap.Run(s)
}
