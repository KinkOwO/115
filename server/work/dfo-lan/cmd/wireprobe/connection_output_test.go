package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

type recordingConnection struct {
	bytes.Buffer
}

func (c *recordingConnection) Close() error {
	return nil
}

func (c *recordingConnection) LocalAddr() net.Addr {
	return nil
}

func (c *recordingConnection) RemoteAddr() net.Addr {
	return nil
}

func (c *recordingConnection) SetDeadline(time.Time) error {
	return nil
}

func (c *recordingConnection) SetReadDeadline(time.Time) error {
	return nil
}

func (c *recordingConnection) SetWriteDeadline(time.Time) error {
	return nil
}

func TestConnectionOutputWritesOneServerFrame(t *testing.T) {
	conn := &recordingConnection{}
	output := newConnectionOutput(conn, make([]byte, wire.SessionKeyBytes), "recording", func(map[string]any) {})
	if err := output.send(1, 1960, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	raw := conn.Bytes()
	if err := wire.ValidateServer(raw); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(raw[1:3]); got != 1960 {
		t.Fatalf("response id = %d, want 1960", got)
	}
	if got := binary.LittleEndian.Uint32(raw[3:7]); int(got) != len(raw) {
		t.Fatalf("response size = %d, bytes written = %d", got, len(raw))
	}
}
