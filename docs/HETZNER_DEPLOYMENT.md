# Hetzner Deployment Guide

## Overview

Deploy the Cetus Marketdata Scanner to Hetzner Cloud for ~$4/month.

**What you get:**
- 2 vCPU, 4GB RAM, 40GB SSD
- 20TB bandwidth/month
- 99.9% uptime SLA
- DDoS protection
- Daily backups
- European data centers (Germany/Finland)

**Capacity:** 1,000-3,000 daily active users

## Quick Start (5 minutes)

### 1. Create Hetzner Account

1. Go to https://www.hetzner.com/cloud
2. Sign up (requires credit card, but no charges until you create a server)
3. Verify email

### 2. Create Project

1. Click "New Project"
2. Name: `cetus-scanner`
3. Click "Add Project"

### 3. Create Server

1. Click "Add Server"
2. **Location:** Choose closest to your users (Falkenstein, Germany recommended)
3. **Image:** Ubuntu 22.04
4. **Type:** CX22 (€3.79/month)
5. **Networking:** Public IPv4 + IPv6 (default)
6. **SSH Keys:** Add your public key (see below)
7. **Name:** `cetus-scanner-1`
8. Click "Create & Buy now"

### 4. Generate SSH Key (if needed)

```bash
# On your local machine
ssh-keygen -t ed25519 -C "your-email@example.com"
cat ~/.ssh/id_ed25519.pub
```

Copy the output and paste into Hetzner's SSH Keys field.

### 5. Deploy

```bash
# SSH into your new server
ssh root@YOUR_SERVER_IP

# Run deployment script
curl -sSL https://raw.githubusercontent.com/jsoprych/json_scanner/main/deploy/hetzner-deploy.sh | bash
```

**Done!** Your scanner is now live.

## Detailed Deployment

### Manual Deployment

If you prefer manual control:

```bash
# SSH into server
ssh root@YOUR_SERVER_IP

# Update system
apt update && apt upgrade -y

# Install dependencies
apt install -y git sqlite3 ufw nginx certbot python3-certbot-nginx

# Create app user
useradd -m -s /bin/bash scanner
usermod -aG sudo scanner

# Switch to app user
su - scanner

# Clone repository
git clone https://github.com/jsoprych/json_scanner.git
cd json_scanner

# Build
make build

# Configure
cp .env.example .env
nano .env  # Edit configuration

# Test
./run.sh
```

### Production Setup

#### 1. Systemd Service

Create `/etc/systemd/system/cetus-scanner.service`:

```ini
[Unit]
Description=Cetus Marketdata Scanner
After=network.target

[Service]
Type=simple
User=scanner
WorkingDirectory=/home/scanner/json_scanner
ExecStart=/home/scanner/json_scanner/bin/scanner serve
Restart=always
RestartSec=10

Environment=SCANNER_DB_PATH=/home/scanner/json_scanner/data/cetus.db
Environment=SCANNER_STORE_DB=/home/scanner/json_scanner/data/scanner.db
Environment=SCANNER_SERVE_ADDR=127.0.0.1:8080
Environment=SCANNER_AUTH_MODE=login

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable cetus-scanner
sudo systemctl start cetus-scanner
sudo systemctl status cetus-scanner
```

#### 2. Nginx Reverse Proxy

Create `/etc/nginx/sites-available/cetus-scanner`:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
```

Enable site:

```bash
sudo ln -s /etc/nginx/sites-available/cetus-scanner /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

#### 3. SSL Certificate

```bash
sudo certbot --nginx -d your-domain.com
```

Follow prompts. Certbot will automatically configure Nginx for HTTPS.

#### 4. Firewall

```bash
sudo ufw allow OpenSSH
sudo ufw allow 'Nginx Full'
sudo ufw enable
sudo ufw status
```

#### 5. Database Setup

```bash
# Create data directory
mkdir -p /home/scanner/json_scanner/data

# Copy warehouse database (if you have one)
scp /path/to/local/cetus.db scanner@YOUR_SERVER_IP:/home/scanner/json_scanner/data/

# Or create empty database
sqlite3 /home/scanner/json_scanner/data/cetus.db "VACUUM;"

# Set permissions
chown -R scanner:scanner /home/scanner/json_scanner/data
chmod 700 /home/scanner/json_scanner/data
```

#### 6. Daily Snapshot Schedule

The scanner automatically creates daily snapshots at 5pm EST. Verify it's working:

```bash
# Check logs
sudo journalctl -u cetus-scanner -f

# Look for: "snapshot created for 2026-07-11"
```

## Configuration

### Environment Variables

Edit `/home/scanner/json_scanner/.env`:

