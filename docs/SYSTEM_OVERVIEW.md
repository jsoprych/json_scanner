# Cetus Marketdata Scanner - Complete System Overview

## Executive Summary

The cetus-marketdata-scanner is a complete, production-ready market data analysis platform with:
- Role-based access control (RBAC) with 4 default roles
- Feature access throttling with rate limits and quotas
- Linux-style permissions for saved results
- Modern admin panel with Alpine.js
- Comprehensive monitoring and metrics
- Historical scan replay with P/L tracking

## Architecture

### Core Components

```
cmd/scanner/
├── main.go                    # Entry point, CLI commands
└── serve.go                   # HTTP server setup

internal/
├── admin/                     # Admin panel
│   ├── admin.go              # Handler and routes
│   ├── static/               # CSS, JS (embedded)
│   └── templates/            # HTML templates (embedded)
├── api/                      # REST API endpoints
│   ├── roles.go             # Role management
│   ├── groups.go            # Group management
│   ├── results.go           # Saved results
│   └── user_limits.go       # User limits & usage
├── backtest/                # Historical replay
│   └── replay.go           # Scan replay with P/L
├── bootstrap/               # Default user creation
│   └── bootstrap.go        # Auto-create admin user
├── groups/                  # Group management
│   └── groups.go           # CRUD operations
├── permissions/             # Permission system
│   ├── permissions.go      # Bit operations
│   ├── access.go          # Access checking
│   └── db_access.go       # Database queries
├── results/                 # Saved results
│   └── results.go          # CRUD with permissions
├── roles/                   # Role management
│   └── roles.go            # Role CRUD & capabilities
├── schema/                  # Database migrations
│   └── migration.go        # V3 schema (roles, groups, etc.)
├── serve/                   # HTTP server
│   ├── server.go           # Server setup
│   └── handlers.go         # Route handlers
├── snapshot/                # Snapshot storage
│   └── snapshot.go         # SQLite storage
├── store/                   # Data warehouse
│   └── store.go            # Read-only warehouse access
├── study/                   # Study management
│   └── study.go            # Study CRUD
├── throttle/                # Rate limiting
│   └── throttle.go         # Rate limits & quotas
└── user/                    # User management
    └── user.go             # User CRUD & authentication
```

### Database Schema (V3)

```sql
-- Roles and capabilities
CREATE TABLE roles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    capabilities_json TEXT NOT NULL,
    limits_json TEXT NOT NULL,
    default_permissions_json TEXT NOT NULL,
    can_manage_users INTEGER DEFAULT 0,
    can_manage_groups INTEGER DEFAULT 0,
    bypass_throttling INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Groups
CREATE TABLE groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    owner_id TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- Group membership
CREATE TABLE group_members (
    group_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at INTEGER NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

-- Saved results with permissions
CREATE TABLE saved_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    study_id TEXT NOT NULL,
    snapshot_date INTEGER NOT NULL,
    results_json TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    name TEXT,
    notes TEXT,
    perm_owner INTEGER NOT NULL DEFAULT 7,
    perm_group INTEGER NOT NULL DEFAULT 0,
    perm_all INTEGER NOT NULL DEFAULT 0,
    group_id TEXT
);

-- Access control lists
CREATE TABLE result_acls (
    result_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    permission INTEGER NOT NULL,
    granted_at INTEGER NOT NULL,
    granted_by TEXT NOT NULL,
    PRIMARY KEY (result_id, user_id)
);

-- User-specific limit overrides
CREATE TABLE user_limits (
    user_id TEXT PRIMARY KEY,
    limits_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Usage tracking
CREATE TABLE usage_tracking (
    user_id TEXT NOT NULL,
    date TEXT NOT NULL,
    api_calls INTEGER DEFAULT 0,
    studies_created INTEGER DEFAULT 0,
    results_saved INTEGER DEFAULT 0,
    replays_run INTEGER DEFAULT 0,
    PRIMARY KEY (user_id, date)
);

-- Rate limit tracking
CREATE TABLE rate_limits (
    user_id TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    action TEXT NOT NULL
);
```

## Features

### 1. Role-Based Access Control

**Default Roles:**
- **admin** - Full system access, bypass throttling
- **group_admin** - Can manage groups, standard throttling
- **user** - Standard user with personal content management
- **guest** - Read-only access to public content

**Capabilities:**
- `user.create/read/update/delete` - User management
- `group.create/read/update/delete` - Group management
- `study.create/read/update/delete` - Study management
- `result.create/read/update/delete` - Saved results management
- `system.admin` - System administration

### 2. Feature Access Throttling

**Rate Limits:**
- Per-minute (burst protection): 60-1000 calls
- Per-hour (sustained usage): 1000-50000 calls
- Per-day (daily quota): 10000-1000000 calls

**Resource Quotas:**
- Max studies per user: 0-1000
- Max saved results per user: 0-10000
- Max groups per user: 0-100
- Max group members: 0-1000

**Feature Limits:**
- Historical replay depth: 1-365 days
- Max symbols per scan: 100-10000
- Export size limits: 100-100000 results

