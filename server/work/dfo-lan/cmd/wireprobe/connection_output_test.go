package main

import (
	"dfolan/internal/game/wire"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestConnectionOutputWritesOneServerFrame(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	output := newConnectionOutput(server, make([]byte, wire.SessionKeyBytes), "pipe", func(map[string]any) {})

	result := make(chan error, 1)
	go func() {
		result <- output.send(1, 1960, []byte{1, 2, 3})
	}()

	header := make([]byte, 16)
	if _, err := io.ReadFull(client, header); err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(header[1:3]); got != 1960 {
		t.Fatalf("response id = %d, want 1960", got)
	}
	size := binary.LittleEndian.Uint32(header[3:7])
	if size < 16 {
		t.Fatalf("response size = %d, want a full server frame", size)
	}
	body := make([]byte, int(size)-16)
	if _, err := io.ReadFull(client, body); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
