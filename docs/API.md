# Scanner REST API Reference

**Base URL:** `/api/v1`

**Authentication:** JWT token in `Authorization: Bearer <token>` header (same as web dashboard)

**Content-Type:** `application/json`

---

## Endpoints

### Health & Metadata

#### `GET /api/v1/health`

Health check endpoint. Returns status and snapshot metadata.

**Response:**
```json
{
  "status": "ok",
  "snapshot_date": "2026-07-09",
  "snapshot_id": "2026-07-09T16:30:00Z",
  "symbol_count": 11370,
  "column_count": 50
}
```

---

### Features Catalog

#### `GET /api/v1/features`

Returns metadata for all available indicators/features.

**Query Parameters:**
- `category` (optional) - filter by category: `trend`, `momentum`, `volatility`, `price_structure`, `returns`, `volume`

**Response:**
```json
{
  "features": [
    {
      "id": "sma200",
      "name": "SMA 200",
      "short_desc": "200-day simple moving average",
      "long_desc": "The 200-day simple moving average of closing prices. Widely used to identify long-term trend direction. Price above SMA200 indicates bullish trend.",
      "category": "trend",
      "data_type": "numeric",
      "sortable": true,
      "wiki_url": "https://wiki.chartgeometry.com/indicators/sma200"
    }
  ]
}
```

#### `GET /api/v1/features/{id}`

Returns metadata for a single feature.

**Response:** Same as above, single object.

---

### Scanning

#### `GET /api/v1/scan`

Run a study against the current snapshot.

**Query Parameters:**
- `where` (required) - SQL WHERE clause (e.g., `close > sma200 AND rsi14 < 35`)
- `sort` (optional) - column to sort by (e.g., `ret_3m`)
- `direction` (optional) - `asc` or `desc` (default: `desc`)
- `limit` (optional) - max results (default: 25, max: 100)

**Response:**
```json
{
  "query": {
    "where": "close > sma200 AND rsi14 < 35",
    "sort": "ret_3m",
    "direction": "desc",
    "limit": 25
  },
  "matches": [
    {
      "symbol": "AAPL",
      "close": 310.69,
      "sma200": 285.42,
      "rsi14": 32.5,
      "ret_3m": 0.20,
      "dollar_vol": 488800000
    }
  ],
  "match_count": 15,
  "snapshot_date": "2026-07-09"
}
```

#### `POST /api/v1/scan`

Run a study with a JSON body (alternative to query params).

**Request Body:**
```json
{
  "where": "close > sma200 AND rsi14 < 35",
  "sort": "ret_3m",
  "direction": "desc",
  "limit": 25
}
```

**Response:** Same as GET above.

---

### Studies (Saved)

#### `GET /api/v1/studies`

List the authenticated user's saved studies.

**Response:**
```json
{
  "studies": [
    {
      "id": "study_abc123",
      "owner": "alice",
      "name": "Oversold Above Trend",
      "where": "close > sma200 AND rsi14 < 35",
      "sort": "ret_3m",
      "direction": "desc",
      "limit": 25,
      "created_at": "2026-07-01T10:30:00Z",
      "updated_at": "2026-07-08T14:20:00Z"
    }
  ]
}
```

#### `POST /api/v1/studies`

Create a new study.

**Request Body:**
```json
{
  "name": "Oversold Above Trend",
  "where": "close > sma200 AND rsi14 < 35",
  "sort": "ret_3m",
  "direction": "desc",
  "limit": 25
}
```

**Response:**
```json
{
  "id": "study_abc123",
  "owner": "alice",
  "name": "Oversold Above Trend",
  "where": "close > sma200 AND rsi14 < 35",
  "sort": "ret_3m",
  "direction": "desc",
  "limit": 25,
  "created_at": "2026-07-09T10:30:00Z",
  "updated_at": "2026-07-09T10:30:00Z"
}
```

#### `GET /api/v1/studies/{id}`

Get a specific study by ID.

**Response:** Same as above, single object.

#### `PUT /api/v1/studies/{id}`

Update a study.

**Request Body:** Same as POST.

**Response:** Same as POST.

#### `DELETE /api/v1/studies/{id}`

Delete a study.

**Response:**
```json
{
  "deleted": true,
  "id": "study_abc123"
}
```

---

### Symbols

#### `GET /api/v1/universe`

List all symbols in the current snapshot.

**Response:**
```json
{
  "symbols": ["AAPL", "MSFT", "GOOGL", ...],
  "count": 11370
}
```

#### `GET /api/v1/symbols/{symbol}`

Get snapshot data for a single symbol.

**Response:**
```json
{
  "symbol": "AAPL",
  "snapshot_date": "2026-07-09",
  "data": {
    "open": 308.50,
    "high": 312.00,
    "low": 307.80,
    "close": 310.69,
    "volume": 1574000,
    "sma200": 285.42,
    "rsi14": 61.2,
    "ret_3m": 0.20,
    "dollar_vol": 488800000,
    ...
  }
}
```

---

### Snapshot History

#### `GET /api/v1/snapshots`

List available historical snapshots (last 90 days).

**Response:**
```json
{
  "snapshots": [
    {
      "date": "2026-07-09",
      "symbol_count": 11370,
      "created_at": "2026-07-09T16:30:00Z"
    },
    {
      "date": "2026-07-08",
      "symbol_count": 11368,
      "created_at": "2026-07-08T16:30:00Z"
    }
  ]
}
```

#### `GET /api/v1/scan?date=2026-07-01&where=...`

Run a study against a historical snapshot.

**Query Parameters:**
- `date` (optional) - snapshot date (default: latest)
- Other parameters same as `/api/v1/scan`

---

## Error Responses

All errors return:
```json
{
  "error": "error message",
  "code": "ERROR_CODE"
}
```

**Common error codes:**
- `INVALID_WHERE` - malformed WHERE clause
- `INVALID_SORT` - unknown sort column
- `UNAUTHORIZED` - missing or invalid JWT
- `NOT_FOUND` - resource not found
- `RATE_LIMITED` - too many requests

---

## Rate Limiting

- Free tier: 100 requests/minute
- Pro tier: 1000 requests/minute

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1625846400
```

---

## Examples

### cURL Examples

```bash
# Health check
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/health

# List features
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/features

# Run a scan
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/scan?where=close%3Esma200&sort=ret_3m&limit=10"

# Create a study
curl -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Study","where":"rsi14<30","sort":"ret_3m"}' \
  http://localhost:8080/api/v1/studies

# Get symbol data
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/symbols/AAPL
```

### Python Example

```python
import requests

base_url = "http://localhost:8080/api/v1"
headers = {"Authorization": f"Bearer {TOKEN}"}

# Get features
features = requests.get(f"{base_url}/features", headers=headers).json()

# Run a scan
scan = requests.get(
    f"{base_url}/scan",
    params={"where": "close > sma200", "sort": "ret_3m", "limit": 10},
    headers=headers
).json()

for match in scan["matches"]:
    print(f"{match['symbol']}: {match['close']}")
```
