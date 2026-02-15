# mhist

Scrollback history for [mosh](https://mosh.org). A lightweight terminal multiplexer that gives you the one thing mosh is missing — the ability to scroll back through your terminal output.

Unlike tmux or screen, mhist doesn't require switching to a "copy mode" to view history. Just use keyboard shortcuts to scroll, and copy/paste works normally at all times.

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

Produces a single `mhist` binary with no runtime dependencies.

**Requirements:** Go 1.23+

## Usage

```bash
# Start a new session
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

### Auto-start with mosh/ssh

Add this to your `~/.bashrc` on the server to automatically start mhist when you connect:

```bash
# Auto-start mhist
if [ -z "$MHIST_SESSION" ] && [ -n "$SSH_CONNECTION" ]; then
    exec mhist
fi
```

Each SSH/mosh connection gets its own session. Mosh reconnections reuse the existing session automatically. Use `Ctrl+a s` to switch between sessions.

## Keybindings

### Normal mode

| Key | Action |
|-----|--------|
| **Ctrl+a d** | Detach from session |
| **Ctrl+a s** | Switch between sessions |
| **Ctrl+a Ctrl+a** | Send literal Ctrl+a |
| **Ctrl+s** | Enter scroll mode |
| **Page Up** | Enter scroll mode (full page) |

### Scroll mode

| Key | Action |
|-----|--------|
| **k / Arrow Up** | Scroll up |
| **j / Arrow Down** | Scroll down |
| **u** | Half-page up |
| **d** | Half-page down |
| **Page Up / Page Down** | Full page up / down |
| **q / Esc / Ctrl+s** | Exit scroll mode |
| Any other key | Exit scroll mode |

A position indicator `[line N/total]` appears at the top-right while scrolling.

Copy/paste works normally in both modes — text selection is never intercepted.

## How It Works

```
Terminal  -->  mosh  -->  mhist (client)  -->  Unix socket  -->  mhist (session)  -->  PTY  -->  shell
```

Each session runs as an independent background process — no central daemon. Sessions persist after you detach and survive mosh reconnections.

- **Scrollback buffer** — ring buffer stores the last 10,000 lines of output
- **Raw PTY replay** — 64 KB circular buffer preserves exact terminal state (colors, cursor, prompt) for lossless screen redraw on reattach
- **Session switching** — `Ctrl+a s` lets you switch between sessions without disconnecting
- **Partial line tracking** — your current shell prompt is preserved in scrollback
- **Binary protocol** — framed messages over Unix domain sockets for efficient client-session communication

## Session Management

Sessions are stored in `$XDG_RUNTIME_DIR/mhist/` (falls back to `/tmp/mhist-$UID/`). Each session creates:

- `<id>.sock` — Unix socket for client connections
- `<id>.json` — metadata (name, PID, creation time)

Stale sessions are automatically cleaned up when you run `mhist ls`. Connecting to a session with a dead client (e.g., from a dropped mosh connection) automatically takes over.

## Mobile (Termius, etc.)

mhist works well over mosh from mobile terminals:

- **Ctrl+s** to enter scroll mode, then swipe (sends arrow keys) to scroll through history
- Copy/paste works normally — no mouse capture to interfere with text selection
- `Ctrl+a s` to switch sessions

## Dependencies

- [creack/pty](https://github.com/creack/pty) — PTY allocation
- [x/term](https://pkg.go.dev/golang.org/x/term) — terminal raw mode and size detection

## License

MIT
