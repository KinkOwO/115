package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"dfolan/internal/ranking"
	"encoding/binary"
	"testing"
)

func TestRankingIdentitySentAfterLoginBeforeCharacterSelection(t *testing.T) {
	w, _ := adventureTestWorld(t)
	client, conn, events := newDispatchTestClient()
	client.worldState, client.bootstrapped = w, false
	client.selectedCharacterID = 0
	cipher, err := wire.EncryptPayload(client.keys, 1, []byte{1, 0, 22, 7, 8, 9})
	if err != nil {
		t.Fatal(err)
	}
	login, err := wire.ServerFrame(1, 1, cipher)
	if err != nil {
		t.Fatal(err)
	}
	client.responses = map[uint16][]byte{1: login}
	if got := client.dispatchFixtureResponse(&clientRequest{frame: wire.Frame{Type: 1, ID: 1}}); got != dispatchNext || !client.bootstrapped {
		t.Fatal("login failed", got)
	}
	raw := conn.Bytes()
	for _, id := range []uint16{1, 1960, 2631} {
		if len(raw) < wire.ServerHeaderSize {
			t.Fatal("missing initialization packet", id)
		}
		n := int(binary.LittleEndian.Uint32(raw[3:7]))
		if n < wire.ServerHeaderSize || n > len(raw) || binary.LittleEndian.Uint16(raw[1:3]) != id {
			t.Fatal("incorrect initialization order", id)
		}
		if err := wire.ValidateServer(raw[:n]); err != nil {
			t.Fatal(err)
		}
		if id == 2631 {
			if raw[0] != 0 {
				t.Fatal("ranking identity must be a notification")
			}
			body, err := wire.DecryptPayload(client.keys, id, raw[wire.ServerHeaderSize:n])
			want, _ := ranking.UserKey(w.account)
			if err != nil || !bytes.HasPrefix(body, want) {
				t.Fatal("native 33-byte identity not restored", err)
			}
		}
		raw = raw[n:]
	}
	if len(raw) != 0 {
		t.Fatal("unexpected initialization packets")
	}
	for _, event := range *events {
		if event["kind"] == "ranking_identity_sent" && len(event) != 3 {
			t.Fatal("ranking identity event disclosed extra data", event)
		}
	}
	conn.Reset()
	client.worldState = nil
	if err := client.sendRankingUserKey(); err != nil || conn.Len() != 0 {
		t.Fatal("identity fabricated without an account", err)
	}
}
