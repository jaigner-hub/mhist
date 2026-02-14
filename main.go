package main

import (
	"fmt"
	"log"
	"os"
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
		if len(arg) > 14 && arg[:14] == "--session-id=" {
			sessionID := arg[14:]
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
		// Default: attach to last session or create new
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

// Placeholder implementations — these will be filled in by later tasks.

func runSession(id, name string) {
	log.Printf("session starting: id=%s name=%s", id, name)
	sess, err := NewSession(id, name, "")
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}
	sess.Run()
}

func cmdNew(name string) {
	fmt.Println("Creating new session...")
}

func cmdAttach(target string) {
	fmt.Println("Attaching to session...")
}

func cmdDefault() {
	fmt.Println("Default: attach or create session...")
}

func cmdList() {
	fmt.Printf("%-8s  %-15s  %-20s  %s\n", "ID", "NAME", "CREATED", "STATUS")
}

func cmdKill(target string) {
	fmt.Printf("Killing session %s...\n", target)
}
