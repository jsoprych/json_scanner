# Deploying Scanner at scan.chartgeometry.com

## Step 1: DNS Setup

Add an A record to chartgeometry.com's DNS:

```
Type:  A
Name:  scan
Value: YOUR_SERVER_IP
TTL:   300
```

Verify with:
```bash
dig scan.chartgeometry.com
```

## Step 2: Server Setup (Hetzner)

```bash
# Connect to your server
ssh root@YOUR_SERVER_IP

# Clone and build
git clone https://github.com/jsoprych/json_scanner.git /opt/cetus-scanner
cd /opt/cetus-scanner
make build

# Create .env
cat > .env << 'EOF'
# Warehouse
CETUS_DB=/path/to/cetus.db
SCANNER_STORE_DB=/opt/cetus-scanner/data/scanner.db

# TLS (auto-HTTPS via Let's Encrypt)
SCANNER_TLS_DOMAIN=scan.chartgeometry.com
SCANNER_TLS_CACHE_DIR=/opt/cetus-scanner/certs

# Server
SCANNER_SERVE_ADDR=:8080
SCANNER_AUTH_MODE=login

# Universe
SCANNER_UNIVERSE=index:r3000

# Open signup (enable self-registration)
SCANNER_OPEN_SIGNUP=true

# Google OAuth (from Step 4 below)
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-xxxxxxxxxxxxx
EOF

# Create data directory
mkdir -p /opt/cetus-scanner/data /opt/cetus-scanner/certs

# Start the server
./run.sh
```

The scanner will:
1. Listen on :443 (HTTPS) with auto Let's Encrypt cert
2. Redirect :80 → :443 automatically
3. Get a free SSL certificate within 30 seconds

## Step 3: Verify TLS

```bash
# Test HTTPS is working
curl -I https://scan.chartgeometry.com/healthz
# Should return: HTTP/2 200

# Check certificate
curl -v https://scan.chartgeometry.com/ 2>&1 | grep "subject.*CN"
```

## Step 4: Google OAuth2 Setup

### 4.1 Create Google Cloud OAuth Credentials

1. Go to https://console.cloud.google.com/apis/credentials
2. Click **Create Credentials** → **OAuth client ID**
3. Select **Web application**
4. Fill in:
   - **Name**: `Cetus Scanner`
   - **Authorized JavaScript origins**: `https://scan.chartgeometry.com`
   - **Authorized redirect URIs**: `https://scan.chartgeometry.com/auth/google/callback`
5. Click **Create**
6. Copy the **Client ID** and **Client Secret**

### 4.2 Set Environment Variables

Add to `.env`:
```bash
GOOGLE_CLIENT_ID=123456789-xxxxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-xxxxxxxxxxxxxxxxxxxx
```

### 4.3 OAuth Consent Screen

If this is your first time, also configure the consent screen:

1. Go to https://console.cloud.google.com/apis/credentials/consent
2. Choose **External** (for public users) or **Internal** (for your org only)
3. Fill in:
   - App name: `Cetus Scanner`
   - User support email: your email
   - Developer contact: your email
4. Add scopes: `email`, `profile`, `openid`
5. Add test users (your email at minimum)
6. **Publish** the app when ready

### 4.4 Refresh Token for Email Lookup

The scanner needs to look up the user's email. The OAuth handler already calls:
```go
client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
```
This endpoint returns `{email, name}` — no extra scopes needed beyond `email` + `profile`.

## Step 5: Systemd Service (Auto-Start)

```bash
cat > /etc/systemd/system/cetus-scanner.service << 'EOF'
[Unit]
Description=Cetus Marketdata Scanner
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/cetus-scanner
ExecStart=/opt/cetus-scanner/bin/scanner serve
Restart=always
RestartSec=10

EnvironmentFile=/opt/cetus-scanner/.env

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable cetus-scanner
systemctl start cetus-scanner
systemctl status cetus-scanner
```

**Note:** Root is needed for Let's Encrypt to bind to :80 and :443. The scanner only opens those ports for HTTPS — everything else runs as a normal process.

## Step 6: Firewall

```bash
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 22/tcp
ufw enable
```

## Step 7: Test Full Flow

```bash
# 1. Health check
curl https://scan.chartgeometry.com/healthz

# 2. Signup
curl -X POST https://scan.chartgeometry.com/signup \
  -d "user=testuser&password=MyStr0ngP@ss&confirm=MyStr0ngP@ss"

# 3. Login
curl -c cookies.txt -X POST https://scan.chartgeometry.com/login \
  -d "user=admin&password=admin" \
  -L

# 4. Admin panel
curl -b cookies.txt https://scan.chartgeometry.com/api/v1/admin/stats
```

## Step 8: First-Time Admin

After deploying:
1. Visit `https://scan.chartgeometry.com/login`
2. Login with `admin` / `admin`
3. **Change admin password immediately** at `/profile`
4. Go to Admin Panel → Users → Reset admin password if locked out

## Config Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `SCANNER_TLS_DOMAIN` | — | Enables HTTPS (e.g., `scan.chartgeometry.com`) |
| `SCANNER_TLS_CACHE_DIR` | `certs` | Where Let's Encrypt certs are stored |
| `SCANNER_OPEN_SIGNUP` | `false` | Allow self-registration |
| `GOOGLE_CLIENT_ID` | — | Google OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | — | Google OAuth2 client secret |

## Troubleshooting

### Certificate fails to issue
```bash
# Check port 80 is reachable (Let's Encrypt validates via HTTP)
curl http://scan.chartgeometry.com/.well-known/acme-challenge/test

# Check certs directory permissions
ls -la /opt/cetus-scanner/certs
```

### Google OAuth "redirect_uri_mismatch"
- Verify the redirect URI in Google Cloud Console matches exactly:
  `https://scan.chartgeometry.com/auth/google/callback`
- No trailing slash, HTTPS required

### Scanner won't start
```bash
journalctl -u cetus-scanner -n 50
# Common issues: port 80/443 in use, missing .env, DB path wrong
```

## Architecture

```
Browser ──HTTPS──► :443 (Go TLS, Let's Encrypt auto-cert)
                      │
                      ├─ /login          — username/password login
                      ├─ /signup         — self-registration
                      ├─ /profile        — change password
                      ├─ /auth/google    — Google OAuth2 redirect
                      ├─ /auth/google/callback — OAuth2 callback
                      ├─ /admin/*        — admin panel
                      └─ /api/v1/*       — REST API

HTTP :80 ──redirect──► :443
```

## Cost Summary

| Item | Cost |
|------|------|
| Hetzner CX22 (2 vCPU, 4GB RAM) | €3.79/mo |
| Domain (chartgeometry.com) | ~$12/year |
| SSL Certificate | Free (Let's Encrypt) |
| Google OAuth2 | Free (unlimited users) |
| **Total** | **~$5/month** |
