# Cetus Scanner — Indicator Reference

## Overview

60+ technical indicators computed with strict no-lookahead. Every indicator at bar T uses only data from bars < T — information available BEFORE the current bar's close. Warm-up positions return NaN.

## Trend Indicators

### SMA(period)
Simple Moving Average — arithmetic mean of the last `period` closing prices.

**Pseudo-code:**
```
for i in [0, n-1]:
    if i < period:
        out[i] = NaN
    else:
        sum = 0
        for j = i-period to i-1:
            sum += close[j]
        out[i] = sum / period
```

**Usage:** `sma20`, `sma50`, `sma200`
**Example:** `close > sma200` — price above 200-day moving average

### EMA(period)
Exponential Moving Average — weighted average giving more weight to recent prices.

**Pseudo-code:**
```
multiplier = 2 / (period + 1)
seed = SMA(close[0:period])
out[period] = seed
for i in [period+1, n-1]:
    out[i] = (close[i-1] - out[i-1]) * multiplier + out[i-1]
```

**Usage:** `ema10`, `ema21`, `ema50`, `ema100`, `ema200`
**Example:** `close > ema50` — price above 50-day EMA

### MA Stack
Boolean: true when EMAs are in perfect bullish order.

**Pseudo-code:**
```
out[i] = ema10[i] > ema21[i] && ema21[i] > ema50[i] && ema50[i] > ema200[i]
```

**Usage:** `ma_stack = 1`
**Example:** `ma_stack = 1` — strong, clean uptrend across all timeframes

### PctFromSMA
Percentage distance of price from a moving average.

**Pseudo-code:**
```
out[i] = (close[i-1] / sma[i]) - 1
```

**Usage:** `pct_from_sma50`, `pct_from_sma200`
**Example:** `pct_from_sma200 > 0.05` — >5% above 200-DMA

### PSAR (Parabolic SAR)
Trailing stop indicator. Rises as trend extends, reverses when hit.

**Pseudo-code:**
```
af = 0.02, ep = first high, isBullish = true
for i in [1, n-1]:
    if isBullish:
        sar = sar + af * (ep - sar)
        if close[i] > ep: ep = close[i]; af = min(af+0.02, 0.20)
        if low[i+1] < sar: reverse to bearish
    else:
        sar = sar - af * (sar - ep)
        if close[i] < ep: ep = close[i]; af = min(af+0.02, 0.20)
        if high[i+1] > sar: reverse to bullish
```

**Usage:** `psar`
**Example:** `close > psar` — price above SAR (uptrend)
**Example:** `close < psar` — price below SAR (downtrend)

### Aroon (Up, Down, Oscillator)
Measures whether a stock is trending or ranging. Period typically 25.

**Pseudo-code:**
```
for i in [period, n-1]:
    hiIdx = index of highest high in [i-period, i-1]
    loIdx = index of lowest low in [i-period, i-1]
    up[i] = 100 * (period - (i - hiIdx)) / period
    down[i] = 100 * (period - (i - loIdx)) / period
    osc[i] = up[i] - down[i]
```

**Usage:** `aroon_up`, `aroon_down`, `aroon_osc`
**Example:** `aroon_up > 70 AND aroon_down < 30` — strong uptrend
**Example:** `aroon_osc > 50` — bullish momentum

---

## Momentum Indicators

### RSI(period)
Relative Strength Index — momentum oscillator measuring speed of price changes.

**Pseudo-code:**
```
gain = sum of positive close-changes over period / period
loss = sum of negative close-changes over period / period
out[period+1] = 100 - 100/(1 + gain/loss)
for i in [period+1, n-2]:
    d = close[i] - close[i-1]
    gain = (gain*(period-1) + max(d,0)) / period
    loss = (loss*(period-1) + max(-d,0)) / period
    out[i+1] = 100 - 100/(1 + gain/loss)
```

**Usage:** `rsi14`
**Example:** `rsi14 < 30` — oversold
**Example:** `rsi14 BETWEEN 55 AND 70` — bullish momentum zone

### MACD
Moving Average Convergence Divergence — trend-following momentum indicator.

