# Architecture Overview

## System Architecture

The scanner is a Go-based market data analysis system that reads from the cetus
pipeline's SQLite database and provides a REST API for running studies.

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                    cetus.db (read-only)                      │
│              (upstream pipeline, OHLCV bars)                 │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/indicators                        │
│  (SMA, RSI, RollingHigh, RollingLow, Return, etc.)          │
│  - No lookahead: indicator[T] uses bars < T                 │
│  - Modular: trend.go, momentum.go, price.go, returns.go     │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/snapshot                         │
│  (in-memory SQLite table, one row per symbol)               │
│  - Columns: symbol, timestamp, close, high, low, etc.       │
│  - Boolean flags: golden_cross, oversold_bounce             │
│  - Rebuilt daily from latest data                           │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      internal/api                            │
│  (REST API endpoints)                                       │
│  - GET /api/v1/health                                       │
│  - GET /api/v1/features                                     │
│  - GET /api/v1/features/{id}                                │
│  - (planned) GET /api/v1/scan                               │
│  - (planned) GET /api/v1/studies                            │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/dashboard                         │
│  (web UI, consumes API)                                     │
│  - Study editor                                             │
│  - Results display                                          │
│  - User management                                          │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

1. **Data Ingestion**
   - Scanner reads OHLCV bars from cetus.db (read-only)
   - Loads ~400 days of history per symbol
   - Filters by universe (e.g., Russell 3000)

2. **Indicator Calculation**
   - For each symbol, compute indicators with no lookahead
   - Indicator at bar T uses only bars < T
   - Store only the latest values (no history)

3. **Snapshot Materialization**
   - Insert computed indicators into in-memory SQLite
   - One row per symbol
   - Boolean flags for cross detection

4. **Study Execution**
   - Studies are SQL WHERE clauses
   - Execute against snapshot table
   - Return matching symbols with indicator values

5. **API Response**
   - Format results as JSON
   - Include metadata (symbol count, snapshot date)
   - Return to client

## Key Design Decisions

### No Lookahead
All indicators exclude the current bar to prevent lookahead bias. This is critical
for accurate backtesting and real-world simulation.

### In-Memory Snapshot
The snapshot is rebuilt daily and held in memory. This provides:
- Fast query execution (milliseconds)
- Simple SQL-based study language
- No need for complex query optimization

### Boolean Cross Detection
Instead of storing prev_* columns, we compute boolean flags during snapshot build:
- `golden_cross`: SMA50 crossed above SMA200
- `oversold_bounce`: RSI14 crossed above 30

This simplifies the schema and makes cross queries trivial.

### Modular Indicators
Indicators are organized by category in separate files:
- `trend.go`: SMA, EMA (planned)
- `momentum.go`: RSI, MACD (planned)
- `price.go`: RollingHigh, RollingLow
- `returns.go`: Return

This makes it easy to add new indicators and maintain the codebase.

## Performance Characteristics

### Snapshot Build
- **Time**: ~8.5 seconds for 11,385 symbols
- **Memory**: ~3-5 MB for snapshot table
- **Disk**: Minimal (in-memory SQLite)

### Query Execution
- **Time**: <100ms for typical studies
- **Throughput**: Thousands of queries per second
- **Scalability**: Limited by CPU, not I/O

### Storage
- **Per snapshot**: ~40 MB (with 50 indicators)
- **90-day retention**: ~3.6 GB
- **Rebuild time**: ~60 seconds (estimated)

## Future Enhancements

### Phase 2
- Expand to 50+ indicators
- Add snapshot history (90-day retention)
- Implement schema versioning
- Add backfill capability

### Phase 3
- Full REST API (scan, studies, symbols)
- User authentication and authorization
- Study sharing and collaboration
- Real-time alerts

### Phase 4
- Backtesting engine
- Strategy optimization
- Portfolio analysis
- Integration with trading platforms
