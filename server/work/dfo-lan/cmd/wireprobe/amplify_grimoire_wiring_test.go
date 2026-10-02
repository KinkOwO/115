package main

import (
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CMD205（用增幅书打红字）曾经「服务层 + 流程层都在、就是没人调」：
// 客户端发 205 之后服务端既不处理也不回包，实机表现就是「增幅书打了没效果」。
// 这条用例把分派钉住 —— 光有 applyAmplifyGrimoire 这个函数不算接线成功。
func TestAmplifyGrimoireDispatchIsWired(t *testing.T) {
	for _, verified := range []bool{false, true} {
		client, conn, events := newDispatchTestClient()
		require.Equal(t, dispatchHandled, client.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 205}, verified: verified}))
		kind := "amplify_grimoire_rejected"
		if verified {
			kind = "amplify_grimoire_refused"
		}
		require.Equal(t, kind, (*events)[len(*events)-1]["kind"])
		if !verified {
			assert.Zero(t, conn.Len())
			continue
		}
		raw := conn.Bytes()
		require.NoError(t, wire.ValidateServer(raw))
		assert.Equal(t, byte(1), raw[0])
		assert.Equal(t, uint16(205), binary.LittleEndian.Uint16(raw[1:3]))
		body, err := wire.DecryptPayload(client.keys, 205, raw[wire.ServerHeaderSize:])
		require.NoError(t, err)
		want := amplifyGrimoireRefusal()
		require.GreaterOrEqual(t, len(body), len(want))
		assert.Equal(t, want, body[:len(want)])
		assert.Equal(t, make([]byte, len(body)-len(want)), body[len(want):], "padding")
	}
}
