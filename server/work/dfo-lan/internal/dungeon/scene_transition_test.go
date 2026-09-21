package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"os"
	"testing"
)

func sceneFixture(t *testing.T) (catalog.DungeonCatalog, *Session, protocol.DungeonRoomTransition) {
	t.Helper()
	c, err := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Select(c, protocol.DungeonSelection{ID: 100004937, Difficulty: 2, Party: 65535}, 35, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, room := range s.Maze.Rooms {
		if room.Boss {
			s, err = s.enterRoom(c, room)
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	for _, m := range s.Monsters {
		if !m.NonCombat {
			s.Dead[m.Entity] = true
		}
	}
	raw, err := os.ReadFile("../../docs/evidence/skycastle-scenes-20260917/captured-cmd45.bin")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeDungeonRoomTransition(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c, s, r
}

func TestSkycastleSceneCapturedRequest(t *testing.T) {
	c, s, r := sceneFixture(t)
	if _, err := s.Move(c, r.Position); err == nil {
		t.Fatal("baseline unexpectedly allowed same-room movement")
	} else {
		t.Log("BASELINE:", err)
	}
	next, err := s.MoveScene(c, r)
	if err != nil || next.Room.Map != 100016000 || next.Loaded || next.RunID != s.RunID {
		t.Fatal(next, err)
	}
	body, err := protocol.StartMap(protocol.StartMapState{Position: r.Position, Map: next.Room.Map, Monsters: next.Monsters, LayerChange: true, Transition: &r.Record})
	if err != nil || body[2] != 1 || hex.EncodeToString(body[13:31]) != "0000000004051405fa000000320032000000" {
		t.Fatal(body, err)
	}
	if _, err = next.MoveScene(c, r); err == nil {
		t.Fatal("accepted retry before load")
	}
	t.Log("MODIFIED: captured CMD45 accepted; next map100016000; NOTI29 layer flag1; original landing record preserved")
}

func TestSkycastleSceneSequenceAndCompletion(t *testing.T) {
	c, s, r := sceneFixture(t)
	for _, route := range c.SceneRoutes {
		if route.Dungeon != s.Definition.ID {
			continue
		}
		for _, m := range s.Monsters {
			if m.Rank == 3 && s.BossCheck(protocol.BossCheckRequest{Actor: 14, Target: m.Entity}, 14) == nil {
				t.Fatal("premature clear")
			}
		}
		r.Record = route.Record
		next, err := s.MoveScene(c, r)
		if err != nil {
			t.Fatalf("from%d to%d: %v", route.From, route.To, err)
		}
		if next.Room.Map != route.To || next.Completed() {
			t.Fatal("wrong layer/early completion")
		}
		s = next
		s.Loaded = true
	}
	var boss uint16
	for _, m := range s.Monsters {
		if m.Template == 109019257 {
			boss = m.Entity
		}
	}
	if boss == 0 {
		t.Fatal("original final boss missing")
	}
	if err := s.BossCheck(protocol.BossCheckRequest{Actor: 14, Target: boss}, 14); err != nil {
		t.Fatal(err)
	}
	if !s.Completed() {
		t.Fatal("expected completion after boss check in Odyssey")
	}
	if applied, err := s.ConfirmDeath(uint32(boss), 14, 14); err != nil || !applied || !s.Completed() {
		t.Fatal(applied, err)
	}
	if applied, err := s.ConfirmDeath(uint32(boss), 14, 14); err != nil || applied {
		t.Fatal("death retry", applied, err)
	}
	if _, err := s.MoveScene(c, r); err == nil {
		t.Fatal("moved after final clear")
	}
	t.Log("SEQUENCE PASS: all four source layers; final109019257 death completes once; no earlier clear")
}

func TestSkycastleSceneRejectsInvalidTransitions(t *testing.T) {
	for _, name := range []string{"wrong-dungeon", "wrong-position", "tampered-landing", "skip-layer", "wrong-source", "ordinary-flag"} {
		t.Run(name, func(t *testing.T) {
			c, s, r := sceneFixture(t)
			switch name {
			case "wrong-dungeon":
				r.Dungeon++
			case "wrong-position":
				r.Position[0]++
			case "tampered-landing":
				r.Record[6]++
			case "skip-layer":
				r.Record = [18]byte{0, 0, 0, 0, 4, 5, 0xf8, 3, 0xfc, 0}
			case "wrong-source":
				c.Source.Checksum = "mismatch"
			case "ordinary-flag":
				r.LayerChange = false
			}
			if _, err := s.MoveScene(c, r); err == nil {
				t.Fatal("invalid scene admitted")
			}
		})
	}
}
