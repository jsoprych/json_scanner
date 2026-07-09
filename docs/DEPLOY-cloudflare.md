# Deployment Roadmap — Local → Tunnel → Public

The scanner runs as a 24/7 daemon (`scanner serve`). The snapshot is built once on
startup, held hot in RAM, and serves all requests with sub-2ms SELECTs.

**Deployment phases:**

1. **Local hosting** — iterate fast, test against real data
2. **Unadvertised tunnel** — expose to a small test group without opening ports
3. **Production auth** — Caddy or Cloudflare Access for public use

---

## Phase 1: Local hosting

Run on a modest box (4-core, 32GB RAM is plenty — the snapshot is ~3MB).

```bash
./run.sh serve                 # http://localhost:8080
```

**Access:**
- Local only: `localhost:8080` (default bind)
- LAN access: set `SCANNER_SERVE_ADDR=0.0.0.0:8080`, hit it from other devices

**Rebuild the snapshot** when new EOD data lands:
- Schedule: cron/systemd timer at ~4:30 PM ET (after cetus pipeline finishes)
- On-demand: restart the service, or call an admin endpoint (future)

Local hosting lets you iterate fast and validate the workflow before paying for
cloud infra.

---

## Phase 2: Unadvertised Cloudflare tunnel

Expose the scanner via a **Cloudflare tunnel** — no open ports on the host. Use an
unadvertised subdomain (e.g., `scanner.chartgeometry.com` or off elko.ai /
darkfabrik.ai). Share the URL only with known testers.

```
browser ──TLS──► Cloudflare tunnel ──► scanner:8080
```

**Setup:**

1. **Tunnel** — Cloudflare Zero Trust → Networks → Tunnels → *Create tunnel*. Add a
   public hostname routed to `http://scanner:8080`. Copy the **tunnel token**.
2. **Run:**

```bash
cp .env.example .env      # fill CF_TUNNEL_TOKEN, CETUS_DB_PATH
docker compose up -d --build
```

**Auth for Phase 2: built-in login**

The scanner's built-in login (`SCANNER_AUTH_MODE=login`) is acceptable for a small,
unadvertised test group:

- Tunnel URL is the first gate (not indexed, not public)
- Passwords are PBKDF2-SHA256 (600k iterations)
- Small number of known testers

**Gaps** (acceptable interim risk):
- No rate limiting — brute-force is possible (but URL is unadvertised)
- No CSRF protection
- Login page is reachable by anyone who has the URL

**Hardening before wider use:**
- Use long, random passwords for test accounts
- Monitor logs for repeated failed logins (`slog` to stderr → container logs)
- Migrate to Phase 3 auth when ready

---

## Phase 3: Production auth

When you're ready for public use or paid tiers, upgrade the auth layer. Two options:

### Option A: Caddy + caddy-security (recommended)

**Caddy** as a reverse proxy with **caddy-security** (Go auth library). Caddy handles
login / OAuth / SSO / MFA, issues or verifies JWTs, and forwards identity to the
scanner in proxy mode.

```
browser ──TLS──► Caddy (auth: login/OAuth/SSO) ──► scanner:8080 (proxy mode: verify JWT)
```

**Advantages:**
- You control the auth stack
- Works with any identity provider
- No vendor lock-in
- Works on any infra (not just Cloudflare)

**Scanner config:**

```
SCANNER_AUTH_MODE=proxy
SCANNER_JWT_HEADER=X-Auth-User          # or whatever Caddy forwards
SCANNER_JWT_USER_CLAIM=email
SCANNER_JWT_HMAC_SECRET=<shared-secret> # or use RS256 with pubkey
```

The scanner's `authjwt` package verifies the JWT (HS256/384/512 or RS256/384/512),
checks `exp`/`nbf`/`iss`/`aud`, and reads the user claim. Unknown users default to
free/user; admin can upgrade tier/role in `/admin`.

### Option B: Cloudflare Access

**Cloudflare Access** provides SSO/MFA at the edge. The scanner verifies the Access
JWT.

```
browser ──TLS──► Cloudflare (Access: SSO/MFA, issues Cf-Access-Jwt-Assertion)
                    │  tunnel
                    ▼
                cloudflared ──► scanner:8080 (proxy mode: verify JWKS)
```

**Scanner config:**

```
SCANNER_AUTH_MODE=proxy
SCANNER_JWT_HEADER=Cf-Access-Jwt-Assertion
SCANNER_JWT_USER_CLAIM=email
SCANNER_JWT_JWKS_URL=https://<team>.cloudflareaccess.com/cdn-cgi/access/certs
SCANNER_JWT_ISSUER=https://<team>.cloudflareaccess.com
SCANNER_JWT_AUDIENCE=<AUD tag>
```

JWKS keys are cached and re-fetched (rate-limited) on an unknown `kid`, so Access
key rotation is handled automatically.

**Advantages:**
- Simpler ops (Cloudflare manages the auth UI, MFA, etc.)
- No separate proxy to run
- Free tier available

**Disadvantages:**
- Cloudflare-dependent
- Less flexible identity provider options

### Migration path

Both options use the scanner's existing `SCANNER_AUTH_MODE=proxy` + JWT verification
(`authjwt` package). Migration from built-in login to either option is a **config
change + restart**, not a rewrite.

---

## Container strategy

**Docker** (already have Dockerfile + docker-compose.yml). The runtime image is
**distroless-nonroot**:

- No shell, no package manager, no curl — can't exec into it even if someone gets in
- Runs as uid 65532 (non-root)
- Read-only mount on cetus.db
- Minimal attack surface: one Go binary, one open port (only reachable through the
  tunnel)

**LXD** is more VM-like (heavier, more ops surface). Use it only if you're already
running LXD clusters elsewhere.

**Compose mounts:**
- `cetus.db` — read-only (`:ro`)
- `users.jsonl` — writable (admin CRUD rewrites it); ensure uid 65532 can write it
- `.env` — tunnel token, JWT secrets, etc.

---

## Snapshot rebuild lifecycle

The snapshot is immutable for its lifetime (`snapshot_id`). Rebuild when new EOD data
lands (after the cetus pipeline runs post-close).

**Triggers:**
- **Schedule:** cron/systemd timer at ~4:30 PM ET
- **On-demand:** restart the service, or call an admin endpoint (future)

**Flow:**

```
REBUILD
  read latest cetus data (read-only)
  compute indicators → new snapshot
  assign new snapshot_id
  serve requests against new snapshot
  cached results keyed on old snapshot_id become stale
```

Result caching keys on `hash(study_hash + snapshot_id)` — identical studies across
users run once per snapshot.

---

## Subdomain choice

- `scanner.chartgeometry.com` — clean, discoverable, separate from marketing sites
- `scanner.elko.ai` or `scanner.darkfabrik.ai` — if those are the main brands

Any of these work. `scanner.` is unambiguous about the service's purpose.

---

## Acceptable risk assessment

**Phase 2 (unadvertised tunnel + built-in login) is acceptable for a small test group:**
- No exposed ports (tunnel-only)
- Read-only DB (can't corrupt upstream)
- Distroless container (can't exec into it)
- Unadvertised URL (not indexed, not public)
- PBKDF2-SHA256 passwords

**Main risks are operational, not architectural:**
- Tunnel URL leaks (use a non-obvious subdomain)
- Weak test passwords (use long, random secrets)
- Local dev box security (if it's your daily driver, patch it)

**When you go public or add paid tiers:**
- Migrate to Phase 3 auth (Caddy or Access)
- Add rate limiting
- Add audit logging
- Consider a WAF

The architecture supports all of this without rewriting.
