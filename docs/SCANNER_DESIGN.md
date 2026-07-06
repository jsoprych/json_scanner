# cetus-marketdata-scanner — System Design

How the scanner turns the cetus warehouse into a configurable, backtestable stock
screener — and how it beats the free-tier scanners it competes with.

**Direction (2026-07):** the scanner is a **standalone, scaled-back screener** for
now. It does **not** depend on `trading-engine-go` (TradeGrid / Cursor /
TradeScript). It stays *compatible in vocabulary* — same indicator names, same
cross semantics — so it can converge later without a rewrite, but it takes no code
dependency today. See §7.

Companion: [`INDICATORS.md`](INDICATORS.md) (the canonical feature set) and the
upstream [`DATA_DICTIONARY.md`](../../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md).

---

## 1. Competitive frame — what we're beating

The free tiers of the popular scanners share the same walls. We aim at the gaps.

| | Finviz free | TradingView free | **us** |
|---|---|---|---|
| Data | 15-min delayed | 15-min delayed | EOD, split-correct, provenance-tagged |
| Universe | ~8,500 curated | large, curated | **whole tape** (Alpaca registry, `EMPTY`-aware), global-bound |
| Saved screens | none | 1 | **unlimited** |
| Filter combos | presets | 150+ filters | **arbitrary predicate / SQL** |
| Cross-sectional rank | limited | limited | **first-class** (breadth, top-N, percentiles) |
| Backtest a screen | Elite only ($39.50/mo) | no | **roadmap** (paid tier, later) |
| Access | web | web/mobile | **API-first, self-hosted, no rate limit** |

**What we beat them on now:** whole-tape universe, honest per-indicator data-quality
flags (the IEX volume caveat surfaced, not hidden), unlimited/arbitrary filters, and
**cross-sectional ranking studies** (breadth, relative strength) that free tiers gate
or lack. **Backtest-the-screen** is the eventual differentiator — deferred to the
paid tier (§7), not built now.

---

## 2. Data ownership — read cetus, write nothing upstream

Non-negotiable: **`cetus.db` is read-only, owned by the pipeline** (Data Dictionary
§1). We read the `adjusted_bars` view and `symbol_pipeline_state`; we never write to
cetus. Any persistence we add is our own, rebuildable sidecar.

---

## 3. Architecture — one lean pass

The scaled-back scanner is a single flow. No full-history in-RAM cube, no engine.

```
cetus.db (RO)
    │  per symbol (status='SUCCESS'), stream adjusted_bars ascending
    ▼
┌────────────────────┐   compute the ample fixed indicator set (INDICATORS.md),
│ internal/indicators│   right-aligned, 1-bar setback (prev_* mirror).
└────────────────────┘   Keep only the LATEST row (+ its prev) per symbol.
    │
    ▼
┌────────────────────┐   cross-sectional SNAPSHOT: one row per symbol,
│  snapshot table    │   columns = indicators. ~10k rows × ~40 cols ≈ 3 MB, in RAM.
│  (in-memory)       │   This is the whole scan surface.
└────────────────────┘
    │
    ▼
┌────────────────────┐   filter (WHERE-style predicates) + rank (ORDER/percentile)
│ internal/screen    │   → matching symbols → JSONL on stdout.
└────────────────────┘
```

**Why a cross-sectional snapshot, not a per-symbol grid map or a dense cube:**
- Equity history is *ragged* (listing dates, halts, `EMPTY` gaps) — a dense
  `symbol × bar` cube would be mostly NaN padding. Rejected.
- A screener only ever queries **one time slice** ("every symbol, as of date `D`").
  That slice *is* dense (every symbol has one latest row) and tiny (~3 MB).
- Holding the whole slice in one table makes **breadth / ranking** first-class:
  "top 50 by `ret_3m`", "`rel_volume` top decile", "% of universe above `sma200`",
  relative-strength rank. These are awkward across per-symbol series and are exactly
  what the free tiers lack.

**Memory:** we do **not** need to hold all history in RAM. Indicators are computed by
streaming each symbol's bars once and keeping only the latest row. The 32 GB /
≤1.1 GB in-RAM idea is reserved for the future backtest tier (§7), not the scan.

