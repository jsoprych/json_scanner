# Scanner Design Document

This is the master design document for the cetus-marketdata-scanner project.

## Table of Contents

1. [Architecture Overview](sections/01-architecture.md)
2. [Indicator Catalog](INDICATORS.md)
3. [Study Editor](ELKO_SCANNER_STUDY_EDITOR_MVP_DESIGN.md)
4. [Runtime Lifecycle](sections/04-runtime-lifecycle.md)
5. [Deployment](DEPLOY-cloudflare.md)
6. [Future Roadmap](sections/06-future-roadmap.md)
7. [Schema Reference](SCHEMA.md)
8. [API Reference](API.md)
9. [Feature Catalog](FEATURE_CATALOG.md)

## Core Principles

### 1. No Lookahead
All indicators are computed with strict no-lookahead: the value at bar T uses only
data from bars < T. This prevents lookahead bias and ensures accurate backtests.

### 2. SQLite-First
Filtering, sorting, aggregation, and lookups are delegated to SQLite. The snapshot
DB uses a PK index `(snapshot_date, symbol)` for O(log N) indexed lookups
(`SymbolClose`, `NearestDate`). Go handles math and business logic; SQLite handles
data storage and retrieval. No Go-side caching of data SQLite can provide.

### 3. Cross-Sectional Snapshot
The scanner maintains a snapshot of all symbols with computed indicators.
Studies are simple SQL WHERE clauses over this snapshot. The snapshot can be
in-memory (ephemeral) or persistent (for backfill/backtest).

### 4. Modular Indicators
50+ indicators organized by category (trend, momentum, volatility, price structure,
returns, volume) in separate files for maintainability and scalability.

### 5. REST API First
The scanner exposes a REST API for programmatic access, with the web dashboard as
one consumer of the API.

### 6. Injection-Proof Study Editor
The structured study editor (`internal/predicate`) compiles user-friendly rule
definitions into SQL using a closed registry of feature/operator/operand IDs.
No raw user SQL ever reaches the database — invalid IDs are rejected before any
SQL is built.

## Current Status

See [IMPLEMENTATION_PROGRESS.md](IMPLEMENTATION_PROGRESS.md) for detailed status.

### Completed
- ✅ 50+ technical indicators with strict no-lookahead
- ✅ Cross-sectional snapshot with SQL-WHERE studies
- ✅ Historical snapshot backfill with retention cleanup
- ✅ Backtest engine with parameterized queries (SQLite-first)
- ✅ HTTP dashboard with session auth, study editor, admin console
- ✅ REST API with full CRUD for studies, subscriptions, alerts, backtest
- ✅ Structured study editor (injection-proof predicate compiler)
- ✅ Data-quality sentinel (Tier-0 deterministic flags)
- ✅ PBKDF2 password hashing, JWT verification (HS/RS, alg-confusion-safe)
- ✅ Exe-relative DB path resolution
- ✅ Comprehensive test coverage (scan, store, config, indicators, snapshot, etc.)
- ✅ Extracted `internal/serve` package from `cmd/scanner` for maintainability

## Quick Start

```bash
# Build
make build

# Run tests
make test

# Start server
./bin/scanner serve

# Access API
curl http://localhost:8080/api/v1/features
```

## Documentation Structure

- **DESIGN.md** (this file) - Master design document
- **SCHEMA.md** - Snapshot schema reference
- **INDICATORS.md** - Indicator catalog and definitions
- **API.md** - REST API reference
- **FEATURE_CATALOG.md** - Feature metadata
- **IMPLEMENTATION_PROGRESS.md** - Implementation status
- **sections/** - Detailed design sections
- **archive/** - Superseded documentation
