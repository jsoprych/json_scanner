# Scanner Snapshot Schema

This document describes the scanner's internal snapshot schema - the SQLite
table that stores computed indicators for all symbols. The snapshot DB can be
in-memory (`:memory:`) or persistent (on-disk for backfill/backtest).

## Schema Overview

The snapshot table contains one row per (snapshot_date, symbol), with columns for
price data and 50+ computed technical indicators. All indicators are computed with
**no lookahead** - the value at bar T uses only data from bars < T.

The primary key is `(snapshot_date, symbol)`, enabling indexed lookups for
historical queries and backtesting.

## Current Schema

```sql
CREATE TABLE IF NOT EXISTS snapshot (
    snapshot_date INTEGER,
    symbol TEXT,
    timestamp INTEGER,
    
    -- Price data
    close REAL, high REAL, low REAL, open REAL,
    
    -- Trend: SMA
    sma5 REAL, sma10 REAL, sma20 REAL, sma30 REAL,
    sma50 REAL, sma100 REAL, sma200 REAL,
    
    -- Trend: EMA
    ema10 REAL, ema21 REAL, ema50 REAL, ema100 REAL, ema200 REAL,
    
    -- Trend: derived
    pct_from_sma50 REAL, pct_from_sma200 REAL,
    ma_stack INTEGER,  -- 1 if EMA10 > EMA21 > EMA50 > EMA200
    
    -- Momentum
    rsi14 REAL, macd REAL, macd_signal REAL, macd_hist REAL,
    stoch_k REAL, stoch_d REAL, willr14 REAL, cci20 REAL,
    roc10 REAL, roc20 REAL,
    adx14 REAL, di_plus REAL, di_minus REAL,
    
    -- Volatility
    atr14 REAL, atr_pct REAL,
    bb_upper REAL, bb_mid REAL, bb_lower REAL,
    bb_bandwidth REAL, bb_pct_b REAL,
    hist_vol20 REAL,
    
    -- Price structure
    high_52w REAL, low_52w REAL,
    is_52w_high INTEGER, is_52w_low INTEGER,
    gap_pct REAL, true_range REAL,
    pct_off_52w_high REAL, pct_above_52w_low REAL,
    
    -- Returns
    ret_1d REAL, ret_5d REAL, ret_1m REAL,
    ret_3m REAL, ret_6m REAL, ret_1y REAL,
    
    -- Volume
    dollar_vol REAL, avg_dollar_vol20 REAL,
    rel_volume REAL, obv REAL, vwap_dist REAL, mfi14 REAL,
    
    -- Cross detection (boolean flags)
    golden_cross INTEGER,      -- 1 if SMA50 crossed above SMA200
    oversold_bounce INTEGER,   -- 1 if RSI14 crossed above 30
    
    PRIMARY KEY (snapshot_date, symbol)
);
```

## Indexed Lookup Methods

The snapshot DB provides two parameterized lookup methods that use the PK index
for O(log N) performance — no full scans, no Go-side caching:

### `SymbolClose(symbol, date) → float64`

Returns the close price for one symbol on one snapshot date. Uses the PK index
`(snapshot_date, symbol)` directly.

```go
close, err := snap.SymbolClose("AAPL", date)
```

### `NearestDate(from) → int64`

Returns the earliest `snapshot_date >= from`. Used by the backtest engine to
find the actual exit date when the exact hold-period date doesn't exist (e.g.,
weekends, holidays).

```go
exitDate, err := snap.NearestDate(entryDate + int64(holdDays*86400))
```

Both methods use **parameterized queries** — no string concatenation, no SQL
injection risk.

## DDL Management

The CREATE TABLE statement is defined once as `createTableSQL` in
`internal/snapshot/snapshot.go`. All load methods (`Load`, `LoadHistory`,
`LoadHistoryBatch`, `LoadHistoryInsert`) call `ensureTable()` which executes
this constant. Schema changes require updating only this one constant.

## SQL Safety

The `ValidateClause` function in `internal/study/study.go` enforces a denylist
for user-authored SQL WHERE/ORDER BY clauses. Blocked keywords:

