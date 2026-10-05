package main

import (
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"encoding/binary"
	"testing"
)

// 第三关卡住的两处纯判据（实机 2026-10-04 09:18:37 正文
// 96020000030000000000000000000000）：关卡号在第二包、查看引导不受 [area index]
// 分区门禁约束。落库那半段仍由 TestBoostGiftSQLRealEntry... 在隔离存储里覆盖。
func TestBoostStepGateGuideQuery(t *testing.T) {
	w := &worldSession{
		loot:    &loot.Service{},
		boostup: &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1, Area: 0}, {Number: 2, Area: 1}, {Number: 3, Area: 2}}},
		role:    database.Character{ID: 1},
		state:   database.WorldState{Position: database.WorldPosition{Town: 222, Area: 1}},
	}
	st := boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 3, Phase: 0, Claimed: map[byte]bool{2: true}}}
	// 681：关卡号在第二包，第三包起是补零（17 字节正文）。
	query := decodeStep(t, boostStepBody(3, 17), false)
	if step, e := w.boostStepGate(query, false, st); e != nil || step != 3 {
		t.Fatal("681 step not read from the second word", step, e)
	}
	// 实机 680 是两包正文：事件号 + 关卡号。
	claim := decodeStep(t, boostStepBody(2, 8), true)
	if step, e := w.boostStepGate(claim, true, st); e != nil || step != 2 {
		t.Fatal("two-word claim broke", step, e)
	}
	// 领奖仍被分区拦：站在 area=1 领 area=2 的第三关。
	if _, e := w.boostStepGate(decodeStep(t, boostStepBody(3, 8), true), true, st); e == nil {
		t.Fatal("area gate stopped guarding the claim")
	}
	w.state.Position.Area = 2
	if _, e := w.boostStepGate(decodeStep(t, boostStepBody(3, 8), true), true, st); e != nil {
		t.Fatal("claim refused inside its own area", e)
	}
	if _, e := w.boostStepGate(decodeStep(t, boostStepBody(0, 17), false), false, st); e == nil {
		t.Fatal("step-less request accepted")
	}
}

func boostStepBody(step uint32, size int) []byte {
	p := make([]byte, size)
	binary.LittleEndian.PutUint32(p, boostup.EventID)
	binary.LittleEndian.PutUint32(p[4:], step)
	return p
}

func decodeStep(t *testing.T, p []byte, claim bool) protocol.EventRequest115 {
	t.Helper()
	r, e := protocol.DecodeEventRequest115(p, claim)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
