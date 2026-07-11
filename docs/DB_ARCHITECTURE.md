# Database Architecture — DBA Reference

## Philosophy

**SQLite is the single source of truth.** Every byte of application data lives in SQLite tables. JSONL files are export-only — generated on demand via `ExportJSONL(w)` and `ExportJSON(w)`, never synced on writes. No Go-side data structures duplicate what SQLite already provides.

**Go handles math, SQLite handles data.** Indicator computation, signal detection, and business logic run in Go. Filtering, sorting, aggregation, indexing, and persistence run in SQLite. Never load data into Go maps just to filter or sort it — SQLite does that faster.

## Schema

### Version Tracking

```sql
CREATE TABLE schema_version (
    version   INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);
```

| Version | Description |
|---------|-------------|
| 1 | Initial: snapshot table |
| 2 | Groups, permissions, saved_results, studies, subscriptions |
| 3 | Roles, users, user_limits, usage_tracking, rate_limits |

Migrations run idempotently at startup via `schema.Migrate(db)`. Only pending versions are applied. See `internal/schema/migration.go`.

### Core Tables

#### users
Source of truth for authentication, roles, and throttling.

```sql
CREATE TABLE users (
    id       TEXT PRIMARY KEY,
    name     TEXT NOT NULL DEFAULT '',
    role_id  TEXT NOT NULL DEFAULT 'user' REFERENCES roles(id),
    disabled INTEGER NOT NULL DEFAULT 0
);
```

CRUD via `user.Store`. All mutations write to this table. Passwords (PBKDF2 hashed) are stored only in the in-memory cache, never in SQLite.

#### roles
Role-based access control definitions.

```sql
CREATE TABLE roles (
    id                      TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    description             TEXT,
    capabilities_json       TEXT NOT NULL,
    limits_json             TEXT NOT NULL,
    default_permissions_json TEXT NOT NULL,
    can_manage_users        INTEGER DEFAULT 0,
    can_manage_groups       INTEGER DEFAULT 0,
    bypass_throttling       INTEGER DEFAULT 0,
    created_at              INTEGER NOT NULL,
    updated_at              INTEGER NOT NULL
);
```

Bootstrapped from `roles.json` on first run. Roles define:
- **Capabilities**: fine-grained access control (e.g., `user.create`, `study.read`)
- **Limits**: rate limits and resource quotas
- **Permissions**: default Linux-style owner/group/all bits

#### studies
User-defined scan filters.

```sql
CREATE TABLE studies (
    key          TEXT PRIMARY KEY,
    owner        TEXT NOT NULL DEFAULT 'global',
    visibility   TEXT NOT NULL DEFAULT 'private',
    group_name   TEXT NOT NULL DEFAULT '',
    tier         TEXT NOT NULL DEFAULT 'free',
    title        TEXT NOT NULL DEFAULT '',
    emoji        TEXT NOT NULL DEFAULT '',
    where_clause TEXT NOT NULL DEFAULT '',
    order_by     TEXT NOT NULL DEFAULT '',
    limit_num    INTEGER NOT NULL DEFAULT 0
);
```

#### subscriptions
User-to-study subscriptions.

```sql
CREATE TABLE subscriptions (
    user_id   TEXT NOT NULL,
    study_key TEXT NOT NULL,
    PRIMARY KEY (user_id, study_key)
);
```

#### groups
User groups for collaborative access.

```sql
CREATE TABLE groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    owner_id    TEXT NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE group_members (
    group_id  TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   TEXT NOT NULL,
    role      TEXT NOT NULL DEFAULT 'member',  -- 'member' | 'leader'
    joined_at INTEGER NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX idx_group_members_user ON group_members(user_id);
```

#### saved_results
User-saved scan results with Linux-style permissions.

```sql
CREATE TABLE saved_results (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       TEXT NOT NULL,
    study_id      TEXT NOT NULL,
    snapshot_date INTEGER NOT NULL,
    results_json  TEXT NOT NULL,
    created_at    INTEGER NOT NULL,
    name          TEXT,
    notes         TEXT,
    perm_owner    INTEGER NOT NULL DEFAULT 7,   -- rwx
    perm_group    INTEGER NOT NULL DEFAULT 0,   ---
    perm_all      INTEGER NOT NULL DEFAULT 0,   ---
    group_id      TEXT REFERENCES groups(id) ON DELETE SET NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_saved_results_user ON saved_results(user_id, created_at DESC);
CREATE INDEX idx_saved_results_date ON saved_results(snapshot_date DESC);
```

#### result_acls
Per-user access control list entries.

