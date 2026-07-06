# Phase 1 (MVP) — Daily Post-Close Signal Digest

The first shippable slice of `cetus-marketdata-scanner`: a **standalone** job that
reads the cetus warehouse after the close, scans the whole universe against a small
fixed set of popular screens, and produces a **daily digest** (HTML + plaintext) —
the free-tier email that captures signups.

Scope discipline: build *only* what the digest needs. Independent of
`trading-engine-go` (see [`SCANNER_DESIGN.md`](SCANNER_DESIGN.md) §7). Delivery
(sending the email) is out of scope — the scanner *produces* the digest; existing
elko mail rails send it.

---

## 1. What ships

`scanner digest` → a rendered digest for the latest trading day:

- a **market-breadth** block (the shareable intelligence hook),
- **4 preset sections**, each a short ranked list of matching symbols,
- rendered to **HTML** (email) and **plaintext**, plus the underlying **JSON**
  (so the mail layer / API can reuse it).

Existing `scanner` (JSONL signal stream) is untouched; `digest` is a new subcommand.

## 2. The digest content (fixed for MVP)

| Block | Rule (fixed params) | Rank / cap |
|---|---|---|
| **Market breadth** | `% of universe with close > sma200` and `> sma50`; count `is_52w_high` vs `is_52w_low`; Δ vs prior day | — |
| 📈 **New 52-week highs** | `is_52w_high` | by `dollar_vol`, top 8 |
| ⚡ **Golden cross today** | `sma50 > sma200 AND prev_sma50 <= prev_sma200` | by `dollar_vol`, top 8 |
| 🔄 **Oversold bounce** | `rsi14 > 30 AND prev_rsi14 <= 30` | by `dollar_vol`, top 8 |
| 🚀 **Momentum leaders** | (no filter — cross-sectional) | by `ret_3m` desc, top 10 |

Universe filter for the digest: `symbol_pipeline_state.status='SUCCESS'`, and (to
avoid feed-thin noise) `avg_dollar_vol` above a config floor. Volume-based ranking
uses `dollar_vol`/`rel_volume` only — the ⚠️ feed-safe metrics, never absolute
volume (Data Dictionary §5; [`INDICATORS.md`](INDICATORS.md)).

## 3. The indicator subset (all we build in Phase 1)

Exactly the columns the blocks above reference — **7 features**, not the full catalog:

| Feature | Definition | Used by |
|---|---|---|
| `sma50` `sma200` (+ `prev_`) | SMA of close | golden cross, breadth |
| `rsi14` (+ `prev_`) | Wilder RSI(14) | oversold bounce |
| `high_52w` `low_52w` | rolling 252-bar high/low → `is_52w_high/low` | new highs, breadth |
| `ret_3m` | `close / close[T−63] − 1` | momentum leaders |
| `dollar_vol` | `close × volume` | ranking, liquidity floor |
| `close` | latest close | display, breadth |

All **right-aligned, 1-bar setback** (value at `T` uses bars `≤ T`; `prev_` = value
at `T−1`). Warm-up rows are `NULL` and simply don't match.

## 4. Architecture (one streaming pass)

```
cetus.db (RO)
  └─ for each SUCCESS symbol: LoadAdjustedBars(since ~ 400 bars back)
        └─ internal/indicators: compute the 7 features on the series
              └─ keep only the latest row (+ prev) → one SnapshotRow
  → snapshot: []SnapshotRow (whole universe, ~10k rows in RAM)
       ├─ internal/screen: breadth aggregate + 4 preset filters/ranks
       └─ internal/digest: assemble → render HTML / text / JSON
```

`SinceDays` must cover the longest window (252-bar 52-wk high + slack) — default
lookback ~400 calendar days. No full-history retention; only latest+prev per symbol.

## 5. Packages (new in Phase 1)

```
internal/indicators   NEW  pure funcs: SMA, RSI(Wilder), rollingHigh/Low, ret, dollarVol (+ tests)
internal/screen       NEW  SnapshotRow, Build(bars)->row, breadth aggregate, the 4 presets + ranking
internal/digest       NEW  Digest struct + html/template + text renderer
cmd/scanner           EXT  add `digest` subcommand (existing default = JSONL scan)
```

Deps unchanged: `modernc.org/sqlite` only; `html/template`/`text/template` are stdlib.

## 6. Config additions (env-first)

| Env | Default | Meaning |
|---|---|---|
| `SCANNER_DIGEST_LOOKBACK_DAYS` | 400 | history window loaded per symbol |
| `SCANNER_MIN_DOLLAR_VOL` | 1e6 | liquidity floor for the digest universe |
| `SCANNER_DIGEST_TOP_N` | 8 | rows per section (momentum = 10) |
| `SCANNER_DIGEST_OUT` | (stdout) | file path for the rendered digest |

## 7. Definition of done

- `scanner digest` runs against a real cetus.db and writes an HTML digest with a
  breadth block + 4 populated sections.
- `internal/indicators` has table-tests with hand-checked values (SMA, RSI, rolling
  high, return) — numbers are trusted before anything renders.
- No writes to cetus; logs to stderr; digest to stdout/file.
- Sending the email is a later, separate step (elko mail).

## 8. Explicitly deferred to later phases

Full indicator catalog · custom params · saved screens & alerts · SQLite snapshot
persistence · backtest-the-screen · email delivery · real-time/intraday.
