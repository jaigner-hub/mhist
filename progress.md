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

### Iteration 7 — Task 7: Client
- Created `client.go` with `Client` struct: Unix socket connect, raw mode, I/O relay
- `relayStdin`: prefix key handling (Ctrl+a d=detach, Ctrl+a Ctrl+a=literal), mouse sequence forwarding
- `relaySocket`: decode messages, write MsgData to stdout
- Sends initial MsgResize with terminal dimensions on connect
- Clean restore: disable mouse mode, restore terminal state, close connection
- Compiles and passes `go vet`
