# Snapshot Data Dictionary

All columns available in the `snapshot` table for study WHERE clauses.

## Price

| Column | Type | Description |
|--------|------|-------------|
| `close` | REAL | Closing price (split-adjusted) |
| `high` | REAL | Day high (split-adjusted) |
| `low` | REAL | Day low (split-adjusted) |
| `open` | REAL | Opening price (split-adjusted) |

## Trend — SMA

| Column | Period | Description |
|--------|--------|-------------|
| `sma5` | 5 | Very short-term trend |
| `sma10` | 10 | Short-term trend |
| `sma20` | 20 | Short/medium trend (Bollinger mid) |
| `sma30` | 30 | Medium-term trend |
| `sma50` | 50 | Medium-term trend (Golden Cross) |
| `sma100` | 100 | Medium/long trend |
| `sma200` | 200 | Long-term trend (widely watched) |

## Trend — EMA

| Column | Period | Description |
|--------|--------|-------------|
| `ema10` | 10 | Exponential, faster than SMA10 |
| `ema21` | 21 | Short-term trend |
| `ema50` | 50 | Medium-term trend |
| `ema100` | 100 | Medium/long trend |
| `ema200` | 200 | Long-term trend |

## Trend — Derived

| Column | Type | Description |
|--------|------|-------------|
| `pct_from_sma50` | REAL | (close / sma50) - 1 |
| `pct_from_sma200` | REAL | (close / sma200) - 1 |
| `ma_stack` | INTEGER | 1 if EMA10 > EMA21 > EMA50 > EMA200 |

## Momentum

| Column | Type | Range | Description |
|--------|------|-------|-------------|
| `rsi14` | REAL | 0–100 | Relative Strength Index (14) |
| `macd` | REAL | — | MACD line (EMA12 - EMA26) |
| `macd_signal` | REAL | — | MACD signal (EMA9 of MACD) |
| `macd_hist` | REAL | — | MACD histogram (MACD - signal) |
| `stoch_k` | REAL | 0–100 | Stochastic %K (14,3,3) |
| `stoch_d` | REAL | 0–100 | Stochastic %D (3-SMA of %K) |
| `willr14` | REAL | -100–0 | Williams %R (14) |
| `cci20` | REAL | — | Commodity Channel Index (20) |
| `roc10` | REAL | — | Rate of Change (10-day) |
| `roc20` | REAL | — | Rate of Change (20-day) |
| `adx14` | REAL | 0–100 | Average Directional Index (14) |
| `di_plus` | REAL | 0–100 | +DI uptrend strength (14) |
| `di_minus` | REAL | 0–100 | -DI downtrend strength (14) |
| `mfi14` | REAL | 0–100 | Money Flow Index (14) |

## Volatility

| Column | Type | Description |
|--------|------|-------------|
| `atr14` | REAL | Average True Range (14) — absolute |
| `atr_pct` | REAL | ATR(14) / close — normalized |
| `bb_upper` | REAL | Bollinger upper (SMA20 + 2σ) |
| `bb_mid` | REAL | Bollinger middle (SMA20) |
| `bb_lower` | REAL | Bollinger lower (SMA20 - 2σ) |
| `bb_bandwidth` | REAL | (upper - lower) / mid — squeeze when low |
| `bb_pct_b` | REAL | Position in bands: 0=lower, 1=upper |
| `hist_vol20` | REAL | 20-day annualized historical volatility |

## Price Structure

| Column | Type | Description |
|--------|------|-------------|
| `high_52w` | REAL | Highest close in 252 trading days |
| `low_52w` | REAL | Lowest close in 252 trading days |
| `is_52w_high` | INTEGER | 1 if close equals 52-week high |
| `is_52w_low` | INTEGER | 1 if close equals 52-week low |
| `gap_pct` | REAL | (open / prev_close) - 1 |
| `true_range` | REAL | Max of (H-L), |H-prevC|, |L-prevC| |
| `pct_off_52w_high` | REAL | (close / high_52w) - 1 |
| `pct_above_52w_low` | REAL | (close / low_52w) - 1 |

## Returns

All values are decimals (0.05 = 5%).

| Column | Period | Description |
|--------|--------|-------------|
| `ret_1d` | 1 day | Yesterday's return |
| `ret_5d` | 5 days | Past week return |
| `ret_1m` | 21 days | Past month return |
| `ret_3m` | 63 days | Past quarter return |
| `ret_6m` | 126 days | Past 6 months return |
| `ret_1y` | 252 days | Past year return |

## Volume

| Column | Type | Description |
|--------|------|-------------|
| `dollar_vol` | REAL | close × volume (split-invariant) |
| `avg_dollar_vol20` | REAL | 20-day average dollar volume |
| `rel_volume` | REAL | volume / 20-day avg (1.0 = average) |
| `obv` | REAL | On-Balance Volume (cumulative) |
| `vwap_dist` | REAL | % distance from VWAP (20-day) |

## Cross Detection

| Column | Type | Value |
|--------|------|-------|
| `golden_cross` | INTEGER | 1 if SMA50 crossed above SMA200 today |
| `oversold_bounce` | INTEGER | 1 if RSI14 crossed above 30 today |

## Operators

| Operator | Symbol |
|----------|--------|
| `=` | Equal |
| `>` | Greater than |
| `>=` | Greater than or equal |
| `<` | Less than |
| `<=` | Less than or equal |
| `BETWEEN x AND y` | Range (inclusive) |
| `AND` | Logical AND |
| `OR` | Logical OR |
| `ORDER BY col ASC/DESC` | Sort direction |
| `LIMIT n` | Max results |
