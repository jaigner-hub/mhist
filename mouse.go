package main

import "strconv"

// MouseEvent represents a parsed SGR mouse event.
type MouseEvent struct {
	Button int
	Col    int
	Row    int
	Press  bool // true = M (press), false = m (release)
}

// ParseSGRMouse parses an SGR mouse sequence from data.
// Format: ESC [ < button ; col ; row M/m
// Returns the event, bytes consumed, and whether parsing succeeded.
func ParseSGRMouse(data []byte) (MouseEvent, int, bool) {
	// Minimum: ESC [ < digit ; digit ; digit M = 10 bytes
	if len(data) < 9 {
		return MouseEvent{}, 0, false
	}
	if data[0] != '\x1b' || data[1] != '[' || data[2] != '<' {
		return MouseEvent{}, 0, false
	}

	// Find terminator M or m
	termIdx := -1
	for i := 3; i < len(data); i++ {
		if data[i] == 'M' || data[i] == 'm' {
			termIdx = i
			break
		}
		// Only digits and semicolons allowed between < and terminator
		if data[i] != ';' && (data[i] < '0' || data[i] > '9') {
			return MouseEvent{}, 0, false
		}
	}
	if termIdx == -1 {
		return MouseEvent{}, 0, false
	}

	// Parse button;col;row
	params := string(data[3:termIdx])
	parts := splitSemicolon(params)
	if len(parts) != 3 {
		return MouseEvent{}, 0, false
	}

	button, err := strconv.Atoi(parts[0])
	if err != nil {
		return MouseEvent{}, 0, false
	}
	col, err := strconv.Atoi(parts[1])
	if err != nil {
		return MouseEvent{}, 0, false
	}
	row, err := strconv.Atoi(parts[2])
	if err != nil {
		return MouseEvent{}, 0, false
	}

	press := data[termIdx] == 'M'
	return MouseEvent{Button: button, Col: col, Row: row, Press: press}, termIdx + 1, true
}

// splitSemicolon splits a string on semicolons.
func splitSemicolon(s string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ';' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return parts
}
