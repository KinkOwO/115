package main

import (
	"bytes"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/character"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dispatchBareClientFrame(typ byte, id uint16) []byte {
	raw := make([]byte, wire.ClientHeaderSize)
	raw[0] = typ
	binary.LittleEndian.PutUint16(raw[1:3], id)
	binary.LittleEndian.PutUint32(raw[3:7], uint32(len(raw)))
	raw[7] = wire.Checksum(raw[11:13])
	return raw
}

func readDispatchServerFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	header := make([]byte, wire.ServerHeaderSize)
	_, err := io.ReadFull(conn, header)
	require.NoError(t, err)
	size := int(binary.LittleEndian.Uint32(header[3:7]))
	require.GreaterOrEqual(t, size, wire.ServerHeaderSize)
	require.LessOrEqual(t, size, wire.MaxPacketSize)
	raw := make([]byte, size)
	copy(raw, header)
	_, err = io.ReadFull(conn, raw[wire.ServerHeaderSize:])
	require.NoError(t, err)
	require.NoError(t, wire.ValidateServer(raw))
	return raw
}

func waitForDispatchConnection(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connection did not release its reader and timers")
	}
}

func TestGameGatewayLoginOrderAndChannelIsolation(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	// Same native login vector as TestChannelIdentityAndLoginAgree.
	cipher, err := wire.EncryptPayload(keys, 1, []byte{1, 0, 22, 7, 8, 9})
	require.NoError(t, err)
	login, err := wire.ServerFrame(1, 1, cipher)
	require.NoError(t, err)
	original := bytes.Clone(login)
	characters := &character.Service{ChannelContext: [2]byte{7, 7}}
	gateway := &gameGateway{
		runtime: &gatewayRuntime{config: Config{ChannelIdentity: true}, characters: characters, responses: map[uint16][]byte{1: login}},
		channels: channelrefresh.Config{ServerID: 1, Channels: []channelrefresh.Channel{
			{ID: 10, Type: 22}, {ID: 6, Type: 3},
		}},
		event: func(map[string]any) {},
	}
	// Keep both connections alive together while checking their separate contexts.
	for _, channel := range []uint32{10, 6} {
		server, peer := net.Pipe()
		done := make(chan struct{})
		go func() { defer close(done); gateway.handleClient(server, channel) }()
		t.Cleanup(func() { peer.Close(); waitForDispatchConnection(t, done) })
		require.NoError(t, peer.SetDeadline(time.Now().Add(2*time.Second)))
		_, err := peer.Write(dispatchBareClientFrame(1, 1))
		require.NoError(t, err)
		_, notice, err := channelIdentity(gateway.channels, channel)
		require.NoError(t, err)
		want, err := channelLoginResponse(keys, original, notice[12])
		require.NoError(t, err)
		assert.Equal(t, want, readDispatchServerFrame(t, peer), "login response comes first")
		clock := readDispatchServerFrame(t, peer)
		assert.Equal(t, byte(1), clock[0])
		assert.Equal(t, uint16(1960), binary.LittleEndian.Uint16(clock[1:3]), "server clock before channel identity")
		identity := readDispatchServerFrame(t, peer)
		assert.Equal(t, byte(0), identity[0])
		assert.Equal(t, uint16(2435), binary.LittleEndian.Uint16(identity[1:3]))
		body, err := wire.DecryptPayload(keys, 2435, identity[wire.ServerHeaderSize:])
		require.NoError(t, err)
		assert.Equal(t, notice, body)
	}
	assert.Equal(t, [2]byte{7, 7}, characters.ChannelContext, "shared service context remains untouched")
	assert.Equal(t, original, login, "shared login fixture remains untouched")
}

func TestGameGatewayChecksEveryFrameBeyondSampleCap(t *testing.T) {
	var mu sync.Mutex
	var frames []map[string]any
	gateway := &gameGateway{runtime: &gatewayRuntime{}, event: func(v map[string]any) {
		if v["kind"] == "client_frame" {
			mu.Lock()
			frames = append(frames, v)
			mu.Unlock()
		}
	}}
	server, peer := net.Pipe()
	done := make(chan struct{})
	go func() { defer close(done); gateway.handleClient(server, 0) }()
	t.Cleanup(func() { peer.Close(); waitForDispatchConnection(t, done) })
	require.NoError(t, peer.SetDeadline(time.Now().Add(2*time.Second)))
	// Repeated unimplemented commands consume diagnostic samples. Implemented
	// CMD205 still retains its validated body after those samples are exhausted.
	for i := 0; i < BodySampleLimit+2; i++ {
		_, err := peer.Write(dispatchBareClientFrame(1, 60000))
		require.NoError(t, err)
	}
	_, err := peer.Write(dispatchBareClientFrame(1, 205))
	require.NoError(t, err)
	require.NoError(t, peer.Close())
	waitForDispatchConnection(t, done)
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, frames, BodySampleLimit+3)
	for _, entry := range frames[:BodySampleLimit] {
		assert.Equal(t, true, entry["checksum_ok"])
	}
	assert.NotContains(t, frames[BodySampleLimit], "plain_hex", "sampling is bounded")
	assert.Equal(t, true, frames[len(frames)-1]["checksum_ok"], "implemented commands always retain validated bodies")
}
