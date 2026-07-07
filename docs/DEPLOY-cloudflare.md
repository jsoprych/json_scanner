# Deploy: container + Cloudflare tunnel

Run the scanner in a container, expose it publicly through a **Cloudflare tunnel**,
and authenticate with **Cloudflare Access** (Zero Trust) — SSO/MFA at the edge, no
exposed login. The scanner runs in `proxy` mode and verifies the Access JWT against
its rotating JWKS.

```
browser ──TLS──► Cloudflare (Access: SSO/MFA, issues Cf-Access-Jwt-Assertion)
                    │  tunnel
                    ▼
                cloudflared ──► scanner:8080 (proxy mode: verify JWKS, read email claim)
```

The scanner is **never published to the host** — only `cloudflared` reaches it over
the compose network.

## One-time Cloudflare setup

1. **Tunnel** — Zero Trust → Networks → Tunnels → *Create tunnel*. Add a public
   hostname (e.g. `scanner.chartgeometry.com`) routed to `http://scanner:8080`. Copy
   the **tunnel token**.
2. **Access application** — Zero Trust → Access → Applications → *Add* (self-hosted),
   the same hostname. Add a policy (e.g. *emails ending in @yourco*). Note the
   **Application Audience (AUD)** tag and your **team name** (`<team>.cloudflareaccess.com`).

## Run

```bash
cp .env.example .env      # fill CF_TUNNEL_TOKEN, CF_TEAM, CF_ACCESS_AUD, CETUS_DB_PATH
docker compose up -d --build
```

That's it — visit the hostname, Cloudflare Access signs you in, the scanner sees your
verified `email` as the acting user. Manage tier/role/groups for those identities in
`/admin` (admin CRUD persists to the mounted `users.jsonl`).

## How auth maps

| Concern | Owner |
|---|---|
| *Who you are* (login, MFA, SSO, TLS) | **Cloudflare Access** → signed JWT |
| *What you can do* (tier / role / group) | **scanner** `users.jsonl` (admin CRUD) |

Env the compose sets for proxy+JWKS:

```
SCANNER_AUTH_MODE=proxy
SCANNER_JWT_HEADER=Cf-Access-Jwt-Assertion
SCANNER_JWT_USER_CLAIM=email
SCANNER_JWT_JWKS_URL=https://<team>.cloudflareaccess.com/cdn-cgi/access/certs
SCANNER_JWT_ISSUER=https://<team>.cloudflareaccess.com
SCANNER_JWT_AUDIENCE=<AUD tag>
```

A first-time authenticated identity with no local profile defaults to **free/user**;
an admin can then upgrade it.

## Simpler (less secure) alternative: built-in login over the tunnel

If you don't want Access, set `SCANNER_AUTH_MODE=login` (comment the `SCANNER_JWT_*`
lines) and the scanner serves its own `/login`. Passwords are **salted PBKDF2-SHA256**
(600k iterations), but the login is still **unthrottled** (no brute-force/rate limit
or CSRF) and was built for a LAN. **Change the dev passwords** and prefer Access for
anything public-facing.

## Notes

- `users.jsonl` must be a **writable** mount (admin CRUD rewrites it); the runtime is
  distroless-nonroot (uid 65532), so ensure the file is writable by that uid, or run
  the service as a uid that owns it.
- The warehouse is mounted **read-only** (`:ro`) — the scanner never writes cetus.
- JWKS keys are cached and re-fetched (rate-limited) on an unknown `kid`, so Access
  key rotation is handled automatically.
