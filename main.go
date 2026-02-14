package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const usage = `Usage: mhist [command] [options]

Commands:
  new [-n name]       Create a new session
  attach [name|id]    Attach to an existing session
  ls                  List sessions
  kill [name|id]      Kill a session

Options:
  --help              Show this help message

With no arguments, attaches to the most recent session or creates a new one.

Prefix key: Ctrl+a
  Ctrl+a d            Detach from session
  Ctrl+a Ctrl+a       Send literal Ctrl+a`

func main() {
	args := os.Args[1:]

	// Internal flag: --session-id=X runs as a session process
	for _, arg := range args {
		if len(arg) > 13 && arg[:13] == "--session-id=" {
			sessionID := arg[13:]
			name := ""
			for _, a := range args {
				if len(a) > 7 && a[:7] == "--name=" {
					name = a[7:]
				}
			}
			runSession(sessionID, name)
			return
		}
	}

	if len(args) == 0 {
		cmdDefault()
		return
	}

	switch args[0] {
	case "new":
		name := ""
		for i := 1; i < len(args); i++ {
			if args[i] == "-n" && i+1 < len(args) {
				name = args[i+1]
				i++
			}
		}
		cmdNew(name)
	case "attach":
		target := ""
		if len(args) > 1 {
			target = args[1]
		}
		cmdAttach(target)
	case "ls":
		cmdList()
	case "kill":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: mhist kill [name|id]\n")
			os.Exit(1)
		}
		cmdKill(args[1])
	case "--help", "-h", "help":
		fmt.Println(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
}

func runSession(id, name string) {
	log.Printf("session starting: id=%s name=%s", id, name)
	sess, err := NewSession(id, name, "")
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	sess.Run()
}

func cmdNew(name string) {
	id := generateID()
	if name == "" {
		name = id[:8]
	}

	socketPath, err := launchSessionProcess(id, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client, err := NewClient(socketPath, id, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to session: %v\n", err)
		os.Exit(1)
	}

	if err := client.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printExitMessage(client, name)
}

func cmdAttach(target string) {
	sessions := listSessions()
	info, err := findSession(sessions, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client, err := NewClient(info.Socket, info.ID, info.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to session: %v\n", err)
		os.Exit(1)
	}

	if err := client.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	printExitMessage(client, info.Name)
}

func cmdDefault() {
	sessions := listSessions()
	if len(sessions) > 0 {
		// Attach to most recent session
		info := sessions[len(sessions)-1]
		client, err := NewClient(info.Socket, info.ID, info.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to session: %v\n", err)
			os.Exit(1)
		}

		if err := client.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		printExitMessage(client, info.Name)
		return
	}

	// No sessions — create new
	cmdNew("")
}

func cmdList() {
	fmt.Printf("%-8s  %-15s  %-20s  %s\n", "ID", "NAME", "CREATED", "STATUS")
	sessions := listSessions()
	for _, info := range sessions {
		shortID := info.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		status := "alive"
		if !isProcessAlive(info.PID) {
			status = "dead"
		}
		fmt.Printf("%-8s  %-15s  %-20s  %s\n", shortID, info.Name, info.Created, status)
	}
}

func cmdKill(target string) {
	sessions := listSessions()
	info, err := findSession(sessions, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Try sending MsgKill via socket
	conn, dialErr := net.Dial("unix", info.Socket)
	if dialErr == nil {
		encoded := Encode(Message{Type: MsgKill, Payload: nil})
		conn.Write(encoded)
		conn.Close()
		fmt.Printf("killed session %s\n", info.Name)
		return
	}

	// Fallback: kill the process directly
	proc, err := os.FindProcess(info.PID)
	if err == nil {
		proc.Kill()
		fmt.Printf("killed session %s (via signal)\n", info.Name)
	}

	// Clean up stale files
	os.Remove(info.Socket)
	infoPath := filepath.Join(socketDir(), info.ID+".json")
	os.Remove(infoPath)
}

// printExitMessage prints the appropriate message after a client exits.
func printExitMessage(client *Client, name string) {
	if client.detached {
		fmt.Fprintf(os.Stderr, "detached from session %s\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "session ended\n")
	}
}

// launchSessionProcess starts a background session process and waits for the socket.
func launchSessionProcess(id, name string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find executable: %w", err)
	}

	dir := socketDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create socket dir: %w", err)
	}

	logPath := filepath.Join(dir, id+".log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return "", fmt.Errorf("create log file: %w", err)
	}

	cmd := exec.Command(self, fmt.Sprintf("--session-id=%s", id), fmt.Sprintf("--name=%s", name))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return "", fmt.Errorf("start session process: %w", err)
	}
	logFile.Close()

	// Wait for socket to appear
	sockPath := filepath.Join(dir, id+".sock")
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(sockPath); err == nil {
			return sockPath, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return "", fmt.Errorf("session socket did not appear within 5 seconds")
}

// listSessions scans the socket directory for session info files.
func listSessions() []SessionInfo {
	dir := socketDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var info SessionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if !isProcessAlive(info.PID) {
			// Clean up stale files
			os.Remove(info.Socket)
			os.Remove(filepath.Join(dir, entry.Name()))
			continue
		}

		sessions = append(sessions, info)
	}
	return sessions
}

// findSession finds a session by name or ID prefix.
func findSession(sessions []SessionInfo, target string) (SessionInfo, error) {
	if target == "" {
		if len(sessions) == 0 {
			return SessionInfo{}, fmt.Errorf("no sessions found")
		}
		return sessions[len(sessions)-1], nil
	}

	for _, info := range sessions {
		if info.Name == target {
			return info, nil
		}
	}

	for _, info := range sessions {
		if strings.HasPrefix(info.ID, target) {
			return info, nil
		}
	}

	return SessionInfo{}, fmt.Errorf("session not found: %s", target)
}

// isProcessAlive checks if a PID is alive.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// generateID generates a random UUID-like identifier.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
