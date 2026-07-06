# AGENTS.md — engineering directives (cetus-marketdata-scanner)

Directives for AI/engineers building this project. Complements the root
`CLAUDE.md`. The upstream pipeline's `docs/AGENTS.md` governs the *warehouse*; this
file governs the *scanner* that reads it.

## 1. Role & boundary

- This is a **read-only downstream consumer** of the cetus warehouse. It **never
  writes** to the pipeline's database.
- The warehouse schema is a **contract you do not own**. The source of truth for
  how to read it is `../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`
  (GitHub: `jsoprych/cetus-marketdata-pipeline`). When in doubt, read it; do not
  reverse-engineer the schema.
- Scanning is **analytics on top of the warehouse** — the pipeline deliberately
  excludes it. Keep that separation: no ingestion, no Alpaca calls, no split math
  here (the `adjusted_bars` view already did it).

## 2. Stack constraints

- **Go 1.25**, idiomatic, concurrent where it helps. **Pure Go, zero CGO**, single
  static binary.
- **Dependencies:** only `modernc.org/sqlite` (same pure-Go driver as the
  pipeline, so bars round-trip identically). Justify any new dependency in the PR.
- **stdlib first** — `database/sql`, `encoding/json`, `log/slog`. No frameworks.
- Every DB call and worker loop takes and honors a `context.Context`; handle
  `SIGINT`/`SIGTERM` for a clean drain.

## 3. Reading the warehouse

- **Open read-only:** `sql.Open("sqlite", "file:"+path+"?mode=ro")` + `PRAGMA
  busy_timeout=5000`. WAL permits concurrent reads alongside a live ingestion run.
  Never open read-write; never run schema DDL.
- **Read `adjusted_bars`** for split-adjusted OHLCV. Do not touch
  `adjustment_index` directly (sparse — missing row means factor 1.0).
- **Universe:** `SELECT symbol FROM symbol_pipeline_state WHERE status='SUCCESS'`.
  Treat `EMPTY` as "no data on this feed" (skip); it's the SIP/Yahoo backfill set.
- **Volume caveat:** free-tier IEX volume is a small fraction of consolidated
  volume. Prefer price-based and `price×volume` (split-invariant) signals; treat
  pure-volume-magnitude thresholds as provisional until a SIP/Yahoo source exists.

## 4. Scanner design

- `scanner.Scan(symbol string, bars []model.Bar, cfg scanner.Config) []model.Signal`
  is a **pure function**: deterministic, no I/O, no globals. This is the unit that
  a back-test engine will call bar-by-bar; keep it that way.
- **Thresholds are data (config), not code.** Volume multiple, gap %, lookback —
  all configurable; the detection *logic* never branches on tuning constants baked
  in source.
- Bars arrive **ascending by timestamp**. Guard against short windows.

## 5. Output & logging

- **Signals → stdout as JSONL** (one `Signal` object per line) so the stream pipes
  cleanly into `jq`, a file, or a downstream sink.
- **Logs → stderr**, structured JSON via `slog`. Never interleave logs into the
  signal stream on stdout.

## 6. Testing

- `scanner.Scan` is pure → table-driven unit tests with synthetic bars (no DB).
- `store` tests open a throwaway SQLite file, create the minimal `adjusted_bars`
  shape (or attach a fixture), and verify reads. Don't depend on a live warehouse
  in unit tests.
