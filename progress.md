# mhist Progress

## Iteration Log

### Iteration 1 — Task 1: Project Setup
- Ran `go mod init mhist`, added `creack/pty` and `x/term` dependencies
- Created `main.go` with CLI dispatch: `new`, `attach`, `ls`, `kill`, `--session-id` (internal), `--help`, and default behavior
- Created `Makefile` with `build`, `test`, `clean`, `vet` targets
- Verified: `go build -o mhist .` succeeds, `./mhist --help` prints usage, `make build` succeeds, `go vet` passes
- Note: Go build cache needed `GOCACHE=/tmp/claude/go-build` due to read-only default cache

### Iteration 2 — Task 2: Scrollback Buffer
- Created `buffer.go` with `ScrollbackBuffer` ring buffer: `Write`, `Lines`, `GetLine`, `GetRange`
- Handles partial lines (no trailing `\n`) by buffering until next write
- Created `buffer_test.go` with 9 tests: empty, single line, multi-line, wraparound, partial lines, GetRange bounds, out-of-range, large wraparound
- All tests pass

### Iteration 3 — Task 3: Wire Protocol
- Created `protocol.go` with `Message` struct, `Encode`/`Decode` functions, 6 message type constants
- Wire format: `[type:1][length:4 BE][payload:N]`
- Created `protocol_test.go` with 11 tests: round-trip for all message types, empty payload, large payload, partial read error, truncated payload, multiple messages
- All tests pass

### Iteration 4 — Task 4: Terminal Helpers
- Created `terminal.go` with: `enableRawMode`, `restoreTerminal`, `enableMouseMode`, `disableMouseMode`, `clearScreen`, `moveCursor`, `getTerminalSize`
- Uses `golang.org/x/term` for raw mode and terminal size
- SGR mouse mode 1006 escape sequences
- Compiles and passes `go vet`

### Iteration 5 — Task 5: SGR Mouse Parsing
- Created `mouse.go` with `MouseEvent` struct and `ParseSGRMouse` function
- Parses `ESC [ < button ; col ; row M/m` format, returns bytes consumed
- Created `mouse_test.go` with 11 tests: scroll up/down, left/middle/right click, release, incomplete, too short, invalid, trailing data, bad params
- All tests pass (31 total across all test files)

### Iteration 6 — Task 6: Session Process
- Created `session.go` with `Session` struct: PTY management, scrollback buffer (10k lines), Unix socket listener
- `NewSession`: allocates PTY, starts shell, creates socket at `socketDir()/<id>.sock`, writes `<id>.json` info file
- Accepts one client at a time (rejects additional with error)
- Reads PTY output: feeds to buffer + forwards to connected client as MsgData
- Handles messages: MsgData→PTY, MsgResize→resize PTY, MsgDetach→close client, MsgKill→kill shell
- MsgHistoryRequest→respond with lines from buffer
- Signal handling (SIGTERM/SIGINT) for clean shutdown
- Cleanup removes socket and info files
- Wired `runSession` in `main.go` to use `NewSession` + `Run`
- Compiles and passes `go vet`

### Iteration 8 — Task 8: Wire Up new/attach Commands
- Implemented `cmdNew`: generates random UUID, launches background session process via re-exec with `--session-id`, waits for socket, connects as client
- Implemented `cmdAttach`: finds session by name or ID prefix, connects as client
- Implemented `cmdDefault`: attaches to most recent session or creates new if none
- Implemented `launchSessionProcess`: sets Setsid for independence, redirects to log file, waits for socket
- Implemented `listSessions`: scans info files, checks PID alive, cleans up stale files
- Implemented `findSession`: matches by name or ID prefix
- Used `crypto/rand` for UUID generation (no external dependency)
- `./mhist ls` works, `make build` succeeds, all tests pass

### Iteration 9 — Task 9: List and Kill Commands
- Already implemented in Task 8 as part of full `cmdList` and `cmdKill` implementation
- `cmdList`: scans info files, displays table with short ID, name, created, status (alive/dead)
- `cmdKill`: finds session, sends MsgKill via socket, falls back to os.Kill, cleans up stale files
- `listSessions` cleans stale files automatically
- Verified: `./mhist ls` displays header, all tests pass

### Iteration 10 — Task 10: Detach and Reattach
- Ctrl+a d detach was already implemented in client (task 7)
- Added `sendRedraw` to session: on client connect, sends last N lines (where N = terminal rows) from scrollback buffer
- Session tracks `lastRows` from MsgResize for redraw sizing (defaults to 24)
- Redraw clears screen and renders recent lines
- Compiles, passes vet and all tests

