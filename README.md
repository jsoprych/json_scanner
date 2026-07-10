# cetus-marketdata-scanner

Ultra-lean **market-data scanner** in Go. It reads the split-adjusted EOD bars
produced by the **[cetus-marketdata-pipeline](https://github.com/jsoprych/cetus-marketdata-pipeline)**
warehouse (read-only) and turns them into signals, a daily digest, and a live
dashboard.

**Ethos:** pure Go, zero CGO, single static binary, minimal dependencies
(`modernc.org/sqlite` — same driver as the pipeline), structured JSON logging.

```
cetus-marketdata-pipeline  ──(SQLite: adjusted_bars)──►  cetus-marketdata-scanner
       (ingestion — upstream)                                 (scan — this repo)
```

## Modes

| Command | What it does |
|---------|--------------|
| `scanner` / `scanner scan` | Per-symbol **JSONL signal stream** (volume / price / gap breakouts) on stdout |
| `scanner digest` | **Daily post-close digest** — whole-universe snapshot → market breadth + preset studies (52-wk highs, golden cross, oversold bounce, momentum leaders), rendered `html` / `text` / `json` |
| `scanner serve` | Live HTTP dashboard with **per-session login**. **`/login`** signs in; **`/`** = user dashboard (breadth + the acting user's studies); **`/admin`** = operator console + **user CRUD** + **study editor** (write/test/save SQL-`WHERE` studies with a live preview), **admin-only**. Sign out switches users. Cached scan (`?refresh=1`) |
| `scanner anomalies` | **Data-quality pass** (Sentinel Tier-0): flags extreme-move × thin-liquidity × price/200-DMA outliers as text/JSONL — the deterministic seam the future cross-source + LLM tiers extend |
| `scanner studies` | Materializes the snapshot into the scanner's **own SQLite store** and runs a user's **tier-accessible SQL-`WHERE` studies** (`studies.jsonl`) — pre-DSL, SQL is the study language |
| `scanner backfill` | **Historical snapshot backfill** — builds date-stamped snapshots for the past N days (`SCANNER_BACKFILL_DAYS`), with retention cleanup (`SCANNER_SNAPSHOT_RETENTION_DAYS`). Requires a persistent `SCANNER_STORE_DB` |
| `scanner users` | List the seeded users (id · tier · role) from `users.jsonl` |

Study access has **three independent axes** (ugo-inspired, but not raw rwx):

- **visibility** `private` \| `group` \| `public` — who may see it (you always see
  your own; `group` requires membership; unset ⇒ global studies are public);
- **tier** `free` \| `pro` — the subscription entitlement gate (separate from
  visibility — it's billing, not an ACL);
- **role** — an **admin** sees everything.

Users (`users.jsonl`) carry `groups: []`; `SCANNER_USER` picks the acting user. The
digest/dashboard render exactly that user's accessible studies. Example: `alice`
(pro, group `desk-a`) sees the pro and desk-a studies; `bob` (free) sees only the
free public ones.

**Self-service studies.** Any user can author their own studies from `/` ("My
Studies": write / **Test WHERE** live-preview / save), with guardrails: owner is
forced to self, tier to free, `public` is admin-only (users get `private`/`group`),
WHERE/ORDER-BY are sandbox-checked, and **creation is tier-capped** —
`SCANNER_FREE_STUDY_QUOTA` (default 3) for free, unlimited for pro/admin. Admins
author any study on `/admin`.

Any mode scans the universe chosen by `SCANNER_UNIVERSE` (`all` · `exchange:NASDAQ`
· `list:sp500` · `file:tickers.txt`).

> **Phase 1 (MVP).** The `digest`/`serve` path is built on a small fixed set of
> indicators (SMA 50/200, RSI(14), 52-wk high/low, 3-mo return, $-volume) computed
> with **strict no-lookahead** — indicator at bar T uses only bars < T. See
> [`docs/DESIGN.md`](docs/DESIGN.md) and [`docs/INDICATORS.md`](docs/INDICATORS.md).

## The read contract

The warehouse schema is owned upstream. **Read the data dictionary before touching
queries:**

- Local (siblings on disk): [`../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md`](../cetus-marketdata-pipeline/docs/DATA_DICTIONARY.md)
- GitHub (private): `jsoprych/cetus-marketdata-pipeline` → `docs/DATA_DICTIONARY.md`

Key points this scanner already honors: open the DB **read-only**, read the
**`adjusted_bars`** view (split-adjusted OHLCV — no client math), scan the
`SUCCESS` universe from `symbol_pipeline_state`, and treat free-tier IEX **volume
as a fraction** of consolidated (prefer price-based signals for now).

## Quickstart

Requires Go 1.25+ and a cetus warehouse DB (the pipeline's `cetus.db`).

```bash
# Easiest: run.sh builds the binary if it's missing, sources ./.env, then runs.
./run.sh                 # serve the dashboard → http://localhost:8080
./run.sh digest          # or: studies | anomalies | scan | users
./run.sh build           # force a rebuild
# Put local config in .env (git-ignored; see .env.example) — e.g. CETUS_DB=/…/cetus.db

# Or drive the binary directly:
make build

# 1) JSONL signal stream (stdout = signals, stderr = logs):
bin/scanner > signals.jsonl
SCANNER_MAX_SYMBOLS=200 bin/scanner | jq -c 'select(.type=="price_breakout")'

# 2) Daily digest — the free-tier report:
bin/scanner digest                                   # HTML to stdout
SCANNER_DIGEST_FORMAT=text bin/scanner digest        # plaintext, terminal-friendly
SCANNER_DIGEST_FORMAT=html SCANNER_DIGEST_OUT=digest.html bin/scanner digest

# 3) Live dashboard — test it in a browser:
bin/scanner serve                                    # http://localhost:8080
```

The warehouse path resolves as **`SCANNER_DB_PATH` → `CETUS_DB` → default**, so one
shared `CETUS_DB` env points every cetus consumer at the central store while any app
can override.

### Test the dashboard from a remote/headless box

```bash
# on the server:
bin/scanner serve                 # binds :8080
# from your laptop, forward the port over SSH, then open http://localhost:8080
ssh -L 8080:localhost:8080 you@server
```

Signal-stream example (JSONL):

```json
{"symbol":"NVDA","type":"volume_breakout","date":1751414400,"value":3.2}
{"symbol":"AMD","type":"gap_up","date":1751414400,"value":0.08}
```

## Configuration

Env-only (stdlib, no flags framework yet).

**Warehouse path** (shared convention — first non-empty wins):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_DB_PATH` | — | Per-app override of the warehouse path |
| `CETUS_DB` | — | Shared warehouse path for all cetus consumers |
| *(fallback)* | `../cetus-marketdata-pipeline/cetus.db` | Sibling default (flips to `../CETUS/cetus.db` post-migration) |

**Signal scan** (`scanner` / `scan`):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_SINCE_DAYS` | `120` | Recent history loaded per symbol |
| `SCANNER_LOOKBACK` | `20` | Trailing bars forming the scan baseline |
| `SCANNER_VOLUME_MULT` | `2.0` | Volume-breakout threshold (× trailing avg) |
| `SCANNER_GAP_PCT` | `0.05` | Gap threshold (fraction vs prior close) |
| `SCANNER_MAX_SYMBOLS` | `0` | Cap the scanned universe (0 = no cap) |
| `SCANNER_UNIVERSE` | `index:r3000` | Scope: `all` \| `common` \| `index:CODE` (r3000/sp500/…) \| `exchange:X` \| `file:PATH`. An unseeded index falls back to `common` |
| `SCANNER_ANOMALY_FORMAT` | `text` | `anomalies` output: `text` \| `jsonl` |

**Digest & dashboard** (`digest` / `serve`):

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_DIGEST_LOOKBACK_DAYS` | `400` | History per symbol (covers the 252-bar 52-wk window) |
| `SCANNER_MIN_DOLLAR_VOL` | `1e6` | Liquidity floor for the digest universe |
| `SCANNER_DIGEST_TOP_N` | `8` | Rows per section |
| `SCANNER_DIGEST_MOMENTUM_N` | `10` | Rows in the momentum leaderboard |
| `SCANNER_DIGEST_FORMAT` | `html` | `html` \| `text` \| `json` |
| `SCANNER_DIGEST_OUT` | *(stdout)* | Output file path |
| `SCANNER_DIGEST_WORKERS` | `0` | Scan parallelism (0 = NumCPU) |
| `SCANNER_SERVE_ADDR` | `:8080` | Dashboard listen address |
| `SCANNER_SERVE_TTL_SECS` | `600` | Dashboard render cache TTL |
| `SCANNER_SESSION_HOURS` | `8` | Login session lifetime |
| `SCANNER_AUTH_MODE` | `login` | `login` (built-in sessions) or `proxy` (trust a reverse-proxy identity header) |
| `SCANNER_TRUSTED_USER_HEADER` | `X-Token-User` | In `proxy` mode, the header carrying the authenticated user id |

**Reverse-proxy auth (caddy-security etc.):** set `SCANNER_AUTH_MODE=proxy`. The
proxy authenticates (OAuth/OIDC/MFA/TLS); the scanner reads the identity, looks up
tier/role in its user store (unknown users default to free/user), and skips its own
login. Two trust levels:

- **Raw header** (`SCANNER_TRUSTED_USER_HEADER`): only safe when the scanner is
  reachable *only* via the proxy — bind loopback (`SCANNER_SERVE_ADDR=127.0.0.1:8080`).
- **Verified JWT** (recommended): set a key and the identity comes from a
  signature-checked token instead of a spoofable header.

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_JWT_HMAC_SECRET` | — | HS256/384/512 shared secret (enables JWT verify) |
| `SCANNER_JWT_PUBKEY_FILE` | — | RS256/384/512 RSA public key PEM (alternative) |
| `SCANNER_JWT_HEADER` | `Authorization` | Header carrying the token (`Bearer` stripped) |
| `SCANNER_JWT_USER_CLAIM` | `sub` | Claim holding the user id |
| `SCANNER_JWT_ISSUER` / `SCANNER_JWT_AUDIENCE` | — | Optional `iss` / `aud` to enforce |

The verifier is stdlib-only, checks `exp`/`nbf`/`iss`/`aud`, and is **bound to one
key type** — an HMAC verifier rejects `RS*` tokens and vice-versa, and `alg=none` is
never accepted (defeats alg-confusion attacks).

**Studies & the scanner's own store** (`studies`) — the store is **separate from
the read-only cetus warehouse**:

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_STORE_DB` | `../../SCANNER/scanner.db` (exe-relative) | The scanner's own SQLite store (snapshot materialization). Resolved relative to the executable; set to `:memory:` for ephemeral snapshots |
| `SCANNER_STUDIES_PATH` | `studies.jsonl` | JSONL of SQL-`WHERE` studies (owner + tier per line) |
| `SCANNER_STUDIES_FORMAT` | `text` | `text` \| `jsonl` |
| `SCANNER_USERS_PATH` | `users.jsonl` | User registry (id · tier · role) |
| `SCANNER_USER` | `global` | Acting user id — resolved against the registry; tier/role gate access |
| `SCANNER_SNAPSHOT_RETENTION_DAYS` | `90` | Days of snapshots to keep (0 = no retention cleanup) |
| `SCANNER_BACKFILL_DAYS` | `0` | Days to backfill on `scanner backfill` (0 = disabled) |

## Layout

```
cmd/scanner/          entrypoint: dispatch scan | digest | serve
internal/model/       neutral types (Bar, Signal)
internal/store/       READ-ONLY reader of the cetus DB (adjusted_bars, universe)
internal/scanner/     pure Scan(symbol, bars, cfg) []Signal  (JSONL signals)
internal/indicators/  pure indicator funcs (modular: trend.go, momentum.go, price.go, returns.go)
internal/screen/      SnapshotRow build, preset studies, market breadth (pure)
internal/scan/        concurrent whole-universe snapshot (worker pool, BarLoader iface)
internal/sentinel/    data-quality Tier-0 flags (deterministic; AI tiers extend it)
internal/snapshot/    scanner's OWN SQLite store; materialize snapshot + run SQL studies + indexed lookups (SymbolClose, NearestDate)
internal/study/       studies as data (SQL-WHERE, JSONL, owner + tier)
internal/user/        User entity + file-backed Store: tiers, roles, sha256 login, CRUD
internal/authjwt/     stdlib JWT verifier (HS/RS, alg-confusion-safe) for proxy auth
internal/api/         REST API handlers (health, features)
internal/features/    feature catalog with metadata (50+ indicators)
internal/digest/      Digest assembly + html/text/json renderers
internal/dashboard/   admin ops-console renderer (serve)
internal/serve/       HTTP server: dashboard, auth, study editor, REST API (extracted from cmd/scanner)
internal/config/      env-first configuration (SCANNER_*, CETUS_DB)
internal/telemetry/   slog JSON logger (stderr)
docs/                 DESIGN.md (master) · SCHEMA.md · INDICATORS.md · API.md · DEPLOY-cloudflare.md · AGENTS.md
```

## Deployment

The scanner runs as a 24/7 daemon (`scanner serve`). The snapshot is built once on
startup, held hot in RAM, and serves all requests with sub-2ms SELECTs.

**Deployment phases:**

1. **Local hosting** — `localhost:8080` or LAN access, iterate fast
2. **Unadvertised tunnel** — Cloudflare tunnel + built-in login for small test group
3. **Production auth** — Caddy + caddy-security or Cloudflare Access for public use

See [`docs/DEPLOY-cloudflare.md`](docs/DEPLOY-cloudflare.md) for the full deployment
roadmap, container strategy, and auth migration path.

## Development

```bash
make test   # go test ./...
make vet    # go vet ./...
```

The `scanner.Scan` function is **pure** (no I/O) — it's the seam a future
back-test/strategy engine calls bar-by-bar, and it's unit-tested with synthetic
bars (no DB needed).

## Documentation

- **[DESIGN.md](docs/DESIGN.md)** — Master design document with table of contents
- **[SCHEMA.md](docs/SCHEMA.md)** — Snapshot schema reference
- **[INDICATORS.md](docs/INDICATORS.md)** — Indicator catalog with no-lookahead principle
- **[API.md](docs/API.md)** — REST API reference
- **[FEATURE_CATALOG.md](docs/FEATURE_CATALOG.md)** — Feature metadata
- **[DEPLOY-cloudflare.md](docs/DEPLOY-cloudflare.md)** — Deployment roadmap
- **[IMPLEMENTATION_PROGRESS.md](docs/IMPLEMENTATION_PROGRESS.md)** — Current status
- **[AGENTS.md](docs/AGENTS.md)** — Engineering directives

See [`docs/DESIGN.md`](docs/DESIGN.md) for the complete architecture overview.
