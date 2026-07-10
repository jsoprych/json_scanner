#!/bin/bash
# Start backfill process in background
# Usage: ./scripts/start_backfill.sh [days]

DAYS="${1:-190}"
LOG_FILE="/tmp/backfill_jan2026.log"

echo "Starting backfill for $DAYS days..."
echo "Log file: $LOG_FILE"

cd /home/john/CODE/DEV/chartgeometry.com/DATA/cetus-marketdata-scanner

SCANNER_BACKFILL_DAYS=$DAYS setsid ./bin/scanner backfill > "$LOG_FILE" 2>&1 &
disown

echo "Backfill started with PID $!"
echo ""
echo "Monitor progress with: ./scripts/monitor_backfill.sh"