**Pseudo-code:**
```
ema12 = EMA(close, 12)
ema26 = EMA(close, 26)
macd[i] = ema12[i] - ema26[i]
signal[i] = EMA(macd, 9)[i]
histogram[i] = macd[i] - signal[i]
```

**Usage:** `macd`, `macd_signal`, `macd_hist`
**Example:** `macd > macd_signal` — bullish crossover
**Example:** `macd_hist > 0 AND macd > 0` — strong bullish momentum

### Stochastic (%K, %D)
Momentum indicator comparing closing price to price range over N periods.

**Pseudo-code:**
```
for i in [period, n-1]:
    highest = max(high[i-period+1 .. i])
    lowest = min(low[i-period+1 .. i])
    rawK[i] = 100 * (close[i] - lowest) / (highest - lowest)
%K = SMA(rawK, smooth)
%D = SMA(%K, dPeriod)
```

**Usage:** `stoch_k`, `stoch_d`
**Example:** `stoch_k < 20` — oversold
**Example:** `stoch_k > stoch_d AND stoch_k > 50` — bullish crossover

### Williams %R
Momentum indicator measuring overbought/oversold levels.

**Pseudo-code:**
```
for i in [period, n-1]:
    highest = max(high[i-period+1 .. i])
    lowest = min(low[i-period+1 .. i])
    out[i] = -100 * (highest - close[i]) / (highest - lowest)
```

**Usage:** `willr14`
**Example:** `willr14 < -80` — oversold
**Example:** `willr14 > -20` — overbought

### CCI(period)
Commodity Channel Index — identifies cyclical turns.

**Pseudo-code:**
```
typ[i] = (high[i] + low[i] + close[i]) / 3
smaTyp = SMA(typ, period)
for i in [period*2-1, n-1]:
    meanDev = sum(|typ[j] - smaTyp[j]| for j in [i-period, i]) / period
    out[i] = (typ[i] - smaTyp[i]) / (0.015 * meanDev)
```

**Usage:** `cci20`
**Example:** `cci20 > 100` — strong uptrend
**Example:** `cci20 < -100` — strong downtrend

### ROC(period)
Rate of Change — percentage price change over period.

**Pseudo-code:**
```
out[i] = (close[i] / close[i-period]) - 1
```

**Usage:** `roc10`, `roc20`
**Example:** `roc10 > 0.05` — up >5% in 10 days

### ADX(14) + DI
Average Directional Index — measures trend strength regardless of direction.

**Pseudo-code:**
```
tr[i] = max(high-low, |high-prevClose|, |low-prevClose|)
+DM = max(0, high - prevHigh) if it exceeds -DM, else 0
-DM = max(0, prevLow - low) if it exceeds +DM, else 0
smoothed TR/DM = Wilder smoothing over period
+DI = 100 * smoothed+DM / smoothedTR
-DI = 100 * smoothed-DM / smoothedTR
DX = 100 * |+DI - -DI| / (+DI + -DI)
ADX = Wilder smoothing of DX over period
```

**Usage:** `adx14`, `di_plus`, `di_minus`
**Example:** `adx14 > 25 AND di_plus > di_minus` — strong uptrend

### MFI(14) — Money Flow Index
Volume-weighted RSI. Uses price × volume for confirmation.

**Pseudo-code:**
```
tp[i] = (high[i] + low[i] + close[i]) / 3
rawMF[i] = tp[i] * volume[i]
positive = rawMF[i] when tp[i] > tp[i-1]
negative = rawMF[i] when tp[i] < tp[i-1]
ratio = sum(positive) / sum(negative) over period
out[i] = 100 - 100/(1 + ratio)
```

**Usage:** `mfi14`
**Example:** `mfi14 < 20` — oversold (volume-confirmed)
**Example:** `mfi14 > 80` — overbought (volume-confirmed)

### CMF(20) — Chaikin Money Flow
Measures buying/selling pressure normalized by volume over period.

**Pseudo-code:**
```
for i in [period, n-1]:
    mfSum = 0, volSum = 0
    for j = i-period+1 to i:
        if high[j] != low[j]:
            clv = ((close[j]-low[j]) - (high[j]-close[j])) / (high[j]-low[j])
        else:
            clv = 0
        mfSum += clv * volume[j]
        volSum += volume[j]
    out[i] = mfSum / volSum
```

