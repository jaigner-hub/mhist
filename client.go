package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

const scrollLines = 3 // lines to scroll per mouse wheel event

// Client connects to a session's Unix socket and relays I/O.
type Client struct {
	conn        net.Conn
	oldState    *term.State
	sessionID   string
	sessionName string
	done        chan struct{}
	once        sync.Once

	// History mode state
	historyMode   bool
	historyOffset int // offset from end of buffer (0 = live)
	termRows      int
	termCols      int

	// Exit state
	detached    bool // true if client initiated detach
}

// NewClient connects to the session at the given socket path.
func NewClient(socketPath, sessionID, sessionName string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to session: %w", err)
	}

	return &Client{
		conn:        conn,
		sessionID:   sessionID,
		sessionName: sessionName,
		done:        make(chan struct{}),
	}, nil
}

// Run starts the client I/O relay. Blocks until detach or disconnect.
func (c *Client) Run() error {
	// Put terminal in raw mode
	fd := int(os.Stdin.Fd())
	oldState, err := enableRawMode(fd)
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("enable raw mode: %w", err)
	}
	c.oldState = oldState

	// Get terminal size
	rows, cols, err := getTerminalSize(fd)
	if err == nil {
		c.termRows = rows
		c.termCols = cols
	} else {
		c.termRows = 24
		c.termCols = 80
	}

	// Mouse mode starts disabled (enables on scroll mode entry for copy/paste compat)

	// Send initial resize
	c.sendResize()

	// Handle SIGWINCH for terminal resize
	go c.handleSigwinch()

	// Start I/O relay goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		c.relayStdin()
	}()

	go func() {
		defer wg.Done()
		c.relaySocket()
	}()

	// Wait for either goroutine to finish
	<-c.done

	// Unblock the other goroutine: close conn (unblocks relaySocket)
	// and close stdin (unblocks relayStdin)
	c.conn.Close()
	os.Stdin.Close()

	wg.Wait()

	c.restore()
	return nil
}

// handleSigwinch handles terminal resize signals.
func (c *Client) handleSigwinch() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	for {
		select {
		case <-sigCh:
			fd := int(os.Stdout.Fd())
			rows, cols, err := getTerminalSize(fd)
			if err == nil {
				c.termRows = rows
				c.termCols = cols
				c.sendResize()
			}
		case <-c.done:
			signal.Stop(sigCh)
			return
		}
	}
}

// relayStdin reads from stdin and sends to the session, handling prefix key and history.
func (c *Client) relayStdin() {
	defer c.signalDone()

	buf := make([]byte, 4096)
	prefixActive := false

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return
		}

		for i := 0; i < n; i++ {
			b := buf[i]

			if prefixActive {
				prefixActive = false
				switch b {
				case 'd':
					// Detach
					c.detached = true
					encoded := Encode(Message{Type: MsgDetach, Payload: nil})
					c.conn.Write(encoded)
					return
				case '[':
					// Enter history/scroll mode
					if !c.historyMode {
						c.historyMode = true
						c.historyOffset = scrollLines
						c.requestHistory()
					}
				case 0x01:
					// Send literal Ctrl+a
					if c.historyMode {
						c.exitHistoryMode()
					}
					encoded := Encode(Message{Type: MsgData, Payload: []byte{0x01}})
					c.conn.Write(encoded)
				default:
					// Unknown prefix command — ignore
				}
				continue
			}

			if b == 0x01 {
				prefixActive = true
				continue
			}

			// Ctrl+s toggles scroll/history mode
			if b == 0x13 {
				if c.historyMode {
					c.exitHistoryMode()
				} else {
					c.historyMode = true
					c.historyOffset = scrollLines
					c.requestHistory()
				}
				continue
			}

			// Check for escape sequences starting at this position
			remaining := buf[i:n]
			if b == '\x1b' && len(remaining) >= 3 && remaining[1] == '[' {
				// SGR mouse: ESC [ < ...
				if remaining[2] == '<' {
					ev, consumed, ok := ParseSGRMouse(remaining)
					if ok {
						c.handleMouse(ev)
						i += consumed - 1 // -1 because loop increments
						continue
					}
				}

				// Page Up: ESC [ 5 ~
				if len(remaining) >= 4 && remaining[2] == '5' && remaining[3] == '~' {
					if !c.historyMode {
						c.historyMode = true
						c.historyOffset = c.termRows
					} else {
						c.historyOffset += c.termRows
					}
					c.requestHistory()
					i += 3 // skip remaining 3 bytes of sequence
					continue
				}

				// Page Down: ESC [ 6 ~
				if len(remaining) >= 4 && remaining[2] == '6' && remaining[3] == '~' {
					if c.historyMode {
						c.historyOffset -= c.termRows
						if c.historyOffset <= 0 {
							c.exitHistoryMode()
						} else {
							c.requestHistory()
						}
					}
					i += 3 // skip remaining 3 bytes of sequence
					continue
				}

				// Arrow keys in history mode: Up (A) scrolls up, Down (B) scrolls down
				if c.historyMode && (remaining[2] == 'A' || remaining[2] == 'B') {
					if remaining[2] == 'A' {
						c.historyOffset += scrollLines
						c.requestHistory()
					} else {
						c.historyOffset -= scrollLines
						if c.historyOffset <= 0 {
							c.exitHistoryMode()
						} else {
							c.requestHistory()
						}
					}
					i += 2 // skip remaining 2 bytes of sequence
					continue
				}
			}

			// History mode key bindings (vim-style)
			if c.historyMode {
				switch b {
				case 'k': // up
					c.historyOffset += scrollLines
					c.requestHistory()
				case 'j': // down
					c.historyOffset -= scrollLines
					if c.historyOffset <= 0 {
						c.exitHistoryMode()
					} else {
						c.requestHistory()
					}
				case 'u': // half page up
					c.historyOffset += c.termRows / 2
					c.requestHistory()
				case 'd': // half page down
					c.historyOffset -= c.termRows / 2
					if c.historyOffset <= 0 {
						c.exitHistoryMode()
					} else {
						c.requestHistory()
					}
				case 'q', 0x1b: // q or Escape exits
					c.exitHistoryMode()
				default:
					c.exitHistoryMode()
				}
				continue
			}

			// Regular data — forward to session
			encoded := Encode(Message{Type: MsgData, Payload: []byte{b}})
			c.conn.Write(encoded)
		}
	}
}

