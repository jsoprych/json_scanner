# Indicator Comparison — Cetus vs. Popular Scanners

## Summary

| Scanner | Total Indicators | Custom WHERE | Historical Replay | Price |
|---------|-----------------|-------------|-------------------|-------|
| **Cetus** | **50+** | ✅ Full SQL | ✅ P/L from any date | Free (self-hosted) |
| Yahoo Finance | 12 | ❌ Dropdown only | ❌ | Free |
| Finviz Free | 65+ | ❌ Dropdown only | ❌ | Free |
| Finviz Elite | 65+ | ❌ Dropdown only | ❌ | $39.50/mo |
| TradingView | 100+ | ✅ Pine Script | ⚠️ Visual replay | Free–$59.95/mo |
| TC2000 | 100+ | ✅ EasyScan | ❌ | $9.99–$39.99/mo |
| Trade Ideas | 60+ | ✅ Custom alerts | ❌ | $84–$199/mo |

## Detailed Comparison

### Trend Indicators

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| SMA (5,10,20,30,50,100,200) | ✅ All 7 | ✅ SMA20,50,200 | ✅ | ✅ |
| EMA (10,21,50,100,200) | ✅ All 5 | ❌ | ✅ EMA20 | ✅ |
| % from SMA50/SMA200 | ✅ | ❌ | ❌ | ✅ via Pine |
| MA Stack (EMA alignment) | ✅ | ❌ | ❌ | ❌ |

**Cetus advantage:** Full SMA/EMA lineup at every common period. MA Stack is unique — no other scanner has a built-in "perfect bullish alignment" check.

### Momentum Indicators

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| RSI (14) | ✅ | ✅ | ✅ | ✅ |
| MACD (line/signal/hist) | ✅ All 3 | ✅ MACD only | ✅ | ✅ |
| Stochastic (%K, %D) | ✅ Both | ✅ | ✅ | ✅ |
| Williams %R | ✅ | ❌ | ✅ | ✅ |
| CCI (20) | ✅ | ❌ | ✅ | ✅ |
| Rate of Change (10,20) | ✅ Both | ❌ | ✅ ROC | ✅ |
| ADX (14) + DI+/DI- | ✅ All 3 | ❌ | ✅ ADX | ✅ |
| Money Flow Index | ✅ | ✅ | ✅ | ✅ |

**Cetus advantage:** Complete MACD triplet (line + signal + histogram). ADX with both +DI and -DI for full directional analysis.

### Volatility Indicators

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| ATR (14) | ✅ | ✅ | ✅ | ✅ |
| ATR as % of price | ✅ | ❌ | ❌ | ❌ |
| Bollinger Bands (full) | ✅ All 3 | ✅ | ✅ | ✅ |
| BB Bandwidth (%B) | ✅ | ❌ | ❌ | ✅ via Pine |
| Historical Volatility 20 | ✅ | ❌ | ✅ | ✅ |

**Cetus advantage:** ATR% normalizes volatility across price levels — unique and practical for position sizing.

### Price Structure

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| 52-Week High/Low | ✅ | ✅ | ✅ | ✅ |
| Is at 52w High/Low (bool) | ✅ | ❌ | ❌ | ✅ via Pine |
| % off 52w High | ✅ | ❌ | ✅ | ✅ |
| % above 52w Low | ✅ | ❌ | ✅ | ✅ |
| Gap % | ✅ | ❌ | ✅ | ✅ |
| True Range | ✅ | ❌ | ✅ | ✅ |

### Performance/Returns

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| 1-Day Return | ✅ | ❌ | ✅ | ✅ |
| 5-Day Return | ✅ | ❌ | ✅ | ✅ |
| 1-Month Return (21d) | ✅ | ❌ | ✅ | ✅ |
| 3-Month Return (63d) | ✅ | ❌ | ✅ | ✅ |
| 6-Month Return (126d) | ✅ | ❌ | ✅ | ✅ |
| 1-Year Return (252d) | ✅ | ❌ | ✅ | ✅ |

