# AI Trading Research Platform — Brainstorming Document

**Status:** Draft / Discussion  
**Created:** 2026-07-09  
**Purpose:** Capture AI/LLM research directions for the scanner platform

---

## Executive Summary

The scanner already has the hardest part: a SQL-queryable cross-sectional snapshot of 12k+ symbols with 50+ pre-computed indicators, plus 10 years of daily history. The AI layer doesn't replace this — it **reduces the friction** of using it and **extracts signal** that SQL alone can't.

This document explores independent research modules that can be built, tested, and validated separately. Each module is a hypothesis to test, not a feature to ship.

---

## Core Philosophy: Research Modules, Not a Platform

Each module is:
- **Independent** — can be enabled/disabled, improved, or discarded without affecting others
- **Hypothesis-driven** — "We believe X will help users make better decisions"
- **Measurable** — "We'll know it works if users who see Y do Z more often"
- **Composable** — modules can feed each other, but don't depend on each other

```
┌─────────────────────────────────────────────────────────┐
│              SCANNER CORE (existing)                    │
│  Snapshot + SQL WHERE + 10 years history + 12k symbols  │
└──────────────────────────┬──────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
          ▼                ▼                ▼
┌─────────────────┐ ┌──────────────┐ ┌──────────────┐
│  Research Mod A │ │ Research Mod B│ │ Research Mod C│
│  "Regime        │ │ "Similarity  │ │ "Historical  │
│   Detection"    │ │   Search"    │ │   Analogs"   │
└─────────────────┘ └──────────────┘ └──────────────┘
```

---

## Module A: Regime Detection

**Hypothesis:** Market regimes exist, and study performance varies by regime. If we can detect regimes, we can tell users which studies work best right now.

**Input:** 11 sector ETF indicators from today's snapshot (XLK, XLF, XLE, XLV, XLI, XLU, XLP, XLY, XLC, XLB, XLRE)

**Process:** LLM classifies regime (momentum, mean-reversion, rotation, etc.)

**Output:** `regime_cache` table (date → regime label + confidence + reasoning)

**Validation:** Do studies actually perform differently by regime? (Compare win rates across regimes. If no difference, the hypothesis is wrong — discard or refine.)

**Independence:** This module can run standalone. It doesn't need any other module. It produces a label that other modules *may* use, but doesn't depend on them.

**Risk:** Regimes may not be real (or may not be detectable from 11 ETFs). If validation fails, we discard this module.

### Implementation Sketch

```go
// Overnight batch job
func DetectRegime(snap *snapshot.DB, llm LLMClient) error {
    // Query 11 sector ETFs from today's snapshot
    etfs := []string{"XLK", "XLF", "XLE", "XLV", "XLI", "XLU", "XLP", "XLY", "XLC", "XLB", "XLRE"}
    
    var indicators []SectorIndicators
    for _, etf := range etfs {
        row := snap.GetSymbol(etf)
        indicators = append(indicators, SectorIndicators{
            Symbol: etf,
            RSI14: row.RSI14,
            MACD: row.MACD,
            PctFromSMA200: row.PctFromSMA200,
            Ret1m: row.Ret1m,
            Ret3m: row.Ret3m,
            // ... more indicators
        })
    }
    
    // Call LLM with structured prompt
    regime, err := llm.ClassifyRegime(indicators)
    if err != nil {
        return err
    }
    
    // Cache result
    return db.InsertRegime(time.Now(), regime)
}
```

---

## Module B: Similarity Search

**Hypothesis:** Stocks with similar recent price behavior will continue to behave similarly. If we can find "stocks like AAPL," users can discover opportunities they wouldn't have found otherwise.

**Input:** Return series for all 12k symbols (60/120/252 day windows)

**Process:** Compute cosine similarity (or Pearson correlation) in Go, store top-20 per symbol

**Output:** `similar_stocks` table (symbol → top-20 similar symbols + scores)

**Validation:** Do similar stocks actually have correlated forward returns? (Compare similarity score vs actual future correlation. If no relationship, the hypothesis is wrong.)

**Independence:** This module is pure Go + SQLite. No LLM, no dependencies. It can run standalone.

**Risk:** Similarity may not predict future behavior (market is not that predictable). If validation fails, we refine the similarity metric or discard.

### Implementation Sketch

