# Future Roadmap

## Phase 1: Foundation (Current)

### Status: In Progress

**Goals:**
- ✅ REST API foundation
- ✅ No-lookahead indicator calculations
- ✅ Core indicators (SMA, RSI, RollingHigh/Low, Return)
- ✅ Boolean cross detection
- ✅ Documentation restructure

**Remaining:**
- ⏳ Expand indicator set to 50+ indicators
- ⏳ Schema versioning
- ⏳ Snapshot history (90-day retention)
- ⏳ Additional API endpoints

**Timeline:** Q1 2026

---

## Phase 2: Indicator Expansion

### Status: Planned

**Goals:**
- Implement all planned indicators:
  - EMA variants (10, 21, 50, 100, 200)
  - MACD (line, signal, histogram)
  - ATR (14-day)
  - Bollinger Bands (upper, middle, lower, bandwidth, %B)
  - Additional moving averages (5, 10, 20, 30, 100)
  - Stochastic, Williams %R, CCI
  - Additional returns (1d, 5d, 1m, 6m, 1y)
  - Volume indicators (relative volume, avg dollar volume)
- Update snapshot schema
- Update feature catalog
- Add tests for all new indicators

**Benefits:**
- Richer study capabilities
- More filtering options
- Better support for technical analysis strategies

**Timeline:** Q2 2026

---

## Phase 3: Schema Versioning & History

### Status: Planned

**Goals:**
- Add schema_version table to cetus.db (pipeline side)
- Add version check to scanner startup
- Implement snapshot history:
  - Store daily snapshots (90-day retention)
  - Add snapshot_date column
  - Implement cleanup job
  - Add BackfillSnapshots(days) function
- Document versioning policy

**Benefits:**
- Detect breaking changes from upstream
- Enable historical analysis
- Support backtesting
- Audit trail

**Timeline:** Q2 2026

---

## Phase 4: Complete REST API

### Status: Planned

**Goals:**
- Implement remaining API endpoints:
  - `GET /api/v1/scan` - run ad-hoc study
  - `POST /api/v1/scan` - run study with JSON body
  - `GET /api/v1/studies` - list saved studies
  - `POST /api/v1/studies` - create study
  - `GET /api/v1/studies/{id}` - get study
  - `PUT /api/v1/studies/{id}` - update study
  - `DELETE /api/v1/studies/{id}` - delete study
  - `GET /api/v1/universe` - list symbols
  - `GET /api/v1/symbols/{symbol}` - get symbol data
  - `GET /api/v1/snapshots` - list historical snapshots
- Add authentication and authorization
- Add rate limiting
- Add API documentation (OpenAPI/Swagger)

**Benefits:**
- Full programmatic access
- Integration with external tools
- Multi-user support
- Study management

**Timeline:** Q3 2026

---

## Phase 5: Advanced Features

### Status: Future

**Goals:**
- Backtesting engine
  - Replay historical snapshots
  - Simulate strategy execution
  - Calculate performance metrics
- Strategy optimization
  - Parameter optimization
  - Walk-forward analysis
  - Monte Carlo simulation
- Portfolio analysis
  - Position sizing
  - Risk metrics
  - Correlation analysis
- Real-time alerts
  - Email notifications
  - Webhook integration
  - Custom alert rules

**Benefits:**
- Complete trading system
- Strategy development platform
- Risk management tools
- Automated monitoring

**Timeline:** Q4 2026+

---

## Phase 6: Integration & Distribution

### Status: Future

**Goals:**
- Integration with trading platforms
  - Alpaca API
  - Interactive Brokers
  - TD Ameritrade
- Study sharing and collaboration
  - Public study library
  - Community features
  - Study ratings and reviews
- Mobile app
  - iOS and Android
  - Push notifications
  - Offline mode
- White-label solution
  - Customizable branding
  - Multi-tenant architecture
  - API for partners

**Benefits:**
- End-to-end trading solution
- Community engagement
- Mobile accessibility
- Revenue opportunities

**Timeline:** 2027+

---

## Technical Debt & Improvements

### Ongoing

**Code Quality:**
- Increase test coverage
- Add integration tests
- Add performance benchmarks
- Refactor complex code

**Performance:**
- Optimize indicator calculations
- Add caching layer
- Parallelize snapshot rebuild
- Optimize query execution

**Observability:**
- Add metrics (Prometheus)
- Add distributed tracing
- Improve logging
- Add health checks

**Security:**
- Add authentication
- Add authorization
- Add audit logging
- Security review

---

## Decision Log

### Decision 1: No Lookahead Principle
**Date:** 2026-07-09
**Context:** Initial indicator implementation included current bar
**Decision:** All indicators must exclude current bar (bars < T)
**Rationale:** Prevents lookahead bias, ensures accurate backtests
**Consequences:** Removed prev_* columns, added boolean cross detection

### Decision 2: In-Memory Snapshot
**Date:** 2026-07-09
**Context:** Need fast query execution
**Decision:** Use in-memory SQLite for snapshot
**Rationale:** Fast queries, simple SQL language, no disk I/O
**Consequences:** Limited to current data, need daily rebuild

### Decision 3: Modular Indicators
**Date:** 2026-07-09
**Context:** Need to scale to 50+ indicators
**Decision:** Organize indicators by category in separate files
**Rationale:** Maintainability, scalability, clear organization
**Consequences:** Easier to add new indicators, better code organization

### Decision 4: REST API First
**Date:** 2026-07-09
**Context:** Need programmatic access
**Decision:** Build REST API, web dashboard consumes API
**Rationale:** Flexibility, integration, multi-client support
**Consequences:** More work upfront, but enables future features

---

## Success Metrics

### Phase 1
- [ ] All tests pass
- [ ] Build successful
- [ ] API endpoints working
- [ ] Documentation complete

### Phase 2
- [ ] 50+ indicators implemented
- [ ] All indicators tested
- [ ] Feature catalog updated
- [ ] Performance acceptable

### Phase 3
- [ ] Schema versioning working
- [ ] 90-day snapshot history
- [ ] Backfill capability
- [ ] Version check on startup

### Phase 4
- [ ] All API endpoints implemented
- [ ] Authentication working
- [ ] Rate limiting working
- [ ] API documentation complete

### Phase 5
- [ ] Backtesting engine working
- [ ] Strategy optimization working
- [ ] Portfolio analysis working
- [ ] Real-time alerts working

### Phase 6
- [ ] Trading platform integrations
- [ ] Study sharing platform
- [ ] Mobile app released
- [ ] White-label solution available