**Cetus advantage:** Trading-day-accurate periods (21d for 1m, 63d for 3m, 252d for 1y). Most scanners use calendar months.

### Volume Indicators

| Indicator | Cetus | Yahoo | Finviz | TradingView |
|-----------|-------|-------|--------|-------------|
| Dollar Volume | ✅ | ❌ | ❌ | ✅ via Pine |
| Average Dollar Vol (20d) | ✅ | ❌ | ❌ | ❌ |
| Relative Volume | ✅ | ❌ | ✅ | ✅ |
| On-Balance Volume | ✅ | ❌ | ✅ | ✅ |
| VWAP Distance | ✅ | ❌ | ❌ | ✅ |

**Cetus advantage:** Dollar volume is split-invariant — more reliable than raw share volume with free-tier data. Average dollar volume for liquidity filtering.

### Cross Detection (Built-in Booleans)

| Signal | Cetus | Yahoo | Finviz | TradingView |
|--------|-------|-------|--------|-------------|
| Golden Cross (SMA50 > SMA200) | ✅ | ❌ | ❌ | ❌ |
| Oversold Bounce (RSI14 > 30) | ✅ | ❌ | ❌ | ❌ |

**Cetus advantage:** Pre-computed cross events as boolean flags. No need to write `sma50 > sma200 AND prev_sma50 <= prev_sma200` — just `golden_cross = 1`.

## Unique Cetus Advantages

### 1. Custom WHERE Clauses (SQL)
No other free scanner lets you write arbitrary SQL WHERE clauses combining any indicator:
```sql
close > sma200 AND rsi14 BETWEEN 55 AND 70 AND dollar_vol > 5000000
AND adx14 > 25 AND di_plus > di_minus
```

Yahoo/Finviz force you through dropdown menus. TradingView has Pine Script but it's a separate language.

### 2. Historical Replay with P/L
- Scan any past date → see what signals fired
- Calculate P/L from that date to today
- Track max profit and max drawdown
- **No other scanner does this**

### 3. No Symbol Limits
Scan the entire Russell 3000 (or all common stocks) in one pass. Finviz free caps at 50 rows. TradingView watchlists cap at 1,000 symbols.

### 4. Self-Hosted / No Subscriptions
Zero monthly fees. Full access to all indicators, no tier restrictions. Comparable service would cost $39-199/month.

### 5. SQLite-First Architecture
All indicators pre-computed into a single indexed table. WHERE clauses execute in milliseconds against 11,000+ symbols. No API rate limits, no page restrictions.

## What Cetus Doesn't Have (Yet)

### Missing Indicators
| Indicator | Present in |
|-----------|-----------|
| Ichimoku Cloud | TradingView, TC2000 |
| Parabolic SAR | TradingView, TC2000 |
| Fibonacci Retracement | TradingView, Finviz |
| Keltner Channels | TradingView |
| Chaikin Money Flow | TradingView, Finviz |
| Elder Ray Index | TradingView |
| Heikin-Ashi candles | TradingView |

### Missing Features
| Feature | Present in |
|---------|-----------|
| Real-time scanning | TradingView, Trade Ideas |
| Mobile app | TC2000, TradingView |
| Alert push notifications | Trade Ideas, Benzinga |
| Paper trading integration | TradingView, Thinkorswim |
| Options screening | Thinkorswim, Barchart |
| Social/community sharing | TradingView |

## Bottom Line

Cetus has **50+ indicators** covering all major categories — more than Yahoo Finance (12), competitive with Finviz (65+), and approaching TradingView (100+). 

**Where Cetus wins:**
- Custom SQL WHERE clauses (unmatched flexibility)
- Historical replay with P/L (unique)
- No symbol limits
- No monthly fees
- Split-invariant dollar volume calculations

**Where competitors win:**
- Real-time data and alerts
- Mobile apps
- Community and social features
- More exotic indicators (Ichimoku, SAR, Fibonacci)
