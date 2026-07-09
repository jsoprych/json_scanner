# Scanner Snapshot Schema

This document describes the scanner's internal snapshot schema - the in-memory SQLite
table that stores computed indicators for all symbols.

## Schema Overview

The snapshot table contains one row per symbol, with columns for price data and
computed technical indicators. All indicators are computed with **no lookahead** -
the value at bar T uses only data from bars < T.

## Current Schema (Phase 1)

```sql
CREATE TABLE snapshot (
    symbol TEXT PRIMARY KEY,
    timestamp INTEGER,
    
    -- Price data
    close REAL,
    high REAL,
    low REAL,
    dollar_vol REAL,
    
    -- Trend indicators
    sma50 REAL,
    sma200 REAL,
    
    -- Momentum indicators
    rsi14 REAL,
    
    -- Returns
    ret_3m REAL,
    
    -- Price structure
    high_52w REAL,
    low_52w REAL,
    
    -- Cross detection (boolean flags)
    golden_cross INTEGER,      -- 1 if SMA50 crossed above SMA200
    oversold_bounce INTEGER    -- 1 if RSI14 crossed above 30
);
```

## Column Descriptions

### Price Data
- **symbol**: Stock ticker symbol
- **timestamp**: Unix timestamp of the bar
- **close**: Closing price (split-adjusted)
- **high**: High price (split-adjusted)
- **low**: Low price (split-adjusted)
- **dollar_vol**: Dollar volume (close × volume)

### Trend Indicators
- **sma50**: 50-day simple moving average (computed from bars < T)
- **sma200**: 200-day simple moving average (computed from bars < T)

### Momentum Indicators
- **rsi14**: 14-day RSI (computed from bars < T)

### Returns
- **ret_3m**: 3-month return (computed from bars < T)

### Price Structure
- **high_52w**: 52-week high (computed from bars < T)
- **low_52w**: 52-week low (computed from bars < T)

### Cross Detection
- **golden_cross**: Boolean flag (1/0) indicating if SMA50 crossed above SMA200
- **oversold_bounce**: Boolean flag (1/0) indicating if RSI14 crossed above 30

## No Lookahead Principle

All indicators are computed with strict no-lookahead:
- Indicator at bar T uses only data from bars 0 to T-1
- No `prev_*` columns needed - indicators are already shifted by 1 bar
- Cross detection uses boolean flags computed during snapshot build

This ensures:
- No lookahead bias in backtests
- Indicators represent information available BEFORE the bar's close
- Accurate simulation of real trading conditions

## Future Schema Expansions

### Phase 2 (Planned)
Additional indicators will be added following the same no-lookahead principle:
- EMA variants (10, 21, 50, 100, 200)
- MACD (line, signal, histogram)
- ATR (14-day)
- Bollinger Bands (upper, middle, lower, bandwidth, %B)
- Additional moving averages (5, 10, 20, 30, 100)
- Additional momentum indicators (Stochastic, Williams %R, CCI, etc.)
- Additional returns (1d, 5d, 1m, 6m, 1y)
- Volume indicators (relative volume, avg dollar volume)

### Schema Versioning
The schema will include version tracking to detect breaking changes:
```sql
CREATE TABLE schema_version (
    version TEXT PRIMARY KEY,
    updated_at INTEGER
);
```

## Upstream vs Scanner Schema

**Upstream (cetus.db):**
- Owned by the pipeline
- Contains raw OHLCV bars
- Read-only access from scanner
- Schema changes require coordination

**Scanner (snapshot):**
- Owned by the scanner
- Contains computed indicators
- In-memory SQLite (rebuilt daily)
- Schema can evolve independently

## Migration Strategy

When expanding the schema:
1. Add new columns to the CREATE TABLE statement
2. Update the INSERT statement in snapshot.go
3. Update the features catalog in internal/features/
4. Update this documentation
5. Bump schema version if breaking changes occur
