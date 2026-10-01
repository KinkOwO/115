package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// connectionOutput serializes every write for one connection. Callers retain
// control of grouped pre-encoding, streaming, packet order and diagnostics.
type connectionOutput struct {
	conn  net.Conn
	keys  []byte
	peer  string
	event func(map[string]any)
	mu    sync.Mutex
}

func newConnectionOutput(conn net.Conn, keys []byte, peer string, event func(map[string]any)) *connectionOutput {
	return &connectionOutput{conn: conn, keys: keys, peer: peer, event: event}
}

func (o *connectionOutput) send(kind byte, id uint16, payload []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	prepared, err := preparePackets(o.keys, []outboundPacket{{"response", kind, id, payload}})
	if err != nil {
		o.event(map[string]any{"kind": "response_encode_error", "peer": o.peer, "error": err.Error()})
		return err
	}
	if err = writePackets(o.conn, prepared, nil); err != nil {
		o.event(map[string]any{"kind": "response_write_error", "peer": o.peer, "error": err.Error()})
	}
	return err
}

// writeRaw is used only when a caller must retain a prebuilt frame for
// diagnostics. It still shares the connection lock with send so timer replies
// cannot interleave with that frame.
func (o *connectionOutput) writeRaw(raw []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("raw response deadline: %w", err)
	}
	if _, err := io.Copy(o.conn, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("raw response write: %w", err)
	}
	return nil
}

// writePrepared sends an already encoded batch without changing its order or
// callbacks. It shares the same lock as single replies and raw frames.
func (o *connectionOutput) writePrepared(packets []preparedPacket, sent func(preparedPacket)) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return writePackets(o.conn, packets, sent)
}