```go
// Pure Go, no vector DB needed for 12k symbols
func ComputeSimilarity(snap *snapshot.DB) error {
    // Load return series for all symbols
    returns := make(map[string][]float64)
    for _, sym := range snap.AllSymbols() {
        returns[sym] = snap.GetReturns(sym, 60) // 60-day returns
    }
    
    // Compute pairwise similarity (12k × 12k = 144M ops, ~1 second)
    for sym1, ret1 := range returns {
        var similar []SimilarStock
        for sym2, ret2 := range returns {
            if sym1 == sym2 {
                continue
            }
            score := CosineSimilarity(ret1, ret2)
            similar = append(similar, SimilarStock{Symbol: sym2, Score: score})
        }
        // Keep top 20
        sort.Slice(similar, func(i, j int) bool { return similar[i].Score > similar[j].Score })
        db.InsertSimilar(sym1, similar[:20])
    }
}
```

---

## Module C: Historical Analogs

**Hypothesis:** Past patterns predict future returns. If a study matched 47 times in the past, and the average return was +4.2%, that's a useful signal.

**Input:** Study matches (from running studies against historical snapshots) + forward returns (from warehouse bars)

**Process:** For each historical match, compute 5d/20d/60d forward returns. Aggregate by study.

**Output:** `study_outcomes` table (study_key, date, symbol, entry_price, ret_5d, ret_20d, ret_60d)

**Validation:** Are historical outcomes actually predictive? (Compare backtested win rates vs live performance. If no relationship, the hypothesis is wrong.)

**Independence:** This module is pure Go + SQLite. No LLM, no dependencies. It can run standalone.

**Risk:** Historical performance may not predict future performance (overfitting, regime changes). If validation fails, we add regime conditioning (combine with Module A) or discard.

### Implementation Sketch

```go
// Already partially implemented in backtest package
func BacktestStudy(snap *snapshot.DB, study study.Study, startDate, endDate int64) error {
    dates := snap.ListSnapshotDates(startDate, endDate)
    
    for _, date := range dates {
        matches := snap.RunStudy(study, date)
        
        for _, match := range matches {
            // Compute forward returns
            ret5d := snap.GetForwardReturn(match.Symbol, date, 5)
            ret20d := snap.GetForwardReturn(match.Symbol, date, 20)
            ret60d := snap.GetForwardReturn(match.Symbol, date, 60)
            
            db.InsertOutcome(study.Key, date, match.Symbol, match.Close, ret5d, ret20d, ret60d)
        }
    }
}
```

---

## Module D: Natural Language → SQL

**Hypothesis:** Users can express trading ideas in natural language, and an LLM can translate them into correct SQL WHERE clauses. This lowers the barrier to creating studies.

**Input:** User's natural language description + schema documentation

**Process:** LLM generates SQL WHERE clause, validate against sandbox

**Output:** SQL WHERE clause (user confirms and saves as study)

**Validation:** Do users who use NL create more studies? Are the generated studies correct (do they match what the user intended)?

**Independence:** This is a UI feature + LLM call. It doesn't depend on any other module. It produces studies that other modules may analyze, but doesn't depend on them.

**Risk:** LLM may generate incorrect SQL (false positives/negatives). If validation fails, we improve the prompt or add a "verify" step.

### Implementation Sketch

```go
func TranslateToSQL(nl string, llm LLMClient) (string, error) {
    prompt := `You are a SQL query builder for a stock scanner. The snapshot table has these columns:
[schema documentation with column descriptions]

Examples:
- "oversold stocks above 200-day MA" → rsi14 < 35 AND close > sma200
- "strong momentum with high volume" → ret_3m > 0.15 AND dollar_vol > 1e7 AND rel_volume > 2.0

Generate a SQL WHERE clause for: ` + nl + `
Return ONLY the WHERE clause, no explanation.`
    
    where, err := llm.Complete(prompt)
    if err != nil {
        return "", err
    }
    
    // Validate against sandbox
    if err := study.ValidateClause(where); err != nil {
        return "", err
    }
    
    return where, nil
}
```

---

## Module E: AI Advisor

**Hypothesis:** Proactive suggestions (based on regime + study performance) improve user outcomes. If we tell users "your study underperforms in this regime," they'll adjust and do better.

**Input:** Regime (from Module A, optional) + study performance (from Module C, optional)

**Process:** LLM generates suggestions based on context

