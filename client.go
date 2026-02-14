package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"

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

	// Enable mouse mode
	enableMouseMode(os.Stdout)

	// Send initial resize
	c.sendResize()

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
	wg.Wait()

	c.restore()
	return nil
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
					encoded := Encode(Message{Type: MsgDetach, Payload: nil})
					c.conn.Write(encoded)
					return
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

			// Check for mouse sequence starting at this position
			remaining := buf[i:n]
			if b == '\x1b' && len(remaining) >= 3 && remaining[1] == '[' && remaining[2] == '<' {
				ev, consumed, ok := ParseSGRMouse(remaining)
				if ok {
					c.handleMouse(ev)
					i += consumed - 1 // -1 because loop increments
					continue
				}
			}

			// Any non-mouse keypress in history mode → exit history mode
			if c.historyMode {
				c.exitHistoryMode()
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

	// We want to request lines ending at (total - historyOffset)
	// The session will figure out the actual range from offset+count
	// We encode: offset from start = we don't know total, so we use a special
	// encoding where we send negative offset meaning "from end"
	// Actually, let's send the offset and count and let the session handle it
	payload := make([]byte, 8)
	// Use a sentinel: high bit set means "from end"
	// offset = historyOffset (from end), count = rows
	binary.BigEndian.PutUint32(payload[0:4], uint32(0x80000000|uint32(c.historyOffset)))
	binary.BigEndian.PutUint32(payload[4:8], uint32(rows))

	encoded := Encode(Message{Type: MsgHistoryRequest, Payload: payload})
	c.conn.Write(encoded)
}

// exitHistoryMode returns to live output mode.
func (c *Client) exitHistoryMode() {
	c.historyMode = false
	c.historyOffset = 0

	// Request a "redraw" — offset 0x80000000 (from end, 0 offset = latest)
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
			// Render history on screen
			clearScreen(os.Stdout)
			os.Stdout.Write(msg.Payload)
		}
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
	disableMouseMode(os.Stdout)
	fd := int(os.Stdin.Fd())
	if c.oldState != nil {
		restoreTerminal(fd, c.oldState)
	}
	c.conn.Close()
}