- Statement terminators: `;`
- Comments: `--`, `/*`, `*/`
- DDL: `create`, `drop`, `alter`
- DML: `insert`, `delete`, `update`, `replace`
- Dangerous: `attach`, `detach`, `pragma`, `load_extension`, `exec`,
  `trigger`, `savepoint`, `union`, `select`

Admin-authored studies bypass validation. The structured study editor
(`internal/predicate`) is injection-proof by construction — IDs only, never
raw user SQL.

## Column Descriptions

### Price Data
- **symbol**: Stock ticker symbol
- **snapshot_date**: Unix timestamp of the snapshot date (midnight UTC)
- **timestamp**: Unix timestamp of the bar
- **close/high/low/open**: Prices (split-adjusted)
- **dollar_vol**: Dollar volume (close × volume) — split-invariant

### Trend Indicators
- **sma{N}**: N-day simple moving average (computed from bars < T)
- **ema{N}**: N-day exponential moving average (computed from bars < T)
- **pct_from_sma{N}**: Percentage distance from SMA{N}
- **ma_stack**: Boolean — true if EMAs in perfect bullish order (10>21>50>200)

### Momentum Indicators
- **rsi14**: 14-day RSI (0-100)
- **macd/macd_signal/macd_hist**: MACD line, signal, histogram
- **stoch_k/stoch_d**: Stochastic oscillator (14,3,3)
- **willr14**: Williams %R (14-day)
- **cci20**: Commodity Channel Index (20-day)
- **roc{N}**: Rate of change over N days
- **adx14/di_plus/di_minus**: Average Directional Index system (14-day)

### Volatility Indicators
- **atr14/atr_pct**: Average True Range (14-day) and as % of price
- **bb_upper/bb_mid/bb_lower**: Bollinger Bands (20,2)
- **bb_bandwidth**: BB width as % of middle band
- **bb_pct_b**: Position within BB (0=lower, 1=upper)
- **hist_vol20**: 20-day annualized historical volatility

### Price Structure
- **high_52w/low_52w**: 52-week high/low (252 trading days)
- **is_52w_high/is_52w_low**: Boolean flags
- **gap_pct**: Overnight gap (open vs prev close)
- **true_range**: Max of H-L, |H-prevC|, |L-prevC|
- **pct_off_52w_high/pct_above_52w_low**: Distance from extremes

### Returns
- **ret_{period}**: Return over period (1d, 5d, 1m=21d, 3m=63d, 6m=126d, 1y=252d)

### Volume Indicators
- **avg_dollar_vol20**: 20-day SMA of dollar volume
- **rel_volume**: Volume / 20-day avg volume
- **obv**: On-balance volume
- **vwap_dist**: Distance from VWAP (20-day)
- **mfi14**: Money Flow Index (14-day, volume-weighted RSI)

### Cross Detection
- **golden_cross**: Boolean — SMA50 crossed above SMA200 on this bar
- **oversold_bounce**: Boolean — RSI14 crossed above 30 on this bar

## No Lookahead Principle

All indicators are computed with strict no-lookahead:
- Indicator at bar T uses only data from bars 0 to T-1
- No `prev_*` columns needed — indicators are already shifted by 1 bar
- Cross detection uses boolean flags computed during snapshot build

This ensures:
- No lookahead bias in backtests
- Indicators represent information available BEFORE the bar's close
- Accurate simulation of real trading conditions

## Upstream vs Scanner Schema

**Upstream (cetus.db):**
- Owned by the pipeline
- Contains raw OHLCV bars
- Read-only access from scanner
- Schema changes require coordination

**Scanner (snapshot):**
- Owned by the scanner
- Contains computed indicators
- In-memory or persistent SQLite
- Schema can evolve independently
- DDL defined in one place (`createTableSQL` constant)

## Migration Strategy

When expanding the schema:
1. Add new columns to the `createTableSQL` constant in `snapshot.go`
2. Add to the `columns` slice (insert order)
3. Update all `stmt.Exec(...)` calls in load methods
4. Update the `SnapshotRow` struct in `screen.go`
5. Update `screen.Build()` to compute the new indicator
6. Update the features catalog in `internal/features/`
7. Update this documentation