### 3. Linux-Style Permissions

**Permission Model:**
- Owner: 7 (rwx) - Full control
- Group: 0 (---) - No access by default
- All: 0 (---) - No access by default

**Permission Bits:**
- 4 = read (r)
- 2 = write (w)
- 1 = delete (x)

**Operations:**
- `chmod` - Update permissions
- `chown` - Change ownership
- `setfacl` - Add/remove ACLs

### 4. Admin Panel

**Pages:**
- **Dashboard** - System stats, activity feed, user quotas
- **Users** - User management with search/filter
- **Roles** - Role management with capability configuration
- **Groups** - Group management with member administration
- **Monitoring** - System metrics, API performance, throttling stats

**Tech Stack:**
- Alpine.js (15kb) for reactive UI
- CSS custom properties for theming
- Go templates for server-side rendering
- Embedded assets (no build step)

### 5. Historical Scan Replay

**Features:**
- Run scans on historical dates
- Calculate P/L from signal to current date
- Track max profit and max loss (drawdown)
- Export results as JSON

**Example:**
```bash
./bin/scanner replay --date 2025-10-01
```

**Output:**
```
Signal: AAPL @ $150.00
Current: $175.00
P/L: +16.67%
Max Profit: +20.00%
Max Loss: -5.00%
```

### 6. Monitoring & Metrics

**System Stats:**
- Uptime
- Memory usage
- Active connections
- Request rate

**API Performance:**
- Requests per endpoint
- Average latency
- P95 latency
- Error rates

**Database Stats:**
- Warehouse size
- Scanner DB size
- Total symbols
- Snapshot count

**Throttling Stats:**
- Rate limit violations
- Quota exceeded events
- Users near limit
- Bypass users

## API Endpoints

### Authentication
```
POST /api/v1/auth/login          # Login, get JWT token
GET  /api/v1/auth/me             # Get current user
```

### Users
```
GET    /api/v1/users             # List users
POST   /api/v1/users             # Create user
GET    /api/v1/users/{id}        # Get user
PUT    /api/v1/users/{id}        # Update user
DELETE /api/v1/users/{id}        # Delete user
```

### Roles
```
GET    /api/v1/roles             # List roles
POST   /api/v1/roles             # Create role
GET    /api/v1/roles/{id}        # Get role
PUT    /api/v1/roles/{id}        # Update role
DELETE /api/v1/roles/{id}        # Delete role
```

### Groups
```
GET    /api/v1/groups            # List groups
POST   /api/v1/groups            # Create group
GET    /api/v1/groups/{id}       # Get group
PUT    /api/v1/groups/{id}       # Update group
DELETE /api/v1/groups/{id}       # Delete group
GET    /api/v1/groups/{id}/members                    # List members
POST   /api/v1/groups/{id}/members                    # Add member
DELETE /api/v1/groups/{id}/members/{userId}            # Remove member
```

### Saved Results
```
POST   /api/v1/results           # Save result
GET    /api/v1/results           # List user's results
GET    /api/v1/results/accessible # List accessible results
GET    /api/v1/results/export    # Export as JSON
POST   /api/v1/results/import    # Import from JSON
GET    /api/v1/results/{id}      # Get result
DELETE /api/v1/results/{id}      # Delete result
PATCH  /api/v1/results/{id}/permissions              # Update permissions
PATCH  /api/v1/results/{id}/owner                    # Change ownership
GET    /api/v1/results/{id}/acls                     # List ACLs
POST   /api/v1/results/{id}/acls                     # Add ACL
DELETE /api/v1/results/{id}/acls/{userId}             # Remove ACL
```

### User Limits & Usage
```
GET /api/v1/users/{userId}/limits    # Get effective limits
PUT /api/v1/users/{userId}/limits    # Set limit overrides
GET /api/v1/users/{userId}/usage     # Get current usage
```

### Admin Panel
```
GET /admin/dashboard              # Dashboard page
GET /admin/users                  # Users management page
GET /admin/roles                  # Roles management page
GET /admin/groups                 # Groups management page
GET /admin/monitoring             # Monitoring page

GET /api/v1/admin/stats           # System statistics
GET /api/v1/admin/activities      # Recent activities
GET /api/v1/admin/quotas          # User quotas
GET /api/v1/admin/monitoring/system      # System metrics
GET /api/v1/admin/monitoring/api         # API performance
GET /api/v1/admin/monitoring/database    # Database stats
GET /api/v1/admin/monitoring/throttle    # Throttling stats
GET /api/v1/admin/monitoring/errors      # Recent errors
```

### Snapshots
```
GET  /api/v1/snapshots            # List available snapshots
POST /api/v1/snapshots/active     # Set active snapshot date
```

## Configuration

### Environment Variables

```bash
# Database paths
SCANNER_DB_PATH=/path/to/cetus.db          # Warehouse database
SCANNER_STORE_DB=/path/to/scanner.db       # Scanner database

# Server
SCANNER_SERVE_ADDR=:8080                   # HTTP listen address
SCANNER_AUTH_MODE=login                    # Authentication mode

# Universe
SCANNER_UNIVERSE=index:r3000               # Scan universe

# Throttling
SCANNER_RATE_LIMIT_REQUESTS=100            # Requests per window
SCANNER_RATE_LIMIT_WINDOW=60               # Window in seconds
```