### Iteration 11 — Task 11: History Scrollback
- Added history mode to client: scroll up (button 64) enters history mode, scroll down (button 65) goes forward
- Client sends MsgHistoryRequest with "from end" offset encoding (high bit sentinel)
- Session handles from-end offset, responds with MsgHistoryResponse containing buffer lines
- History response clears screen and renders history lines
- Any non-mouse keypress exits history mode, requests redraw of latest lines
- Live output suppressed while in history mode
- Scrolls 3 lines per wheel event
- Compiles, passes vet and all tests

### Iteration 12 — Task 12: Window Resize and Scroll Indicator
- Added SIGWINCH handler to client: catches signal, reads new terminal size, sends MsgResize
- Session already handles MsgResize → PTY resize (from task 6)
- Added scroll position indicator: `[line N/total]` at top-right corner in reverse video
- Indicator uses save/restore cursor to avoid disrupting display
- History response now includes 8-byte header (startLine + totalLines) for position tracking
- Compiles, passes vet and all tests

### Iteration 13 — Task 13: Session Cleanup and Client Notification
- Session already detects shell exit via PTY EOF and cleans up socket + info files (from task 6)
- Session already handles SIGTERM/SIGINT: kills shell, runs cleanup (from task 6)
- Added `detached` field to Client to track detach vs unexpected disconnect
- Client now prints "detached from session <name>" on Ctrl+a d
- Client now prints "session ended" on unexpected connection close (shell exit)
- Compiles, passes vet and all 31 tests

### Iteration 14 — Task 14: Integration Test Script
- Created `test_integration.sh` with 6 tests: create session, ls shows it, socket exists, kill session, process exits, ls shows nothing
- Fixed `--session-id=` flag parsing: prefix is 13 chars, not 14 (off-by-one bug)
- Script launches session process directly, tests lifecycle without interactive I/O
- All 6 integration tests pass, all 31 unit tests pass

### Iteration 15 — Task 15: Final Verification
- `make build` — produces `./mhist` binary (3.8MB)
- `make test` — all 31 unit tests pass (buffer: 9, mouse: 11, protocol: 11)
- `make vet` — no issues
- `bash test_integration.sh` — all 6 integration tests pass
- `./mhist ls` — exits 0, shows header with no sessions
- All 15 tasks pass

--- Tasks 1-15 complete. Starting fix cycle for reattach display + scrollback history (tasks 16-19). ---

### Iteration 18 — Task 18: Include partial line in history responses
- In `handleHistoryRequest`, after building lines from `GetRange()`, checks if response includes the most recent lines
- If so, calls `buffer.GetPartial()` and appends the partial line (current prompt) to the response data
- This ensures the shell prompt appears in history scrollback and redraw responses
- Build, vet, and all 34 tests pass

### Iteration 17 — Task 17: Raw PTY replay buffer for sendRedraw
- Added `rawBuf []byte` (64KB), `rawHead int`, `rawLen int` to Session struct for circular raw PTY buffer
- Initialized `rawBuf = make([]byte, 65536)` in NewSession
- In `readPTY`, after `buffer.Write(data)`, appends each byte to rawBuf circular buffer
- Rewrote `sendRedraw` to extract raw bytes from circular buffer, prepend clear screen escape, and send as single MsgData — no longer reconstructs from parsed lines
- Build, vet, and all 34 tests pass

### Iteration 16 — Task 16: GetPartial() method for ScrollbackBuffer
- Added `GetPartial()` method to `buffer.go`: returns a copy of the current partial line, or nil if empty
- Added 3 tests to `buffer_test.go`: `TestBufferGetPartial` (partial data present), `TestBufferGetPartialEmpty` (fresh buffer), `TestBufferGetPartialAfterNewline` (after complete line)
- All 34 tests pass (12 buffer, 11 mouse, 11 protocol), `go vet` clean

### Iteration 7 — Task 7: Client
- Created `client.go` with `Client` struct: Unix socket connect, raw mode, I/O relay
- `relayStdin`: prefix key handling (Ctrl+a d=detach, Ctrl+a Ctrl+a=literal), mouse sequence forwarding
- `relaySocket`: decode messages, write MsgData to stdout
- Sends initial MsgResize with terminal dimensions on connect
- Clean restore: disable mouse mode, restore terminal state, close connection
- Compiles and passes `go vet`
