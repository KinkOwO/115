package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"testing"
)

// Native CMD45 at 2026-09-28T05:36:59.3128486Z, after Last_0.act.
const restingPlaceChaseMove = "060026040000c100000000f6ad01000000000000000000000000000000090000000000000000000000000000000000000000000000000000000000000059005ade010026040000c100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000050597004a0100001e001400000000aceaf5050000000000"

func TestRestingPlaceChaseForcedMove(t *testing.T) {
	c, err := catalog.LoadDungeons("testdata/resting_place_chase.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, protocol.DungeonSelection{ID: 100002476, Quest: 12415, Difficulty: 1, Party: 65535}, 99, map[uint16]bool{12415: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.enterRoom(c, catalog.DungeonRoom{X: 5, Y: 0, Map: 100003288})
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	if len(s.Monsters) != 1 || s.Monsters[0].Template != 109013068 || s.RoomCleared() {
		t.Fatal("fixture must retain the living chase actor")
	}
	body, _ := hex.DecodeString(restingPlaceChaseMove)
	r, err := protocol.DecodeDungeonRoomTransition(body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Move(c, r.Position); err == nil {
		t.Fatal("ordinary movement bypassed room clear")
	}
	next, err := s.MoveScript(c, r)
	if err != nil || next.Room.Map != 100003289 {
		t.Fatalf("source forced move did not enter boss room: %v", err)
	}
	if next.RunID != s.RunID || next.Loaded || next.Completed() || len(next.Dead) != 0 || len(next.Unowned) != 0 || s.ScriptWarps[s.Room.Map] || !next.ScriptWarps[s.Room.Map] {
		t.Fatal("forced movement fabricated combat progress or changed original session")
	}
	packet, err := protocol.StartMap(protocol.StartMapState{Position: r.Position, Map: next.Room.Map, Monsters: next.LivingMonsters(), Transition: &r.Record})
	if err != nil || string(packet[13:31]) != string(r.Record[:]) || packet[2] != 0 || packet[31] != 1 {
		t.Fatalf("native landing record or new-room mode lost: %v", err)
	}
	for _, mutate := range []func(*protocol.DungeonRoomTransition){
		func(v *protocol.DungeonRoomTransition) { v.Dungeon++ },
		func(v *protocol.DungeonRoomTransition) { v.Position[0]-- },
		func(v *protocol.DungeonRoomTransition) { v.Record[6]++ },
		func(v *protocol.DungeonRoomTransition) { v.LayerChange = true },
	} {
		bad := r
		mutate(&bad)
		if _, err := s.MoveScript(c, bad); err == nil {
			t.Fatal("unproven forced move admitted")
		}
	}
	s.Loaded = false
	if _, err := s.MoveScript(c, r); err == nil {
		t.Fatal("unloaded source admitted")
	}
	s.Loaded = true
	c.Source.Checksum = "other-source"
	if _, err := s.MoveScript(c, r); err == nil {
		t.Fatal("different source admitted")
	}
}
