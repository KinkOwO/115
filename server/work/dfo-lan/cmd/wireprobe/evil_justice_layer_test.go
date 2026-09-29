package main

import (
	"bytes"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

func evilJusticeLayer(t *testing.T) (catalog.DungeonCatalog, *dungeon.Session) {
	t.Helper()
	c, err := catalog.LoadDungeons("testdata/evil_justice_layer.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.AttachLayerRevisits(&c, "../../configs/dungeons.layer-revisits.json"); err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100002721, Quest: 12893, Party: 65535}, 100, map[uint16]bool{12893: true})
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	s, err = s.Move(c, [2]byte{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	s, err = s.MoveScene(c, protocol.DungeonRoomTransition{Dungeon: 100002721, Position: [2]byte{1, 1}, LayerChange: true,
		Record: [18]byte{0, 1, 0, 0, 4, 5, 0xcb, 1, 0x0e, 1, 0, 0, 41, 0, 30}})
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	return c, s
}

// Native CMD45 at 2026-09-28T17:23:16 ended CMT13042 at (165,289).
// The old fallback selected (0,1), and the next doorway reloaded the cutscene.
func TestEvilJusticeClosingResumesCachedCombatRoom(t *testing.T) {
	c, s := evilJusticeLayer(t)
	w := &worldSession{dungeons: &c, activeDungeon: s}
	r := protocol.DungeonRoomTransition{Dungeon: 100002721, Position: [2]byte{1, 1}, LayerChange: true, Record: c.LayerRevisits[0].Record}
	next, plan, err := w.moveDungeonRoomDecoded(r)
	if err != nil {
		t.Fatal(err)
	}
	if next.Room.X != s.Room.X || next.Room.Y != s.Room.Y || next.Room.Map != 100004325 || next.Completed() || next.Loaded {
		t.Fatalf("closing scene changed room or completion: %+v", next.Room)
	}
	if len(plan) != 2 || plan[0].ID != 45 || plan[1].ID != 29 {
		t.Fatalf("closing packets: %+v", plan)
	}
	body := plan[1].Payload
	if len(body) != 34 || !bytes.Equal(body[:3], []byte{1, 1, 2}) || body[31] != 0 ||
		!bytes.Equal(body[13:31], r.Record[:]) {
		t.Fatalf("expected same-cell cached combat room and original landing: %x", body)
	}
	if len(next.Monsters) != len(s.Visited[100004325]) || next.NextEntity != s.NextEntity {
		t.Fatal("cached actors were reallocated")
	}
	if len(next.Monsters) != 12 || !next.IsResumedSceneBase() || s.IsResumedSceneBase() {
		t.Fatal("combat resume did not preserve its original actors and independent scene state")
	}
	next.Loaded = true
	if _, err := next.Move(c, [2]byte{2, 1}); err == nil {
		t.Fatal("cinematic bypassed a surviving hostile actor")
	}
	for _, m := range next.Monsters {
		if !m.NonCombat {
			if _, err := next.ConfirmDeath(uint32(m.Entity), 11, 11); err != nil {
				t.Fatal(err)
			}
		}
	}
	forward, err := next.Move(c, [2]byte{2, 1})
	if err != nil || forward.Room.Map != 100004554 {
		t.Fatalf("next room remains blocked: next=%+v err=%v", forward, err)
	}
	forward.Loaded = true
	for _, m := range forward.Monsters {
		forward.Dead[m.Entity] = true
	}
	back, err := forward.Move(c, [2]byte{1, 1})
	if err != nil || back.Room.Map != 100004325 || back.NextEntity != forward.NextEntity {
		t.Fatalf("reentry lost cached layer: %v", err)
	}
	w.activeDungeon = forward
	_, returnPlan, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: 100002721, Position: [2]byte{1, 1}})
	if err != nil || len(returnPlan) != 2 || len(returnPlan[1].Payload) != 34 ||
		!bytes.Equal(returnPlan[1].Payload[:3], []byte{1, 1, 0}) || returnPlan[1].Payload[31] != 0 {
		t.Fatalf("doorway reentry would replay the entrance cinematic: plan=%+v err=%v", returnPlan, err)
	}
}

func TestEvilJusticeRevisitRequiresSourceAndExactRecord(t *testing.T) {
	for _, change := range []string{"source", "record", "quest", "map hash", "resume hash", "resume map", "unvisited base"} {
		t.Run(change, func(t *testing.T) {
			c, s := evilJusticeLayer(t)
			r := protocol.DungeonRoomTransition{Dungeon: 100002721, Position: [2]byte{1, 1}, LayerChange: true, Record: c.LayerRevisits[0].Record}
			switch change {
			case "source":
				c.LayerRevisits[0].Source = "different"
			case "record":
				r.Record[6]++
			case "quest":
				c.LayerRevisits[0].Quest++
			case "map hash":
				c.LayerRevisits[0].MapSHA256 = "different"
			case "resume hash":
				c.LayerRevisits[0].ResumeMapSHA256 = "different"
			case "resume map":
				c.LayerRevisits[0].ResumeMap = 100004322
			case "unvisited base":
				delete(s.Visited, 100004325)
			}
			next, err := s.MoveScene(c, r)
			if err == nil && next.Room.Map == 100004325 && next.Room.X == s.Room.X && next.Room.Y == s.Room.Y {
				t.Fatal("accepted closing revisit without matching source evidence")
			}
		})
	}
}
