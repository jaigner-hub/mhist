# mhist -- Iteration Prompt

You are an autonomous coding agent building mhist, a terminal multiplexer for mosh scrollback. Each iteration you pick ONE task, implement it, and commit.

## Workflow

1. **Read `tasks.json`** -- this is your source of truth. Find the lowest-id task where `"passes": false`.
2. **Read `progress.md`** for context on what happened in previous iterations.
3. **Read `PRD.md`** for detailed design context and specifications.
4. **Implement the task** following the conventions below.
5. **Run the verification steps** listed in the task's `"steps"` array.
6. **Run tests** if test files exist: `go test ./... -v -count=1`
7. **Update `tasks.json`** -- set `"passes": true` for the completed task. **ONLY change the `passes` field. Never edit, remove, or reorder tasks.**
8. **Update `progress.md`** with a log entry for this iteration.
9. **Commit** with descriptive message using `feat:` or `fix:` prefix.

## Rules for tasks.json

- **NEVER** remove or edit task descriptions, steps, categories, or ids.
- **ONLY** change `"passes": false` to `"passes": true` after verifying all steps pass.
- If a task cannot be completed, leave `"passes": false` and log the blocker in `progress.md`.
- JSON format is intentional -- treat it as immutable test definitions, not a scratchpad.

## Critical Conventions

### Go Patterns
- **Package:** `package main` (single binary)
- **Error handling:** Return errors, don't panic. Use `fmt.Errorf("context: %w", err)` for wrapping.
- **Naming:** camelCase locals, PascalCase exports. Short names for receivers (`b` for buffer, `s` for session).
- **No global state.** Pass dependencies explicitly.
- **Logging:** Use `log.Printf` for session process logs (stderr). Client should be silent except errors.

### File Layout
```
main.go              # CLI dispatch: new, attach, ls, kill, --session-id
buffer.go            # ScrollbackBuffer ring buffer
buffer_test.go       # Buffer unit tests
protocol.go          # Message types, Encode/Decode functions
protocol_test.go     # Protocol round-trip tests
terminal.go          # RawMode, EnableMouseMode, DisableMouseMode, ClearScreen, MoveCursor
mouse.go             # ParseSGRMouse (CSI < params M/m)
mouse_test.go        # Mouse parsing tests
session.go           # Session struct: PTY management, socket listener, client handler
client.go            # Client struct: connect, raw mode, I/O relay, prefix key, history mode
go.mod
go.sum
Makefile
```

### Wire Protocol
Binary framed: `[type:1][length:4 BE][payload:N]`

```go
const (
    MsgData           byte = 0x01
    MsgResize         byte = 0x02
    MsgDetach         byte = 0x03
    MsgKill           byte = 0x04
    MsgHistoryRequest byte = 0x05
    MsgHistoryResponse byte = 0x06
)

type Message struct {
    Type    byte
    Payload []byte
}
```

### Scrollback Buffer Interface
```go
type ScrollbackBuffer struct {
    lines [][]byte
    head  int
    count int
    cap   int
}

func NewScrollbackBuffer(capacity int) *ScrollbackBuffer
func (b *ScrollbackBuffer) Write(data []byte)       // Process raw PTY output, split into lines
func (b *ScrollbackBuffer) Lines() int               // Number of stored lines
func (b *ScrollbackBuffer) GetLine(index int) []byte  // 0 = oldest
func (b *ScrollbackBuffer) GetRange(start, count int) [][]byte
```

### Session Socket Path
```go
func socketDir() string {
    if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
        return filepath.Join(dir, "mhist")
    }
    return fmt.Sprintf("/tmp/mhist-%d", os.Getuid())
}
```

### Terminal Helpers
```go
func enableRawMode(fd int) (*term.State, error)      // term.MakeRaw
func restoreTerminal(fd int, state *term.State)       // term.Restore
func enableMouseMode(w io.Writer)                     // Write "\x1b[?1006h"
func disableMouseMode(w io.Writer)                    // Write "\x1b[?1006l"
func clearScreen(w io.Writer)                         // Write "\x1b[2J\x1b[H"
func getTerminalSize(fd int) (rows, cols int, err error)
```

### Prefix Key Handling
```go
// In client read loop:
// 1. Read byte from stdin
// 2. If byte == 0x01 (Ctrl+a), set prefixActive = true, continue
// 3. If prefixActive:
//    - 'd' → send Detach message, exit
//    - 0x01 → send literal Ctrl+a as Data message
//    - anything else → ignore, clear prefix
// 4. Otherwise → send as Data message
```

### Mouse Parsing
```go
// SGR mouse mode 1006: ESC [ < button ; col ; row M/m
// button 64 = scroll up, button 65 = scroll down
type MouseEvent struct {
    Button int
    Col    int
    Row    int
    Press  bool  // true = M (press), false = m (release)
}

func ParseSGRMouse(data []byte) (MouseEvent, int, bool)  // returns event, bytes consumed, ok
```

### Build
```makefile
.PHONY: build test clean

build:
	go build -o mhist .

test:
	go test ./... -v -count=1

clean:
	rm -f mhist

vet:
	go vet ./...
```

### Dependencies
```
github.com/creack/pty v1.1.24
golang.org/x/term v0.27.0
```

## Completion Signal

When ALL tasks (including 16-19) have `"passes": true` in tasks.json and final verification is done, write this exact line to progress.md:

```
ALL_TASKS_COMPLETE
```

This signals the loop script to stop.

## Important Notes

- **One task per iteration.** Don't try to do multiple tasks at once.
- **Commit after each task.** Use `git add <specific files>` then `git commit -m "feat: ..."`.
- **If a task fails**, note the issue in progress.md and move on to the next task if possible. Come back to fix it later.
- **Read existing code** before writing. Check what files already exist.
- **Don't modify unrelated code.** Stay focused on the current task.
- **Test after every change.** Run `go test ./... -v -count=1` and `go vet ./...` frequently.
