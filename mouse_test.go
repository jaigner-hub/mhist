package main

import "testing"

func TestMouseScrollUp(t *testing.T) {
	// ESC [ < 64 ; 1 ; 1 M
	data := []byte("\x1b[<64;1;1M")
	ev, n, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 64 {
		t.Errorf("button: expected 64, got %d", ev.Button)
	}
	if !ev.Press {
		t.Error("expected press=true")
	}
	if n != len(data) {
		t.Errorf("consumed: expected %d, got %d", len(data), n)
	}
}

func TestMouseScrollDown(t *testing.T) {
	data := []byte("\x1b[<65;10;20M")
	ev, n, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 65 {
		t.Errorf("button: expected 65, got %d", ev.Button)
	}
	if ev.Col != 10 {
		t.Errorf("col: expected 10, got %d", ev.Col)
	}
	if ev.Row != 20 {
		t.Errorf("row: expected 20, got %d", ev.Row)
	}
	if n != len(data) {
		t.Errorf("consumed: expected %d, got %d", len(data), n)
	}
}

func TestMouseLeftClick(t *testing.T) {
	data := []byte("\x1b[<0;5;10M")
	ev, _, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 0 {
		t.Errorf("button: expected 0, got %d", ev.Button)
	}
	if !ev.Press {
		t.Error("expected press=true for M terminator")
	}
}

func TestMouseRelease(t *testing.T) {
	data := []byte("\x1b[<0;5;10m")
	ev, _, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Press {
		t.Error("expected press=false for m terminator")
	}
}

func TestMouseMiddleClick(t *testing.T) {
	data := []byte("\x1b[<1;15;25M")
	ev, _, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 1 {
		t.Errorf("button: expected 1, got %d", ev.Button)
	}
}

func TestMouseRightClick(t *testing.T) {
	data := []byte("\x1b[<2;15;25M")
	ev, _, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 2 {
		t.Errorf("button: expected 2, got %d", ev.Button)
	}
}

func TestMouseIncompleteSequence(t *testing.T) {
	// Missing terminator
	data := []byte("\x1b[<64;1;1")
	_, _, ok := ParseSGRMouse(data)
	if ok {
		t.Error("expected failure for incomplete sequence")
	}
}

func TestMouseTooShort(t *testing.T) {
	data := []byte("\x1b[<")
	_, _, ok := ParseSGRMouse(data)
	if ok {
		t.Error("expected failure for too-short data")
	}
}

func TestMouseInvalidInput(t *testing.T) {
	data := []byte("not a mouse event")
	_, _, ok := ParseSGRMouse(data)
	if ok {
		t.Error("expected failure for invalid input")
	}
}

func TestMouseWithTrailingData(t *testing.T) {
	// Mouse event followed by extra bytes
	data := []byte("\x1b[<64;1;1Mextra")
	ev, n, ok := ParseSGRMouse(data)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if ev.Button != 64 {
		t.Errorf("button: expected 64, got %d", ev.Button)
	}
	if n != 10 { // length of "\x1b[<64;1;1M"
		t.Errorf("consumed: expected 10, got %d", n)
	}
}

func TestMouseBadParams(t *testing.T) {
	// Only 2 params instead of 3
	data := []byte("\x1b[<64;1M")
	_, _, ok := ParseSGRMouse(data)
	if ok {
		t.Error("expected failure for bad params")
	}
}