```bash
# Database paths
SCANNER_DB_PATH=/home/scanner/json_scanner/data/cetus.db
SCANNER_STORE_DB=/home/scanner/json_scanner/data/scanner.db

# Server
SCANNER_SERVE_ADDR=127.0.0.1:8080
SCANNER_AUTH_MODE=login

# Universe
SCANNER_UNIVERSE=index:r3000

# Throttling
SCANNER_RATE_LIMIT_REQUESTS=100
SCANNER_RATE_LIMIT_WINDOW=60
```

### Roles Configuration

Edit `/home/scanner/json_scanner/roles.json` to customize roles and limits.

## Monitoring

### Check Service Status

```bash
sudo systemctl status cetus-scanner
```

### View Logs

```bash
# Live logs
sudo journalctl -u cetus-scanner -f

# Last 100 lines
sudo journalctl -u cetus-scanner -n 100

# Logs from today
sudo journalctl -u cetus-scanner --since today
```

### Check Disk Usage

```bash
df -h
du -sh /home/scanner/json_scanner/data/
```

### Check Memory Usage

```bash
free -h
ps aux | grep scanner
```

## Maintenance

### Update Scanner

```bash
ssh scanner@YOUR_SERVER_IP
cd json_scanner
git pull
make build
sudo systemctl restart cetus-scanner
```

### Backup Database

```bash
# Manual backup
sqlite3 /home/scanner/json_scanner/data/scanner.db ".backup /home/scanner/backups/scanner-$(date +%Y%m%d).db"

# Automated backup (add to crontab)
crontab -e

# Add this line:
0 2 * * * sqlite3 /home/scanner/json_scanner/data/scanner.db ".backup /home/scanner/backups/scanner-$(date +\%Y\%m\%d).db"
```

### Rotate Logs

```bash
sudo journalctl --vacuum-time=7d
```

### Clean Old Snapshots

```bash
# Delete snapshots older than 90 days
sqlite3 /home/scanner/json_scanner/data/scanner.db \
  "DELETE FROM snapshots WHERE date < date('now', '-90 days');"
```

## Scaling

### Upgrade Server

1. Go to Hetzner Cloud Console
2. Select your server
3. Click "Resize"
4. Choose larger plan (CX32, CX42, etc.)
5. Click "Resize" (server will reboot)

### Load Balancer (for high traffic)

1. Create second server
2. Deploy scanner on both
3. Create Hetzner Load Balancer
4. Add both servers as targets
5. Point DNS to load balancer IP

### Database Separation

For very high traffic:
1. Create separate database server
2. Use PostgreSQL instead of SQLite
3. Update `SCANNER_DB_PATH` to point to remote database

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -u cetus-scanner -n 50

# Common issues:
# - Database file missing or wrong permissions
# - Port 8080 already in use
# - Invalid configuration in .env
```

### Can't Access Admin Panel

```bash
# Check if service is running
sudo systemctl status cetus-scanner

# Check if port is listening
sudo netstat -tlnp | grep 8080

# Check firewall
sudo ufw status
```

### High Memory Usage

```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head -10

# Restart service if needed
sudo systemctl restart cetus-scanner
```

### Database Locked

```bash
# Check for zombie processes
ps aux | grep scanner

# Kill if necessary
sudo pkill -9 scanner

# Restart service
sudo systemctl restart cetus-scanner
```

## Security

### Change Default Password

**IMMEDIATELY** after first login:
1. Go to https://your-domain.com/admin
2. Login with admin/admin
3. Click Users
4. Edit admin user
5. Change password

### Enable 2FA (Future)

Not yet implemented. For now, use strong passwords.

### Restrict Admin Access

Edit Nginx config to restrict `/admin` to specific IPs:

```nginx
location /admin {
    allow YOUR_IP;
    deny all;
    proxy_pass http://127.0.0.1:8080;
    # ... rest of proxy config
}
```

### Regular Updates

```bash
# Update system packages weekly
sudo apt update && sudo apt upgrade -y

# Update scanner monthly
cd /home/scanner/json_scanner
git pull
make build
sudo systemctl restart cetus-scanner
```

## Cost Breakdown

| Service | Cost/Month |
|---------|-----------|
| Hetzner CX22 | €3.79 (~$4.10) |
| Domain (optional) | $1-15 |
| SSL Certificate | Free (Let's Encrypt) |
| **Total** | **$4-19/month** |

## Support

- **Hetzner Support:** https://docs.hetzner.com/
- **Scanner Docs:** See `docs/` directory
- **Community:** GitHub Issues

## Next Steps

1. ✅ Deploy to Hetzner
2. ✅ Configure domain and SSL
3. ✅ Change admin password
4. ✅ Create additional users
5. ✅ Set up monitoring
6. ✅ Configure backups
7. ✅ Test with real users

---

**Need help?** Check the troubleshooting section or open a GitHub issue.
