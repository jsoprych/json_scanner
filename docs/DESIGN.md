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

### 2. Cross-Sectional Snapshot
The scanner maintains an in-memory snapshot of all symbols with computed indicators.
Studies are simple SQL WHERE clauses over this snapshot.

### 3. Modular Indicators
Indicators are organized by category (trend, momentum, volatility, etc.) in separate
files for maintainability and scalability.

### 4. REST API First
The scanner exposes a REST API for programmatic access, with the web dashboard as
one consumer of the API.

## Current Status

See [IMPLEMENTATION_PROGRESS.md](IMPLEMENTATION_PROGRESS.md) for detailed status.

### Completed
- ✅ REST API foundation with feature catalog
- ✅ No-lookahead indicator calculations
- ✅ Modular indicator organization
- ✅ Boolean cross-detection (golden_cross, oversold_bounce)
- ✅ Documentation restructure

### In Progress
- ⏳ Expand indicator set to 50+ indicators
- ⏳ Schema versioning
- ⏳ Snapshot history (90-day retention)
- ⏳ Additional API endpoints

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
