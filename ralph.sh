#!/usr/bin/env bash
#
# ralph.sh - Autonomous development loop
#
# Runs claude --print in a loop, feeding PROMPT.md each iteration.
# Claude reads tasks.json, checks progress, implements the next task,
# commits, and repeats until all tasks pass.
#
# Usage: ./ralph.sh [max_iterations]
#   max_iterations: Maximum loop iterations (default: 20)

set -euo pipefail

MAX_ITERATIONS="${1:-20}"
LOG_DIR="logs/ralph"
PROMPT_FILE="PROMPT.md"
PRD_FILE="PRD.md"
TASKS_FILE="tasks.json"
PROGRESS_FILE="progress.md"
COMPLETION_SIGNAL="ALL_TASKS_COMPLETE"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log() { echo -e "${CYAN}[ralph]${NC} $1"; }
warn() { echo -e "${YELLOW}[ralph]${NC} $1"; }
error() { echo -e "${RED}[ralph]${NC} $1"; }
success() { echo -e "${GREEN}[ralph]${NC} $1"; }

# Check how many tasks pass / total
check_tasks() {
    if [[ ! -f "$TASKS_FILE" ]]; then
        echo "0/0"
        return
    fi
    local total passed
    total=$(python3 -c "import json; t=json.load(open('$TASKS_FILE')); print(len(t))")
    passed=$(python3 -c "import json; t=json.load(open('$TASKS_FILE')); print(sum(1 for x in t if x.get('passes')))")
    echo "$passed/$total"
}

# Check if all tasks pass
all_tasks_pass() {
    python3 -c "
import json, sys
tasks = json.load(open('$TASKS_FILE'))
sys.exit(0 if all(t.get('passes') for t in tasks) else 1)
" 2>/dev/null
}

# Validate required files exist
for f in "$PROMPT_FILE" "$PRD_FILE" "$TASKS_FILE" "$PROGRESS_FILE"; do
    if [[ ! -f "$f" ]]; then
        error "Missing required file: $f"
        exit 1
    fi
done

# Create log directory
mkdir -p "$LOG_DIR"

log "Starting Ralph Wiggum loop"
log "Max iterations: $MAX_ITERATIONS"
log "Tasks: $(check_tasks) passing"
log "Logs: $LOG_DIR/"
echo ""

for i in $(seq 1 "$MAX_ITERATIONS"); do
    TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
    LOG_FILE="$LOG_DIR/iteration_${i}_${TIMESTAMP}.log"

    log "=== Iteration $i/$MAX_ITERATIONS === [$(check_tasks) tasks passing]"

    # Check for completion before running
    if all_tasks_pass; then
        success "All tasks in tasks.json pass -- done!"
        break
    fi
    if grep -q "$COMPLETION_SIGNAL" "$PROGRESS_FILE" 2>/dev/null; then
        success "Found $COMPLETION_SIGNAL in progress.md -- done!"
        break
    fi

    # Run Claude
    log "Running claude --print (logging to $LOG_FILE)..."
    if claude --print --dangerously-skip-permissions -p "$(cat "$PROMPT_FILE")" > "$LOG_FILE" 2>&1; then
        success "Iteration $i completed successfully"
    else
        EXIT_CODE=$?
        warn "Iteration $i exited with code $EXIT_CODE (see $LOG_FILE)"
    fi

    log "Progress: $(check_tasks) tasks passing"

    # Check for completion after running
    if all_tasks_pass; then
        success "All tasks in tasks.json pass -- done!"
        break
    fi
    if grep -q "$COMPLETION_SIGNAL" "$PROGRESS_FILE" 2>/dev/null; then
        success "Found $COMPLETION_SIGNAL in progress.md -- done!"
        break
    fi

    # Safety pause between iterations
    if [[ $i -lt $MAX_ITERATIONS ]]; then
        log "Pausing 2 seconds..."
        sleep 2
    fi
done

echo ""
FINAL_STATUS=$(check_tasks)
if all_tasks_pass || grep -q "$COMPLETION_SIGNAL" "$PROGRESS_FILE" 2>/dev/null; then
    success "Ralph finished: $FINAL_STATUS tasks complete."
else
    warn "Ralph finished: $FINAL_STATUS tasks passing after $MAX_ITERATIONS iterations."
    warn "Check tasks.json, progress.md, and logs for status."
fi
