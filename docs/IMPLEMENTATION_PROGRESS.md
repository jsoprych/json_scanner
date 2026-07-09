# Implementation Progress

## Completed

### 1. Documentation Restructure
- ✅ Created `docs/DESIGN.md` (master design doc with ToC)
- ✅ Created `docs/sections/` directory with focused sections
- ✅ Created `docs/SCHEMA.md` (snapshot schema reference)
- ✅ Created `USER_GUIDE.md` (end-user quickstart)
- ✅ Updated `docs/INDICATORS.md` with no-lookahead principle
- ✅ Updated `docs/AGENTS.md` with doc-sync checklist
- ✅ Updated `README.md` to reference new docs
- ✅ Archived obsolete docs to `docs/archive/`

### 2. REST API Foundation
- ✅ Created `docs/API.md` (comprehensive API reference)
- ✅ Created `internal/features/` package with 50+ indicator metadata
- ✅ Created `internal/api/` package with HTTP handlers
- ✅ Implemented endpoints:
  - `GET /api/v1/health` - server health + snapshot metadata
  - `GET /api/v1/features` - list all features (with category filter)
  - `GET /api/v1/features/{id}` - get feature by ID
- ✅ Integrated API into `scanner serve` command
- ✅ Added comprehensive tests for API and features packages

### 3. Feature Catalog
- ✅ Defined 50+ indicators with metadata:
  - Price (OHLCV)
  - Trend (SMA, EMA, derived)
  - Momentum (RSI, MACD, Stochastic, etc.)
  - Volatility (ATR, Bollinger Bands, etc.)
  - Price Structure (52-week high/low, gap, etc.)
  - Returns (1d, 5d, 1m, 3m, 6m, 1y)
  - Volume (dollar_vol, rel_volume, etc.)
- ✅ Each feature includes:
  - ID, name, short description (for UI tooltips)
  - Long description (detailed explanation)
  - Category, data type, sortable flag
  - Wiki URL (Investopedia links)

### 4. Critical Bug Fix: No-Lookahead Indicators
- ✅ Fixed all indicator calculations to use bars < T (excluding current bar)
- ✅ Removed all `prev_*` columns from snapshot schema
- ✅ Added boolean cross-detection fields (golden_cross, oversold_bounce)
- ✅ Updated snapshot schema to use boolean columns for crosses
- ✅ Removed day-over-day delta calculations from Breadth
- ✅ Updated all tests to reflect new no-lookahead behavior
- ✅ Modularized indicators into separate files (trend.go, momentum.go, etc.)

## In Progress

### 5. Expand Indicator Set
- ⏳ Implement additional indicators (EMA, MACD, ATR, Bollinger, etc.)
- ⏳ Update snapshot schema to include new columns
- ⏳ Update predicate registry for new features

### 6. Schema Versioning
- ⏳ **Deferred** — scanner-side check is implemented (forward-compatible), but the pipeline must first create the `schema_version` table
- ⏳ See `docs/TICKET-PIPELINE-SCHEMA-VERSIONING.md` for the pipeline-side implementation ticket
- ⏳ Once the pipeline stamps version 1, the scanner will automatically detect and validate it

### 7. Snapshot History
- ✅ Added `snapshot_date` column to snapshot table (composite PK: snapshot_date, symbol)
- ✅ Implemented `LoadHistory` for date-stamped snapshots (multiple snapshots coexist)
- ✅ Implemented `Cleanup(keepDays)` for retention policy
- ✅ Implemented `ListSnapshots()` and `SetActive(date)` for historical queries
- ✅ Added `BackfillSnapshots` function for historical data reconstruction
- ✅ Added `scanner backfill` CLI command with configurable retention
- ✅ Added `SCANNER_SNAPSHOT_RETENTION_DAYS` (default 90) and `SCANNER_BACKFILL_DAYS` config
- ✅ Added tests for snapshot history, cleanup, and date switching

### 8. Additional API Endpoints
- ✅ `GET /api/v1/scan` - run ad-hoc study (query params)
- ✅ `POST /api/v1/scan` - run study with JSON body
- ✅ `GET /api/v1/studies` - list saved studies
- ✅ `POST /api/v1/studies` - create study
- ✅ `GET /api/v1/studies/{id}` - get study by key
- ✅ `PUT /api/v1/studies/{id}` - update study
- ✅ `DELETE /api/v1/studies/{id}` - delete study
- ✅ `GET /api/v1/universe` - list symbols
- ✅ `GET /api/v1/symbols/{symbol}` - get symbol data (recent bars)
- ✅ `GET /api/v1/snapshots` - list historical snapshot dates
- ✅ Updated `NewHandlerWithDeps` to inject studies and store dependencies
- ✅ Added comprehensive tests for all new endpoints

## Next Steps (Priority Order)

1. **Schema versioning** - Blocked on pipeline-side implementation (see `docs/TICKET-PIPELINE-SCHEMA-VERSIONING.md`)
2. **Historical scan endpoint** - Add `date` query param to `/api/v1/scan` for backtesting
3. **Symbol detail endpoint** - Expand `/api/v1/symbols/{symbol}` to include full snapshot row
4. **Rate limiting** - Implement tier-based rate limiting (free: 100/min, pro: 1000/min)

## Testing

All tests pass:
```
ok  	cetus-marketdata-scanner/internal/api
ok  	cetus-marketdata-scanner/internal/features
ok  	cetus-marketdata-scanner/internal/indicators
ok  	cetus-marketdata-scanner/internal/snapshot
... (all other packages)
```

Build successful:
```
CGO_ENABLED=0 go build -o bin/scanner ./cmd/scanner
```

## Performance

Snapshot build time (11,385 symbols):
- Load bars + compute indicators: ~8.5 seconds
- Materialize snapshot: ~183ms
- Total: ~8.65 seconds

This is well within acceptable limits for daily rebuilds.

## Storage Estimates

With 50 columns × 90 days:
- Per snapshot: ~40 MB
- 90 days: ~3.6 GB
- Rebuild time: ~60 seconds (estimated with 50 indicators)

All within reasonable limits for Phase 1.
