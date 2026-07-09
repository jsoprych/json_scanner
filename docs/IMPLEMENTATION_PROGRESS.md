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
- ⏳ Add `schema_version` table to cetus.db (pipeline side)
- ⏳ Add version check to scanner startup
- ⏳ Document versioning policy in SCHEMA.md

### 7. Snapshot History
- ⏳ Add `snapshot_date` column to snapshot table
- ⏳ Implement snapshot retention (last 90 days)
- ⏳ Add cleanup job for old snapshots
- ⏳ Implement `BackfillSnapshots(days)` function

### 8. Additional API Endpoints
- ⏳ `GET /api/v1/scan` - run ad-hoc study
- ⏳ `POST /api/v1/scan` - run study with JSON body
- ⏳ `GET /api/v1/studies` - list saved studies
- ⏳ `POST /api/v1/studies` - create study
- ⏳ `GET /api/v1/studies/{id}` - get study
- ⏳ `PUT /api/v1/studies/{id}` - update study
- ⏳ `DELETE /api/v1/studies/{id}` - delete study
- ⏳ `GET /api/v1/universe` - list symbols
- ⏳ `GET /api/v1/symbols/{symbol}` - get symbol data
- ⏳ `GET /api/v1/snapshots` - list historical snapshots

## Next Steps (Priority Order)

1. **Expand indicator set** - Implement the additional indicators (EMA, MACD, ATR, Bollinger, etc.)
2. **Add schema versioning** - Prevent breaking changes from upstream
3. **Implement snapshot history** - Enable backtesting and audit trail
4. **Complete API endpoints** - Full REST API for programmatic access

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
