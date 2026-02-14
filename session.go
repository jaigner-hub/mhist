package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Session holds the state for a running session process.
type Session struct {
	id         string
	name       string
	ptmx       *os.File
	cmd        *exec.Cmd
	buffer     *ScrollbackBuffer
	listener   net.Listener
	socketPath string
	infoPath   string
	client     net.Conn
	clientMu   sync.Mutex
	lastRows   int // last known terminal rows for redraw
	rawBuf     []byte // 64KB circular buffer for raw PTY replay
	rawHead    int    // next write position in rawBuf
	rawLen     int    // bytes currently stored in rawBuf
}

// SessionInfo is the JSON metadata written to the info file.
type SessionInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	Created string `json:"created"`
	Socket  string `json:"socket"`
}

// socketDir returns the directory for session sockets and info files.
func socketDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "mhist")
	}
	return fmt.Sprintf("/tmp/mhist-%d", os.Getuid())
}

// NewSession creates and starts a new session.
func NewSession(id, name, shell string) (*Session, error) {
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	cmd := exec.Command(shell)
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	dir := socketDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		ptmx.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("create socket dir: %w", err)
	}

	sockPath := filepath.Join(dir, id+".sock")
	infoPath := filepath.Join(dir, id+".json")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		ptmx.Close()
		cmd.Process.Kill()
		return nil, fmt.Errorf("listen socket: %w", err)
	}

	s := &Session{
		id:         id,
		name:       name,
		ptmx:       ptmx,
		cmd:        cmd,
		buffer:     NewScrollbackBuffer(10000),
		listener:   listener,
		socketPath: sockPath,
		infoPath:   infoPath,
		rawBuf:     make([]byte, 65536),
	}

	if err := s.writeInfoFile(); err != nil {
		s.cleanup()
		return nil, fmt.Errorf("write info file: %w", err)
	}

	return s, nil
}

// writeInfoFile writes session metadata to the info JSON file.
func (s *Session) writeInfoFile() error {
	info := SessionInfo{
		ID:      s.id,
		Name:    s.name,
		PID:     os.Getpid(),
		Created: time.Now().Format(time.RFC3339),
		Socket:  s.socketPath,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(s.infoPath, data, 0600)
}

// Run starts the session event loop. Blocks until the session ends.
func (s *Session) Run() {
	// Handle signals for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Channel to signal PTY EOF
	ptyDone := make(chan struct{})

	// Read PTY output, feed to buffer and forward to client
	go s.readPTY(ptyDone)

	// Accept client connections
	go s.acceptClients()

	// Wait for shell exit or signal
	select {
	case <-ptyDone:
		log.Printf("session %s: shell exited", s.id)
	case sig := <-sigCh:
		log.Printf("session %s: received %v, shutting down", s.id, sig)
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	}

	s.cleanup()
}

// readPTY reads from the PTY and distributes output.
func (s *Session) readPTY(done chan<- struct{}) {
	defer close(done)
	buf := make([]byte, 4096)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			s.buffer.Write(data)

			// Append to raw circular replay buffer
			cap := len(s.rawBuf)
			for _, b := range data {
				s.rawBuf[s.rawHead] = b
				s.rawHead = (s.rawHead + 1) % cap
				if s.rawLen < cap {
					s.rawLen++
				}
			}

			s.clientMu.Lock()
			if s.client != nil {
				encoded := Encode(Message{Type: MsgData, Payload: data})
				s.client.Write(encoded)
			}
			s.clientMu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// acceptClients listens for incoming client connections.
func (s *Session) acceptClients() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.clientMu.Lock()
		if s.client != nil {
			// Reject — already has a client
			s.clientMu.Unlock()
			errMsg := Encode(Message{Type: MsgData, Payload: []byte("session already attached\r\n")})
			conn.Write(errMsg)
			conn.Close()
			continue
		}
		s.client = conn
		s.clientMu.Unlock()

		log.Printf("session %s: client connected", s.id)

		// Send recent scrollback lines for screen redraw
		s.sendRedraw(conn)

		go s.handleClient(conn)
	}
}

// handleClient reads messages from a connected client.
func (s *Session) handleClient(conn net.Conn) {
	defer func() {
		s.clientMu.Lock()
		if s.client == conn {
			s.client = nil
		}
		s.clientMu.Unlock()
		conn.Close()
		log.Printf("session %s: client disconnected", s.id)
	}()

	for {
		msg, err := Decode(conn)
		if err != nil {
			return
		}

		switch msg.Type {
		case MsgData:
			s.ptmx.Write(msg.Payload)

		case MsgResize:
			if len(msg.Payload) >= 4 {
				rows := int(msg.Payload[0])<<8 | int(msg.Payload[1])
				cols := int(msg.Payload[2])<<8 | int(msg.Payload[3])
				s.lastRows = rows
				pty.Setsize(s.ptmx, &pty.Winsize{
					Rows: uint16(rows),
					Cols: uint16(cols),
				})
			}

		case MsgDetach:
			return

		case MsgKill:
			if s.cmd.Process != nil {
				s.cmd.Process.Kill()
			}
			return

		case MsgHistoryRequest:
			s.handleHistoryRequest(conn, msg.Payload)
		}
	}
}

// sendRedraw replays raw PTY output from the circular buffer to the client.
func (s *Session) sendRedraw(conn net.Conn) {
	if s.rawLen == 0 {
		return
	}

	// Extract rawLen bytes from the circular buffer
	cap := len(s.rawBuf)
	startPos := (s.rawHead - s.rawLen + cap) % cap
	raw := make([]byte, s.rawLen)
	for i := 0; i < s.rawLen; i++ {
		raw[i] = s.rawBuf[(startPos+i)%cap]
	}

	// Prepend clear screen, then send raw replay
	var redraw []byte
	redraw = append(redraw, []byte("\x1b[2J\x1b[H")...)
	redraw = append(redraw, raw...)

	encoded := Encode(Message{Type: MsgData, Payload: redraw})
	conn.Write(encoded)
}


// handleHistoryRequest responds to a client's history request.
func (s *Session) handleHistoryRequest(conn net.Conn, payload []byte) {
	if len(payload) < 8 {
		return
	}
	rawOffset := binary.BigEndian.Uint32(payload[0:4])
	count := int(binary.BigEndian.Uint32(payload[4:8]))

	totalLines := s.buffer.Lines()
	var start int

	if rawOffset&0x80000000 != 0 {
		// "From end" mode: offset is distance from end
		fromEnd := int(rawOffset & 0x7FFFFFFF)
		start = totalLines - fromEnd - count
		if start < 0 {
			start = 0
		}
	} else {
		start = int(rawOffset)
	}

	lines := s.buffer.GetRange(start, count)

	// Build response: [startLine:4 BE][totalLines:4 BE][line data]
	var result []byte
	header := make([]byte, 8)
	binary.BigEndian.PutUint32(header[0:4], uint32(start))
	binary.BigEndian.PutUint32(header[4:8], uint32(totalLines))
	result = append(result, header...)

	for i, line := range lines {
		result = append(result, line...)
		if i < len(lines)-1 {
			result = append(result, '\r', '\n')
		}
	}

	resp := Encode(Message{Type: MsgHistoryResponse, Payload: result})
	conn.Write(resp)
}

// cleanup removes socket and info files and reaps the child process.
func (s *Session) cleanup() {
	s.clientMu.Lock()
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	s.clientMu.Unlock()

	s.listener.Close()
	s.ptmx.Close()
	s.cmd.Wait() // reap child process
	os.Remove(s.socketPath)
	os.Remove(s.infoPath)
	log.Printf("session %s: cleaned up", s.id)
}
