# Study Syntax Guide — Available Indicators & WHERE Clauses

Studies are SQL WHERE clauses that run against the snapshot table. Each row represents one symbol's latest indicator values. Use any column below in your WHERE clause.

## Price Columns

| Column | Type | Description |
|--------|------|-------------|
| `close` | REAL | Closing price (split-adjusted) |
| `high` | REAL | Day high (split-adjusted) |
| `low` | REAL | Day low (split-adjusted) |
| `open` | REAL | Opening price (split-adjusted) |

**Examples:**
```sql
close > 100                           -- stocks above $100
close BETWEEN 50 AND 200              -- stocks between $50 and $200
high >= high_52w                      -- stocks at 52-week highs
```

## Trend Indicators

### Simple Moving Averages

| Column | Type | Period | Description |
|--------|------|--------|-------------|
| `sma5` | REAL | 5 days | Very short-term trend |
| `sma10` | REAL | 10 days | Short-term trend |
| `sma20` | REAL | 20 days | Short/medium trend (Bollinger mid) |
| `sma30` | REAL | 30 days | Medium-term trend |
| `sma50` | REAL | 50 days | Medium-term trend (Golden Cross) |
| `sma100` | REAL | 100 days | Medium/long trend |
| `sma200` | REAL | 200 days | Long-term trend (widely watched) |

### Exponential Moving Averages

| Column | Type | Period | Description |
|--------|------|--------|-------------|
| `ema10` | REAL | 10 days | Faster than SMA10 |
| `ema21` | REAL | 21 days | Short-term trend |
| `ema50` | REAL | 50 days | Medium-term trend |
| `ema100` | REAL | 100 days | Medium/long trend |
| `ema200` | REAL | 200 days | Long-term trend |

### Trend-Derived

| Column | Type | Description |
|--------|------|-------------|
| `pct_from_sma50` | REAL | % distance from SMA50 (close/sma50 - 1) |
| `pct_from_sma200` | REAL | % distance from SMA200 (close/sma200 - 1) |
| `ma_stack` | INTEGER | 1 if EMA10 > EMA21 > EMA50 > EMA200 (perfect bullish alignment) |

**Examples:**
```sql
close > sma200                        -- price above 200-day moving average
close > sma50 AND close > sma200      -- above both 50 and 200 DMA
close > ema50 AND sma50 > sma200      -- above 50 DMA in uptrend
pct_from_sma200 > 0.05                -- more than 5% above 200 DMA
pct_from_sma200 < -0.05               -- more than 5% below 200 DMA
ma_stack = 1                          -- perfect bullish EMA alignment
sma50 > sma200                        -- uptrend (50 above 200)
sma50 < sma200                        -- downtrend (50 below 200)
```

## Momentum Indicators

| Column | Type | Range | Description |
|--------|------|-------|-------------|
| `rsi14` | REAL | 0-100 | Relative Strength Index (14-day) |
| `macd` | REAL | — | MACD line (EMA12 - EMA26) |
| `macd_signal` | REAL | — | MACD signal line (EMA9 of MACD) |
| `macd_hist` | REAL | — | MACD histogram (MACD - Signal) |
| `stoch_k` | REAL | 0-100 | Stochastic %K (14,3,3) |
| `stoch_d` | REAL | 0-100 | Stochastic %D (3-day SMA of %K) |
| `willr14` | REAL | -100 to 0 | Williams %R (14-day) |
| `cci20` | REAL | — | Commodity Channel Index (20-day) |
| `roc10` | REAL | — | Rate of Change (10-day) as decimal |
| `roc20` | REAL | — | Rate of Change (20-day) as decimal |
| `adx14` | REAL | 0-100 | Average Directional Index (trend strength) |
| `di_plus` | REAL | 0-100 | +DI (uptrend strength) |
| `di_minus` | REAL | 0-100 | -DI (downtrend strength) |
| `mfi14` | REAL | 0-100 | Money Flow Index (volume-weighted RSI) |

**Common RSI Patterns:**
```sql
rsi14 < 30                               -- oversold (potential bounce)
rsi14 > 70                               -- overbought (potential reversal)
rsi14 BETWEEN 55 AND 70                   -- RSI in bullish momentum zone
rsi14 BETWEEN 30 AND 45                   -- RSI in weak/bearish zone
rsi14 > 50 AND rsi14 < 60                 -- just crossed above 50 (bullish)
```

**MACD Patterns:**
```sql
macd > macd_signal                        -- MACD above signal (bullish momentum)
macd < macd_signal                        -- MACD below signal (bearish momentum)
macd_hist > 0                             -- MACD histogram positive
macd > 0                                  -- MACD above zero line
macd > 0 AND macd > macd_signal           -- bullish on both measures
```

**Stochastic Patterns:**
```sql
stoch_k < 20                              -- oversold (potential bounce)
stoch_k > 80                              -- overbought (potential reversal)
stoch_k > stoch_d                         -- %K above %D (bullish crossover)
stoch_k > 50 AND stoch_k > stoch_d        -- bullish above midpoint
```

