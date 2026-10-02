package main

import (
	"dfolan/internal/game/wire"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDispatchTestClient() (*gameConnection, *recordingConnection, *[]map[string]any) {
	var events []map[string]any
	conn := &recordingConnection{}
	client := &gameConnection{
		gatewayRuntime: &gatewayRuntime{},
		worldState:     &worldSession{},
		bootstrapped:   true,
		keys:           make([]byte, wire.SessionKeyBytes),
		event:          func(v map[string]any) { events = append(events, v) },
	}
	client.output = newConnectionOutput(conn, client.keys, "test", client.event)
	return client, conn, &events
}

func TestClientDispatchUnknownTypeAndFallback(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	fixture, err := wire.ServerFrame(0, 60000, nil)
	require.NoError(t, err)
	client.responses = map[uint16][]byte{60000: fixture}
	for _, typ := range []byte{0, 1} {
		result := client.dispatch(&clientRequest{frame: wire.Frame{Type: typ, ID: 60000}})
		if typ == 0 {
			assert.Equal(t, dispatchHandled, result)
			assert.Zero(t, conn.Len())
			assert.Equal(t, "unsupported_client_type", (*events)[0]["kind"])
		} else {
			assert.Equal(t, dispatchNext, result)
			assert.Equal(t, fixture, conn.Bytes())
		}
	}
}

func TestClientDispatchPreservesPreTypeGateAndBranchPriority(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	// The legacy SORT_ITEM path precedes the general type gate. Its unsupported
	// container branch consumes the frame before the later type diagnostic.
	result := client.dispatch(&clientRequest{frame: wire.Frame{Type: 0, ID: 20}, plaintext: []byte{99}, verified: true})
	assert.Equal(t, dispatchHandled, result)
	require.Len(t, *events, 1)
	assert.Equal(t, "sort_unsupported_container", (*events)[0]["kind"])
	assert.Zero(t, conn.Len())
	// Both 1565 branches remain in the original order. The first consumes an
	// unverified frame silently, so the later sync rejection must not run.
	*events = nil
	assert.Equal(t, dispatchHandled, client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 1565}}))
	assert.Empty(t, *events)
}

func TestClientDispatchStopsAfterTransportFailure(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	closed := &failingDispatchConnection{recordingConnection: *conn}
	client.output.conn = closed
	client.responses = map[uint16][]byte{205: []byte("must not fall through")}
	assert.Equal(t, dispatchClose, client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 205}, verified: true}))
	assert.Zero(t, closed.Len())
	for _, v := range *events {
		assert.NotEqual(t, "server_response", v["kind"])
	}
}

type failingDispatchConnection struct{ recordingConnection }

func (c *failingDispatchConnection) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestClientDispatchExitDialogDoesNotFallThrough(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	client.responses = map[uint16][]byte{2285: []byte("must not fall through")}
	assert.Equal(t, dispatchHandled, client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 2285}, verified: true}))
	raw := conn.Bytes()
	require.NoError(t, wire.ValidateServer(raw))
	assert.Equal(t, uint16(2285), binary.LittleEndian.Uint16(raw[1:3]))
	for _, v := range *events {
		assert.NotEqual(t, "server_response", v["kind"])
	}
}
