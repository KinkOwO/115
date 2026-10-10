package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestSocialSelfAddNativeFeedbackWithoutSaveMutation(t *testing.T) {
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	_, _, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, "social-test-name", func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Name, a.Data.NameChosen = "谷木语", true
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := w.prepareAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client, conn, _ := newDispatchTestClient()
	client.worldState, client.selectedCharacterID = w, w.role.ID
	plain, _ := hex.DecodeString("09000000e8b0b7e69ca8e8afad06000000313233313233000000000001050000004465656d6f730010038f3802000000")
	request := &clientRequest{frame: wire.Frame{Type: 1, ID: 1431}, plaintext: plain, verified: true}
	if got := client.dispatchAccountQueries(request); got != dispatchHandled {
		t.Fatalf("self add fell through: %v", got)
	}
	raw := conn.Bytes()
	if err := wire.ValidateServer(raw); err != nil {
		t.Fatal(err)
	}
	body, err := wire.DecryptPayload(client.keys, 1431, raw[wire.ServerHeaderSize:])
	if err != nil || !bytes.HasPrefix(body, []byte{0, 217, 0}) {
		t.Fatalf("native failure feedback: %x %v", body, err)
	}
	after, err := w.prepareAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		t.Fatal("self request changed account save")
	}
	conn.Reset()
	request.verified = false
	if client.dispatchAccountQueries(request) != dispatchHandled || conn.Len() != 0 {
		t.Fatal("unverified request reached response or storage path")
	}
}

func TestSocialMissingTargetReturnsNativeAbsentWithoutWriting(t *testing.T) {
	w, _ := adventureTestWorld(t)
	ctx := context.Background()
	before, err := w.prepareAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := hex.DecodeString("09000000e8b0b7e69ca8e8afad06000000313233313233000000000001050000004465656d6f730010038f3802000000")
	name := []byte("社交测试甲")
	p := binary.LittleEndian.AppendUint32(nil, uint32(len(name)))
	p = append(p, name...)
	p = append(p, plain[13:46]...)
	client, conn, _ := newDispatchTestClient()
	client.worldState = w
	client.selectedCharacterID = w.role.ID
	if client.dispatchAccountQueries(&clientRequest{frame: wire.Frame{Type: 1, ID: 1431}, plaintext: p, verified: true}) != dispatchHandled {
		t.Fatal("missing target fell through")
	}
	body, err := wire.DecryptPayload(client.keys, 1431, conn.Bytes()[16:])
	if err != nil || !bytes.HasPrefix(body, []byte{0, 21, 0}) {
		t.Fatalf("missing native absence %x %v", body, err)
	}
	after, err := w.prepareAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		t.Fatal("missing target changed profile")
	}
}