**ADX / Trend Strength:**
```sql
adx14 > 25                                -- strong trend (any direction)
adx14 > 25 AND di_plus > di_minus         -- strong uptrend
adx14 > 25 AND di_minus > di_plus         -- strong downtrend
adx14 < 20                                -- weak trend / rangebound
```

**Williams %R:**
```sql
willr14 > -20                             -- overbought (near 0)
willr14 < -80                             -- oversold (near -100)
```

**CCI:**
```sql
cci20 > 100                               -- strong uptrend / overbought
cci20 < -100                              -- strong downtrend / oversold
```

## Volatility Indicators

| Column | Type | Description |
|--------|------|-------------|
| `atr14` | REAL | Average True Range (14-day) — absolute volatility |
| `atr_pct` | REAL | ATR(14) / close — normalized volatility |
| `bb_upper` | REAL | Bollinger Band upper (SMA20 + 2σ) |
| `bb_mid` | REAL | Bollinger Band middle (SMA20) |
| `bb_lower` | REAL | Bollinger Band lower (SMA20 - 2σ) |
| `bb_bandwidth` | REAL | BB width / mid (low = squeeze) |
| `bb_pct_b` | REAL | Position in bands: 0=lower, 1=upper |
| `hist_vol20` | REAL | 20-day annualized historical volatility |

**Examples:**
```sql
close > bb_upper                         -- price above upper BB band
close < bb_lower                         -- price below lower BB band
bb_bandwidth < 0.05                      -- Bollinger squeeze (low volatility)
bb_pct_b < 0.1                           -- near lower band (oversold)
bb_pct_b > 0.9                           -- near upper band (overbought)
atr_pct < 0.02                           -- low volatility (less than 2% ATR)
hist_vol20 < 0.20                        -- annualized vol under 20%
```

## Price Structure

| Column | Type | Description |
|--------|------|-------------|
| `high_52w` | REAL | Highest price in 252 trading days |
| `low_52w` | REAL | Lowest price in 252 trading days |
| `is_52w_high` | INTEGER | 1 if close equals 52-week high |
| `is_52w_low` | INTEGER | 1 if close equals 52-week low |
| `gap_pct` | REAL | Overnight gap: (open - prev_close) / prev_close |
| `true_range` | REAL | Max of (H-L), |H-prevC|, |L-prevC| |
| `pct_off_52w_high` | REAL | % below 52-week high |
| `pct_above_52w_low` | REAL | % above 52-week low |

**Examples:**
```sql
is_52w_high = 1                          -- at 52-week high today
is_52w_low = 1                           -- at 52-week low today
pct_off_52w_high < 0.02                  -- within 2% of 52-week high
pct_above_52w_low < 0.05                 -- within 5% of 52-week low
gap_pct > 0.03                           -- gapped up more than 3%
gap_pct < -0.03                          -- gapped down more than 3%
```

## Returns (Performance)

All return values are decimals (0.05 = 5%).

| Column | Type | Period | Description |
|--------|------|--------|-------------|
| `ret_1d` | REAL | 1 day | Yesterday's return |
| `ret_5d` | REAL | 5 days | Past week return |
| `ret_1m` | REAL | 21 days | Past month return |
| `ret_3m` | REAL | 63 days | Past quarter return |
| `ret_6m` | REAL | 126 days | Past 6 months return |
| `ret_1y` | REAL | 252 days | Past year return |

**Examples:**
```sql
ret_3m > 0.10                            -- up more than 10% in 3 months
ret_1m > 0 AND ret_3m > 0                -- positive 1-month AND 3-month momentum
ret_1d < -0.05                           -- down more than 5% yesterday
ret_3m > 0 AND ret_6m > 0 AND ret_1y > 0 -- all timeframes positive
ret_3m > ret_6m                          -- accelerating momentum (3m > 6m)
ret_1y < -0.20                           -- down more than 20% from year ago
```

## Volume Indicators

| Column | Type | Description |
|--------|------|-------------|
| `dollar_vol` | REAL | Close × volume (split-invariant) |
| `avg_dollar_vol20` | REAL | 20-day average dollar volume |
| `rel_volume` | REAL | Volume / 20-day avg volume (1.0 = average) |
| `obv` | REAL | On-Balance Volume (cumulative) |
| `vwap_dist` | REAL | % distance from VWAP (20-day) |
| `mfi14` | REAL | Money Flow Index (14-day, 0-100) |

**Examples:**
```sql
dollar_vol > 5000000                    -- trades > $5M/day
dollar_vol > 10000000                   -- trades > $10M/day (liquid)
rel_volume > 2.0                        -- volume 2x normal (unusual activity)
rel_volume > 1.5 AND close > sma50      -- high volume above 50 DMA
vwap_dist > 0                           -- trading above VWAP (bullish)
vwap_dist < 0                           -- trading below VWAP (bearish)
```