---

## 4. The indicator layer

`internal/indicators` — pure Go, one function per indicator over `[]model.Bar`, the
canonical set in [`INDICATORS.md`](INDICATORS.md). Rules:

- **Right-aligned, 1-bar setback:** the value at bar `T` uses bars `≤ T`; store the
  `prev_*` mirror so crosses are a plain predicate (`sma50 > sma200 AND prev_sma50 <=
  prev_sma200`) — no window functions, no self-joins.
- **Feed-trust flags** carried per indicator (✅ price-only / ⚠️ same-feed ratio /
  ❌ absolute volume). The screener surfaces these; free scanners hide them.
- **Data-driven windows:** lengths live in config, never in logic.

## 5. The screen layer

`internal/screen` — evaluate a filter over the snapshot table.

- **Predicate model:** a screen is a set of comparisons + boolean `and`/`or` over
  snapshot columns, plus optional `order_by` / `limit` / percentile-rank for
  cross-sectional studies. Each screen is **data, not code** — saveable, shareable.
- **Two front-ends, one predicate:** a Finviz-style filter form and a raw
  expression are two surfaces over the same per-row test. (Vocabulary is kept
  TradeScript-compatible — `crosses_above(sma50, sma200)` desugars to the `prev_*`
  predicate — so a future merge with the engine's DSL is cheap. See §7.)
- **Presets** ship the popular studies day one:

| Preset | Core predicate |
|---|---|
| `new_52w_high` | `is_52w_high` |
| `golden_cross` | `crosses_above(sma50, sma200)` |
| `oversold_bounce` | `crosses_above(rsi14, 30)` |
| `gap_up_on_volume` | `gap_pct >= 0.03 and rel_volume >= 2` |
| `ma_stack` | `ema9>ema21>ema50>ema200` |
| `bollinger_squeeze` | `bb_bandwidth <= p10(bb_bandwidth)` |
| `momentum_leaders` | `order_by ret_3m desc limit 50` (cross-sectional) |
| `above_200dma_breadth` | breadth metric: `% of universe where close > sma200` |

### Optional persistence (later, not now)

If per-invocation recompute becomes a cost (CLI use), materialize the snapshot to a
rebuildable `snapshot.db` (SQLite) and `SELECT` against it — same predicates, just
cold-cached. A long-running scan server recomputes on boot and skips this entirely.
**Deferred until measured.**

---

## 6. Package layout

```
internal/model        # Bar, Signal (exists)
internal/store        # RO reader of cetus (exists)
internal/indicators   # NEW — pure Go, the ample fixed catalog (INDICATORS.md)
internal/screen       # NEW — snapshot build + predicate filter/rank + presets
internal/config       # env-first config (exists)
internal/telemetry    # JSON logs to stderr (exists)
cmd/scanner           # load universe → build snapshot → screen → JSONL (exists, extended)
```

No `tradegrid` / `cursor` / `tradescript` imports. Self-contained, single static
binary, `modernc.org/sqlite` the only dep — matching the pipeline and the repo ethos.

---

## 7. Future convergence (kept open, not built)

The scanner is independent **for now**. Two convergence points are deliberately
left cheap:

1. **Shared DSL vocabulary.** Indicator names (`SMA`, `RSI`, `MACD`…) and cross
   semantics match `trading-engine-go`'s **TradeScript**. A scan study is the
   predicate-half of a strategy; when we unify, the screen predicates map onto
   TradeScript without re-authoring.
2. **Backtest-the-screen (paid tier).** The differentiator. When built, it replays
   a screen forward from a past date `D` — the *same* snapshot projection, just at a
   historical index — over full per-symbol history. That's where the in-RAM history
   load and (potentially) the engine's TradeGrid/Cursor substrate come back in,
   behind the elko license gate.

Until then: read cetus, compute an ample indicator set, snapshot, filter. Lean and
standalone.

---

## 8. Ethos alignment

Pure Go / zero CGO, single static binary, `modernc.org/sqlite` only. Indicators stay
pure and data-driven — windows are config, never hardcoded. Read cetus read-only.
`context.Context` on every I/O path, JSON logs to stderr, graceful signal handling,
env-first config (`SCANNER_*`).
