package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestBufferEmpty(t *testing.T) {
	b := NewScrollbackBuffer(100)
	if b.Lines() != 0 {
		t.Errorf("expected 0 lines, got %d", b.Lines())
	}
	if line := b.GetLine(0); line != nil {
		t.Errorf("expected nil for empty buffer, got %q", line)
	}
}

func TestBufferSingleLine(t *testing.T) {
	b := NewScrollbackBuffer(100)
	b.Write([]byte("hello world\n"))
	if b.Lines() != 1 {
		t.Fatalf("expected 1 line, got %d", b.Lines())
	}
	if !bytes.Equal(b.GetLine(0), []byte("hello world")) {
		t.Errorf("expected 'hello world', got %q", b.GetLine(0))
	}
}

func TestBufferMultipleLines(t *testing.T) {
	b := NewScrollbackBuffer(100)
	b.Write([]byte("line1\nline2\nline3\n"))
	if b.Lines() != 3 {
		t.Fatalf("expected 3 lines, got %d", b.Lines())
	}
	if !bytes.Equal(b.GetLine(0), []byte("line1")) {
		t.Errorf("line 0: expected 'line1', got %q", b.GetLine(0))
	}
	if !bytes.Equal(b.GetLine(1), []byte("line2")) {
		t.Errorf("line 1: expected 'line2', got %q", b.GetLine(1))
	}
	if !bytes.Equal(b.GetLine(2), []byte("line3")) {
		t.Errorf("line 2: expected 'line3', got %q", b.GetLine(2))
	}
}

func TestBufferWraparound(t *testing.T) {
	b := NewScrollbackBuffer(3)
	b.Write([]byte("a\nb\nc\nd\ne\n"))
	if b.Lines() != 3 {
		t.Fatalf("expected 3 lines (capacity), got %d", b.Lines())
	}
	// Oldest should be "c", newest "e"
	if !bytes.Equal(b.GetLine(0), []byte("c")) {
		t.Errorf("oldest: expected 'c', got %q", b.GetLine(0))
	}
	if !bytes.Equal(b.GetLine(1), []byte("d")) {
		t.Errorf("middle: expected 'd', got %q", b.GetLine(1))
	}
	if !bytes.Equal(b.GetLine(2), []byte("e")) {
		t.Errorf("newest: expected 'e', got %q", b.GetLine(2))
	}
}

func TestBufferPartialLines(t *testing.T) {
	b := NewScrollbackBuffer(100)
	// Write partial line
	b.Write([]byte("hel"))
	if b.Lines() != 0 {
		t.Errorf("expected 0 lines after partial write, got %d", b.Lines())
	}
	// Complete the line
	b.Write([]byte("lo\n"))
	if b.Lines() != 1 {
		t.Fatalf("expected 1 line after completing, got %d", b.Lines())
	}
	if !bytes.Equal(b.GetLine(0), []byte("hello")) {
		t.Errorf("expected 'hello', got %q", b.GetLine(0))
	}
}

func TestBufferPartialAcrossWrites(t *testing.T) {
	b := NewScrollbackBuffer(100)
	b.Write([]byte("first\nsec"))
	if b.Lines() != 1 {
		t.Fatalf("expected 1 line, got %d", b.Lines())
	}
	b.Write([]byte("ond\nthird\n"))
	if b.Lines() != 3 {
		t.Fatalf("expected 3 lines, got %d", b.Lines())
	}
	if !bytes.Equal(b.GetLine(1), []byte("second")) {
		t.Errorf("expected 'second', got %q", b.GetLine(1))
	}
}

func TestBufferGetRangeBounds(t *testing.T) {
	b := NewScrollbackBuffer(100)
	b.Write([]byte("a\nb\nc\nd\ne\n"))

	// Normal range
	r := b.GetRange(1, 3)
	if len(r) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(r))
	}
	if !bytes.Equal(r[0], []byte("b")) {
		t.Errorf("expected 'b', got %q", r[0])
	}
	if !bytes.Equal(r[2], []byte("d")) {
		t.Errorf("expected 'd', got %q", r[2])
	}

	// Range exceeding end
	r = b.GetRange(3, 10)
	if len(r) != 2 {
		t.Fatalf("expected 2 lines (clamped), got %d", len(r))
	}

	// Range starting past end
	r = b.GetRange(10, 5)
	if r != nil {
		t.Errorf("expected nil for out-of-range start, got %v", r)
	}

	// Negative start clamped to 0
	r = b.GetRange(-1, 2)
	if len(r) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(r))
	}
	if !bytes.Equal(r[0], []byte("a")) {
		t.Errorf("expected 'a', got %q", r[0])
	}
}

func TestBufferGetLineOutOfRange(t *testing.T) {
	b := NewScrollbackBuffer(100)
	b.Write([]byte("only\n"))
	if b.GetLine(-1) != nil {
		t.Error("expected nil for negative index")
	}
	if b.GetLine(1) != nil {
		t.Error("expected nil for index beyond count")
	}
}

func TestBufferLargeWraparound(t *testing.T) {
	b := NewScrollbackBuffer(5)
	for i := 0; i < 20; i++ {
		b.Write([]byte(fmt.Sprintf("line%d\n", i)))
	}
	if b.Lines() != 5 {
		t.Fatalf("expected 5 lines, got %d", b.Lines())
	}
	// Should have lines 15-19
	if !bytes.Equal(b.GetLine(0), []byte("line15")) {
		t.Errorf("oldest: expected 'line15', got %q", b.GetLine(0))
	}
	if !bytes.Equal(b.GetLine(4), []byte("line19")) {
		t.Errorf("newest: expected 'line19', got %q", b.GetLine(4))
	}
}
