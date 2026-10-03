package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/hex"
	"testing"
)

func TestCardTransportPreflight(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	p, _ := protocol.CardSelected(2)
	plan := []outboundPacket{{"scroll", 1, 69, []byte{1}}, {"layout", 1, 70, protocol.CardLayout()}, {"pick", 1, 71, p}, {"exit", 1, 72, protocol.SettlementExitSuccess(protocol.SettlementExit{State: 1, Option: 2})}}
	rows, e := preparePackets(keys, plan)
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		v, e := wire.DecryptPayload(keys, r.ID, r.Raw[16:])
		if e != nil || len(v) < len(r.Payload) || !bytes.Equal(v[:len(r.Payload)], r.Payload) {
			t.Fatal(r.Name, e)
		}
	}
	w := &worldSession{}
	if _, e = w.cardPick([]byte{0, 0}); e == nil {
		t.Fatal("card accepted outside session")
	}
	if _, _, e = w.settlementExit([]byte{1, 2}); e == nil {
		t.Fatal("exit accepted outside session")
	}
}

// Exercise the actual transport and the native common-dispatch/handler cursor.
// Cipher padding must not hide a missing CMD status byte by supplying option 0.
func TestSettlementExitTransportNativeCursor(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	for _, raw := range []string{
		"01000100000000000000000000000000", // retry
		"01010100000000000000000000000000", // selection
		"01020100000000000000000000000000", // town (live)
		"01030100000000000000000000000000", // next quest (live)
		"01050100000000000000000000000000", // seamless
		"02020100000000000000000000000000", // focus
	} {
		t.Run(raw[:6], func(t *testing.T) {
			request, err := hex.DecodeString(raw)
			if err != nil {
				t.Fatal(err)
			}
			exit, err := protocol.DecodeSettlementExit(request)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := preparePackets(keys, []outboundPacket{{"exit", 1, 72, protocol.SettlementExitSuccess(exit)}})
			if err != nil {
				t.Fatal(err)
			}
			plain, err := wire.DecryptPayload(keys, 72, rows[0].Raw[16:])
			if err != nil || len(plain) < 3 {
				t.Fatalf("decrypt CMD72: %x, %v", plain, err)
			}
			// 0x1459a1ca2 consumes status; 0x1452445ad/5b9 consume state/option.
			if plain[0] != 1 || plain[1] != request[0] || plain[2] != request[1] {
				t.Fatalf("native cursor reads status/state/option=%x, request=%x", plain[:3], request[:3])
			}
			for _, b := range plain[3:] {
				if b != 0 {
					t.Fatalf("nonzero trailing transport padding: %x", plain)
				}
			}
		})
	}
}