**Usage:** `cmf20`
**Example:** `cmf20 > 0.05` — accumulation (buying pressure)
**Example:** `cmf20 < -0.05` — distribution (selling pressure)

### Ultimate Oscillator
Multi-timeframe momentum oscillator combining 7, 14, and 28 periods.

**Pseudo-code:**
```
bp[i] = close[i] - min(low[i], close[i-1])
tr[i] = max(high[i], close[i-1]) - min(low[i], close[i-1])
for i in [p3, n-1]:
    avg1 = sum(bp[i-p1+1..i]) / sum(tr[i-p1+1..i])
    avg2 = sum(bp[i-p2+1..i]) / sum(tr[i-p2+1..i])
    avg3 = sum(bp[i-p3+1..i]) / sum(tr[i-p3+1..i])
    out[i] = 100 * (4*avg1 + 2*avg2 + avg3) / 7
```

**Usage:** `ultimate_osc`
**Example:** `ultimate_osc < 30` — oversold (multi-timeframe confirmed)
**Example:** `ultimate_osc > 70` — overbought

---

## Volatility Indicators

### ATR(14)
Average True Range — measures volatility in absolute price terms.

**Pseudo-code:**
```
tr[i] = max(high-low, |high-prevClose|, |low-prevClose|)
out[period] = SMA(tr[0..period-1])
for i in [period+1, n-1]:
    out[i] = (out[i-1] * (period-1) + tr[i-1]) / period
```

**Usage:** `atr14`
**Example:** `atr14 < close * 0.01` — low absolute volatility

### ATR%
ATR normalized as percentage of price — comparable across price levels.

**Pseudo-code:**
```
out[i] = atr[i] / close[i]
```

**Usage:** `atr_pct`
**Example:** `atr_pct < 0.02` — daily range under 2% of price

### Bollinger Bands (20, 2.0)
Volatility bands around SMA20 — 95% of prices should fall within.

**Pseudo-code:**
```
mid[i] = SMA(close, 20)[i]
std = standard deviation of close over 20 bars
upper[i] = mid[i] + 2.0 * std
lower[i] = mid[i] - 2.0 * std
bandwidth[i] = (upper[i] - lower[i]) / mid[i]
pctB[i] = (close[i] - lower[i]) / (upper[i] - lower[i])
```

**Usage:** `bb_upper`, `bb_mid`, `bb_lower`, `bb_bandwidth`, `bb_pct_b`
**Example:** `bb_pct_b < 0.1` — near lower band (oversold)
**Example:** `bb_bandwidth < 0.05` — squeeze (low volatility)

### Historical Volatility (20)
Annualized standard deviation of daily returns.

**Pseudo-code:**
```
returns[i] = ln(close[i] / close[i-1])
std = standard deviation of returns over 20 bars
out[i] = std * sqrt(252)
```

**Usage:** `hist_vol20`
**Example:** `hist_vol20 < 0.20` — annualized vol under 20%

### Keltner Channels (20, 10, 2.0)
Volatility-based envelope using ATR — similar to Bollinger but with ATR instead of standard deviation.

**Pseudo-code:**
```
mid[i] = EMA(close, 20)[i]
atr[i] = ATR(close, 10)[i]
upper[i] = mid[i] + 2.0 * atr[i]
lower[i] = mid[i] - 2.0 * atr[i]
```

**Usage:** `keltner_upper`, `keltner_mid`, `keltner_lower`
**Example:** `close > keltner_upper` — breakout above Keltner Channel
**Example:** `close < keltner_lower` — breakdown below Keltner Channel

---

## Price Structure

### Rolling High/Low (252)
Highest and lowest price over N bars. Used for 52-week extremes.

**Pseudo-code:**
```
out[i] = max(high[i-period+1 .. i])
```

**Usage:** `high_52w`, `low_52w`
**Example:** `close >= high_52w` — at 52-week high

### Gap%
Overnight price gap as percentage.

**Pseudo-code:**
```
out[i] = (open[i] / close[i-1]) - 1
```

**Usage:** `gap_pct`
**Example:** `gap_pct > 0.03` — gapped up >3%

### True Range
Maximum price movement accounting for overnight gaps.

**Pseudo-code:**
```
out[i] = max(high-low, |high-prevClose|, |low-prevClose|)
```

