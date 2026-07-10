#!/bin/bash
# Monitor backfill progress
# Usage: ./scripts/monitor_backfill.sh [log_file]

LOG_FILE="${1:-/tmp/backfill_jan2026.log}"
DB_PATH="/home/john/CODE/DEV/DATA/SCANNER/scanner.db"

echo "Monitoring backfill progress..."
echo "Log file: $LOG_FILE"
echo "Database: $DB_PATH"
echo ""

while true; do
    echo "=== $(date) ==="
    tail -1 "$LOG_FILE" | jq -r '"\(.msg) - date: \(.date // "N/A") - symbols: \(.symbols // "N/A")"'
    echo "DB size: $(ls -lh "$DB_PATH" 2>/dev/null | awk '{print $5}' || echo 'not found')"
    echo "Snapshots: $(sqlite3 "$DB_PATH" 'SELECT COUNT(DISTINCT snapshot_date) FROM snapshot;' 2>/dev/null || echo '0')"
    echo ""
    sleep 60
done
