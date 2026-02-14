package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"

	"golang.org/x/term"
)

// Client connects to a session's Unix socket and relays I/O.
type Client struct {
	conn       net.Conn
	oldState   *term.State
	sessionID  string
	sessionName string
	done       chan struct{}
	once       sync.Once
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

// relayStdin reads from stdin and sends to the session, handling prefix key.
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
				_, consumed, ok := ParseSGRMouse(remaining)
				if ok {
					// Forward the entire mouse sequence as data
					encoded := Encode(Message{Type: MsgData, Payload: remaining[:consumed]})
					c.conn.Write(encoded)
					i += consumed - 1 // -1 because loop increments
					continue
				}
			}

			// Regular data
			encoded := Encode(Message{Type: MsgData, Payload: []byte{b}})
			c.conn.Write(encoded)
		}
	}
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
			os.Stdout.Write(msg.Payload)
		}
	}
}

// sendResize sends the current terminal dimensions to the session.
func (c *Client) sendResize() {
	fd := int(os.Stdout.Fd())
	rows, cols, err := getTerminalSize(fd)
	if err != nil {
		return
	}

	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], uint16(rows))
	binary.BigEndian.PutUint16(payload[2:4], uint16(cols))

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
