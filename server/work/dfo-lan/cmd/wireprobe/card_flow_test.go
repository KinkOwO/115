package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
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
