package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
)

func TestDungeonActorLifecyclePreflight(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 5, WireID: 503}, activeDungeon: &dungeon.Session{}, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	enter, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	leave, err := w.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	for i, plan := range [][]outboundPacket{enter, leave} {
		state := byte(1 - i)
		if len(plan) < 3 || plan[1].ID != 3 || plan[1].Kind != 0 || !bytes.Equal(plan[1].Payload, []byte{1, 247, 1, state}) {
			t.Fatalf("missing actor transition before scene: %+v", plan)
		}
		prepared, err := preparePackets(keys, plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range prepared {
			decoded, err := wire.DecryptPayload(keys, p.ID, p.Raw[16:])
			if err != nil || len(decoded) < len(p.Payload) || !bytes.Equal(decoded[:len(p.Payload)], p.Payload) {
				t.Fatalf("lifecycle preflight %s: %v", p.Name, err)
			}
		}
	}
	for _, p := range [][]byte{nil, make([]byte, 8), append([]byte{1}, make([]byte, 15)...)} {
		if _, err = w.finishDungeonLoading(p); err == nil {
			t.Fatal("accepted malformed loading request")
		}
	}
}

func TestPriestTutorialDoorAdvancesAfterRoomClear(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/tutorial-dungeons.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	definition := c.Dungeons[7113]
	if definition.ID == 0 || len(definition.Mazes) == 0 {
		t.Fatal("priest tutorial dungeon 7113 is missing")
	}
	var room catalog.DungeonRoom
	for _, candidate := range definition.Mazes[0].Rooms {
		if candidate.Map == 76026 {
			room = candidate
			break
		}
	}
	if room.Map == 0 {
		t.Fatal("tutorial room 76026 is missing from source maze")
	}
	s := &dungeon.Session{
		Definition: definition,
		Maze:       definition.Mazes[0],
		Room:       room,
		Loaded:     true,
		Monsters:   []protocol.DungeonMonster{{Entity: 0x1006, Template: 22006, Rank: 5, Team: 100, APC: true}},
		Dead:       map[uint16]bool{},
		NextEntity: 0x1007,
	}
	w := &worldSession{dungeons: &c, activeDungeon: s}

	// 原生 CMD45 换房请求：目标格就是 76027 所在房间。
	var target catalog.DungeonRoom
	for _, candidate := range definition.Mazes[0].Rooms {
		if candidate.Map == 76027 {
			target = candidate
			break
		}
	}
	if target.Map == 0 {
		t.Fatal("tutorial room 76027 is missing from source maze")
	}
	req := make([]byte, 160)
	req[0], req[1] = target.X, target.Y
	binary.LittleEndian.PutUint32(req[151:155], definition.ID)

	if _, _, err := w.moveDungeonRoom(req); err == nil {
		t.Fatal("door advanced with a live room monster")
	}

	s.Dead[0x1006] = true
	next, plan, err := w.moveDungeonRoom(req)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Room.Map != 76027 || next.Room.X != 3 || next.Room.Y != 0 {
		t.Fatalf("cleared tutorial room did not advance to 76027: next=%+v", next)
	}
	if len(plan) != 2 || plan[0].ID != 45 || plan[1].ID != 29 {
		t.Fatalf("unexpected tutorial transition packets: %+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[1].Payload[32:36]); got != 76027 {
		t.Fatalf("next-map packet names map %d, want 76027", got)
	}
}
