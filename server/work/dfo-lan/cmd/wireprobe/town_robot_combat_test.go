package main

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func robotCombatFixture(t *testing.T) (*worldSession, *townRobotTransport, []byte) {
	w, _, tr, _ := robotDeathFixture(t, 2)
	w.loot = nil
	w.activeDungeon.Room.X, w.activeDungeon.Room.Y = 1, 2
	w.activeDungeon.Monsters = []protocol.DungeonMonster{{Entity: 0x1048}, {Entity: 0x1049}}
	w.activeDungeon.Dead = map[uint16]bool{}
	w.deathSent = map[uint16]bool{}
	w.hub.townRobots.parties[w.peer].bots = []*townRobot{{peer: &lanPeer{actorID: 65534, robot: true}}}
	// Actual C39 from the HP=0 / lingering-bar room, not an invented packet.
	p, err := hex.DecodeString("48100000feff0000000000000000000000000f44854f050000001500000002110102000043854f0500000006001101feff48000000000000000e009e07d00000000000ce7300a5070000cf000000ba070000d600000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ce54a6af0000000000")
	if err != nil {
		t.Fatal(err)
	}
	return w, tr, p
}

func TestRobotCombatCapturedDeathClearsRoomAndConfirms(t *testing.T) {
	w, tr, p := robotCombatFixture(t)
	events := 0
	event := func(v map[string]any) {
		if v["kind"] == "town_robot_combat_death" {
			events++
		}
	}
	for _, entity := range []uint16{0x1048, 0x1049} {
		binary.LittleEndian.PutUint32(p, uint32(entity))
		plan, err := w.monsterDeath(p, event)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, frame := range plan {
			if frame.ID == 38 && frame.Kind == 0 && binary.LittleEndian.Uint16(frame.Payload) == entity {
				found = true
			}
		}
		if !found || !w.activeDungeon.Dead[entity] || w.activeDungeon.Unowned[entity] {
			t.Fatal("teammate death not confirmed/owned", entity, plan)
		}
		w.deathSent[entity] = true // outbound plan was committed
	}
	if !w.activeDungeon.RoomCleared() || events != 2 {
		t.Fatal("room still blocked", events)
	}
	again, err := w.monsterDeath(p, event)
	if err != nil || events != 2 {
		t.Fatal("duplicate was not idempotent", err, events)
	}
	for _, frame := range again {
		if frame.ID == 38 {
			t.Fatal("duplicate rewards/confirmation")
		}
	}
	// A dead companion's already released projectile still belongs to the room.
	tr.nativeAI.deaths[0].death.Dead = true
	if w.townRobotCombatKiller(65534) != w.role.WireID {
		t.Fatal("late projectile lost ownership")
	}
}

func TestRobotCombatRejectsForeignOrStaleController(t *testing.T) {
	cases := map[string]func(*worldSession, *townRobotTransport){
		"no party":           func(w *worldSession, tr *townRobotTransport) { delete(w.hub.townRobots.parties, w.peer) },
		"town":               func(w *worldSession, tr *townRobotTransport) { w.hub.townRobots.parties[w.peer].dungeon = false },
		"not loaded":         func(w *worldSession, tr *townRobotTransport) { w.activeDungeon.Loaded = false },
		"wrong room":         func(w *worldSession, tr *townRobotTransport) { tr.room = &[2]uint16{2, 2} },
		"loading":            func(w *worldSession, tr *townRobotTransport) { tr.roomLoading = true },
		"closed":             func(w *worldSession, tr *townRobotTransport) { tr.closed = true },
		"expired":            func(w *worldSession, tr *townRobotTransport) { tr.nativeAI.lastAck = time.Now().Add(-3 * time.Second) },
		"future":             func(w *worldSession, tr *townRobotTransport) { tr.nativeAI.lastAck = time.Now().Add(time.Hour) },
		"prepared only":      func(w *worldSession, tr *townRobotTransport) { tr.nativeAI.phase = 1 },
		"old generation":     func(w *worldSession, tr *townRobotTransport) { tr.nativeAI.generation = 0 },
		"wrong owner":        func(w *worldSession, tr *townRobotTransport) { tr.ownerActor++ },
		"kicked":             func(w *worldSession, tr *townRobotTransport) { w.hub.townRobots.parties[w.peer].bots = nil },
		"transport mismatch": func(w *worldSession, tr *townRobotTransport) { tr.peers[0].actor-- },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			w, tr, p := robotCombatFixture(t)
			change(w, tr)
			if _, err := w.monsterDeath(p, func(map[string]any) {}); err == nil || w.activeDungeon.Dead[0x1048] {
				t.Fatal("accepted invalid companion kill", err)
			}
		})
	}
	w, _, p := robotCombatFixture(t)
	for _, killer := range []uint16{0, 123, 65533} {
		binary.LittleEndian.PutUint16(p[4:], killer)
		if _, err := w.monsterDeath(p, func(map[string]any) {}); err == nil {
			t.Fatal("foreign kill accepted", killer)
		}
	}
	binary.LittleEndian.PutUint16(p[4:], 65535)
	if _, err := w.monsterDeath(p, func(map[string]any) {}); err != nil || !w.activeDungeon.Unowned[0x1048] {
		t.Fatal("unowned semantics changed", err)
	}
}
