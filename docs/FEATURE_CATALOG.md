# Feature Catalog & Parameter Documentation

## Summary

All 50+ indicators in the scanner have been documented with their parameter windows in the feature metadata. This ensures users can understand exactly what calculations are being performed when they use each indicator in their studies.

## Documentation Standard

Each feature includes:
- **ID**: Unique identifier (e.g., `sma200`, `rsi14`)
- **Name**: Human-readable name (e.g., "SMA 200", "RSI 14")
- **ShortDesc**: Brief description for UI tooltips (includes parameter window)
- **LongDesc**: Detailed explanation with parameter window and usage guidance
- **Category**: Grouping (trend, momentum, volatility, etc.)
- **DataType**: numeric, price, volume, or boolean
- **Sortable**: Whether the feature can be used in ORDER BY clauses
- **WikiURL**: Link to Investopedia or other reference material

## Indicators with Parameter Windows

### Trend Indicators
- **SMA**: 5, 10, 20, 30, 50, 100, 200 days
- **EMA**: 10, 21, 50, 100, 200 days
- **% From SMA**: 50, 200 days
- **MA Stack**: Uses EMA 10, 21, 50, 200

### Momentum Indicators
- **RSI**: 14-day
- **MACD**: 12-26-9 (EMA 12, EMA 26, Signal 9)
- **Stochastic**: 14,3,3 (%K 14, %D 3, smoothing 3)
- **Williams %R**: 14-day
- **CCI**: 20-day
- **MFI**: 14-day
- **ROC**: 10-day, 20-day
- **ADX**: 14-day
- **DI+/DI-**: 14-day

### Volatility Indicators
- **ATR**: 14-day
- **ATR %**: 14-day (normalized)
- **Bollinger Bands**: 20-day, 2σ (upper, middle, lower)
- **BB Bandwidth**: 20-day, 2σ
- **BB %B**: 20-day, 2σ
- **Historical Volatility**: 20-day

### Price Structure Indicators
- **52-Week High/Low**: 252 trading days
- **Is 52-Week High/Low**: 252 trading days (boolean)
- **Gap %**: Single-day calculation
- **True Range**: Single-day calculation

### Return Indicators
- **Returns**: 1-day, 5-day, 21-day (1-month), 63-day (3-month), 126-day (6-month), 252-day (1-year)

### Volume Indicators
- **Dollar Volume**: Single-day (close × volume)
- **Avg Dollar Vol**: 20-day SMA
- **Relative Volume**: 20-day average
- **OBV**: Cumulative (no parameter window)
- **VWAP Distance**: Single-day calculation

### Price Indicators (OHLCV)
- **Open, High, Low, Close, Volume**: Single-day values (split-adjusted)

## Single-Day Indicators (No Parameter Window)

These indicators don't require a parameter window because they're calculated from a single bar:
- OHLCV data (open, high, low, close, volume)
- Gap % (overnight gap)
- True Range (daily volatility)
- Dollar Volume (daily activity)
- VWAP Distance (intraday benchmark)
- OBV (cumulative volume)

## Testing

All features are tested in `internal/features/registry_test.go`:
- `TestRegistry`: Verifies all features have required fields
- `TestByID`: Tests feature lookup by ID
- `TestByCategory`: Tests category filtering
- `TestCategories`: Verifies all categories exist
- `TestFeatureCount`: Ensures we have 40+ features

## API Endpoints

The feature catalog is exposed via REST API:
- `GET /api/v1/features` - List all features
- `GET /api/v1/features?category=trend` - Filter by category
- `GET /api/v1/features/{id}` - Get specific feature

## Future Enhancements

1. **Custom Parameter Windows**: Allow users to specify custom periods (e.g., RSI 7 instead of RSI 14)
2. **Multi-Timeframe Analysis**: Add weekly/monthly indicators
3. **Indicator Combinations**: Pre-built study templates using multiple indicators
4. **Parameter Validation**: Ensure users can't create invalid combinations

## Maintenance

When adding new indicators:
1. Add to `internal/features/features.go` with complete metadata
2. Include parameter window in both ShortDesc and LongDesc
3. Add tests to verify the feature is properly registered
4. Update this documentation

## References

- Investopedia: https://www.investopedia.com/
- Feature metadata: `internal/features/features.go`
- API documentation: `docs/API.md`
- Indicator catalog: `docs/INDICATORS.md`
