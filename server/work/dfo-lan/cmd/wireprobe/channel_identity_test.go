package main

import (
	"bytes"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"os"
	"testing"
)

func TestChannelLoginCurrentFixture(t *testing.T) {
	raw, err := os.ReadFile("../../configs/login-normal22.bin")
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	got, err := channelLoginResponse(keys, raw, 22)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, got) {
		t.Fatal("same-type login changed native fixture")
	}
}

func TestChannelIdentityAndLoginAgree(t *testing.T) {
	c := channelrefresh.Config{ServerID: 1, Channels: []channelrefresh.Channel{{ID: 10, Type: 22}, {ID: 6, Type: 3}}}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	for _, id := range []uint32{10, 6} {
		ctx, p, err := channelIdentity(c, id)
		if err != nil || ctx != [2]byte{1, byte(id)} || len(p) != 16 {
			t.Fatalf("identity: %v %v", ctx, err)
		}
		if binary.LittleEndian.Uint32(p[8:]) != 0 {
			t.Fatal("unexpected channel auxiliary mode")
		}
		original := []byte{1, 0, 22, 7, 8, 9}
		cipher, err := wire.EncryptPayload(keys, 1, original)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := wire.ServerFrame(1, 1, cipher)
		if err != nil {
			t.Fatal(err)
		}
		saved := bytes.Clone(raw)
		got, err := channelLoginResponse(keys, raw, p[12])
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := wire.DecryptPayload(keys, 1, got[16:])
		if err != nil {
			t.Fatal(err)
		}
		if decoded[3] != p[12] || decoded[2] != 22 || !bytes.Equal(raw, saved) {
			t.Fatal("login mismatch or shared response mutated")
		}
	}
	for _, id := range []uint32{0, 256, 99} {
		if _, _, err := channelIdentity(c, id); err == nil {
			t.Fatal("invalid channel accepted", id)
		}
	}
	c.ServerID = 256
	if _, _, err := channelIdentity(c, 10); err == nil {
		t.Fatal("truncated server accepted")
	}
}
