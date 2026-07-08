#!/usr/bin/env bash
#
# run.sh — build the scanner if the binary is missing, then run it.
#
#   ./run.sh                     # serve the dashboard (default) → http://localhost:8080
#   ./run.sh digest              # or: studies | anomalies | scan | users
#   ./run.sh digest --format...  # args pass straight through (env vars still apply)
#   ./run.sh build               # force a clean rebuild, don't run
#   ./run.sh --rebuild <cmd>     # rebuild, then run <cmd>
#
set -euo pipefail
cd "$(dirname "$0")"
BIN=bin/scanner

# Load local config from .env if present — CETUS_DB, SCANNER_*, secrets. (.env is
# git-ignored; see .env.example.) The app reads OS env, so this is how you keep the
# DB path (and everything else) in a file instead of the shell.
if [ -f .env ]; then set -a; . ./.env; set +a; fi

build() {
	echo "▸ building $BIN …"
	# GOWORK=off: the scanner is a standalone module — build it independently of any
	# parent go.work, so this works from a fresh clone or in-tree alike.
	GOWORK=off CGO_ENABLED=0 go build -trimpath -o "$BIN" ./cmd/scanner
	echo "  → built $BIN"
}

case "${1:-}" in
	build)     build; exit 0 ;;
	--rebuild) build; shift ;;
esac

# Default behavior: build only if the binary doesn't exist yet.
[ -x "$BIN" ] || build

# Default subcommand is `serve`; anything passed overrides it.
exec "$BIN" "${@:-serve}"