**Usage:** `true_range`
**Example:** `true_range > 2 * atr14` — unusually large daily range

---

## Returns

All returns are computed as ratios: 0.05 = +5%, -0.03 = -3%.

**Pseudo-code:**
```
out[i] = (close[i] / close[i-period]) - 1
```

**Usage:** `ret_1d` (1), `ret_5d` (5), `ret_1m` (21), `ret_3m` (63), `ret_6m` (126), `ret_1y` (252)

**Example:** `ret_3m > 0.10 AND ret_1m > 0` — positive momentum on both timeframes

---

## Volume Indicators

### Dollar Volume
Split-invariant measure: close × volume. More reliable than raw share volume.

**Pseudo-code:**
```
out[i] = close[i] * volume[i]
```

**Usage:** `dollar_vol`
**Example:** `dollar_vol > 5000000` — trades > $5M/day

### Relative Volume
Current volume relative to 20-day average.

**Pseudo-code:**
```
avg = SMA(volume, 20)
out[i] = volume[i] / avg[i]
```

**Usage:** `rel_volume`
**Example:** `rel_volume > 2.0` — 2x normal volume

### OBV (On-Balance Volume)
Cumulative volume indicator showing buying/selling pressure.

**Pseudo-code:**
```
out[0] = volume[0]
for i in [1, n-1]:
    if close[i] > close[i-1]: out[i] = out[i-1] + volume[i]
    elif close[i] < close[i-1]: out[i] = out[i-1] - volume[i]
    else: out[i] = out[i-1]
```

**Usage:** `obv`
**Example:** (used for divergence detection via charting, not direct WHERE comparison)

### VWAP Distance
Percentage distance from volume-weighted average price over 20 days.

**Pseudo-code:**
```
twap[i] = sum(typical_price[j] * volume[j]) / sum(volume[j]) for j in [i-19, i]
out[i] = (close[i] / vwap[i]) - 1
```

**Usage:** `vwap_dist`
**Example:** `vwap_dist > 0` — trading above VWAP (bullish institutional flow)

---

## Cross Detection

Pre-computed boolean flags for common crossover events. Detect crossing on the current bar without requiring `prev_*` columns.

**Golden Cross (SMA50 > SMA200):**
```
isGoldenCross(i) = sma50[i] > sma200[i] AND sma50[i-1] <= sma200[i-1]
```

**Oversold Bounce (RSI14 > 30):**
```
isOversoldBounce(i) = rsi14[i] > 30 AND rsi14[i-1] <= 30
```

**Usage:** `golden_cross = 1`, `oversold_bounce = 1`

---

## Complete Column Reference

| Category | Columns |
|----------|---------|
| **Price** | close, high, low, open |
| **Trend — SMA** | sma5, sma10, sma20, sma30, sma50, sma100, sma200 |
| **Trend — EMA** | ema10, ema21, ema50, ema100, ema200 |
| **Trend — Derived** | pct_from_sma50, pct_from_sma200, ma_stack |
| **Trend — SAR** | psar |
| **Trend — Aroon** | aroon_up, aroon_down, aroon_osc |
| **Momentum** | rsi14, macd, macd_signal, macd_hist |
| **Momentum — Stoch** | stoch_k, stoch_d |
| **Momentum — Other** | willr14, cci20, roc10, roc20 |
| **Momentum — ADX** | adx14, di_plus, di_minus |
| **Momentum — Flow** | mfi14, cmf20, ultimate_osc |
| **Volatility** | atr14, atr_pct, hist_vol20 |
| **Volatility — BB** | bb_upper, bb_mid, bb_lower, bb_bandwidth, bb_pct_b |
| **Volatility — Keltner** | keltner_upper, keltner_mid, keltner_lower |
| **Price Structure** | high_52w, low_52w, is_52w_high, is_52w_low, gap_pct, true_range, pct_off_52w_high, pct_above_52w_low |
| **Returns** | ret_1d, ret_5d, ret_1m, ret_3m, ret_6m, ret_1y |
| **Volume** | dollar_vol, avg_dollar_vol20, rel_volume, obv, vwap_dist |
| **Cross Detection** | golden_cross, oversold_bounce |

**Total: 63 indicator columns**
