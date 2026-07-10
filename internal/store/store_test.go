package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func createTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ddls := []string{
		`CREATE TABLE symbol_pipeline_state (
			symbol TEXT PRIMARY KEY,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE symbols (
			symbol TEXT PRIMARY KEY,
			exchange TEXT,
			security_type TEXT
		)`,
		`CREATE TABLE eod_bars (
			symbol TEXT,
			timestamp INTEGER,
			open REAL, high REAL, low REAL, close REAL,
			volume INTEGER
		)`,
		`CREATE TABLE split_factors (
			symbol TEXT,
			ex_date INTEGER,
			ratio REAL
		)`,
		`CREATE TABLE adjusted_bars (
			symbol TEXT,
			timestamp INTEGER,
			open REAL, high REAL, low REAL, close REAL,
			volume INTEGER,
			vwap REAL,
			source TEXT
		)`,
		`CREATE TABLE schema_version (
			id INTEGER PRIMARY KEY,
			version INTEGER
		)`,
	}
	for _, ddl := range ddls {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}

	inserts := []string{
		`INSERT INTO symbol_pipeline_state VALUES ('AAPL', 'SUCCESS')`,
		`INSERT INTO symbol_pipeline_state VALUES ('MSFT', 'SUCCESS')`,
		`INSERT INTO symbol_pipeline_state VALUES ('EMPTY', 'EMPTY')`,
		`INSERT INTO symbol_pipeline_state VALUES ('FAILED', 'FAILED')`,
		`INSERT INTO symbols VALUES ('AAPL', 'NASDAQ', 'common')`,
		`INSERT INTO symbols VALUES ('MSFT', 'NASDAQ', 'common')`,
		`INSERT INTO symbols VALUES ('EMPTY', 'NYSE', 'common')`,
		`INSERT INTO symbols VALUES ('FAILED', 'NYSE', 'common')`,
		`INSERT INTO adjusted_bars VALUES ('AAPL', 1000, 10, 11, 9, 10.5, 1000000, 10.5, 'iex')`,
		`INSERT INTO adjusted_bars VALUES ('MSFT', 1000, 20, 21, 19, 20.5, 2000000, 20.5, 'iex')`,
		`INSERT INTO schema_version VALUES (1, 1)`,
	}
	for _, ins := range inserts {
		if _, err := db.Exec(ins); err != nil {
			t.Fatal(err)
		}
	}

	return path
}

func TestOpenReadOnly(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if st.BarsTable() != "adjusted_bars" {
		t.Errorf("BarsTable: got %q want %q", st.BarsTable(), "adjusted_bars")
	}
	if st.SchemaVersion() != 1 {
		t.Errorf("SchemaVersion: got %d want 1", st.SchemaVersion())
	}
}

func TestCheckSchema(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.CheckSchema(); err != nil {
		t.Errorf("CheckSchema: unexpected error: %v", err)
	}
}

func TestUniverse(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	syms, err := st.Universe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 2 {
		t.Fatalf("Universe: got %d symbols want 2 (only SUCCESS)", len(syms))
	}
	if syms[0] != "AAPL" || syms[1] != "MSFT" {
		t.Errorf("Universe: got %v want [AAPL MSFT]", syms)
	}
}

func TestUniverseCommon(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	syms, err := st.UniverseCommon(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 2 {
		t.Errorf("UniverseCommon: got %d want 2", len(syms))
	}
}

func TestLoadAdjustedBars(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	bars, err := st.LoadAdjustedBars(ctx, "AAPL", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 1 {
		t.Fatalf("LoadAdjustedBars: got %d bars want 1", len(bars))
	}
	if bars[0].Symbol != "AAPL" {
		t.Errorf("Symbol: got %q want %q", bars[0].Symbol, "AAPL")
	}
	if bars[0].Close != 10.5 {
		t.Errorf("Close: got %f want 10.5", bars[0].Close)
	}
}

func TestBarsTablePreference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE adjusted_bars (symbol TEXT, timestamp INTEGER, open REAL, high REAL, low REAL, close REAL, volume INTEGER, vwap REAL, source TEXT);
		CREATE TABLE published_bars (symbol TEXT, timestamp INTEGER, open REAL, high REAL, low REAL, close REAL, volume INTEGER, vwap REAL, source TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	ctx := context.Background()
	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if st.BarsTable() != "published_bars" {
		t.Errorf("BarsTable preference: got %q want %q", st.BarsTable(), "published_bars")
	}
}

func TestStats(t *testing.T) {
	path := createTestDB(t)
	ctx := context.Background()

	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stats, err := st.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.RegistrySymbols != 4 {
		t.Errorf("RegistrySymbols: got %d want 4", stats.RegistrySymbols)
	}
	if stats.Count("SUCCESS") != 2 {
		t.Errorf("SUCCESS count: got %d want 2", stats.Count("SUCCESS"))
	}
}

func TestOpenReadOnlyMissing(t *testing.T) {
	ctx := context.Background()
	_, err := OpenReadOnly(ctx, "/nonexistent/path.db")
	if err == nil {
		t.Error("expected error for missing db")
	}
}

func TestSchemaVersionZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE adjusted_bars (symbol TEXT, timestamp INTEGER, open REAL, high REAL, low REAL, close REAL, volume INTEGER, vwap REAL, source TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	ctx := context.Background()
	st, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if st.SchemaVersion() != 0 {
		t.Errorf("SchemaVersion: got %d want 0 (no schema_version table)", st.SchemaVersion())
	}
	if err := st.CheckSchema(); err != nil {
		t.Errorf("CheckSchema with v0: unexpected error: %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
