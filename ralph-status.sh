#!/usr/bin/env bash
#
# ralph-status.sh - Monitor a running ralph.sh loop
#
# Usage: ./ralph-status.sh          # One-time status check
#        ./ralph-status.sh --watch  # Live refresh every 5 seconds
#        ./ralph-status.sh --tail   # Follow the latest iteration log

set -euo pipefail

TASKS_FILE="tasks.json"
PROGRESS_FILE="progress.md"
LOG_DIR="logs/ralph"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
NC='\033[0m'

show_status() {
    clear
    echo -e "${CYAN}=== Ralph Wiggum Status ===${NC}"
    echo ""

    # Current iteration log
    if [[ -d "$LOG_DIR" ]] && ls "$LOG_DIR"/*.log &>/dev/null; then
        LATEST_LOG=$(ls -t "$LOG_DIR"/*.log | head -1)
        ITERATION_COUNT=$(ls "$LOG_DIR"/*.log | wc -l)
        LOG_SIZE=$(du -h "$LATEST_LOG" | cut -f1)
        LOG_AGE=$(( ($(date +%s) - $(stat -c %Y "$LATEST_LOG")) ))

        if [[ $LOG_AGE -lt 60 ]]; then
            AGE_STR="${LOG_AGE}s ago"
            STATUS="${GREEN}RUNNING${NC}"
        elif [[ $LOG_AGE -lt 300 ]]; then
            AGE_STR="$((LOG_AGE / 60))m ago"
            STATUS="${YELLOW}MAYBE RUNNING${NC}"
        else
            AGE_STR="$((LOG_AGE / 60))m ago"
            STATUS="${RED}STOPPED${NC}"
        fi

        echo -e "  Status:     $STATUS"
        echo -e "  Iterations: ${ITERATION_COUNT} completed"
        echo -e "  Latest log: ${DIM}${LATEST_LOG}${NC} (${LOG_SIZE}, ${AGE_STR})"
    else
        echo -e "  Status:     ${DIM}No logs found${NC}"
    fi
    echo ""

    # Task progress
    if [[ -f "$TASKS_FILE" ]]; then
        echo -e "${CYAN}=== Tasks ===${NC}"
        echo ""
        python3 -c "
import json
tasks = json.load(open('$TASKS_FILE'))
passed = sum(1 for t in tasks if t.get('passes'))
total = len(tasks)
pct = int(passed / total * 100) if total > 0 else 0

# Progress bar
bar_len = 30
filled = int(bar_len * passed / total) if total > 0 else 0
bar = '#' * filled + '-' * (bar_len - filled)
print(f'  Progress: [{bar}] {passed}/{total} ({pct}%)')
print()

# Task list
for t in tasks:
    status = '\033[0;32mPASS\033[0m' if t.get('passes') else '\033[2m    \033[0m'
    cat = t.get('category', '?')[:8].ljust(8)
    desc = t.get('description', '')[:65]
    print(f'  {status}  {t[\"id\"]:>2}. [{cat}] {desc}')
"
    fi
    echo ""

    # Progress file last lines
    if [[ -f "$PROGRESS_FILE" ]] && [[ -s "$PROGRESS_FILE" ]]; then
        echo -e "${CYAN}=== Latest Progress ===${NC}"
        echo ""
        tail -10 "$PROGRESS_FILE" | sed 's/^/  /'
        echo ""
    fi

    # Last few lines of latest log
    if [[ -d "$LOG_DIR" ]] && ls "$LOG_DIR"/*.log &>/dev/null; then
        echo -e "${CYAN}=== Latest Log (last 5 lines) ===${NC}"
        echo ""
        tail -5 "$LATEST_LOG" | sed 's/^/  /'
        echo ""
    fi
}

case "${1:-}" in
    --watch|-w)
        while true; do
            show_status
            echo -e "${DIM}Refreshing every 5s... Ctrl+C to stop${NC}"
            sleep 5
        done
        ;;
    --tail|-t)
        if [[ -d "$LOG_DIR" ]] && ls "$LOG_DIR"/*.log &>/dev/null; then
            LATEST_LOG=$(ls -t "$LOG_DIR"/*.log | head -1)
            echo -e "${CYAN}Following: ${LATEST_LOG}${NC}"
            echo -e "${DIM}Ctrl+C to stop${NC}"
            echo ""
            tail -f "$LATEST_LOG"
        else
            echo "No logs found in $LOG_DIR/"
            exit 1
        fi
        ;;
    *)
        show_status
        ;;
esac
