package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestDungeon25DisplayBossWithoutCheckCompletesOnSourceMap(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	d := c.Dungeons[25]
	var maze catalog.DungeonMaze
	for _, m := range d.Mazes {
		if m.Index == 2 {
			maze = m
			break
		}
	}
	var room catalog.DungeonRoom
	for _, r := range maze.Rooms {
		if r.Map == 53500 {
			room = r
			break
		}
	}
	monsters, err := fixedMonsters(c.Maps[53500], d.BasisLevel)
	if err != nil {
		t.Fatal(err)
	}
	if !room.Boss || len(monsters) != 1 || monsters[0].Rank != 3 || !monsters[0].NonCombat || monsters[0].Team != 100 || len(maze.Layers) != 0 {
		t.Fatalf("unexpected dungeon 25 source shape: room=%+v monsters=%+v layers=%+v", room, monsters, maze.Layers)
	}
	s := &Session{Definition: d, Maze: maze, Room: room, Monsters: monsters, Dead: map[uint16]bool{}}
	s.TryComplete()
	if s.Completed() {
		t.Fatal("completed before loading confirmation")
	}
	s.Loaded = true
	s.TryComplete()
	if !s.Completed() || s.CompletionTarget() != monsters[0].Entity {
		t.Fatalf("display boss room did not complete: completed=%v target=%d", s.Completed(), s.CompletionTarget())
	}
	if _, err := protocol.BossCheckConfirmed(s.CompletionTarget()); err != nil {
		t.Fatalf("completion identity rejected: %v", err)
	}

	ordinary := *s
	ordinary.completed = false
	ordinary.Room = catalog.DungeonRoom{X: 1, Y: 0, Map: 53499}
	ordinary.TryComplete()
	if ordinary.Completed() {
		t.Fatal("ordinary room completed without a boss check")
	}

	occupied := *s
	occupied.completed = false
	occupied.Monsters = append(append([]protocol.DungeonMonster{}, monsters...), protocol.DungeonMonster{Entity: 0x1100, Team: 100, Rank: 0})
	occupied.TryComplete()
	if occupied.Completed() {
		t.Fatal("boss room completed with a live enemy")
	}
	occupied.Dead = map[uint16]bool{0x1100: true}
	occupied.TryComplete()
	if !occupied.Completed() {
		t.Fatal("settled display boss room did not complete")
	}
}