**Output:** `advisor_suggestions` table (date, study_key, suggestion, reasoning)

**Validation:** Do users who receive suggestions actually adjust their studies? Do their studies perform better after adjustment?

**Independence:** This module *can* use outputs from Modules A and C, but doesn't *require* them. It can run with just study performance data (no regime). If Modules A or C fail validation, this module still works (just less effective).

**Risk:** Suggestions may not be actionable or may be ignored. If validation fails, we refine the suggestion logic or discard.

---

## Module F: Explainable Signals

**Hypothesis:** Explaining *why* a match occurred (and what happened historically) increases user confidence and leads to more trades.

**Input:** Match details (which indicators triggered, how extreme) + historical outcomes (from Module C, optional)

**Process:** LLM generates narrative explanation

**Output:** `match_explanations` table (study_key, date, symbol, explanation, triggers, rarity)

**Validation:** Do explained matches get more trades? Do users who see explanations have higher confidence?

**Independence:** This module *can* use outputs from Module C, but doesn't *require* them. It can run with just match details (no historical context). If Module C fails validation, this module still works (just less informative).

**Risk:** Explanations may be verbose or unhelpful. If validation fails, we refine the narrative logic or discard.

---

## Proprietary Concepts (The "Art/Alchemy")

We can create a whole vocabulary of AI-tinted concepts that become *our* brand:

### 1. Market Temperature (Heat Map Concept)
- Each sector has a "temperature" based on momentum, breadth, volatility
- Visualized as a heat map (red = hot/momentum, blue = cold/mean-reversion)
- AI insight: "Tech is overheating (RSI > 70, 95% above SMA200). Energy is freezing (RSI < 30, only 20% above SMA200). Rotation likely."
- **Unique angle:** Not just "tech is up" — it's "tech is *overheating*" (a concept, not just a data point)

### 2. Momentum Gravity
- Some stocks pull others (e.g., NVDA pulls AMD, MSFT, GOOGL)
- Compute: for each stock, how correlated is it with the top-5 momentum leaders?
- AI insight: "NVDA has high gravity (0.87 correlation with top momentum leaders). When NVDA sneezes, tech catches a cold."
- **Unique angle:** Not just "NVDA is up" — it's "NVDA is a *momentum leader* that pulls others"

### 3. Volatility Compression (Coiled Springs)
- Stocks with low volatility (tight Bollinger Bands) that are about to break out
- Compute: Bollinger Band width < 10th percentile + approaching resistance
- AI insight: "AAPL is coiled (BB width = 2.1%, 5th percentile). Last 10 times this happened, AAPL averaged +8% in 20 days."
- **Unique angle:** Not just "AAPL is consolidating" — it's "AAPL is a *coiled spring*"

### 4. Correlation Clusters
- Groups of stocks that move in lockstep (high intra-cluster correlation)
- Compute: hierarchical clustering on 60-day returns
- AI insight: "Tech cluster (NVDA, AMD, MSFT, GOOGL, META) has 0.92 intra-cluster correlation. If one breaks down, they all follow."
- **Unique angle:** Not just "these stocks are correlated" — it's "these stocks form a *cluster* that moves as one"

### 5. Regime Shift Signals
- Early warnings that the regime is about to change
- Compute: divergence between sectors (e.g., defensives outperforming cyclicals)
- AI insight: "Regime shift detected: utilities (+5% 1m) outperforming tech (-3% 1m). Last 5 times this happened, momentum regime ended within 2 weeks."
- **Unique angle:** Not just "regime changed" — it's "regime is *about to change*" (predictive, not reactive)

### 6. Breadth Divergence
- When the index is up but breadth is narrowing (fewer stocks participating)
- Compute: % of stocks above SMA200 vs index performance
- AI insight: "SPX is up +2% this week, but only 45% of stocks are above SMA200 (down from 65%). Breadth divergence = fragile rally."
- **Unique angle:** Not just "SPX is up" — it's "SPX rally is *fragile* (breadth divergence)"

### 7. Relative Strength Leaders/Laggards
- Stocks that are outperforming/underperforming their sector
- Compute: stock return vs sector ETF return (1m/3m/6m)
- AI insight: "NVDA is a relative strength leader (+45% vs XLK's +22%). INTC is a laggard (-12% vs XLK's +22%)."
- **Unique angle:** Not just "NVDA is up" — it's "NVDA is a *sector leader*"

