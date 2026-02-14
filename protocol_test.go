package main

import (
	"bytes"
	"testing"
)

func TestProtocolRoundTripData(t *testing.T) {
	msg := Message{Type: MsgData, Payload: []byte("hello world")}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgData {
		t.Errorf("expected type %d, got %d", MsgData, decoded.Type)
	}
	if !bytes.Equal(decoded.Payload, msg.Payload) {
		t.Errorf("expected payload %q, got %q", msg.Payload, decoded.Payload)
	}
}

func TestProtocolRoundTripResize(t *testing.T) {
	payload := []byte{0x00, 0x18, 0x00, 0x50} // 24 rows, 80 cols
	msg := Message{Type: MsgResize, Payload: payload}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgResize {
		t.Errorf("expected type %d, got %d", MsgResize, decoded.Type)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("expected payload %v, got %v", payload, decoded.Payload)
	}
}

func TestProtocolRoundTripDetach(t *testing.T) {
	msg := Message{Type: MsgDetach, Payload: []byte{}}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgDetach {
		t.Errorf("expected type %d, got %d", MsgDetach, decoded.Type)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("expected empty payload, got %v", decoded.Payload)
	}
}

func TestProtocolRoundTripKill(t *testing.T) {
	msg := Message{Type: MsgKill, Payload: []byte{}}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgKill {
		t.Errorf("expected type %d, got %d", MsgKill, decoded.Type)
	}
}

func TestProtocolRoundTripHistoryRequest(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x0A, 0x00, 0x00, 0x00, 0x14} // offset=10, count=20
	msg := Message{Type: MsgHistoryRequest, Payload: payload}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgHistoryRequest {
		t.Errorf("expected type %d, got %d", MsgHistoryRequest, decoded.Type)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestProtocolRoundTripHistoryResponse(t *testing.T) {
	payload := []byte("line1\nline2\nline3\n")
	msg := Message{Type: MsgHistoryResponse, Payload: payload}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != MsgHistoryResponse {
		t.Errorf("expected type %d, got %d", MsgHistoryResponse, decoded.Type)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Errorf("payload mismatch")
	}
}

func TestProtocolEmptyPayload(t *testing.T) {
	msg := Message{Type: MsgData, Payload: []byte{}}
	encoded := Encode(msg)
	if len(encoded) != 5 {
		t.Errorf("expected 5 bytes for empty payload, got %d", len(encoded))
	}
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(decoded.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(decoded.Payload))
	}
}

func TestProtocolLargePayload(t *testing.T) {
	payload := make([]byte, 100000)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	msg := Message{Type: MsgData, Payload: payload}
	encoded := Encode(msg)
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !bytes.Equal(decoded.Payload, payload) {
		t.Error("large payload mismatch")
	}
}

func TestProtocolPartialReadError(t *testing.T) {
	// Only 3 bytes — not enough for the 5-byte header
	_, err := Decode(bytes.NewReader([]byte{0x01, 0x00, 0x00}))
	if err == nil {
		t.Error("expected error for partial header read")
	}
}

func TestProtocolTruncatedPayload(t *testing.T) {
	// Header says 10 bytes of payload, but only 3 provided
	header := []byte{0x01, 0x00, 0x00, 0x00, 0x0A, 0xAA, 0xBB, 0xCC}
	_, err := Decode(bytes.NewReader(header))
	if err == nil {
		t.Error("expected error for truncated payload")
	}
}

func TestProtocolMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	msg1 := Message{Type: MsgData, Payload: []byte("first")}
	msg2 := Message{Type: MsgDetach, Payload: []byte{}}
	msg3 := Message{Type: MsgData, Payload: []byte("third")}
	buf.Write(Encode(msg1))
	buf.Write(Encode(msg2))
	buf.Write(Encode(msg3))

	r := bytes.NewReader(buf.Bytes())

	d1, err := Decode(r)
	if err != nil {
		t.Fatalf("decode msg1: %v", err)
	}
	if !bytes.Equal(d1.Payload, []byte("first")) {
		t.Errorf("msg1 payload: %q", d1.Payload)
	}

	d2, err := Decode(r)
	if err != nil {
		t.Fatalf("decode msg2: %v", err)
	}
	if d2.Type != MsgDetach {
		t.Errorf("msg2 type: %d", d2.Type)
	}

	d3, err := Decode(r)
	if err != nil {
		t.Fatalf("decode msg3: %v", err)
	}
	if !bytes.Equal(d3.Payload, []byte("third")) {
		t.Errorf("msg3 payload: %q", d3.Payload)
	}

	// No more messages
	_, err = Decode(r)
	if err == nil {
		t.Error("expected error after all messages consumed")
	}
}
