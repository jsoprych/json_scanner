package scan

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"cetus-marketdata-scanner/internal/model"
)

type fakeLoader struct {
	bars map[string][]model.Bar
	err  error
}

func (f *fakeLoader) LoadAdjustedBars(_ context.Context, symbol string, _ int64) ([]model.Bar, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bars[symbol], nil
}

func makeBars(n int, base float64) []model.Bar {
	bars := make([]model.Bar, n)
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	for i := 0; i < n; i++ {
		bars[i] = model.Bar{
			Symbol:    "TEST",
			Timestamp: ts + int64(i)*86400,
			Open:      base,
			High:      base + 1,
			Low:       base - 1,
			Close:     base,
			Volume:    1000000,
		}
	}
	return bars
}

func testLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestUniverseBasic(t *testing.T) {
	bars := makeBars(300, 50.0)
	loader := &fakeLoader{bars: map[string][]model.Bar{
		"AAPL": bars,
		"MSFT": bars,
	}}

	res := Universe(context.Background(), loader, []string{"AAPL", "MSFT"}, Options{
		Since:   0,
		Workers: 2,
	}, testLog())

	if res.Scanned != 2 {
		t.Errorf("Scanned: got %d want 2", res.Scanned)
	}
	if len(res.Rows) != 2 {
		t.Errorf("Rows: got %d want 2", len(res.Rows))
	}
	if res.Rows[0].Symbol > res.Rows[1].Symbol {
		t.Error("rows not sorted by symbol")
	}
}

func TestUniverseMinDollarVol(t *testing.T) {
	liquid := makeBars(300, 50.0)
	illiquid := makeBars(300, 1.0)
	for i := range illiquid {
		illiquid[i].Volume = 100
	}

	loader := &fakeLoader{bars: map[string][]model.Bar{
		"LIQUID":   liquid,
		"ILLIQUID": illiquid,
	}}

	res := Universe(context.Background(), loader, []string{"LIQUID", "ILLIQUID"}, Options{
		Since:        0,
		MinDollarVol: 1e6,
		Workers:      2,
	}, testLog())

	if res.Scanned != 2 {
		t.Errorf("Scanned: got %d want 2", res.Scanned)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("Rows: got %d want 1 (only LIQUID should pass)", len(res.Rows))
	}
	if res.Rows[0].Symbol != "LIQUID" {
		t.Errorf("got %q want LIQUID", res.Rows[0].Symbol)
	}
}

func TestUniverseCancellation(t *testing.T) {
	bars := makeBars(300, 50.0)
	loader := &fakeLoader{bars: map[string][]model.Bar{"AAPL": bars}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := Universe(ctx, loader, []string{"AAPL"}, Options{Since: 0, Workers: 1}, testLog())
	if res.Scanned != 0 {
		t.Errorf("Scanned: got %d want 0 (cancelled)", res.Scanned)
	}
}

func TestUniverseLoadError(t *testing.T) {
	loader := &fakeLoader{err: errors.New("db error")}

	res := Universe(context.Background(), loader, []string{"AAPL"}, Options{
		Since:   0,
		Workers: 1,
	}, testLog())

	if res.Scanned != 0 {
		t.Errorf("Scanned: got %d want 0 (load error)", res.Scanned)
	}
	if len(res.Rows) != 0 {
		t.Errorf("Rows: got %d want 0", len(res.Rows))
	}
}

func TestUniverseEmptyBars(t *testing.T) {
	loader := &fakeLoader{bars: map[string][]model.Bar{
		"AAPL": nil,
		"MSFT": makeBars(300, 50.0),
	}}

	res := Universe(context.Background(), loader, []string{"AAPL", "MSFT"}, Options{
		Since:   0,
		Workers: 2,
	}, testLog())

	if res.Scanned != 1 {
		t.Errorf("Scanned: got %d want 1 (AAPL has no bars)", res.Scanned)
	}
	if len(res.Rows) != 1 {
		t.Errorf("Rows: got %d want 1", len(res.Rows))
	}
}