---

## Visualization Layer

### 1. Market Temperature Heat Map
- Grid of sectors (rows) × timeframes (columns: 1d, 1w, 1m, 3m, 6m)
- Color: red (hot/momentum) → blue (cold/mean-reversion)
- Interactive: click a cell to see the stocks in that sector/timeframe
- **AI overlay:** "Tech is overheating across all timeframes. Energy is freezing on 3m/6m."

### 2. Regime Timeline
- X-axis: time (last 2 years)
- Y-axis: regime classification (momentum, mean-reversion, rotation, etc.)
- Color: confidence (dark = high, light = low)
- **AI overlay:** "We've been in a momentum regime for 6 months. Last time this happened (2021), it ended abruptly in Feb 2022."

### 3. Correlation Cluster Map
- Network graph: stocks as nodes, correlations as edges
- Clusters are visually grouped (color-coded)
- **AI overlay:** "Tech cluster (red) has 0.92 intra-cluster correlation. If NVDA breaks down, watch for contagion."

### 4. Study Performance Dashboard
- X-axis: time
- Y-axis: cumulative return
- Line: study performance vs benchmark (SPX)
- **AI overlay:** "Your 'Oversold Above Trend' study is up +42% YTD vs SPX's +18%. Win rate: 62%."

### 5. Similarity Radar
- Radar chart: target stock (AAPL) in center
- Surrounding stocks: similar stocks (MSFT, GOOGL, NVDA, etc.)
- Axes: key indicators (RSI, MACD, returns, volatility)
- **AI overlay:** "AAPL's pattern matches MSFT (0.94 similarity). Both are coiled springs (BB width < 5th percentile)."

### 6. Lava-Lamp Regime Visualization (The "Wow" Factor)
- **Concept:** Flowing blobs represent sector momentum, color = temperature, size = momentum, movement = rotation
- **Data mapping:**
  - Sector return (1m) → Blob size
  - RSI14 → Blob color (red/green/blue)
  - Relative strength Δ → Blob vertical movement
  - Correlation (60d) → Blob merging/splitting
  - Volatility (ATR/VIX) → Flow speed (viscosity)
  - Breadth (% > SMA200) → Blob count (density)
- **Regime detection:**
  - Momentum regime: large red blobs rising
  - Mean-reversion: small blue blobs sinking
  - Rotation: blobs moving horizontally
  - High volatility: fast, chaotic flow
  - Low volatility: slow, smooth flow
- **Why it works:** Humans are pattern-matching machines — a lava-lamp makes patterns *visceral*
- **Uniqueness:** No one else does this. Most scanners use static visualizations (heat maps, bar charts). Dynamic flow visualizations are rare.

---

## Audio Layer (The Fun Part)

Sounds can be:
- **Alerts:** different tones for different regimes/signals
  - Momentum regime: ascending chime (optimistic)
  - Mean-reversion regime: descending tone (cautious)
  - Regime shift: alarm (attention!)
- **Ambient:** background sounds that reflect market state
  - Hot market: fast tempo, major key
  - Cold market: slow tempo, minor key
  - High volatility: dissonant, chaotic
- **Feedback:** sounds when you interact with the app
  - Save study: satisfying "click"
  - Match found: positive chime
  - Backtest complete: triumphant fanfare

**The vision:** The app becomes a *sensory experience*, not just a data dump. You can *feel* the market state.

---

## Newsletter Strategy

The newsletter isn't just a distribution channel — it's a **research lab** where you test concepts before building them into the app.

**The flow:**
1. AI pipeline processes data overnight (regime detection, similarity, historical analogs)
2. You curate the most interesting insights into a newsletter
3. Readers get actionable intelligence (not just data dumps)
4. Feedback loops back: "this was useful" → build into app; "this was confusing" → refine or discard

**The newsletter becomes:**
- A way to build an audience (free tier)
- A way to test concepts (research lab)
- A way to create proprietary content (not just "scanner found X")
- A conversion funnel (free newsletter → paid app)

### Free Tier (Build Audience)
- **Weekly market regime report:** "This week's regime: Broad Momentum (high confidence). Tech is leading, energy is lagging. Here's what to watch."
- **Top 3 study matches:** "These 3 studies matched this week: 'Oversold Above Trend' (47 matches), 'Golden Cross' (23 matches), 'Momentum Leaders' (15 matches)."
- **One proprietary concept:** "Market Temperature: Tech is overheating (RSI > 70, 95% above SMA200). Here's what that means."

