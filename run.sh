#!/usr/bin/env bash
#
# run.sh — build and run the cetus-marketdata-scanner
#
# Quick Start:
#   ./run.sh                     # Start the server (default) → http://localhost:8080
#   ./run.sh admin               # Open admin panel info (default admin/admin)
#
# Commands:
#   ./run.sh serve               # Start HTTP server with dashboard
#   ./run.sh digest              # Generate daily market digest
#   ./run.sh studies             # Run saved studies
#   ./run.sh anomalies           # Detect market anomalies
#   ./run.sh scan                # Run market scan
#   ./run.sh users               # List users
#   ./run.sh replay --date DATE  # Historical scan replay
#
# Options:
#   ./run.sh build               # Build only, don't run
#   ./run.sh --rebuild <cmd>     # Force rebuild, then run <cmd>
#   ./run.sh --help              # Show this help
#   ./run.sh dev                 # Development mode with verbose logging
#
# Examples:
#   ./run.sh                     # Start server (default)
#   ./run.sh digest --format json
#   ./run.sh replay --date 2025-10-01
#   ./run.sh --rebuild serve
#
set -euo pipefail
cd "$(dirname "$0")"
BIN=bin/scanner

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored message
info() { echo -e "${BLUE}ℹ${NC} $1"; }
success() { echo -e "${GREEN}✓${NC} $1"; }
warn() { echo -e "${YELLOW}⚠${NC} $1"; }
error() { echo -e "${RED}✗${NC} $1"; }

# Show help
show_help() {
    cat << 'EOF'
Cetus Marketdata Scanner - Run Script

USAGE:
    ./run.sh [command] [options]

QUICK START:
    ./run.sh                     Start the server (default)
    ./run.sh admin               Show admin panel info

COMMANDS:
    serve           Start HTTP server with dashboard (default)
    backfill        Rebuild historical snapshots (uses SCANNER_BACKFILL_DAYS)
    rebuild-backfill Rebuild binary, drop old snapshot, backfill from scratch
    digest          Generate daily market digest
    studies         Run saved studies
    anomalies       Detect market anomalies
    scan            Run market scan
    users           List users
    replay          Historical scan replay (requires --date)
    build           Build only, don't run
    admin           Show admin panel information
    dev             Development mode with verbose logging

OPTIONS:
    --rebuild       Force rebuild before running
    --help, -h      Show this help message

EXAMPLES:
    ./run.sh                              # Start server
    ./run.sh serve                        # Start server (explicit)
    ./run.sh digest --format json         # Generate JSON digest
    ./run.sh replay --date 2025-10-01     # Replay from specific date
    ./run.sh --rebuild serve              # Rebuild and start server
    ./run.sh dev                          # Development mode

ENVIRONMENT:
    Configuration is loaded from .env file (if present).
    See .env.example for available options.

    Key variables:
        SCANNER_DB_PATH       Path to warehouse database
        SCANNER_STORE_DB      Path to scanner database
        SCANNER_SERVE_ADDR    Server listen address (default: :8080)
        SCANNER_AUTH_MODE     Authentication mode (default: login)

DEFAULT ADMIN:
    On first run, a default admin user is created:
        Username: admin
        Password: admin
    
    ⚠️  Change this immediately after first login!

ADMIN PANEL:
    After starting the server, visit:
        http://localhost:8080/admin

MORE INFO:
    See docs/SYSTEM_OVERVIEW.md for complete documentation
    See docs/ADMIN_PANEL.md for admin panel guide

EOF
}

# Check prerequisites
check_prereqs() {
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go 1.21 or later."
        error "Visit: https://golang.org/dl/"
        exit 1
    fi
    
    local go_version=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
    local major=$(echo "$go_version" | cut -d. -f1)
    local minor=$(echo "$go_version" | cut -d. -f2)
    
    if [ "$major" -lt 1 ] || ([ "$major" -eq 1 ] && [ "$minor" -lt 21 ]); then
        warn "Go version $go_version detected. Recommended: 1.21 or later"
    fi
}

# Build the binary
build() {
    info "Building $BIN..."
    # GOWORK=off: the scanner is a standalone module — build it independently of any
    # parent go.work, so this works from a fresh clone or in-tree alike.
    if GOWORK=off CGO_ENABLED=0 go build -trimpath -o "$BIN" ./cmd/scanner; then
        success "Built $BIN"
    else
        error "Build failed!"
        exit 1
    fi
}

# Show admin panel info
show_admin_info() {
    cat << 'EOF'

╔═══════════════════════════════════════════════════════════════╗
║                    ADMIN PANEL INFO                           ║
╚═══════════════════════════════════════════════════════════════╝

Default Admin Credentials (created on first run):
  Username: admin
  Password: admin

⚠️  IMPORTANT: Change the password immediately after first login!

Access the Admin Panel:
  1. Start the server: ./run.sh
  2. Open browser: http://localhost:8080/admin
  3. Login with admin/admin
  4. Navigate to Users → Edit admin → Change password

Admin Panel Features:
  • Dashboard - System stats and activity
  • Users - User management
  • Roles - Role and permission management
  • Groups - Group administration
  • Monitoring - System metrics and logs

For more information, see:
  • docs/ADMIN_PANEL.md
  • docs/SYSTEM_OVERVIEW.md

EOF
}

# Development mode
run_dev() {
    info "Starting in development mode..."
    export SCANNER_LOG_LEVEL=debug
    export SCANNER_SERVE_ADDR=${SCANNER_SERVE_ADDR:-":8080"}
    
    success "Development server starting..."
    info "Admin panel: http://localhost:${SCANNER_SERVE_ADDR#:}/admin"
    info "API docs: http://localhost:${SCANNER_SERVE_ADDR#:}/api/v1/docs"
    info "Press Ctrl+C to stop"
    echo ""
    
    exec "$BIN" serve
}

# Load local config from .env if present — CETUS_DB, SCANNER_*, secrets. (.env is
# git-ignored; see .env.example.) The app reads OS env, so this is how you keep the
# DB path (and everything else) in a file instead of the shell.
if [ -f .env ]; then 
    info "Loading configuration from .env"
    set -a; . ./.env; set +a
fi

# Handle help flag
case "${1:-}" in
    --help|-h) show_help; exit 0 ;;
esac

# Check prerequisites
check_prereqs

# Handle special commands
case "${1:-}" in
    build)     build; exit 0 ;;
    admin)     show_admin_info; exit 0 ;;
    dev)       [ -x "$BIN" ] || build; run_dev ;;
    rebuild-backfill) build; sqlite3 "${SCANNER_STORE_DB}" "DROP TABLE IF EXISTS snapshot;" 2>/dev/null; exec "$BIN" backfill ;;
    --rebuild) build; shift ;;
esac

# Default behavior: build only if the binary doesn't exist yet.
[ -x "$BIN" ] || build

# Show startup message for serve command
if [ "${1:-serve}" = "serve" ]; then
    echo ""
    success "Starting Cetus Marketdata Scanner..."
    info "Dashboard: http://localhost:${SCANNER_SERVE_ADDR:-":8080"}/"
    info "Admin Panel: http://localhost:${SCANNER_SERVE_ADDR:-":8080"}/admin"
    info "API: http://localhost:${SCANNER_SERVE_ADDR:-":8080"}/api/v1/"
    info "Press Ctrl+C to stop"
    echo ""
fi

# Default subcommand is `serve`; anything passed overrides it.
exec "$BIN" "${@:-serve}"
