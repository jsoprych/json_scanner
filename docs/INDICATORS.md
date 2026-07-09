# cetus-marketdata-scanner — Indicator Catalog

The **canonical indicator set** computed globally, per symbol, every build. This is
the contract the feature warehouse (`features.db`) and the screener DSL are built
on. If it isn't listed here, the screener can't filter on it.

Read this alongside:
- The upstream schema contract: [`../../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`](../../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md)
- The system design: [`SCANNER_DESIGN.md`](SCANNER_DESIGN.md)

---

## Conventions (read first)

### No Lookahead Principle (CRITICAL)

**Every indicator at bar T uses only data from bars < T (excluding the current bar).**

This is the fundamental rule of technical indicator calculation. An indicator value
stored at bar T represents information available BEFORE bar T's close. This prevents
lookahead bias and ensures backtests reflect real trading conditions.

**Implementation:**
- Indicator at index `i` is computed from `bars[0..i-1]` (not including `bars[i]`)
- No `prev_*` columns needed - indicators are already shifted by 1 bar
- Cross detection uses boolean fields: `golden_cross`, `oversold_bounce`

**Example:**
```sql
-- Golden cross: SMA50 crossed above SMA200 (computed at bar T-1)
WHERE golden_cross = 1
```

**Warm-up:** an indicator with window `n` has no value until bar `n+1` (needs `n`
bars before the current bar). Rows before warm-up store `NULL`, never a partial
value. Screeners must tolerate `NULL` (SQL `NULL` comparisons are falsy — a
half-warmed symbol simply doesn't match, which is correct).

### Prices vs. volume — the feed-trust flag

Prices come from `adjusted_bars` (split-adjusted, trustworthy). **Volume on the
free IEX feed is a single-digit-% fraction of consolidated volume** (Data
Dictionary §5). Each indicator carries a trust flag:

| Flag | Meaning |
|:---:|---|
| ✅ | **Price-only** — fully trustworthy today. |
| ⚠️ | **Ratio of same-feed volume** — the IEX fraction largely cancels (today's fraction ÷ trailing fraction ≈ true ratio). Usable now, exact when SIP/Yahoo lands. |
| ❌ | **Absolute volume magnitude** — IEX-fractional, provisional. Compute and store it, but flag it; it upgrades automatically when a full-volume source backfills. |

`price × volume` metrics (dollar-volume, VWAP) are **split-invariant** (the factor
cancels) but still ride the IEX volume fraction — so they're ⚠️, not ✅.

### Data-driven, never hardcoded

Window lengths below are the **defaults we materialize**. They live in config, not
in logic (repo ethos: "tuning never touches logic"). Adding a new window = a config
line + a rebuild, not a code change.

---

## The catalog

### 1. Trend / moving averages — ✅

| Column | Definition | Notes |
|---|---|---|
| `sma5` `sma10` `sma20` `sma30` `sma50` `sma100` `sma200` | Simple MA of close over N | Computed with no lookahead |
| `ema10` `ema21` `ema50` `ema100` `ema200` | Exponential MA, `α = 2/(N+1)` | Recursive; seed with SMA(N) |
| `pct_from_sma50` `pct_from_sma200` | `close / smaN − 1` | "% above/below the N-day" |
| `ma_stack` | bool: `ema10>ema21>ema50>ema200` | Clean-uptrend flag |
| `golden_cross` | bool: SMA50 crossed above SMA200 | Cross detection (no lookahead) |

### 2. Momentum / oscillators — ✅

| Column | Definition | Notes |
|---|---|---|
| `rsi14` | Wilder RSI, 14 | Computed with no lookahead |
| `oversold_bounce` | bool: RSI14 crossed above 30 | Cross detection (no lookahead) |
| `macd` `macd_signal` `macd_hist` | EMA12−EMA26; EMA9 of that; diff | (planned) |
| `stoch_k` `stoch_d` | Stochastic %K(14), %D(3) | (planned) |
| `roc10` `roc20` | `close/close[T−k] − 1` | Rate of change (planned) |
| `willr14` | Williams %R, 14 | (planned) |
| `cci20` | Commodity Channel Index, 20 | (planned) |

### 3. Volatility / bands — ✅

| Column | Definition | Notes |
|---|---|---|
| `atr14` | Wilder ATR, 14 (uses true range) | Position-sizing / stop distance |
| `atr_pct` | `atr14 / close` | Volatility normalized to price |
| `bb_mid` `bb_upper` `bb_lower` | SMA20 ± 2·σ20 | Bollinger(20,2) |
| `bb_pct_b` | `(close − lower)/(upper − lower)` | 0=lower band, 1=upper |
| `bb_bandwidth` | `(upper − lower)/mid` | **Squeeze** = low bandwidth |
| `donchian20_high` `donchian20_low` | rolling 20-bar high/low | + 55 variant |
| `donchian55_high` `donchian55_low` | rolling 55-bar high/low | Turtle breakout |
| `hist_vol20` | stdev of daily log-returns × √252, 20 | Annualized realized vol |

### 4. Directional / trend-strength — ✅

| Column | Definition | Notes |
|---|---|---|
| `adx14` `di_plus` `di_minus` | Wilder ADX / ±DI, 14 | ADX>25 = trending |
| `aroon_up` `aroon_down` | Aroon(25) | |
| `supertrend` `supertrend_dir` | ATR(10)×3 SuperTrend + direction | |
| `psar` | Parabolic SAR (0.02/0.2) | |

### 5. Price structure — ✅

| Column | Definition | Notes |
|---|---|---|
| `high_52w` `low_52w` | 252-bar high/low | |
| `pct_off_52w_high` | `close/high_52w − 1` (≤0) | "N% off the high" |
| `pct_above_52w_low` | `close/low_52w − 1` (≥0) | |
| `gap_pct` | `open/close[T−1] − 1` | Overnight gap |
| `true_range` | `max(H−L, |H−Cₚ|, |L−Cₚ|)` | |
| `is_52w_high` `is_52w_low` | bool on this bar | |

### 6. Returns / relative strength — ✅

| Column | Definition | Notes |
|---|---|---|
| `ret_1d` `ret_5d` `ret_1m` `ret_3m` `ret_6m` `ret_1y` | `close[T-1]/close[T-1-k] − 1` | k = 1,5,21,63,126,252 (no lookahead) |
| `rs_spy_3m` `rs_spy_6m` | symbol return − SPY return | Relative strength vs benchmark (planned) |

### 7. Volume — trust-flagged

| Column | Definition | Trust |
|---|---|:---:|
| `avg_vol20` `avg_vol50` | SMA of volume | ❌ absolute |
| `obv` | On-balance volume | ❌ absolute |
| `rel_volume` | `volume / avg_vol20` | ⚠️ ratio survives feed |
| `dollar_vol` | `close × volume` | ⚠️ split-invariant, feed-fractional |
| `avg_dollar_vol20` | SMA20 of `dollar_vol` | ⚠️ liquidity filter |
| `vwap_dist` | `close / vwap − 1` | ⚠️ (vwap may be absent → NULL) |
| `mfi14` | Money Flow Index, 14 | ⚠️ uses price×volume |

> **Rule of thumb for now:** filter with the ⚠️ ratio metrics (`rel_volume`,
> `avg_dollar_vol20`) for "unusual activity" and "is it liquid enough to trade."
> Avoid ❌ absolute-volume thresholds until a full-volume source backfills.

---

## What we deliberately defer

- **Fundamentals** (P/E, float, short interest, market cap) — not in the cetus
  warehouse yet. Finviz's edge here; a later data source, not an indicator.
- **Candlestick/chart patterns** (engulfing, flags, H&S) — a pattern pass on top of
  this feature layer, phase 2.
- **Intraday / multi-timeframe** — cetus is EOD. Weekly/monthly can be resampled
  from daily; sub-daily needs an intraday feed.
- **Dividend/total-return** indicators — warehouse is split-only today (Dict §4).
