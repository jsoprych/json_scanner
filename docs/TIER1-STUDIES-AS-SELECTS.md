# Architecture Axiom — Tier-1 Studies Are Live SELECTs

**Status:** Foundational. This is the load-bearing idea of the scanner. Everything
in Phase 1 is a consequence of it.

---

## The axiom

> **A Tier-1 study is nothing more than a `SELECT` statement executed against a
> live, in-memory table of current prices + precomputed indicators/features.**

One row per symbol. Columns are the latest price fields and their derived
features (SMA, RSI, returns, dollar-volume, 52-week extremes, and the `prev_*`
mirrors for one-bar transitions). A study is a `WHERE` / `ORDER BY` / `LIMIT`
over that table. There is no rule interpreter, no formula VM, no bespoke scan
engine. **SQLite is the scan engine; SQL is the study language.**

```
study.Where = "close > sma200 AND rsi14 < 35"
        │
        ▼
SELECT symbol FROM snapshot WHERE (close > sma200 AND rsi14 < 35)
        ORDER BY ret_3m DESC LIMIT 26
        │
        ▼
matches   (~1–2 ms over ~14k rows, in memory)
```

The structured study editor (`internal/predicate`) is just the safe **authoring
front-end** for that `WHERE` string: editor `Definition` → `Compile()` →
`Compiled.Where` → **verbatim** `study.Study.Where`. Core studies, custom user
studies, the digest, and the live preview endpoint all run through the one
`snapshot.Run` path. There is no second engine.

## The shape: materialize once, query many

The expensive work happens **once** per snapshot; every study after that is
trivial. This is a feature-store / OLAP pattern, not a per-query computation.

```
BUILD PHASE   (once per EOD snapshot_id — seconds)          ← all the cost
  cetus bars (read-only) → indicators → in-mem snapshot
  (one row/symbol, latest bar + prev_* mirrors)

QUERY PHASE   (live, unlimited — milliseconds each)         ← ~free
  study.WHERE → SELECT over snapshot → matches
```

**Corollary:** the snapshot's *columns* define the expressive ceiling. A study
can only filter on what exists as a column. Adding expressiveness means adding a
**feature column in the build phase**, never adding query machinery. Compute-heavy
logic lives at build time; query time stays a dumb, fast comparison.

## What the axiom buys

- **Studies are data, not code.** A study is a row of text — storable, cloneable,
  shareable, diffable, hashable, importable/exportable. Adding a study ships zero
  code and requires zero deploy.
- **One uniform execution path.** Core, custom, digest, preview — all `snapshot.Run`.
  One place to secure, cache, and optimize.
- **Instant, unlimited iteration.** Sub-2ms SELECTs mean a user tunes a screen
  live; there is no "run job and wait."
- **Deterministic ⇒ cacheable.** A snapshot is immutable for its lifetime
  (`snapshot_id`). The same study over the same snapshot always yields the same
  rows, so results key cleanly on `hash(study_hash + snapshot_id)` — identical
  studies across users run **once**.
- **Free boolean/ordering algebra.** AND/OR, comparisons, ranking, limits all come
  from the query planner. We reimplement none of it.
- **Structurally injection-proof (Tier-1).** Tier-1 `WHERE` clauses are emitted by
  the registry compiler from finite IDs, never from request strings. The freeform
  `WHERE` box is an admin/power escape hatch, not the Tier-1 surface.

## The constraints (what makes it *Tier 1*)

- **Cross-sectional, not time-series.** The snapshot is a single time-slice: "state
  now" plus "one-bar transition" (`crossed_above` via `prev_*`). It **cannot**
  express arbitrary multi-bar windows ("RSI under 30 for 3 of the last 5 days",
  "today is the highest volume in 20 days" unless that's already a column). Anything
  needing a window beyond a precomputed feature is **Tier 2** (a different, heavier
  engine — eventual convergence with `trading-engine-go`), not Tier 1.
- **Features must pre-exist as columns.** The standard/"core" feature set *is* the
  Tier-1 product. Richer studies = richer columns, decided at build time.
- **"Live" = hot EOD snapshot, not ticks.** The table is refreshed when new
  end-of-day data lands (after the cetus pipeline runs post-close) and held hot in
  RAM. "Live" means *current snapshot, queryable instantly* — not real-time
  streaming (that is a separate feed/tier).
- **Volume is feed-limited (IEX).** Prefer `dollar_vol` and price-derived features;
  raw-volume-only predicates are unreliable until the feed improves.
- **Under-warmed features are NULL.** A young symbol whose SMA200 isn't ready is
  `NULL` and simply never matches — screens fail safe rather than error.

## Tier boundary in one line

- **Tier 1 (free / live):** cross-sectional `SELECT` over the standard feature
  columns, held in memory, refreshed each EOD. Instant, unlimited, authored through
  the injection-proof editor.
- **Tier 2+ (paid):** a *superset* — more features, full history / windowed queries
  / backtest — kept vocabulary-compatible so the on-ramp from Tier 1 is seamless.

## Why this is the whole MVP

If the standard prices + core indicators live in an in-memory SQLite table and a
study is a `SELECT` over it, then "run a scanner study" is already solved and
already fast. The remaining Phase-1 work is not the query engine — it's the
**lifecycle around the snapshot**: build it eagerly on start, give it a
`snapshot_id`, rebuild it when new EOD data lands, and cache/batch study results
against that id for the daily digest. The engine is done; the pipeline is the work.