```sql
CREATE TABLE result_acls (
    result_id  INTEGER NOT NULL REFERENCES saved_results(id) ON DELETE CASCADE,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission INTEGER NOT NULL,   -- 0-7 (r=4, w=2, x=1)
    granted_at INTEGER NOT NULL,
    granted_by TEXT NOT NULL,
    PRIMARY KEY (result_id, user_id)
);
```

#### snapshot
Cross-sectional feature snapshot — one row per (date, symbol). 50+ indicator columns. The primary key `(snapshot_date, symbol)` enables O(log N) indexed lookups for backtest and replay.

```sql
CREATE TABLE snapshot (
    snapshot_date INTEGER,
    symbol        TEXT,
    timestamp     INTEGER,
    close  REAL, high REAL, low REAL, open REAL,
    sma5   REAL, sma10  REAL, sma20  REAL, sma30  REAL,
    sma50  REAL, sma100 REAL, sma200 REAL,
    ema10  REAL, ema21  REAL, ema50  REAL, ema100 REAL, ema200 REAL,
    pct_from_sma50  REAL, pct_from_sma200 REAL, ma_stack INTEGER,
    rsi14      REAL, macd REAL, macd_signal REAL, macd_hist REAL,
    stoch_k    REAL, stoch_d REAL, willr14 REAL, cci20 REAL,
    roc10      REAL, roc20  REAL, adx14 REAL, di_plus REAL, di_minus REAL,
    atr14      REAL, atr_pct REAL,
    bb_upper   REAL, bb_mid REAL, bb_lower REAL, bb_bandwidth REAL, bb_pct_b REAL,
    hist_vol20 REAL,
    high_52w   REAL, low_52w  REAL, is_52w_high INTEGER, is_52w_low INTEGER,
    gap_pct    REAL, true_range REAL,
    pct_off_52w_high  REAL, pct_above_52w_low REAL,
    ret_1d REAL, ret_5d REAL, ret_1m REAL, ret_3m REAL, ret_6m REAL, ret_1y REAL,
    dollar_vol REAL, avg_dollar_vol20 REAL, rel_volume REAL,
    obv REAL, vwap_dist REAL, mfi14 REAL,
    golden_cross INTEGER, oversold_bounce INTEGER,
    PRIMARY KEY (snapshot_date, symbol)
);
```

#### Throttling Tables

```sql
CREATE TABLE user_limits (
    user_id     TEXT PRIMARY KEY REFERENCES users(id),
    limits_json TEXT NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE usage_tracking (
    user_id          TEXT NOT NULL,
    date             TEXT NOT NULL,   -- YYYY-MM-DD
    api_calls        INTEGER DEFAULT 0,
    studies_created  INTEGER DEFAULT 0,
    results_saved    INTEGER DEFAULT 0,
    replays_run      INTEGER DEFAULT 0,
    PRIMARY KEY (user_id, date)
);

CREATE TABLE rate_limits (
    user_id   TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    action    TEXT NOT NULL
);
CREATE INDEX idx_rate_limits_timestamp ON rate_limits(timestamp);
```

## Connection Management

A single SQLite connection is used for all scanner operations:

```go
// internal/snapshot/snapshot.go
func Open(path string, log *slog.Logger) (*DB, error) {
    dsn := ":memory:"
    if path != "" && path != ":memory:" {
        dsn = "file:" + path
    }
    rawDB, err := sql.Open("sqlite", dsn)
    rawDB.SetMaxOpenConns(1)  // single connection for in-memory persistence
    db := dblog.New(rawDB, log)
    return &DB{db: db}, nil
}
```

**Connection pool:** `SetMaxOpenConns(1)` ensures the in-memory database survives across goroutines. For persistent databases, increase this to `runtime.NumCPU()`.

**Read-only warehouse:** The cetus pipeline database is opened with `file:cetus.db?mode=ro` + `PRAGMA busy_timeout=5000`. WAL mode allows concurrent reads alongside the pipeline writer.

## Logging (`dblog`)

Every SQL operation flows through `internal/dblog/dblog.go`:

```go
db := dblog.New(rawDB, log)

// All operations automatically logged:
db.Query("SELECT ...", args...)       // Debug: query + duration
db.Exec("INSERT ...", args...)         // Debug or Error: query + args + duration
db.QueryRow("SELECT ...", args...)     // Debug: query + duration
db.Begin()                             // Debug: duration
db.Prepare("...")                      // Debug: query + duration

// Raw access when needed (migrations, setMaxOpenConns):
rawDB := db.DB()
```

**Log levels:**
- `Debug`: Successful query, normal operation
- `Error`: Failed query with full arguments for debugging

**Stats:** `dblog.DB.Stats()` logs connection pool metrics (max_open, open, in_use, idle, wait_count, wait_duration).

