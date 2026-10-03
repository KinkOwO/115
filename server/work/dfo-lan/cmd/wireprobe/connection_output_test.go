package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, output.send(1, 1960, []byte{1, 2, 3}))
	raw := conn.Bytes()
	require.NoError(t, wire.ValidateServer(raw))
	assert.Equal(t, uint16(1960), binary.LittleEndian.Uint16(raw[1:3]), "response id")
	assert.Equal(t, uint32(len(raw)), binary.LittleEndian.Uint32(raw[3:7]), "response size")
}

type gatedConnection struct {
	recordingConnection
	first           sync.Once
	started, resume chan struct{}
}

func (c *gatedConnection) Write(p []byte) (int, error) {
	c.first.Do(func() { close(c.started); <-c.resume })
	return c.Buffer.Write(p)
}

func TestConnectionOutputKeepsPreparedBatchTogether(t *testing.T) {
	conn := &gatedConnection{started: make(chan struct{}), resume: make(chan struct{})}
	var resume sync.Once
	release := func() { resume.Do(func() { close(conn.resume) }) }
	defer release()
	output := newConnectionOutput(conn, make([]byte, wire.SessionKeyBytes), "recording", func(map[string]any) {})
	packets, err := preparePackets(output.keys, []outboundPacket{{"first", 0, 13, []byte{1}}, {"second", 1, 19, []byte{2}}})
	require.NoError(t, err)
	batchDone, rawDone := make(chan error, 1), make(chan error, 1)
	var sent []uint16
	go func() {
		batchDone <- output.writePrepared(packets, func(p preparedPacket) { sent = append(sent, p.ID) })
	}()
	select {
	case <-conn.started:
	case <-time.After(2 * time.Second):
		t.Fatal("batch never started")
	}
	raw := []byte{9, 8, 7}
	go func() { rawDone <- output.writeRaw(raw) }()
	release()
	for _, done := range []chan error{batchDone, rawDone} {
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			t.Fatal("connection output stalled")
		}
	}
	want := append(append(append([]byte{}, packets[0].Raw...), packets[1].Raw...), raw...)
	assert.Equal(t, want, conn.Bytes(), "prepared batch changed or raw write interleaved")
	assert.Equal(t, []uint16{13, 19}, sent, "callback order")
}
