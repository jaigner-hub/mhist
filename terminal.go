package main

import (
	"fmt"
	"io"

	"golang.org/x/term"
)

// enableRawMode puts the terminal into raw mode and returns the previous state.
func enableRawMode(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
}

// restoreTerminal restores the terminal to the given state.
func restoreTerminal(fd int, state *term.State) {
	term.Restore(fd, state)
}

// enableMouseMode enables SGR mouse mode 1006.
func enableMouseMode(w io.Writer) {
	io.WriteString(w, "\x1b[?1006h")
}

// disableMouseMode disables SGR mouse mode 1006.
func disableMouseMode(w io.Writer) {
	io.WriteString(w, "\x1b[?1006l")
}

// clearScreen clears the terminal screen and moves cursor to top-left.
func clearScreen(w io.Writer) {
	io.WriteString(w, "\x1b[2J\x1b[H")
}

// moveCursor moves the cursor to the given row and column (1-based).
func moveCursor(w io.Writer, row, col int) {
	fmt.Fprintf(w, "\x1b[%d;%dH", row, col)
}

// getTerminalSize returns the current terminal dimensions.
func getTerminalSize(fd int) (rows, cols int, err error) {
	cols, rows, err = term.GetSize(fd)
	return rows, cols, err
}
