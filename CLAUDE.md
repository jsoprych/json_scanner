# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repo.

## Status

**Mature application.** Full-featured market-data scanner with: read-only reader of
the cetus warehouse (`internal/store`), pure scan logic (`internal/scanner`), 50+
technical indicators (`internal/indicators`), cross-sectional snapshot with SQL-
driven studies (`internal/snapshot`), HTTP dashboard with auth and study editor
(`internal/serve`), historical backfill/backtest (`internal/backtest`), data-quality
sentinel (`internal/sentinel`), and REST API (`internal/api`).

## What this is

`cetus-marketdata-scanner` is a **downstream consumer** of the
[`cetus-marketdata-pipeline`](https://github.com/jsoprych/cetus-marketdata-pipeline)
warehouse. The pipeline's *only* deliverable is a SQLite database of split-adjusted
EOD bars; this project **reads that database (read-only) and detects
breakout/anomaly signals**. It writes nothing back to the warehouse.

```
cetus-marketdata-pipeline  ──(SQLite: published_bars)──►  cetus-marketdata-scanner
        (ingestion, upstream)                                (scan, this repo)
```

## The read contract (READ THIS FIRST)

The warehouse schema is an external contract owned by the pipeline. **Do not guess
it — read the data dictionary:**

- **Local (siblings on disk):** [`../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`](../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md)
- **GitHub (private):** https://github.com/jsoprych/cetus-marketdata-pipeline → `docs/DATA_DICTIONARY.md`

Also see the consumer quickstart:
[`../cetus-marketdata-pipeline/docs/DOWNSTREAM.md`](../cetus-marketdata-pipeline/docs/DOWNSTREAM.md).

Non-negotiables from that contract:
- **Open the DB read-only** (`file:cetus.db?mode=ro`). WAL lets us read while an
  ingestion run is in progress; never open read-write.
- **Read the best clean surface** — `published_bars` (materialized, split-adjusted,
  quarantine-free), else the `clean_bars` view, else `adjusted_bars`. `store` picks
  this automatically (`barsPreference`). Never do split or quality math client-side.
- **Scope with `index_membership` + `security_type`** — default to the Russell 3000
  common stock (`index:r3000`); an unseeded index falls back to `common`
  (`security_type='common'` drops warrants/units/rights/ETFs). This is scope-at-read
  (cetus ingests the full universe).
- **Timestamps are Unix SECONDS** (UTC), except `logs.ts` (nanoseconds).
- **`volume` is feed-limited** — on the free IEX feed it's a single-digit-% fraction
  of consolidated volume. Prices are trustworthy; volume is not (yet). Any
  `price × volume` metric is split-invariant and fine.
- **"Has data" = `symbol_pipeline_state.status='SUCCESS'`** (combined with the scope
  above); `EMPTY` = no data on this feed (skip), `FAILED` = a real upstream error.
- **Alpaca symbol naming is canonical** — pass symbols through as stored; translate
  only at an external boundary (e.g. Yahoo dot→dash), which the scanner doesn't cross.

## Architecture

- `internal/model` — neutral types: `Bar` (split-adjusted), `Signal`.
- `internal/store` — **read-only** reader of the cetus DB: `Universe`,
  `LoadAdjustedBars` (from the `adjusted_bars` view).
- `internal/scanner` — **pure** `Scan(symbol, bars, cfg) []Signal`. No I/O, no
  state — the seam for a future strategy/back-test engine. Thresholds are
  data-driven (config), never hardcoded.
- `internal/indicators` — 50+ pure technical indicator functions (SMA, EMA, RSI,
  MACD, Bollinger Bands, ATR, etc.) with strict no-lookahead.
- `internal/screen` — builds `SnapshotRow` from bars, preset studies, market breadth.
- `internal/scan` — concurrent whole-universe snapshot with worker pool and
  `BarLoader` interface for testability.
- `internal/snapshot` — scanner's own SQLite store: materializes cross-sectional
  snapshot, runs SQL-WHERE studies, provides indexed lookups (`SymbolClose`,
  `NearestDate`) for backtest performance.
- `internal/study` — studies as data: SQL-WHERE clauses in JSONL, owner + tier +
  visibility, file-backed store with CRUD.
- `internal/user` — user entity with tiers (free/pro), roles (user/admin),
  PBKDF2 password hashing, file-backed store.
- `internal/serve` — HTTP server: dashboard, session auth, study editor with
  live preview, admin console, REST API mounting. Extracted from `cmd/scanner`
  for maintainability.
- `internal/api` — REST API handlers: health, features catalog, studies CRUD,
  subscriptions, alerts, backtest, universe/symbols, auth.
- `internal/authjwt` — stdlib JWT verifier (HS/RS, alg-confusion-safe) for
  proxy auth mode.
- `internal/digest` — digest assembly + html/text/json renderers.
- `internal/dashboard` — admin ops-console HTML renderer.
- `internal/backtest` — historical backtesting engine with parameterized queries
  (SQLite-first, indexed lookups).
- `internal/sentinel` — data-quality Tier-0 flags (deterministic; AI tiers extend it).
- `internal/predicate` — structured study compiler: IDs → SQL, injection-proof.
- `internal/features` — feature catalog with metadata for 50+ indicators.
- `internal/config` — env-first configuration (`SCANNER_*`), exe-relative DB path resolution.
- `internal/telemetry` — structured JSON logger to **stderr** (so stdout carries
  the signal stream cleanly).
- `cmd/scanner` — load config → dispatch subcommands (scan, digest, serve,
  anomalies, studies, users, backfill).

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
