# mhist

Scrollback history for [mosh](https://mosh.org). A lightweight terminal multiplexer that gives you the one thing mosh is missing — the ability to scroll back through your terminal output.

Unlike tmux or screen, mhist doesn't require switching to a "copy mode" to view history. Just scroll with your mouse wheel, swipe, or use keyboard shortcuts.

## Install

```bash
go install github.com/your-username/mhist@latest
```

Or build from source:

```bash
git clone https://github.com/your-username/mhist.git
cd mhist
make build
```

Produces a single `mhist` binary (~3.8 MB) with no runtime dependencies.

**Requirements:** Go 1.23+

## Usage

```bash
# Start a new session (or reattach to the most recent one)
mhist

# Start a named session
mhist new -n work

# List sessions
mhist ls

# Attach to a session by name or ID prefix
mhist attach work

# Kill a session
mhist kill work
```

### With mosh

```bash
mosh myserver -- mhist
```

This gives you a mosh connection with full scrollback history.

## Keybindings

| Key | Action |
|-----|--------|
| **Mouse wheel up/down** | Scroll through history |
| **Page Up / Page Down** | Scroll by page |
| **Ctrl+s** | Toggle scroll mode |
| **Ctrl+a [** | Enter scroll mode |
| **q / Esc** | Exit scroll mode |
| **k / j** | Scroll up / down (vim-style) |
| **u / d** | Half-page up / down (vim-style) |
| **Arrow Up / Down** | Scroll up / down (in scroll mode) |
| **Ctrl+a d** | Detach from session |
| **Ctrl+a Ctrl+a** | Send literal Ctrl+a |

A position indicator `[line N/total]` appears at the top-right while scrolling.

## How It Works

```
Terminal  -->  mosh  -->  mhist (client)  -->  Unix socket  -->  mhist (session)  -->  PTY  -->  shell
```

Each session runs as an independent background process — no central daemon. Sessions persist after you detach and survive mosh reconnections.

- **Scrollback buffer** — ring buffer stores the last 10,000 lines of output
- **Raw PTY replay** — 64 KB circular buffer preserves exact terminal state for lossless screen redraw on reattach
- **Binary protocol** — framed messages over Unix domain sockets for efficient client-session communication
- **SGR mouse mode** — mouse wheel events work reliably through mosh tunnels
- **Partial line tracking** — your current shell prompt is preserved in scrollback

## Session Management

Sessions are stored in `$XDG_RUNTIME_DIR/mhist/` (falls back to `/tmp/mhist-$UID/`). Each session creates:

- `<id>.sock` — Unix socket for client connections
- `<id>.json` — metadata (name, PID, creation time)

Stale sessions are automatically cleaned up when you run `mhist ls`.

Only one client can be attached to a session at a time. Connecting a new client disconnects the previous one.

## Dependencies

- [creack/pty](https://github.com/creack/pty) — PTY allocation
- [x/term](https://pkg.go.dev/golang.org/x/term) — terminal raw mode and size detection

## License

MIT