### Paid Tier (Convert to App Users)
- **Daily regime updates:** "Regime shifted from 'momentum' to 'rotation' yesterday. Here's what to do."
- **Full study backtests:** "Your 'Oversold Above Trend' study has a 62% win rate over 10 years. Here's the equity curve."
- **Similarity search:** "Stocks like AAPL: MSFT (0.94), GOOGL (0.91), NVDA (0.87). Here's what to watch."
- **AI advisor:** "Your 'Oversold Bounce' study underperforms in momentum regimes. Consider pausing it."

### App (Monetize)
- **Free tier:** 3 studies, basic screening
- **Pro tier ($25/mo):** unlimited studies, alerts, backtesting
- **Pro+ tier ($50/mo):** AI features (regime detection, similarity, advisor)

---

## The Overnight Pipeline

```
Market closes (4 PM ET)
    │
    ▼
Pipeline ingests bars (existing)
    │
    ▼
Scanner computes today's snapshot (existing)
    │
    ▼
AI pipeline runs overnight (NEW):
    │
    ├── 1. Regime detection
    │       Input: 11 sector ETF indicators from today's snapshot
    │       LLM call: 1 (classify regime)
    │       Output: regime_cache table
    │       Cost: ~$0.001
    │       Time: ~2s
    │
    ├── 2. Similarity pre-computation
    │       Input: return series for all 12k symbols
    │       Go computation: cosine similarity matrix
    │       Output: similar_stocks table (top-20 per symbol)
    │       Cost: $0 (pure Go)
    │       Time: ~1-5s
    │
    ├── 3. Historical analog update
    │       Input: today's snapshot + all user studies
    │       Go computation: run each study against today's snapshot,
    │                       compute forward returns for yesterday's matches
    │       Output: study_outcomes table
    │       Cost: $0 (pure Go)
    │       Time: ~5-30s (depends on # studies)
    │
    ├── 4. AI advisor
    │       Input: regime (from step 1) + study performance (from step 3)
    │       LLM calls: 1 per user with active studies (or top-N studies)
    │       Output: advisor_suggestions table
    │       Cost: ~$0.005
    │       Time: ~5-30s
    │
    └── 5. Explainable signals
            Input: new matches from today (symbols that entered studies)
            LLM calls: 1 per new match (or batch multiple matches per call)
            Output: match_explanations table
            Cost: ~$0.05
            Time: ~10-60s
    │
    ▼
All results cached in SQLite
    │
    ▼
User opens dashboard (8 AM next day)
Everything is instant — regime, suggestions, similar stocks, backtests
```

**Total overnight cost:** ~$0.06/day = **~$1.80/month in LLM calls.** Negligible.

---

## Technical Decisions

### 1. No Vector DB (for now)

| Scale | Approach | Why |
|-------|----------|-----|
| 12k symbols | Go + SQLite (brute force cosine) | ~3M ops = microseconds. No index needed. |
| 100k symbols | Go + SQLite (brute force) | ~25M ops = ~1ms. Still fine. |
| 1M+ symbols | Add vector DB (Qdrant/Weaviate) | ANN index needed for sub-ms latency. |

**When to add a vector DB:** Only when brute-force Go becomes too slow. For our scale, it's unnecessary complexity.

### 2. LLM API Choice

| Option | Cost | Latency | Quality |
|--------|------|---------|---------|
| OpenAI GPT-4o-mini | $0.15/1M input tokens | ~500ms | Good |
| Anthropic Claude Haiku | $0.25/1M input tokens | ~300ms | Good |
| Local model (Llama 3) | $0 (need GPU) | ~200ms | OK |

**Recommendation:** Start with GPT-4o-mini (cheap, fast, good enough). Cache aggressively (regime detection = 1 call/day).

### 3. Storage Estimates

| Data | Size | Where |
|------|------|-------|
| 10 years snapshots (30M rows × 50 cols) | ~12 GB | SQLite on disk |
| Embeddings (12k symbols × 252 dims × 8 bytes) | ~24 MB | SQLite BLOBs |
| Correlation top-K (12k × 20 × 16 bytes) | ~4 MB | SQLite table |
| Study outcomes (10 years × 100 studies × 50 matches/day) | ~50 MB | SQLite table |
| Regime cache (10 years × 365 days × 1 KB) | ~4 MB | SQLite table |

