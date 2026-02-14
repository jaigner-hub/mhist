package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Message type constants for the wire protocol.
const (
	MsgData            byte = 0x01
	MsgResize          byte = 0x02
	MsgDetach          byte = 0x03
	MsgKill            byte = 0x04
	MsgHistoryRequest  byte = 0x05
	MsgHistoryResponse byte = 0x06
)

// Message represents a wire protocol message.
// Wire format: [type:1][length:4 BE][payload:N]
type Message struct {
	Type    byte
	Payload []byte
}

// Encode serializes a message into wire format.
func Encode(msg Message) []byte {
	buf := make([]byte, 5+len(msg.Payload))
	buf[0] = msg.Type
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(msg.Payload)))
	copy(buf[5:], msg.Payload)
	return buf
}

// Decode reads a single message from the reader.
func Decode(r io.Reader) (Message, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return Message{}, fmt.Errorf("read header: %w", err)
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:5])

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return Message{}, fmt.Errorf("read payload: %w", err)
		}
	}

	return Message{Type: msgType, Payload: payload}, nil
}
