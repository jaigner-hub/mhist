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
