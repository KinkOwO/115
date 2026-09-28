package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestDungeon27OpeningSceneIsNotCompletion(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[27]
	var maze catalog.DungeonMaze
	for _, m := range d.Mazes {
		if m.Index == 1 {
			maze = m
			break
		}
	}
	opening, err := fixedMonsters(c.Maps[53510], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{Definition: d, Maze: maze, Room: catalog.DungeonRoom{X: maze.Start[0], Y: maze.Start[1], Map: 53510}, Monsters: opening, Dead: map[uint16]bool{}, Loaded: true}
	for _, m := range opening {
		s.Dead[m.Entity] = true
	}
	s.TryComplete()
	if s.Completed() {
		t.Fatal("opening layer completed before the boss room")
	}

	if maze.Boss == maze.Start {
		t.Fatal("opening and boss coordinates unexpectedly match")
	}
}

func TestAPCBossCheckUsesClientBossRank(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[60]
	var s *Session
	var boss uint16
	for _, maze := range d.Mazes {
		for _, room := range maze.Rooms {
			if !room.Boss {
				continue
			}
			monsters, err := fixedMonsters(c.Maps[room.Map], d.BasisLevel)
			if err != nil {
				continue
			}
			for _, m := range monsters {
				if m.APC && m.Rank >= 5 && m.Rank <= 8 && !m.NonCombat {
					s = &Session{Definition: d, Maze: maze, Room: room, Monsters: monsters, Dead: map[uint16]bool{}, Loaded: true}
					boss = m.Entity
					break
				}
			}
			if boss != 0 {
				break
			}
		}
		if boss != 0 {
			break
		}
	}
	if boss == 0 {
		t.Fatal("current dungeon 60 source has no fightable APC boss")
	}
	check := protocol.BossCheckRequest{Actor: 7, Target: boss}
	if err := s.BossCheck(check, 7); err != nil {
		t.Fatalf("APC boss check rejected: %v", err)
	}
	if s.Completed() {
		t.Fatal("APC boss completed before its death")
	}
	for _, m := range s.Monsters {
		if m.Entity != boss && m.Team != 0 {
			s.Dead[m.Entity] = true
		}
	}
	if _, err := s.ConfirmDeath(uint32(boss), 7, 7); err != nil {
		t.Fatal(err)
	}
	if !s.Completed() || s.CompletionTarget() != boss {
		t.Fatalf("APC boss death did not complete run: completed=%v target=%d", s.Completed(), s.CompletionTarget())
	}
}

func TestCompletedBossRoomsHaveEncodableIdentity(t *testing.T) {
	c, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, d := range c.Dungeons {
		for _, maze := range d.Mazes {
			for _, room := range maze.Rooms {
				if !room.Boss {
					continue
				}
				script, ok := c.Maps[room.Map]
				if !ok {
					continue
				}
				monsters, err := fixedMonsters(script, d.BasisLevel)
				if err != nil {
					continue
				}
				s := &Session{Definition: d, Maze: maze, Room: room, Monsters: monsters, Dead: map[uint16]bool{}, Loaded: true}
				for _, m := range monsters {
					s.Dead[m.Entity] = true
				}
				s.TryComplete()
				if s.CompletionNeedsBossCheck() {
					if _, err := protocol.BossCheckConfirmed(s.CompletionTarget()); err != nil {
						t.Errorf("dungeon %d maze %d map %d completed without an encodable identity: %v", d.ID, maze.Index, room.Map, err)
					}
				}
				checked++
			}
		}
	}
	if checked < 3000 {
		t.Fatalf("boss room catalog unexpectedly small: %d", checked)
	}
}
