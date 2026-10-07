package main

import (
	"dfolan/internal/game/protocol"
	"testing"
)

func TestTempGuardProbe(t *testing.T) {
	m, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 1, Mode: 0}, {Slot: 2, Mode: 2}})
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("2 行标记 = %d 字节: % x", len(m), m)
	pkt, ok := boostRosterFrame("probe", m, true)
	t.Logf("护栏结果 ok=%v payload=%d 字节: % x（上限 %d）", ok, len(pkt.Payload), pkt.Payload, maxVerifiedBoostRosterBytes)
}
