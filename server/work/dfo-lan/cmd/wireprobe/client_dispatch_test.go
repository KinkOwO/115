package main

import (
	"dfolan/internal/game/wire"
	"dfolan/internal/servermod"
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

// probeRequestID 是给 server 层 mod 的 protocol.request 钩子用的探针号。
// 刻意选一个不会被内置分发表认领的值，且钩子只认它——注册是进程级的，同包其它
// 用例的 dispatch 也会经过这个钩子，所以它**必须**对其余帧静默返回 false。
const probeRequestID = 59999

// TestClientDispatchRequestHookWinsOverBuiltins 钉住 protocol.request 的接入语义：
// 钩子在**内置分发之前**拿到这一帧，返回 handled 后内置 handler 一个都不跑。
func TestClientDispatchRequestHookWinsOverBuiltins(t *testing.T) {
	var sawPlain []byte
	servermod.RegisterRequest("probe.mod", func(ctx *servermod.RequestContext) (bool, error) {
		if ctx.Type != 1 || ctx.ID != probeRequestID {
			return false, nil
		}
		sawPlain = append([]byte(nil), ctx.Plaintext...)
		return true, ctx.Reply(0, probeRequestID, []byte("from-mod"))
	})

	client, conn, _ := newDispatchTestClient()
	// 内置表里给同一个号放一份"绝不能被打出去"的固定应答。
	builtin := []byte("must not fall through")
	client.responses = map[uint16][]byte{probeRequestID: builtin}

	result := client.dispatch(&clientRequest{
		frame:     wire.Frame{Type: 1, ID: probeRequestID},
		plaintext: []byte{0x11, 0x22},
		verified:  true,
	})

	assert.Equal(t, dispatchHandled, result, "钩子接手后必须短路内置分发")
	assert.Equal(t, []byte{0x11, 0x22}, sawPlain, "钩子应拿到已解密的正文")
	raw := conn.Bytes()
	require.NotEmpty(t, raw, "mod 经 Reply 发出的报文应真的写到连接上")
	assert.NotEqual(t, builtin, raw, "内置应答不该被打出去")
	assert.NoError(t, wire.ValidateServer(raw), "mod 的应答仍走服务端正规编码（校验和/加密）")
	assert.Equal(t, uint16(probeRequestID), binary.LittleEndian.Uint16(raw[1:3]))
}

// TestClientDispatchRequestHookPassesThrough 钉住"放行"语义：不认领的帧照旧走内置分发，
// 且放行路径不产生任何额外副作用。
func TestClientDispatchRequestHookPassesThrough(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	fixture, err := wire.ServerFrame(0, 60000, nil)
	require.NoError(t, err)
	client.responses = map[uint16][]byte{60000: fixture}

	assert.Equal(t, dispatchNext, client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 60000}}))
	assert.Equal(t, fixture, conn.Bytes(), "放行的帧仍由内置 handler 应答")
	require.Len(t, *events, 1)
	assert.Equal(t, "server_response", (*events)[0]["kind"])
}