**Total: ~12 GB** (dominated by snapshots, which we already have).

---

## Implementation Order

| Phase | Time | Dependencies | Revenue Impact |
|-------|------|-------------|---------------|
| 1. Regime Detection | 1-2 weeks | Sector ETFs in snapshot, LLM API | Medium (context) |
| 2. Historical Analogs | 2-3 weeks | Forward returns, study outcomes | Very High (backtesting) |
| 3. NL → SQL | 1-2 weeks | LLM API, schema docs | High (accessibility) |
| 4. Similarity Search | 2-3 weeks | Return series computation | High ("stocks like X") |
| 5. Explainable Signals | 1-2 weeks | Phase 2 | Medium (actionability) |
| 6. AI Advisor | 2-3 weeks | Phases 1 + 2 | High (retention) |

**Total: 9-15 weeks** (sequential), or 5-8 weeks with parallelism.

**Recommended order:** 1 → 2 → 3 → 4 → 5 → 6

**Why this order:**
- Phase 1 (regime) is standalone, quick win
- Phase 2 (analogs/backtesting) is the highest-value feature — do it next
- Phase 3 (NL → SQL) makes the product accessible — do it before similarity
- Phase 4 (similarity) is cool but not as valuable as backtesting
- Phase 5 (explainable) builds on Phase 2
- Phase 6 (advisor) needs everything else first

---

## The Full Loop (Product Vision)

```
User describes idea in plain English (Module D)
        │
        ▼
AI translates to SQL study
        │
        ▼
AI backtests over 10 years of snapshots (Module C)
        │
        ▼
AI shows equity curve, win rate, regime performance
        │
        ▼
User saves study, AI monitors for matches daily
        │
        ▼
AI detects regime (Module A) + finds similar stocks (Module B)
        │
        ▼
AI sends alert: "AAPL entered your study — here's why (Module F),
   here's what happened last time (Module C),
   here's similar stocks (Module B)"
        │
        ▼
AI suggests improvements (Module E)
        │
        ▼
User refines study → loop continues
```

**This is the full loop: idea → expression → validation → monitoring → learning.**

No other platform does this.

---

## What This Unlocks (Revenue)

| Feature | Tier | Price Impact |
|---------|------|-------------|
| Core scanner + alerts | Pro ($25/mo) | Baseline |
| Regime detection | Pro ($25/mo) | Retention |
| Historical analogs / backtesting | Pro+ ($50/mo) | +$25/mo |
| NL → SQL | Pro+ ($50/mo) | +$25/mo |
| Similarity search | Pro+ ($50/mo) | Retention |
| AI Advisor | Pro+ ($50/mo) | Retention |
| Explainable signals | Pro+ ($50/mo) | Retention |

**The AI features justify the $50/mo Pro+ tier.** The core scanner is $25/mo; the AI layer doubles the price.

---

## Questions for Discussion

1. **Which module do you want to build first?**
   - Module A (regime) — quick win, 1-2 weeks
   - Module C (historical analogs) — highest value, 2-3 weeks
   - Module D (NL → SQL) — accessibility, 1-2 weeks
   - Or all three in parallel?

2. **Should I verify sector ETFs are in the warehouse?** (Quick query, needed for Module A)

3. **LLM provider preference?** (OpenAI, Anthropic, local — affects all modules that use LLM)

4. **Validation plan:** How do you want to measure whether each module "works"? (User surveys? A/B testing? Performance metrics?)

5. **Scope:** Build all 6 modules, or start with 2-3 and iterate?

6. **Newsletter:** Do you want to start the newsletter *now* (before the app is built), or wait until the AI modules are working?

7. **Lava-lamp visualization:** Is this a serious priority, or just fun to think about? (It's actually a cool idea — regime shifts as flowing blobs.)

8. **Audio layer:** Is this a serious priority, or just fun to think about? (I can include it, but mark it as "nice to have" if you want.)

---

## Next Steps

1. **Finish Tier-1 headless scanner** (current focus)
2. **Build PWA UI** (next phase)
3. **Start AI modules** (after PWA is stable)
4. **Launch newsletter** (can start before app is fully built)

---

**Document status:** Ready for discussion  
**Last updated:** 2026-07-09
