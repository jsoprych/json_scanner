# Architecture Overview

## System Architecture

The scanner is a Go-based market data analysis system that reads from the cetus
pipeline's SQLite database and provides a REST API, HTTP dashboard, and CLI tools
for running studies, backtests, and generating digests.

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                    cetus.db (read-only)                      │
│              (upstream pipeline, OHLCV bars)                 │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/store                            │
│  (read-only warehouse reader)                                │
│  - barsPreference: published_bars > clean_bars > adjusted   │
│  - Universe resolution (index, exchange, file, common)       │
│  - Schema version checking                                   │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/indicators                        │
│  (50+ pure technical indicator functions)                    │
│  - No lookahead: indicator[T] uses bars < T                 │
│  - Modular: trend.go, momentum.go, volatility.go,           │
│             price.go, returns.go, volume.go                  │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/screen                           │
│  (SnapshotRow builder, preset studies, market breadth)       │
│  - Build(symbol, bars) → SnapshotRow with 50+ indicators    │
│  - NaN for under-warmed features (fails comparisons safely)  │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    internal/snapshot                         │
│  (SQLite table, PK: snapshot_date + symbol)                  │
│  - 50+ indicator columns                                     │
│  - Indexed lookups: SymbolClose, NearestDate                 │
│  - SQL-WHERE study execution                                 │
│  - DDL defined once (createTableSQL constant)                │
└─────────────────────────────────────────────────────────────┘
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
┌──────────────────┐ ┌──────────────┐ ┌──────────────────┐
│  internal/serve  │ │internal/api  │ │internal/backtest │
│  (HTTP server)   │ │(REST API)    │ │(historical)      │
│  - Dashboard     │ │- CRUD        │ │- Entry/exit      │
│  - Auth          │ │- Studies     │ │- Returns         │
│  - Study editor  │ │- Alerts      │ │- Win rate        │
│  - Admin console │ │- Backtest    │ │- Hold days       │
└──────────────────┘ └──────────────┘ └──────────────────┘
```

## Data Flow

1. **Data Ingestion**
   - Scanner reads OHLCV bars from cetus.db (read-only)
   - Loads ~400 days of history per symbol
   - Filters by universe (e.g., Russell 3000 via index_membership)

2. **Indicator Calculation**
   - For each symbol, compute 50+ indicators with no lookahead
   - Indicator at bar T uses only bars < T
   - NaN for under-warmed features (safe comparison semantics)

3. **Snapshot Materialization**
   - Insert computed indicators into SQLite (in-memory or persistent)
   - One row per (snapshot_date, symbol)
   - Boolean flags for cross detection (golden_cross, oversold_bounce)
   - PK index enables O(log N) lookups for backtest

4. **Study Execution**
   - Studies are SQL WHERE clauses (validated/sandboxed for non-admin users)
   - Structured study editor: IDs → SQL (injection-proof by construction)
   - Execute against snapshot table
   - Return matching symbols with indicator values

5. **Backtest**
   - For each entry date, find matches via study
   - Use `SymbolClose` (parameterized, indexed) for entry/exit prices
   - Use `NearestDate` (parameterized) for actual exit date
   - No Go-side caching — SQLite handles all lookups

6. **API/Dashboard Response**
   - Format results as JSON or HTML
   - Include metadata (symbol count, snapshot date, scan duration)
   - Return to client

## Key Design Decisions

### SQLite-First
All data storage, filtering, sorting, and lookups are delegated to SQLite.
Go handles math (indicator computation) and business logic. The snapshot DB
uses a PK index `(snapshot_date, symbol)` for O(log N) indexed lookups.
No Go-side maps or caches for data SQLite can provide efficiently.

### No Lookahead
All indicators exclude the current bar to prevent lookahead bias. This is
critical for accurate backtesting and real-world simulation.

### DDL Deduplication
The CREATE TABLE statement is defined once as `createTableSQL` constant.
All load methods call `ensureTable()`. Schema changes require updating
only this one constant.

### Boolean Cross Detection
Instead of storing prev_* columns, we compute boolean flags during snapshot build:
- `golden_cross`: SMA50 crossed above SMA200
- `oversold_bounce`: RSI14 crossed above 30

This simplifies the schema and makes cross queries trivial.

### Injection-Proof Study Editor
The structured study editor (`internal/predicate`) compiles user-friendly rule
definitions into SQL using a closed registry of feature/operator/operand IDs.
No raw user SQL ever reaches the database — invalid IDs are rejected before
any SQL is built. The compatibility matrix ensures only legal combinations
are constructible.

### Exe-Relative DB Paths
Default DB paths are resolved relative to the executable, not CWD.
This makes the binary work correctly regardless of the working directory.
Absolute paths from env vars pass through unchanged.

### Extracted Server Package
The HTTP server logic was extracted from `cmd/scanner/main.go` into
`internal/serve/` for maintainability. The main.go file is now a thin
dispatcher that delegates to subcommand implementations.

## Performance Characteristics

### Snapshot Build
- **Time**: ~8.5 seconds for 11,385 symbols
- **Memory**: ~3-5 MB for snapshot table (in-memory)
- **Disk**: Persistent mode supports backfill/backtest

### Query Execution
- **Time**: <100ms for typical studies
- **Throughput**: Thousands of queries per second
- **Scalability**: Limited by CPU, not I/O

### Backtest
- **SymbolClose**: O(log N) via PK index
- **NearestDate**: O(log N) via MIN() on indexed column
- **No Go-side caching**: SQLite handles all lookups

### Storage
- **Per snapshot**: ~40 MB (with 50+ indicators)
- **90-day retention**: ~3.6 GB
- **Rebuild time**: ~60 seconds (estimated)

## Package Responsibilities

| Package | Responsibility |
|---------|---------------|
| `internal/model` | Neutral types: Bar, Signal |
| `internal/store` | Read-only warehouse reader |
| `internal/scanner` | Pure Scan(symbol, bars, cfg) → []Signal |
| `internal/indicators` | 50+ pure indicator functions |
| `internal/screen` | SnapshotRow builder, presets, breadth |
| `internal/scan` | Concurrent universe scan, backfill |
| `internal/snapshot` | SQLite snapshot store, indexed lookups |
| `internal/study` | Studies as data, CRUD, validation |
| `internal/user` | User entity, tiers, roles, PBKDF2 |
| `internal/serve` | HTTP server, dashboard, auth, study editor |
| `internal/api` | REST API handlers |
| `internal/authjwt` | JWT verification (HS/RS, alg-safe) |
| `internal/digest` | Digest assembly + renderers |
| `internal/dashboard` | Admin ops-console renderer |
| `internal/backtest` | Historical backtesting engine |
| `internal/sentinel` | Data-quality Tier-0 flags |
| `internal/predicate` | Injection-proof study compiler |
| `internal/features` | Feature catalog with metadata |
| `internal/config` | Env-first configuration |
| `internal/telemetry` | Structured JSON logger |
| `cmd/scanner` | Entrypoint, subcommand dispatch |