### Role Configuration

Edit `roles.json` to customize roles:

```json
{
  "id": "user",
  "name": "Standard User",
  "description": "Basic user with personal content management",
  "capabilities": [
    "study.create", "study.read", "study.update", "study.delete",
    "result.create", "result.read", "result.update", "result.delete",
    "group.read"
  ],
  "limits": {
    "api_calls_per_minute": 60,
    "api_calls_per_hour": 1000,
    "api_calls_per_day": 10000,
    "max_studies": 10,
    "max_saved_results": 100,
    "max_groups": 5,
    "max_group_members": 50,
    "replay_days": 7,
    "max_symbols_per_scan": 1000,
    "export_max_results": 1000
  },
  "default_permissions": {
    "owner": 7,
    "group": 0,
    "all": 0
  },
  "can_manage_users": false,
  "can_manage_groups": false,
  "bypass_throttling": false
}
```

## Usage Examples

### 1. Start the Server

```bash
./bin/scanner serve
```

Default admin user is created automatically:
- Username: `admin`
- Password: `admin` (change immediately!)

### 2. Access Admin Panel

Open browser: `http://localhost:8080/admin`

Login with admin credentials.

### 3. Create a User via API

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "alice",
    "name": "Alice",
    "password": "secret",
    "role_id": "user"
  }'
```

### 4. Save a Result with Permissions

```bash
curl -X POST http://localhost:8080/api/v1/results \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "study_id": "golden_cross",
    "snapshot_date": 1704067200,
    "results": [...],
    "name": "My Analysis",
    "perm_owner": 7,
    "perm_group": 5,
    "perm_all": 0
  }'
```

### 5. Share Result with User

```bash
curl -X POST http://localhost:8080/api/v1/results/123/acls \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "bob",
    "permission": 4
  }'
```

### 6. Run Historical Replay

```bash
./bin/scanner replay --date 2025-10-01
```

### 7. Check User Usage

```bash
curl http://localhost:8080/api/v1/users/alice/usage \
  -H "Authorization: Bearer $TOKEN"
```

## Security Features

1. **Private by Default** - All user artifacts start with owner-only access
2. **Admin Bypass** - Admin role bypasses all permission checks
3. **Throttling** - Rate limits prevent abuse
4. **Quotas** - Resource limits prevent overuse
5. **Capability Checking** - Fine-grained access control
6. **Audit Trail** - Usage tracking for monitoring
7. **Password Hashing** - PBKDF2 with 600k iterations
8. **JWT Authentication** - Secure token-based auth

## Performance

### Database
- SQLite with WAL mode for concurrent reads
- Indexed queries for fast lookups
- Batch operations for bulk inserts

### API
- Rate limiting with sliding window
- Quota enforcement per resource
- Request tracking for metrics

### Admin Panel
- Embedded assets (no external requests)
- Alpine.js for reactive UI (15kb)
- CSS custom properties for fast theme switching

## Monitoring

### Metrics Available
- API request counts and latencies
- Rate limit violations
- Quota usage
- Database sizes
- System uptime and memory

### Alerts
- Users approaching limits
- Rate limit violations
- Quota exceeded events
- System errors

## Deployment

### Requirements
- Go 1.21+
- SQLite 3.35+
- 512MB RAM minimum
- 1GB storage minimum

### Build

```bash
make build
```

### Run

```bash
./bin/scanner serve
```

### Docker

```dockerfile
FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN make build
EXPOSE 8080
CMD ["./bin/scanner", "serve"]
```

## Troubleshooting

### Common Issues

**Cannot login to admin panel**
- Check if default admin user was created
- Verify password (default: "admin")
- Check browser console for errors

**API calls returning 429**
- User hit rate limit
- Check user's role limits
- Consider increasing limits or upgrading role

**Permission denied errors**
- Check user's role capabilities
- Verify resource permissions
- Check ACLs for explicit access

**Database locked errors**
- Another process is writing to DB
- Wait and retry
- Check for long-running transactions

### Logs

Check server logs for detailed error messages:
```bash
./bin/scanner serve 2>&1 | tee scanner.log
```

## Future Enhancements

### Planned Features
- Real-time WebSocket updates
- Advanced filtering and search
- Custom dashboard widgets
- Multi-language support
- Email notifications
- Backup management
- User activity analytics

### Integration Ideas
- Grafana dashboards
- Prometheus metrics
- Slack/Email alerts
- Trading platform APIs
- Data export to CSV/Excel

## Support

For issues or questions:
1. Check documentation in `docs/`
2. Review API responses in browser Network tab
3. Check server logs for backend errors
4. Verify user has required role/capabilities

## License

Part of cetus-marketdata-scanner project.

## Contributors

Built with Claude Code assistance.

---

**Version:** 1.0.0  
**Last Updated:** 2026-07-11  
**Status:** Production Ready