## Cross Detection (Boolean Flags)

| Column | Type | Value |
|--------|------|-------|
| `golden_cross` | INTEGER | 1 if SMA50 crossed above SMA200 today |
| `oversold_bounce` | INTEGER | 1 if RSI14 crossed above 30 today |

**Examples:**
```sql
golden_cross = 1                        -- golden cross fired today
oversold_bounce = 1                     -- oversold bounce fired today
```

## Compound Study Examples

These combine multiple indicators for more precise screening:

### Momentum + Trend
```sql
close > sma50 AND close > sma200 
AND rsi14 BETWEEN 55 AND 70
AND ret_3m > 0.05
```
**Meaning:** Price above both MAs, RSI in bullish zone, positive 3-month momentum.

### Oversold Reversal Candidates
```sql
close < sma50 
AND rsi14 < 35 
AND dollar_vol > 5000000
AND pct_above_52w_low < 0.10
```
**Meaning:** Pulled back below 50 DMA, RSI oversold, liquid, near 52-week low.

### High-Volume Breakouts
```sql
close > sma200 
AND rel_volume > 1.5 
AND close > bb_upper
AND dollar_vol > 10000000
```
**Meaning:** Above 200 DMA, high volume, above upper Bollinger Band, liquid (>$10M/day).

### Trend Following
```sql
sma50 > sma200 AND ema10 > ema21
AND close > sma50 AND adx14 > 25
AND di_plus > di_minus
```
**Meaning:** Uptrend confirmed on MAs, EMA, ADX for strength, all bullish.

### Squeeze Setup (Low Volatility → Breakout Watch)
```sql
bb_bandwidth < 0.05
AND dollar_vol > 5000000
AND close > sma50
```
**Meaning:** Bollinger squeeze, liquid, above 50 DMA — watching for breakout.

### Gap Opportunities
```sql
gap_pct > 0.02 AND rel_volume > 1.5 
AND close > sma20
```
**Meaning:** Gapped up >2% on above-average volume, above 20 DMA.

### Value / Mean Reversion
```sql
pct_from_sma200 < -0.15
AND dollar_vol > 5000000
AND rsi14 < 40
```
**Meaning:** >15% below 200 DMA, liquid, RSI oversold — potential mean reversion.

## Multi-Rule Logic

Combine rules with `AND` / `OR` and parentheses:

```sql
(close > sma50 AND rsi14 > 50) 
OR (close < sma200 AND rsi14 < 30 AND dollar_vol > 20000000)
```
**Meaning:** Either (uptrend) OR (deeply oversold liquid stocks).

```sql
close > sma200 
AND (rsi14 BETWEEN 55 AND 70 OR macd > macd_signal)
AND dollar_vol > 5000000
```
**Meaning:** Above 200 DMA, at least one momentum signal, and liquid.

## ORDER BY Options

Sort results by any numeric column:

```sql
-- Sort by momentum
WHERE close > sma200 AND rsi14 > 50 ORDER BY ret_3m DESC

-- Sort by liquidity
WHERE close > sma200 ORDER BY dollar_vol DESC

-- Sort by RSI
WHERE close < sma50 ORDER BY rsi14 ASC        -- most oversold first

-- Sort by proximity to 52-week high
WHERE close > sma200 ORDER BY pct_off_52w_high ASC

-- Sort by volatility
ORDER BY atr_pct DESC                           -- most volatile first
ORDER BY bb_bandwidth ASC                       -- tightest squeeze first
```

## LIMIT

Cap results (default 10-20):

```sql
WHERE close > sma200 AND rsi14 > 50 ORDER BY ret_3m DESC LIMIT 20
```

## Tips

1. **Test first**: Click "Test WHERE" to see a quick count and sample before saving
2. **Be specific**: `rsi14 > 70` is broad — add `AND close > sma50` for quality
3. **Liquidity matters**: Add `dollar_vol > 5000000` to filter out thin stocks
4. **Trend confirmation**: `close > sma200` is the simplest trend filter
5. **Use BETWEEN for ranges**: `rsi14 BETWEEN 50 AND 70` is cleaner than `rsi14 > 50 AND rsi14 < 70`
6. **ORDER BY adds meaning**: Sort by the metric you care about most
7. **Paranoid parentheses**: `(a > b AND c > d) OR (e < f)` — SQL follows your grouping

## Advanced: Structured Study Editor

For users who prefer a UI over raw SQL, the Dashboard → Study Editor offers a structured predicate builder. Each rule is a pre-validated comparison:

1. Pick a **feature** (Close, SMA50, RSI14, Return 3m, Dollar Vol)
2. Pick an **operator** (Above, Below, Crossed Above, Crossed Below)
3. Pick an **operand** (fixed value like $100, SMA200, RSI Level 30)
4. Chain rules with **AND / OR**
5. Add a **sort** direction

The editor compiles your rules into safe SQL — no risk of syntax errors or injection.