## Store Patterns

Every data store follows a consistent pattern:

### Constructor
```go
func NewStore(db *sql.DB) *Store           // SQLite-backed
func OpenStoreWithDB(db *sql.DB, path string) (*Store, error)  // with JSONL seed
```

### CRUD
```go
func (s *Store) All() []T            // read all
func (s *Store) Find(id string) (T, bool)  // read one
func (s *Store) Create(t T) error    // insert
func (s *Store) Update(t T) error    // update
func (s *Store) Delete(id string) error  // delete
```

### Import/Export
```go
func (s *Store) ExportJSON(w io.Writer) error      // wrapped JSON with metadata
func (s *Store) ImportJSON(r io.Reader) (int, error)   // from wrapped JSON
func (s *Store) ExportJSONL(w io.Writer) error     // line-by-line JSONL
func (s *Store) ImportJSONL(path string) error     // from JSONL file
```

### Mutex Pattern
All stores use `sync.Mutex` for thread safety:
```go
func (s *Store) Create(u User) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    // validate, insert, return
}
```

## Permission Model

Linux-style permission bits stored as integers 0-7:

| Bit | Value | Meaning |
|-----|-------|---------|
| r | 4 | Read |
| w | 2 | Write |
| x | 1 | Delete |

**Common combinations:**
- `7` (rwx) — full control
- `5` (r-x) — read + delete
- `4` (r--) — read only
- `0` (---) — no access

**Defaults:** `perm_owner=7, perm_group=0, perm_all=0` (private by default).

**Resolution priority:** Admin > ACL > Owner > Group > All.

See `internal/permissions/permissions.go` and `internal/permissions/access.go`.

## Indexed Lookups

Key queries use the primary key index for O(log N) performance:

```go
// SymbolClose — PK lookup, <1ms
SELECT close FROM snapshot WHERE snapshot_date = ? AND symbol = ?

// NearestDate — MIN on indexed column
SELECT MIN(snapshot_date) FROM snapshot WHERE snapshot_date >= ?
```

Never load data into Go maps for lookups — use indexed SQL queries instead.

## SQL Injection Prevention

All user-authored SQL passes through `ValidateClause` (`internal/study/study.go`):

```go
func ValidateClause(clause string) error {
    // Block: ; -- /* */ attach detach pragma load_extension
    //        union select insert delete update replace
    //        create drop alter exec trigger savepoint
}
```

The structured study editor (`internal/predicate`) is injection-proof by construction — the browser sends opaque IDs, the server maps them to known column names and operators. No raw user SQL reaches the database.

All programmatic queries use parameterized placeholders (`?`), never string concatenation:

```go
// ✅ Correct
db.QueryRow("SELECT close FROM snapshot WHERE snapshot_date = ? AND symbol = ?", date, symbol)

// ❌ Never
db.QueryRow("SELECT close FROM snapshot WHERE symbol = '" + symbol + "'")
```

## Backup & Recovery

### Export (full dump)
```go
w, _ := os.Create("backup.json")
userStore.ExportJSON(w)     // wrapped JSON with metadata
studyStore.ExportJSON(w)
resultsStore.ExportJSON(w)
```

### Import (restore)
```go
r, _ := os.Open("backup.json")
userStore.ImportJSON(r)
studyStore.ImportJSON(r)
resultsStore.ImportJSON(r)
```

### SQLite Backup
```bash
sqlite3 scanner.db ".backup scanner-$(date +%Y%m%d).db"
```

### Export JSONL (for external tools)
```go
w, _ := os.Create("users.jsonl")
userStore.ExportJSONL(w)
```

## Performance Guidelines

1. **Batch inserts** — use transactions for bulk operations (`LoadHistoryBatch`)
2. **Indexed columns** — primary keys are indexed; add indexes for frequent WHERE columns
3. **Avoid N+1** — use joins or subqueries instead of per-row lookups
4. **Single connection** — for in-memory DBs, `SetMaxOpenConns(1)` is required
5. **WAL mode** — read-only warehouse DB uses WAL for concurrent reads
6. **SQLite does the work** — filter with WHERE, sort with ORDER BY, aggregate with GROUP BY. Go does math.

## Migration Checklist

When adding a new table or column:

1. ✅ Add `CREATE TABLE IF NOT EXISTS` to `schema/migration.go`
2. ✅ Bump `CurrentVersion` constant
3. ✅ Add migration call in `Migrate()`
4. ✅ Create Go store package with CRUD methods
5. ✅ Add `ExportJSON` / `ImportJSON` methods
6. ✅ Register API routes in `router.go`
7. ✅ Update this document
