package main

import "bytes"

// ScrollbackBuffer is a ring buffer holding terminal output lines.
type ScrollbackBuffer struct {
	lines [][]byte
	head  int // index where the next line will be written
	count int // number of lines currently stored
	cap   int // maximum number of lines
	partial []byte // incomplete line (no trailing \n yet)
}

// NewScrollbackBuffer creates a new scrollback buffer with the given capacity.
func NewScrollbackBuffer(capacity int) *ScrollbackBuffer {
	return &ScrollbackBuffer{
		lines: make([][]byte, capacity),
		cap:   capacity,
	}
}

// Write processes raw PTY output, splitting into lines on \n boundaries.
// Partial lines (no trailing \n) are buffered until the next Write.
func (b *ScrollbackBuffer) Write(data []byte) {
	// Prepend any partial line from previous write
	if len(b.partial) > 0 {
		data = append(b.partial, data...)
		b.partial = nil
	}

	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx == -1 {
			// No newline found — buffer as partial line
			b.partial = make([]byte, len(data))
			copy(b.partial, data)
			return
		}

		// Store the line (including content up to but not including \n)
		line := make([]byte, idx)
		copy(line, data[:idx])
		b.addLine(line)
		data = data[idx+1:]
	}
}

// addLine appends a line to the ring buffer.
func (b *ScrollbackBuffer) addLine(line []byte) {
	b.lines[b.head] = line
	b.head = (b.head + 1) % b.cap
	if b.count < b.cap {
		b.count++
	}
}

// Lines returns the number of lines currently stored.
func (b *ScrollbackBuffer) Lines() int {
	return b.count
}

// GetLine returns the line at the given index, where 0 is the oldest line.
// Returns nil if index is out of range.
func (b *ScrollbackBuffer) GetLine(index int) []byte {
	if index < 0 || index >= b.count {
		return nil
	}
	// oldest line is at (head - count) mod cap
	actual := (b.head - b.count + index + b.cap) % b.cap
	return b.lines[actual]
}

// GetPartial returns a copy of the current partial line (data written without
// a trailing newline). Returns nil if there is no partial line.
func (b *ScrollbackBuffer) GetPartial() []byte {
	if len(b.partial) == 0 {
		return nil
	}
	out := make([]byte, len(b.partial))
	copy(out, b.partial)
	return out
}

// GetRange returns count lines starting from start index.
// Clamps to available range.
func (b *ScrollbackBuffer) GetRange(start, count int) [][]byte {
	if start < 0 {
		start = 0
	}
	if start >= b.count {
		return nil
	}
	end := start + count
	if end > b.count {
		end = b.count
	}
	result := make([][]byte, end-start)
	for i := start; i < end; i++ {
		result[i-start] = b.GetLine(i)
	}
	return result
}
