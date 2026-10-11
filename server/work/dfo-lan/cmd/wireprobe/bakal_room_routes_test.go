package main

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"testing"
)

// Audit actual source rooms and both directions of every adjacent edge.
// The raid completion path must not turn a cleared boss into a generic
// completed dungeon that prevents the player from reaching its exit.
func TestBakalNativeRoomRoutesAfterClear(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	rooms, edges, returns := 0, 0, 0
	for _, source := range w.bakalRules.Dungeons {
		d := w.bakalRules.DungeonCatalog.Dungeons[source.Index]
		for _, maze := range d.Mazes {
			if maze.Index != 0 {
				continue // gateway selects maze 0 for this mode
			}
			for _, from := range maze.Rooms {
				grid := [2]byte{from.X, from.Y}
				if _, err := w.bakalDungeonEntry(source.Index, 0, &grid, at, false); err != nil {
					t.Fatalf("enter %d/%v: %v", source.Index, grid, err)
				}
				if _, _, err := c.bakalLoadingDone(nil); err != nil {
					t.Fatalf("load %d/%v: %v", source.Index, grid, err)
				}
				rooms++
				base := w.activeDungeon
				live := !base.RoomCleared()
				for _, to := range maze.Rooms {
					dx, dy := int(to.X)-int(from.X), int(to.Y)-int(from.Y)
					if dx*dx+dy*dy != 1 {
						continue
					}
					w.activeDungeon = base
					_, _, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: source.Index, Position: [2]byte{to.X, to.Y}})
					blocked := live && !d.MoveMapEvenEnemy
					if (err != nil) != blocked {
						t.Fatalf("live door %d/%v -> %v source blocked=%v err=%v", source.Index, grid, [2]byte{to.X, to.Y}, blocked, err)
					}
					if live {
						w.activeDungeon = base
						if _, _, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: source.Index, Position: [2]byte{to.X, to.Y}, RaidReturn: true}); err == nil {
							t.Fatalf("uncleared %d/%v bypassed ACT return gate", source.Index, grid)
						}
					}
				}
				for _, actor := range base.Monsters {
					base.Dead[actor.Entity] = true
				}
				if !base.RoomCleared() || base.Completed() {
					t.Fatalf("raid clear/completion gate %d/%v", source.Index, grid)
				}
				for _, to := range maze.Rooms {
					target := [2]byte{to.X, to.Y}
					dx, dy := int(to.X)-int(from.X), int(to.Y)-int(from.Y)
					// Cleared ACT returns can jump to any source maze room,
					// including the same grid; normal doors require adjacency.
					w.activeDungeon = base
					next, _, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: source.Index, Position: target, RaidReturn: true})
					if err != nil || next.Room.Map != to.Map {
						t.Fatalf("return %d/%v -> %v: %v", source.Index, grid, target, err)
					}
					returns++
					if dx*dx+dy*dy != 1 {
						continue
					}
					w.activeDungeon = base
					next, plan, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: source.Index, Position: target})
					if err != nil {
						t.Fatalf("door %d/%v -> %v: %v", source.Index, grid, target, err)
					}
					if next.Room.Map != to.Map || next.Loaded || len(plan) < 2 || plan[1].ID != 29 || plan[1].Payload[0] != to.X || plan[1].Payload[1] != to.Y {
						t.Fatalf("door %d/%v -> %v did not publish correct unloaded room", source.Index, grid, target)
					}
					edges++
				}
			}
			t.Logf("dungeon=%d type=%s rooms=%d move_even_enemy=%v", source.Index, source.Type, len(maze.Rooms), d.MoveMapEvenEnemy)
		}
	}
	if rooms == 0 || edges == 0 || returns == 0 {
		t.Fatal("native routes were not exercised")
	}
	t.Logf("rooms=%d directed adjacent edges=%d source return pairs=%d", rooms, edges, returns)
}

func TestBakalDragonDeathOpensNativeReturnWithoutGenericCompletion(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	for _, source := range w.bakalRules.Dungeons {
		if source.Type != "skasa" && source.Type != "sparazzi" && source.Type != "hisma" {
			continue
		}
		grid := bakalNativeBossGrid(t, w, source.Type)
		if _, err := w.bakalDungeonEntry(source.Index, 0, &grid, at, false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.bakalLoadingDone(nil); err != nil {
			t.Fatal(err)
		}
		boss := w.bakalRules.Monsters[source.Type].Template
		found := false
		for _, actor := range w.activeDungeon.Monsters {
			if actor.Template == boss && actor.Rank == 3 {
				if ok, err := w.activeDungeon.ConfirmDeath(uint32(actor.Entity), w.role.WireID, w.role.WireID); err != nil || !ok {
					t.Fatalf("%s owned death: %v", source.Type, err)
				}
				found = true
			}
		}
		if !found || !w.activeDungeon.RoomCleared() || w.activeDungeon.Completed() {
			t.Fatalf("%s boss-only death left exit blocked", source.Type)
		}
		if _, err := w.bakalConfirmDefeats(at); err != nil {
			t.Fatal(err)
		}
		if !w.bakal.IsCleared(source.Index) {
			t.Fatalf("%s source clear not recorded", source.Type)
		}
		for _, loc := range w.bakalRules.Locations {
			if loc.Dungeon != source.Index {
				continue
			}
			p := make([]byte, 48)
			binary.LittleEndian.PutUint32(p[13:], uint32(loc.X))
			binary.LittleEndian.PutUint32(p[17:], uint32(loc.Y))
			p[21] = 1
			if _, _, err := c.bakalScriptWarp(p); err != nil {
				t.Fatalf("%s closed dungeon blocked existing room exit: %v", source.Type, err)
			}
			if _, _, err := c.bakalLoadingDone(nil); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s source clear -> service room %d/%d loaded", source.Type, loc.X, loc.Y)
			break
		}
	}
}
