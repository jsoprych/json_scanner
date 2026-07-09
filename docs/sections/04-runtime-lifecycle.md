# Runtime Lifecycle

## Overview

The scanner runs as a long-lived process that maintains an in-memory snapshot of
market data with computed indicators. The snapshot is rebuilt daily when new data
arrives from the cetus pipeline.

## Startup Sequence

1. **Initialize Configuration**
   - Load environment variables
   - Set up logging
   - Validate configuration

2. **Open Database Connections**
   - Open cetus.db (read-only)
   - Create in-memory snapshot database
   - Set up connection pooling

3. **Build Initial Snapshot**
   - Load universe (symbols to scan)
   - For each symbol:
     - Load historical bars (~400 days)
     - Compute indicators (no lookahead)
     - Insert into snapshot table
   - Log snapshot statistics

4. **Start API Server**
   - Bind to configured port
   - Register API routes
   - Start HTTP server
   - Log startup complete

## Daily Rebuild Cycle

### Trigger
The snapshot is rebuilt when:
- New data arrives from cetus pipeline (typically after market close)
- Manual trigger via admin API
- Scheduled rebuild (configurable)

### Rebuild Process
1. **Load New Data**
   - Read latest bars from cetus.db
   - Identify symbols with new data

2. **Recompute Indicators**
   - For each symbol with new data:
     - Load historical bars
     - Compute indicators (no lookahead)
     - Update snapshot table

3. **Update Metadata**
   - Update snapshot timestamp
   - Update symbol count
   - Log rebuild statistics

4. **Notify Clients**
   - Invalidate cached results
   - Notify connected clients (future)

## Snapshot Management

### In-Memory Storage
- SQLite database in memory (`:memory:`)
- One row per symbol
- Indexed by symbol (primary key)
- No persistence to disk

### Memory Usage
- **Per symbol**: ~300 bytes (30 columns × 8 bytes + overhead)
- **14,000 symbols**: ~4.2 MB
- **With 50 indicators**: ~6 MB
- **Peak during rebuild**: ~10 MB (temporary buffers)

### Rebuild Performance
- **Current (7 indicators)**: ~8.5 seconds
- **Estimated (50 indicators)**: ~60 seconds
- **Bottleneck**: Indicator computation (CPU-bound)
- **Optimization**: Parallel computation per symbol

## Shutdown Sequence

1. **Receive Shutdown Signal**
   - SIGTERM or SIGINT
   - Set shutdown flag

2. **Stop Accepting Requests**
   - Close HTTP listener
   - Drain in-flight requests

3. **Clean Up Resources**
   - Close database connections
   - Release memory
   - Log shutdown complete

## Monitoring

### Health Checks
- `GET /api/v1/health` returns:
  - Status (ok/error)
  - Snapshot timestamp
  - Symbol count
  - Uptime

### Metrics (Future)
- Snapshot build time
- Query execution time
- Memory usage
- Error rates
- Request rates

### Logging
- Structured JSON logs
- Log levels: debug, info, warn, error
- Key events:
  - Startup/shutdown
  - Snapshot rebuild start/complete
  - Errors and warnings

## Configuration

### Environment Variables
```bash
# Database
SCANNER_DB_PATH=/path/to/cetus.db

# Server
SCANNER_SERVE_ADDR=:8080
SCANNER_SERVE_TIMEOUT=30s

# Snapshot
SCANNER_UNIVERSE=index:r3000
SCANNER_LOOKBACK_DAYS=400

# Logging
SCANNER_LOG_LEVEL=info
```

### Runtime Changes
Most configuration requires restart. Future enhancements may support:
- Hot reload of configuration
- Dynamic universe updates
- On-demand snapshot rebuild

## Error Handling

### Database Errors
- Read errors: Log and continue (skip symbol)
- Write errors: Log and abort rebuild
- Connection errors: Retry with backoff

### Computation Errors
- Invalid data: Skip symbol, log warning
- NaN values: Store as NULL in database
- Timeout: Abort rebuild, log error

### API Errors
- Invalid requests: Return 400 with error message
- Internal errors: Return 500, log error
- Rate limiting: Return 429 (future)

## Security

### Database Access
- cetus.db opened read-only
- No write access to upstream data
- Snapshot is isolated (in-memory)

### API Security
- Authentication (future)
- Rate limiting (future)
- Input validation
- SQL injection prevention (parameterized queries)

### Resource Limits
- Max symbols per universe
- Max query timeout
- Max concurrent requests
- Memory limits
