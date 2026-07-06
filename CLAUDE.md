# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repo.

## Status

**First stab / scaffold.** Buildable skeleton: read-only reader of the cetus
warehouse (`internal/store`), pure scan logic (`internal/scanner`), env-first
config, JSON logging, and a `cmd/scanner` entrypoint that emits signals as JSONL.
The scan rules (volume/price/gap breakouts) are a starting point to iterate on —
not a finished strategy library.

## What this is

`cetus-marketdata-scanner` is a **downstream consumer** of the
[`cetus-marketdata-pipeline`](https://github.com/jsoprych/cetus-marketdata-pipeline)
warehouse. The pipeline's *only* deliverable is a SQLite database of split-adjusted
EOD bars; this project **reads that database (read-only) and detects
breakout/anomaly signals**. It writes nothing back to the warehouse.

```
cetus-marketdata-pipeline  ──(SQLite: adjusted_bars)──►  cetus-marketdata-scanner
        (ingestion, upstream)                                (scan, this repo)
```

## The read contract (READ THIS FIRST)

The warehouse schema is an external contract owned by the pipeline. **Do not guess
it — read the data dictionary:**

- **Local (siblings on disk):** [`../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`](../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md)
- **GitHub (private):** https://github.com/jsoprych/cetus-marketdata-pipeline → `docs/DATA_DICTIONARY.md`

Non-negotiables from that contract:
- **Open the DB read-only** (`file:cetus.db?mode=ro`). WAL lets us read while an
  ingestion run is in progress; never open read-write.
- **Read the `adjusted_bars` view** for split-adjusted OHLCV — never do split math
  client-side, and never read `adjustment_index` directly (missing row = factor 1.0).
- **Timestamps are Unix SECONDS** (UTC), except `logs.ts` (nanoseconds).
- **`volume` is feed-limited** — on the free IEX feed it's a single-digit-% fraction
  of consolidated volume. Prices are trustworthy; volume is not (yet). Any
  `price × volume` metric is split-invariant and fine.
- **Universe = symbols with data.** Scan `symbol_pipeline_state.status = 'SUCCESS'`;
  `EMPTY` = no data on this feed (skip), `FAILED` = a real upstream error.

## Architecture

- `internal/model` — neutral types: `Bar` (split-adjusted), `Signal`.
- `internal/store` — **read-only** reader of the cetus DB: `Universe`,
  `LoadAdjustedBars` (from the `adjusted_bars` view).
- `internal/scanner` — **pure** `Scan(symbol, bars, cfg) []Signal`. No I/O, no
  state — the seam for a future strategy/back-test engine. Thresholds are
  data-driven (config), never hardcoded.
- `internal/config` — env-first configuration (`SCANNER_*`).
- `internal/telemetry` — structured JSON logger to **stderr** (so stdout carries
  the signal stream cleanly).
- `cmd/scanner` — load config → open DB read-only → for each symbol load the
  window, `Scan`, emit signals as JSONL to stdout.

## Ethos (inherited from `../../ETHOS.md`)

Go-first, pure Go / zero CGO, single static binary, minimal dependencies (only
`modernc.org/sqlite`, matching the pipeline's driver), structured JSON logging,
`context.Context` on every I/O path, graceful `SIGINT`/`SIGTERM`. Config is
env-first (may adopt the pipeline's single-source-of-truth `settings` table
pattern as it grows). Scan functions stay **pure and data-driven** — tuning never
touches logic.

## Commands

```bash
make build    # CGO_ENABLED=0 static binary → bin/scanner
make run      # go run ./cmd/scanner
make test     # go test ./...
make vet      # go vet ./...
```

Config is env-only; see `README.md`. Key knobs: `SCANNER_DB_PATH` (the cetus DB),
`SCANNER_LOOKBACK`, `SCANNER_VOLUME_MULT`, `SCANNER_GAP_PCT`, `SCANNER_SINCE_DAYS`.
