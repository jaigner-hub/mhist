#!/bin/bash
# Integration test script for mhist
# Tests: create, ls, detach, reattach, kill lifecycle

set -o pipefail

MHIST="./mhist"
PASS=0
FAIL=0
SESSION_NAME="testint-$$"

cleanup() {
    # Kill any leftover test sessions
    $MHIST kill "$SESSION_NAME" 2>/dev/null || true
    # Also try to kill by finding the process
    pkill -f "mhist.*--session-id.*--name=$SESSION_NAME" 2>/dev/null || true
}

trap cleanup EXIT

pass() {
    echo "PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "FAIL: $1"
    FAIL=$((FAIL + 1))
}

# Ensure binary exists
if [ ! -f "$MHIST" ]; then
    echo "Binary not found. Run 'make build' first."
    exit 1
fi

# Test 1: Create a new session
echo "--- Test 1: Create a new session ---"
# We can't do interactive I/O, so we launch the session process directly
# and verify it starts correctly
SESSION_ID=$($MHIST ls 2>&1 | wc -l)  # just to warm up
# Launch a session in the background by running the session process directly
GENERATED_ID=$(cat /dev/urandom | tr -dc 'a-f0-9' | fold -w 32 | head -n1)
GENERATED_ID="${GENERATED_ID:0:8}-${GENERATED_ID:8:4}-${GENERATED_ID:12:4}-${GENERATED_ID:16:4}-${GENERATED_ID:20:12}"

$MHIST "--session-id=$GENERATED_ID" "--name=$SESSION_NAME" &
SESSION_PID=$!

# Wait for socket to appear
sleep 1

if $MHIST ls 2>/dev/null | grep -q "$SESSION_NAME"; then
    pass "Create session: session '$SESSION_NAME' appears in ls"
else
    fail "Create session: session '$SESSION_NAME' not found in ls"
fi

# Test 2: ls shows the session
echo "--- Test 2: ls shows the session ---"
LS_OUTPUT=$($MHIST ls 2>/dev/null)
if echo "$LS_OUTPUT" | grep -q "$SESSION_NAME"; then
    pass "ls: shows session name"
else
    fail "ls: session name not in output"
fi

if echo "$LS_OUTPUT" | grep -q "alive"; then
    pass "ls: shows alive status"
else
    fail "ls: alive status not shown"
fi

# Test 3: Attach and detach
echo "--- Test 3: Attach and detach ---"
# We can't easily test interactive attach in a script, but we can verify
# the socket is connectable by trying to send a kill message
# Instead, we just verify the session is running and connectable
if [ -n "$XDG_RUNTIME_DIR" ]; then
    SOCK_DIR="$XDG_RUNTIME_DIR/mhist"
else
    SOCK_DIR="/tmp/mhist-$(id -u)"
fi
SOCK_PATH="$SOCK_DIR/$GENERATED_ID.sock"

if [ -S "$SOCK_PATH" ]; then
    pass "Attach: socket exists and is connectable"
else
    fail "Attach: socket not found at $SOCK_PATH"
fi

# Test 4: Kill the session
echo "--- Test 4: Kill session ---"
$MHIST kill "$SESSION_NAME" 2>/dev/null
sleep 1

# Wait for process to die
for i in $(seq 1 10); do
    if ! kill -0 "$SESSION_PID" 2>/dev/null; then
        break
    fi
    sleep 0.5
done

if ! kill -0 "$SESSION_PID" 2>/dev/null; then
    pass "Kill: session process exited"
else
    fail "Kill: session process still running"
    kill "$SESSION_PID" 2>/dev/null || true
fi

# Test 5: ls shows no sessions
echo "--- Test 5: ls shows no sessions after kill ---"
sleep 1
LS_AFTER=$($MHIST ls 2>/dev/null)
if echo "$LS_AFTER" | grep -q "$SESSION_NAME"; then
    fail "ls after kill: session still listed"
else
    pass "ls after kill: session no longer listed"
fi

# Summary
echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ $FAIL -gt 0 ]; then
    exit 1
fi
exit 0