// handleMouse processes a parsed mouse event.
func (c *Client) handleMouse(ev MouseEvent) {
	switch ev.Button {
	case 64: // Scroll up
		if !c.historyMode {
			c.historyMode = true
			c.historyOffset = scrollLines
		} else {
			c.historyOffset += scrollLines
		}
		c.requestHistory()

	case 65: // Scroll down
		if c.historyMode {
			c.historyOffset -= scrollLines
			if c.historyOffset <= 0 {
				c.exitHistoryMode()
				return
			}
			c.requestHistory()
		}
		// If not in history mode, ignore scroll down

	default:
		// Other mouse events in history mode → exit
		if c.historyMode && ev.Press {
			c.exitHistoryMode()
		}
	}
}

// requestHistory sends a history request to the session.
func (c *Client) requestHistory() {
	rows := c.termRows
	if rows <= 0 {
		rows = 24
	}

	payload := make([]byte, 8)
	// High bit set means "from end"
	binary.BigEndian.PutUint32(payload[0:4], uint32(0x80000000|uint32(c.historyOffset)))
	binary.BigEndian.PutUint32(payload[4:8], uint32(rows))

	encoded := Encode(Message{Type: MsgHistoryRequest, Payload: payload})
	c.conn.Write(encoded)
}

// exitHistoryMode returns to live output mode.
func (c *Client) exitHistoryMode() {
	c.historyMode = false
	c.historyOffset = 0

	// Request redraw of latest lines
	rows := c.termRows
	if rows <= 0 {
		rows = 24
	}
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], uint32(0x80000000))
	binary.BigEndian.PutUint32(payload[4:8], uint32(rows))

	encoded := Encode(Message{Type: MsgHistoryRequest, Payload: payload})
	c.conn.Write(encoded)
}

// relaySocket reads messages from the session socket and writes to stdout.
func (c *Client) relaySocket() {
	defer c.signalDone()

	for {
		msg, err := Decode(c.conn)
		if err != nil {
			return
		}

		switch msg.Type {
		case MsgData:
			if !c.historyMode {
				os.Stdout.Write(msg.Payload)
			}
			// In history mode, suppress live output

		case MsgHistoryResponse:
			c.renderHistory(msg.Payload)
		}
	}
}

// renderHistory renders history lines and optional position indicator.
func (c *Client) renderHistory(payload []byte) {
	if len(payload) < 8 {
		return
	}

	startLine := int(binary.BigEndian.Uint32(payload[0:4]))
	totalLines := int(binary.BigEndian.Uint32(payload[4:8]))
	lineData := payload[8:]

	clearScreen(os.Stdout)
	os.Stdout.Write(lineData)

	// Show scroll position indicator at top-right if in history mode
	if c.historyMode && totalLines > 0 {
		indicator := fmt.Sprintf("[line %d/%d]", startLine+1, totalLines)
		col := c.termCols - len(indicator) + 1
		if col < 1 {
			col = 1
		}
		// Save cursor, move to top-right, print indicator, restore cursor
		io.WriteString(os.Stdout, "\x1b7")           // save cursor
		moveCursor(os.Stdout, 1, col)                 // move to top-right
		io.WriteString(os.Stdout, "\x1b[7m")          // reverse video
		io.WriteString(os.Stdout, indicator)           // print indicator
		io.WriteString(os.Stdout, "\x1b[27m")         // reset reverse
		io.WriteString(os.Stdout, "\x1b8")            // restore cursor
	}
}

// sendResize sends the current terminal dimensions to the session.
func (c *Client) sendResize() {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], uint16(c.termRows))
	binary.BigEndian.PutUint16(payload[2:4], uint16(c.termCols))

	encoded := Encode(Message{Type: MsgResize, Payload: payload})
	c.conn.Write(encoded)
}

// signalDone signals that the client should shut down.
func (c *Client) signalDone() {
	c.once.Do(func() {
		close(c.done)
	})
}

// restore restores terminal state and disables mouse mode.
func (c *Client) restore() {
	fd := int(os.Stdin.Fd())
	if c.oldState != nil {
		restoreTerminal(fd, c.oldState)
	}
	c.conn.Close()
}
