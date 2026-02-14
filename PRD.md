# PRD: mhist

## Overview

mhist is a terminal multiplexer that solves one problem: **mosh has no scrollback history**. Unlike tmux/screen which require "copy mode", mhist lets you scroll back with mouse wheel or phone swipe — no mode switching.

Task tracking is in `tasks.json`. This document is a design reference only.

## Architecture

Per-session process model (like screen):

```
Terminal → mosh → mhist (client) → Unix socket → mhist (session process) → PTY → bash/zsh
```

- `mhist new` starts a background session process (`mhist --session-id=X`), then connects to it as a client
- Each session = its own background process holding a PTY + scrollback buffer + Unix socket
- When you detach, client exits, session process stays alive
- When you reattach, new client connects to the session's socket
- **No central daemon.** Each session is independent.

## Key Design Decisions

- **Language:** Go, single binary, no CGo
- **Dependencies:** `github.com/creack/pty`, `golang.org/x/term` (nothing else)
- **Session sockets:** `$XDG_RUNTIME_DIR/mhist/<id>.sock` (fallback `/tmp/mhist-$UID/`)
- **Session info:** `$XDG_RUNTIME_DIR/mhist/<id>.json` (metadata: name, pid, created time)
- **Scrollback:** Ring buffer, default 10,000 lines
- **Mouse detection:** SGR mouse mode 1006 (mosh passes these through)
- **Protocol:** `[type:1][length:4 BE][payload:N]` over Unix socket
- **Prefix key:** Ctrl+a (like screen)

---

## Commands

| Command | Action |
|---------|--------|
| `mhist` | Attach to last session, or create new if none |
| `mhist new [-n name]` | Create new session, attach to it |
| `mhist attach [name\|id]` | Attach to existing session |
| `mhist ls` | List sessions (from info files) |
| `mhist kill [name\|id]` | Kill a session |

---

## Key Bindings

| Key | Action |
|-----|--------|
| Ctrl+a d | Detach |
| Ctrl+a Ctrl+a | Send literal Ctrl+a |
| Mouse wheel up | Scroll back through history |
| Mouse wheel down | Scroll forward / return to live |
| Any key (in history) | Return to live mode |

---

## Wire Protocol

Binary framed protocol over Unix domain socket:

```
[type:1 byte][length:4 bytes big-endian][payload:N bytes]
```

### Message Types

| Type | Value | Direction | Payload |
|------|-------|-----------|---------|
| Data | 0x01 | Both | Raw terminal data |
| Resize | 0x02 | Client→Session | `[rows:2 BE][cols:2 BE]` |
| Detach | 0x03 | Client→Session | Empty |
| Kill | 0x04 | Client→Session | Empty |
| HistoryRequest | 0x05 | Client→Session | `[offset:4 BE][count:4 BE]` |
| HistoryResponse | 0x06 | Session→Client | History lines data |

---

## Scrollback Buffer

Ring buffer holding N lines (default 10,000):

- Each line = byte slice (raw terminal output for that line)
- New output appends; when full, oldest line is overwritten
- Client requests history chunks via HistoryRequest
- Session responds with rendered lines

---

## Session Process

Each session runs as:

```
mhist --session-id=<uuid> [--name=<name>] [--shell=<path>]
```

Responsibilities:
1. Allocate PTY, exec shell
2. Read PTY output → feed to scrollback buffer + forward to connected client
3. Listen on Unix socket for one client at a time
4. Handle resize, detach, kill messages
5. On shell exit → clean up socket + info file, exit
6. Write `<id>.json` with: `{"id": "...", "name": "...", "pid": N, "created": "...", "socket": "..."}`

---

## Client

The client process:
1. Connect to session's Unix socket
2. Put terminal in raw mode
3. Enable SGR mouse mode (1006)
4. Relay stdin → socket (with prefix key interception)
5. Relay socket → stdout
6. On detach → restore terminal, exit cleanly
7. Handle Ctrl+a prefix: `d` = detach, Ctrl+a = send literal

---

## History Scrollback

When user scrolls up (mouse wheel):
1. Client enters "history mode"
2. Sends HistoryRequest to session for lines at offset
3. Receives HistoryResponse with line data
4. Renders history lines on screen with scroll position indicator
5. Any non-scroll keypress → exits history mode, returns to live output

Scroll position indicator: `[123/10000]` displayed at top-right corner.

---

## Session Discovery

`mhist ls` and `mhist attach`:
- Scan `$XDG_RUNTIME_DIR/mhist/*.json` for info files
- Read each, check if PID is alive (`kill -0`)
- Display: ID (short), name, created time, status
- Clean up stale info files (PID dead)

---

## File Layout

```
mhist/
├── main.go              # CLI dispatch (new, attach, ls, kill, session mode)
├── buffer.go            # Ring buffer for scrollback
├── buffer_test.go       # Buffer tests
├── protocol.go          # Wire protocol encode/decode
├── protocol_test.go     # Protocol tests
├── terminal.go          # Raw mode, mouse mode, screen operations
├── mouse.go             # SGR mouse event parsing
├── mouse_test.go        # Mouse parsing tests
├── session.go           # Session process (PTY + buffer + socket)
├── client.go            # Client (connect, relay, prefix key, history)
├── go.mod
├── go.sum
├── Makefile
├── PRD.md
├── PROMPT.md
├── tasks.json
├── progress.md
└── ralph.sh
```

---

## Edge Cases

- **No sessions exist:** `mhist` with no args creates a new session
- **Session already has client:** Reject with error "session already attached"
- **Shell exits:** Session process detects EOF on PTY, cleans up, exits
- **Client crashes:** Session detects socket close, returns to waiting for client
- **Stale sockets/info files:** Cleaned up on `mhist ls` and `mhist attach`
- **Terminal resize:** Client catches SIGWINCH, sends Resize message, session resizes PTY
- **Ctrl+a in shell:** Ctrl+a Ctrl+a sends literal Ctrl+a through
