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
