package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	"dfolan/internal/game/wire"
)

// The request is verified, but each supported container's service rejects it.
// Exercise the actual dispatcher and encrypted output: previously nonzero
// containers sent nothing, while container 0 sent no CMD20 protobuf body.
func TestSortRefusalReachesRequestedContainer(t *testing.T) {
	for _, space := range []byte{0, 1, 2, 7, 12, 45} {
		t.Run(fmt.Sprint(space), func(t *testing.T) {
			client, conn, _ := newDispatchTestClient()
			client.worldState.role.ID = 7
			original := json.RawMessage(`{"level":115,"future_field":{"keep":true}}`)
			client.worldState.role.State = append(json.RawMessage(nil), original...)
			request := &clientRequest{frame: wire.Frame{Type: 1, ID: 20},
				verified: true, plaintext: []byte{space}}
			if action := client.dispatchEquipmentSkillsAndMoves(request); action != dispatchHandled {
				t.Fatalf("action = %v", action)
			}
			raw := conn.Bytes()
			if err := wire.ValidateServer(raw); err != nil {
				t.Fatalf("missing or invalid refusal frame: %v", err)
			}
			if raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:3]) != 20 {
				t.Fatal("refused the wrong command")
			}
			body, err := wire.DecryptPayload(client.keys, 20, raw[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			// Native 1459A1BB0 consumes status/u16, then 1452982C0 consumes
			// u32 length and field 1. No CMD20-specific error is fabricated.
			want := []byte{0, 4, 0, 2, 0, 0, 0, 8, space}
			if len(body) < len(want) || !bytes.Equal(body[:len(want)], want) {
				t.Fatalf("native refusal = %x, want %x", body, want)
			}
			// The existing block cipher pads the transport to eight bytes;
			// CMD20's u32 length bounds its protobuf before that padding.
			if !bytes.Equal(body[len(want):], make([]byte, len(body)-len(want))) {
				t.Fatal("nonzero data after the length-prefixed refusal")
			}
			if !bytes.Equal(client.worldState.role.State, original) {
				t.Fatal("refusal modified character state")
			}
		})
	}
}

func TestSortRefusalTransportFailureClosesDispatch(t *testing.T) {
	client, conn, _ := newDispatchTestClient()
	client.output.conn = &failingDispatchConnection{recordingConnection: *conn}
	action := client.dispatchEquipmentSkillsAndMoves(&clientRequest{
		frame: wire.Frame{Type: 1, ID: 20}, verified: true, plaintext: []byte{7},
	})
	if action != dispatchClose {
		t.Fatalf("write failure action = %v, want close", action)
	}
}

func TestSortUnverifiedRequestDoesNotReleaseQueue(t *testing.T) {
	client, conn, _ := newDispatchTestClient()
	action := client.dispatchEquipmentSkillsAndMoves(&clientRequest{
		frame: wire.Frame{Type: 1, ID: 20}, plaintext: []byte{7},
	})
	if action != dispatchNext || conn.Len() != 0 {
		t.Fatal("unverified input released a container queue")
	}
}
